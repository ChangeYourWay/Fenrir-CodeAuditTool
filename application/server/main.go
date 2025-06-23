package main

import (
	"Fenrir-CodeAuditTool/configs"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"Fenrir-CodeAuditTool/internal/utils"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// loadPrompt 从指定路径读取 prompt 文件内容
func loadPrompt(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func main() {

	fmt.Println("服务器启动")
	s := server.NewMCPServer(
		"Fenrir - 基于 MCP 的自动化代码审计工具",
		"1.1.0",
	)

	// --- 注册 Prompt ---
	// 1. 读取 prompts/CodeGetPrompts.txt 文件
	promptContent, err := loadPrompt("prompts/CodeGetPrompts.txt")
	if err != nil {
		log.Fatalf("加载 prompts/CodeGetPrompts.txt 失败：%v", err)
	}

	// 2. 定义一个不带参数的 Prompt
	codePrompt := mcp.NewPrompt(
		"code-get-prompts",
		mcp.WithPromptDescription("返回 CodeGetPrompts.txt 中定义的所有 prompt 文本"),
	)

	// 3. 注册 Prompt，处理函数直接返回文件内容
	s.AddPrompt(codePrompt, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		// 先将 promptContent 转为 TextContent
		content, ok := mcp.AsTextContent(promptContent)
		if !ok {
			return nil, fmt.Errorf("无法将 promptContent 转换为 TextContent")
		}

		// 构造一条用户角色的 PromptMessage，内容就是整个文件
		msg := mcp.NewPromptMessage(
			mcp.RoleUser,
			content,
		)
		// 返回结果，不包含任何错误
		return mcp.NewGetPromptResult(
			codePrompt.GetName(),
			[]mcp.PromptMessage{msg},
		), nil
	})

	// 加载配置文件
	config, err := configs.LoadDefaultConfig()
	if err != nil {
		fmt.Printf("加载配置文件失败: %v\n", err)
		os.Exit(1)
	}

	// 检查代码仓库路径
	if config.CodeAudit.RepositoryPath == "" {
		fmt.Println("错误：配置文件中未指定代码仓库路径")
		os.Exit(1)
	}

	// 获取绝对路径
	absPath, err := filepath.Abs(config.CodeAudit.RepositoryPath)
	if err != nil {
		fmt.Printf("错误：无法获取绝对路径：%v\n", err)
		os.Exit(1)
	}

	// 检查路径是否存在
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		fmt.Printf("错误：指定的路径不存在：%s\n", absPath)
		os.Exit(1)
	}

	// 创建AST构建服务
	astService := utils.NewASTBuilderService(config)

	// 构建或加载AST索引
	index, err := astService.BuildOrLoadAST()
	if err != nil {
		log.Fatalf("构建或加载AST失败：%v", err)
	}

	// 创建查询引擎
	query := utils.NewQueryEngine(index)

	// 打印统计信息
	err = astService.PrintStatistics(index)
	if err != nil {
		log.Printf("打印统计信息失败：%v", err)
	}

	// 注册基于 AST 的代码搜索工具
	codeSearchTool := mcp.NewTool("code_search",
		mcp.WithDescription("这是一个基于AST的代码搜索工具，用于在代码仓库中搜索符合特定模式的代码片段。"+
			"你可以通过三个参数传入想要搜索的类、方法或字段。如果你只想搜索类，只需要传入 className 参数，methodName 和 fieldName 置为空字符串。"+
			"如果你想搜索类的方法或字段，需要同时传入（className 和 methodName）或（className 和 fieldName），另一个参数置为空字符串。"+
			"该方法会将结果代码段以字符串形式返回。"),
		mcp.WithString("className",
			mcp.Required(),
			mcp.Description("本参数 className 用于指定要搜索的类名，为必需选项。例如 com.example.myClass ，"+
				"对于内部类，则用 $ 表示，例如 com.example.myClass$insideClass 。"+
				"在不知道包名的情况下，你也可以不传入全类名，只传入类名，例如 myClass 。这会返回所有匹配到的类。"+
				"如果你只搜索类的全部代码，可以只传入 className ，methodName 和 fieldName 置为空字符串。"),
		),
		mcp.WithString("methodName",
			mcp.Required(),
			mcp.Description("本参数 methodName 用于指定要搜索的方法，可以选择带上参数类型，为可选选项。"+
				"例如 myMethod 表示仅根据方法名搜索，仅根据方法名搜索时会返回所有重构方法。"+
				"myMethod() 表示根据方法名和参数类型搜索，此时为无参。"+
				"myMethod(ArgTpye1 arg1, ArgType2[] arg2, String arg3) 表示根据方法名和参数类型搜索，此时为有参。"+
				"在没有具体的参数名称时，你可以用 arg0, arg1，arg2... 表示参数名称"),
		),
		mcp.WithString("fieldName",
			mcp.Required(),
			mcp.Description("本参数 fieldName 用于指定要搜索的字段名，为可选选项。例如 myField 。 "),
		),
	)
	s.AddTool(codeSearchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var className string
		var methodName string
		var fieldName string
		if args, ok := request.Params.Arguments.(map[string]any); ok {
			className = args["className"].(string)
			methodName = args["methodName"].(string)
			fieldName = args["fieldName"].(string)
		}

		// 调用统一搜索函数
		results, err := utils.UnifiedSearch(query, className, methodName, fieldName)
		if err != nil {
			return nil, err
		}

		// 格式化结果
		resultStr := utils.FormatSearchResults(results)

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: resultStr,
				},
			},
		}, nil
	})

	// 注册类父类/子类查找工具
	classHierarchyTool := mcp.NewTool("class_hierarchy",
		mcp.WithDescription("查找指定类的所有父类或所有子类。注意，位于依赖包中的类的子类是无法找到的，但是你可以在项目类中找到依赖包中的父类。"),
		mcp.WithString("className",
			mcp.Required(),
			mcp.Description("本参数 className 指定要查找的类名，可以是全类名（如 com.example.Foo）或简单类名（如 Foo）。"),
		),
		mcp.WithString("type",
			mcp.Required(),
			mcp.Description("本参数 type 指定要查找的是父类还是子类，只有两个可选项：super 表示查找所有父类，sub 表示查找所有子类。"),
		),
	)

	s.AddTool(classHierarchyTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var className string
		var typ string
		if args, ok := request.Params.Arguments.(map[string]any); ok {
			className = args["className"].(string)
			typ = args["type"].(string)
		}

		var resultStr string
		if typ == "super" {
			allSupers := utils.GetAllSuperClasses(query, className)
			resultStr += fmt.Sprintf("类 %s 的所有父类：\n", className)
			found := false
			for _, superList := range allSupers {
				for _, sup := range superList {
					resultStr += fmt.Sprintf("  %s.%s\n", sup.Package, sup.Name)
					found = true
				}
			}
			if !found {
				resultStr += "  (无父类)\n"
			}
		} else if typ == "sub" {
			allSubs := utils.GetAllSubClasses(query, className)
			resultStr += fmt.Sprintf("类 %s 的所有子类：\n", className)
			found := false
			for _, subList := range allSubs {
				for _, sub := range subList {
					resultStr += fmt.Sprintf("  %s.%s\n", sub.Package, sub.Name)
					found = true
				}
			}
			if !found {
				resultStr += "  (无子类)\n"
			}
		} else {
			resultStr = "type 参数只能为 super 或 sub"
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Type: "text", Text: resultStr},
			},
		}, nil
	})

	// 创建基于 SSE 的服务器实例
	sseServer := server.NewSSEServer(s,
		server.WithBaseURL("http://localhost:8338"),
	)

	log.Printf("SSE server listening on http://localhost:8338")
	// 启动服务器
	err = sseServer.Start(":8338")
	if err != nil {
		panic(err)
	}

	// 保持程序运行
	fmt.Println("\n服务器已启动，按 Ctrl+C 退出...")
	select {}
}
