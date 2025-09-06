package main

import (
	"Fenrir-CodeAuditTool/configs"
	"Fenrir-CodeAuditTool/internal/utils"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

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

func ensureDefaultResourcesAndCache() error {
	// 确保 resources/config.yaml 存在；若不存在创建并写入 AST-only 内容
	resourcesDir := "resources"
	configPath := filepath.Join(resourcesDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.MkdirAll(resourcesDir, 0755); err != nil {
			return fmt.Errorf("创建 resources 目录失败: %v", err)
		}
		defaultYaml := `# 代码审计配置
code_audit:
  # 代码仓库路径
  repository_path: ""
  
  # AST缓存配置
  ast_cache:
    # 是否启用AST缓存
    enabled: true
    # AST缓存目录
    cache_dir: "./cache"
    # 是否在启动时重新构建AST（当enabled为true时，此选项生效）
    rebuild_on_startup: false
`
		if err := os.WriteFile(configPath, []byte(defaultYaml), 0644); err != nil {
			return fmt.Errorf("写入默认 resources/config.yaml 失败: %v", err)
		}
		fmt.Println("resources/config.yaml 不存在，已创建默认配置。")
	}

	// 确保 ./cache 目录存在（默认）。注意：具体cache目录我们之后还会用config里的cache_dir进一步确保
	if _, err := os.Stat("./cache"); os.IsNotExist(err) {
		if err := os.MkdirAll("./cache", 0755); err != nil {
			return fmt.Errorf("创建 cache 目录失败: %v", err)
		}
		fmt.Println("./cache 目录不存在，已创建。")
	}

	return nil
}

func main() {
	// 解析 -i 参数（仓库路径）
	repoPathFlag := flag.String("i", "", "代码仓库路径（优先于 resources/config.yaml 中的配置）")
	flag.Parse()

	// 在加载配置前，确保 resources/config.yaml 与 ./cache 在当前目录下存在（若不存在则创建）
	if err := ensureDefaultResourcesAndCache(); err != nil {
		fmt.Printf("启动前初始化失败: %v\n", err)
		os.Exit(1)
	}

	// 加载配置文件（resources/config.yaml）
	config, err := configs.LoadDefaultConfig()
	if err != nil {
		fmt.Printf("加载配置文件失败: %v\n", err)
		os.Exit(1)
	}

	// 如果命令行提供了 -i，则以命令行参数为准（覆盖配置文件中的 repository_path）
	if repoPathFlag != nil && *repoPathFlag != "" {
		config.CodeAudit.RepositoryPath = *repoPathFlag
	}

	// 检查代码仓库路径是否存在（必须要有路径）
	if config.CodeAudit.RepositoryPath == "" {
		fmt.Println("错误：未指定代码仓库路径。请通过 -i 参数或 resources/config.yaml 中的 code_audit.repository_path 提供。")
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

	// 根据配置确保 cache 目录存在（如果配置中指定了其他目录）
	cacheDir := config.CodeAudit.ASTCache.CacheDir
	if cacheDir == "" {
		cacheDir = "./cache"
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Printf("无法创建/访问缓存目录 %s: %v\n", cacheDir, err)
		os.Exit(1)
	}

	fmt.Println("服务器启动")
	s := server.NewMCPServer(
		"Fenrir - 基于 MCP 的自动化代码审计工具",
		"1.1.0",
	)

	// 创建AST构建服务
	astService := utils.NewASTBuilderService(config)
	//ps := &ProjectService{config: config}

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

	//ps.RegisterMcpServer(s)

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
			mcp.Description("本参数 methodName 用于指定要搜索的方法，可以选择带上参数类型，为可选选项。下面是调用方法："+
				"1、myMethod 表示仅根据方法名搜索，仅根据方法名搜索时会返回所有重构方法。"+
				"2、myMethod() 表示根据方法名和参数类型搜索，此时为无参。"+
				"3、myMethod(ArgTpye1 arg1, ArgType2[] arg2, String arg3) 表示根据方法名和参数类型搜索，此时为有参。在没有具体的参数名称时，你需要用 arg0, arg1，arg2... 表示参数名称",
			),
		),
		mcp.WithString("fieldName",
			mcp.Required(),
			mcp.Description("本参数 fieldName 用于指定要搜索的字段名，为可选选项。例如 myField 。没有指定时候必须为空字符串 "),
		),
	)
	s.AddTool(codeSearchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var className string
		var methodName string
		var fieldName string
		if request.Params.Arguments != nil {
			if args, ok := request.Params.Arguments.(map[string]any); ok {
				// className
				if v, exists := args["className"]; exists && v != nil {
					if s, ok := v.(string); ok {
						className = s
					} else {
						className = fmt.Sprint(v)
					}
				} else {
					className = ""
				}

				// methodName
				if v, exists := args["methodName"]; exists && v != nil {
					if s, ok := v.(string); ok {
						methodName = s
					} else {
						methodName = fmt.Sprint(v)
					}
				} else {
					methodName = ""
				}

				// fieldName
				if v, exists := args["fieldName"]; exists && v != nil {
					if s, ok := v.(string); ok {
						fieldName = s
					} else {
						fieldName = fmt.Sprint(v)
					}
				} else {
					fieldName = ""
				}
			} else {
				// Arguments 不是 map[string]any 时也赋默认空串，防止后续使用 panic
				className = ""
				methodName = ""
				fieldName = ""
			}
		} else {
			// Params 或 Arguments 为 nil
			className = ""
			methodName = ""
			fieldName = ""
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

		if request.Params.Arguments != nil {
			if args, ok := request.Params.Arguments.(map[string]any); ok {
				// className
				if v, exists := args["className"]; exists && v != nil {
					if s, ok := v.(string); ok {
						className = s
					} else {
						className = fmt.Sprint(v)
					}
				} else {
					className = ""
				}

				// methodName
				if v, exists := args["type"]; exists && v != nil {
					if s, ok := v.(string); ok {
						typ = s
					} else {
						typ = fmt.Sprint(v)
					}
				} else {
					typ = ""
				}
			} else {
				// Arguments 不是 map[string]any 时也赋默认空串，防止后续使用 panic
				className = ""
				typ = ""
			}
		} else {
			// Params 或 Arguments 为 nil
			className = ""
			typ = ""
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
