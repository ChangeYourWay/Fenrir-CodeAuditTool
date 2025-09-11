package main

import (
	"Fenrir-CodeAuditTool/configs"
	"Fenrir-CodeAuditTool/internal/utils"
	"archive/zip"
	"context"
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// loadPrompt 从指定路径读取 prompt 文件内容
func loadPrompt(path string) (string, error) {
	data, err := os.ReadFile(path)
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

# 远程仓库配置
remote_repository:
  enabled: false
  type: ""  # zip, git, local
  url: ""
  branch: "main"
  target_path: "/tmp/fenrir_remote"
  auto_clean: true
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

// RemoteRepositoryManager 远程仓库管理器
type RemoteRepositoryManager struct {
	config *configs.Config
}

// NewRemoteRepositoryManager 创建远程仓库管理器
func NewRemoteRepositoryManager(config *configs.Config) *RemoteRepositoryManager {
	return &RemoteRepositoryManager{
		config: config,
	}
}

// DownloadAndPrepare 下载并准备远程仓库
func (m *RemoteRepositoryManager) DownloadAndPrepare() (string, error) {
	repoConfig := m.config.RemoteRepository

	switch repoConfig.Type {
	case "zip":
		return m.downloadAndExtractZip(repoConfig.URL, repoConfig.TargetPath)
	case "git":
		return m.cloneGitRepository(repoConfig.URL, repoConfig.Branch, repoConfig.TargetPath)
	case "local":
		return repoConfig.TargetPath, nil
	default:
		return "", fmt.Errorf("不支持的仓库类型: %s", repoConfig.Type)
	}
}

// downloadAndExtractZip 下载并解压ZIP文件
func (m *RemoteRepositoryManager) downloadAndExtractZip(url, targetPath string) (string, error) {
	// 清理目标目录
	if err := os.RemoveAll(targetPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("清理目录失败: %v", err)
	}

	// 创建临时目录
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %v", err)
	}

	// 下载文件
	zipPath := filepath.Join(targetPath, "download.zip")
	if err := m.downloadFile(url, zipPath); err != nil {
		return "", err
	}

	// 解压文件
	if err := m.extractZip(zipPath, targetPath); err != nil {
		return "", err
	}

	// 删除zip文件
	os.Remove(zipPath)

	return targetPath, nil
}

// downloadFile 下载文件
func (m *RemoteRepositoryManager) downloadFile(url, filepath string) error {
	fmt.Printf("开始下载: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("下载失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	fmt.Printf("下载完成: %s\n", filepath)
	return nil
}

// extractZip 解压ZIP文件
func (m *RemoteRepositoryManager) extractZip(zipPath, targetPath string) error {
	fmt.Printf("开始解压: %s\n", zipPath)

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开zip文件失败: %v", err)
	}
	defer r.Close()

	for _, f := range r.File {
		// 防止路径遍历攻击
		fpath := filepath.Join(targetPath, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(targetPath)+string(os.PathSeparator)) {
			return fmt.Errorf("非法文件路径: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, 0755); err != nil {
				return fmt.Errorf("创建目录失败: %v", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return fmt.Errorf("创建父目录失败: %v", err)
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("创建文件失败: %v", err)
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("打开zip内文件失败: %v", err)
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			return fmt.Errorf("解压文件失败: %v", err)
		}
	}

	fmt.Printf("解压完成: %s\n", targetPath)
	return nil
}

// cloneGitRepository 克隆Git仓库
func (m *RemoteRepositoryManager) cloneGitRepository(url, branch, targetPath string) (string, error) {
	fmt.Printf("开始克隆Git仓库: %s (分支: %s)\n", url, branch)

	// 清理目标目录
	if err := os.RemoveAll(targetPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("清理目录失败: %v", err)
	}

	// 构建git命令
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, url, targetPath)

	cmd := exec.Command("git", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git克隆失败: %v, 输出: %s", err, string(output))
	}

	fmt.Printf("Git克隆完成: %s\n", targetPath)
	return targetPath, nil
}

// Cleanup 清理临时文件
func (m *RemoteRepositoryManager) Cleanup() error {
	if m.config.RemoteRepository.AutoClean {
		fmt.Printf("清理临时目录: %s\n", m.config.RemoteRepository.TargetPath)
		return os.RemoveAll(m.config.RemoteRepository.TargetPath)
	}
	return nil
}

// handleRemoteRepository 处理远程仓库下载和准备
func handleRemoteRepository(config *configs.Config, remoteSpec, branch string) (string, error) {
	parts := strings.SplitN(remoteSpec, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("远程仓库格式错误，应为 type:url")
	}

	repoType, repoURL := parts[0], parts[1]

	// 设置临时目录
	tempDir := filepath.Join(os.TempDir(), "fenrir_remote", fmt.Sprintf("%x", md5.Sum([]byte(remoteSpec))))

	// 确保目录存在
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("创建临时目录失败: %v", err)
	}

	// 更新配置
	config.RemoteRepository.Enabled = true
	config.RemoteRepository.Type = repoType
	config.RemoteRepository.URL = repoURL
	config.RemoteRepository.Branch = branch
	config.RemoteRepository.TargetPath = tempDir
	config.RemoteRepository.AutoClean = true

	// 下载和处理仓库
	repoManager := NewRemoteRepositoryManager(config)
	repoPath, err := repoManager.DownloadAndPrepare()
	if err != nil {
		return "", err
	}

	fmt.Printf("远程仓库已下载到: %s\n", repoPath)
	return repoPath, nil
}

// ServerState 服务器状态
type ServerState struct {
	config     *configs.Config
	astService *utils.ASTBuilderService
	query      *utils.QueryEngine
	index      *utils.ASTIndex
	ready      bool
	readyMutex sync.RWMutex
	readyCond  *sync.Cond
}

// NewServerState 创建服务器状态
func NewServerState(config *configs.Config) *ServerState {
	state := &ServerState{
		config: config,
	}
	state.readyCond = sync.NewCond(&state.readyMutex)
	return state
}

// WaitUntilReady 等待直到服务器就绪
func (s *ServerState) WaitUntilReady() {
	s.readyMutex.RLock()
	defer s.readyMutex.RUnlock()

	for !s.ready {
		s.readyMutex.RUnlock()
		s.readyCond.L.Lock()
		s.readyCond.Wait()
		s.readyCond.L.Unlock()
		s.readyMutex.RLock()
	}
}

// SetReady 设置服务器就绪状态
func (s *ServerState) SetReady(ready bool) {
	s.readyMutex.Lock()
	defer s.readyMutex.Unlock()

	s.ready = ready
	if ready {
		s.readyCond.Broadcast()
	}
}

// InitializeAST 初始化AST索引
func (s *ServerState) InitializeAST() error {
	// 创建AST构建服务
	s.astService = utils.NewASTBuilderService(s.config)

	// 构建或加载AST索引
	index, err := s.astService.BuildOrLoadAST()
	if err != nil {
		return fmt.Errorf("构建或加载AST失败：%v", err)
	}

	s.index = index

	// 创建查询引擎
	s.query = utils.NewQueryEngine(index)

	// 打印统计信息
	err = s.astService.PrintStatistics(index)
	if err != nil {
		log.Printf("打印统计信息失败：%v", err)
	}

	s.SetReady(true)
	return nil
}

// IsReady 检查服务器是否就绪
func (s *ServerState) IsReady() bool {
	s.readyMutex.RLock()
	defer s.readyMutex.RUnlock()
	return s.ready
}

func main() {
	// 解析 -i 参数（仓库路径）
	repoPathFlag := flag.String("i", "", "代码仓库路径（优先于 resources/config.yaml 中的配置）")
	remoteRepoFlag := flag.String("remote", "", "远程仓库URL (格式: type:url, 如 zip:https://example.com/repo.zip)")
	branchFlag := flag.String("branch", "main", "Git分支名 (仅用于git类型)")
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

	// 处理命令行远程仓库
	if *remoteRepoFlag != "" {
		repoPath, err := handleRemoteRepository(config, *remoteRepoFlag, *branchFlag)
		if err != nil {
			log.Fatalf("处理远程仓库失败: %v", err)
		}
		config.CodeAudit.RepositoryPath = repoPath
	}

	// 如果命令行提供了 -i，则以命令行参数为准（覆盖配置文件中的 repository_path）
	if repoPathFlag != nil && *repoPathFlag != "" {
		config.CodeAudit.RepositoryPath = *repoPathFlag
	}

	// 创建服务器状态
	serverState := NewServerState(config)

	// 如果已经指定了代码仓库路径，立即初始化AST
	if config.CodeAudit.RepositoryPath != "" {
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

		if err := serverState.InitializeAST(); err != nil {
			log.Fatalf("初始化AST失败: %v", err)
		}
	} else {
		fmt.Println("未指定本地代码仓库路径，等待远程连接...")
		fmt.Println("请使用 MCP 客户端调用 remote_code_audit 工具来提供远程仓库地址")
	}

	fmt.Println("服务器启动")
	s := server.NewMCPServer(
		"Fenrir - 基于 MCP 的自动化代码审计工具",
		"1.2.0", // 更新版本号
	)

	// 注册远程代码审计工具（主要工具）
	remoteAuditTool := mcp.NewTool("remote_code_audit",
		mcp.WithDescription("从远程仓库下载代码并进行自动化代码审计。如果服务器未初始化，此工具将设置代码仓库路径并初始化AST索引"),
		mcp.WithString("repository_url",
			mcp.Required(),
			mcp.Description("远程仓库URL，支持格式: zip:https://example.com/repo.zip 或 git:https://github.com/user/repo.git"),
		),
		mcp.WithString("branch",
			mcp.Description("Git分支名 (仅用于git仓库)"),
		),
	)

	s.AddTool(remoteAuditTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var repoURL, branch string

		if request.Params.Arguments != nil {
			if args, ok := request.Params.Arguments.(map[string]any); ok {
				if url, exists := args["repository_url"]; exists {
					repoURL = fmt.Sprint(url)
				}
				if br, exists := args["branch"]; exists {
					branch = fmt.Sprint(br)
				}
			}
		}

		if repoURL == "" {
			return nil, fmt.Errorf("repository_url 参数是必需的")
		}

		// 处理远程仓库
		repoPath, err := handleRemoteRepository(serverState.config, repoURL, branch)
		if err != nil {
			return nil, fmt.Errorf("处理远程仓库失败: %v", err)
		}

		// 更新配置
		serverState.config.CodeAudit.RepositoryPath = repoPath

		// 初始化AST
		if err := serverState.InitializeAST(); err != nil {
			return nil, fmt.Errorf("初始化AST失败: %v", err)
		}

		// 获取统计信息
		stats := make(map[string]interface{})
		nodes := serverState.query.GetAllNodes()
		stats["total_nodes"] = len(nodes)

		// 按类型统计
		typeCount := make(map[string]int)
		for _, node := range nodes {
			typeCount[node.Type]++
		}
		stats["by_type"] = typeCount

		// 按语言统计
		languageCount := make(map[string]int)
		for _, node := range nodes {
			languageCount[node.Language]++
		}
		stats["by_language"] = languageCount

		resultJSON, _ := json.MarshalIndent(stats, "", "  ")

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("远程代码审计完成!\n仓库路径: %s\n审计统计:\n%s",
						repoPath, string(resultJSON)),
				},
			},
		}, nil
	})

	// 注册基于 AST 的代码搜索工具（只有在AST初始化后才可用）
	codeSearchTool := mcp.NewTool("code_search",
		mcp.WithDescription("这是一个基于AST的代码搜索工具，用于在代码仓库中搜索符合特定模式的代码片段。"+
			"你需要先使用 remote_code_audit 工具设置代码仓库。"+
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
		// 等待直到AST就绪
		serverState.WaitUntilReady()

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
		results, err := utils.UnifiedSearch(serverState.query, className, methodName, fieldName)
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

	// 注册类父类/子类查找工具（只有在AST初始化后才可用）
	classHierarchyTool := mcp.NewTool("class_hierarchy",
		mcp.WithDescription("查找指定类的所有父类或所有子类。注意，位于依赖包中的类的子类是无法找到的，但是你可以在项目类中找到依赖包中的父类。"+
			"你需要先使用 remote_code_audit 工具设置代码仓库。"),
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
		// 等待直到AST就绪
		serverState.WaitUntilReady()

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
			allSupers := utils.GetAllSuperClasses(serverState.query, className)
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
			allSubs := utils.GetAllSubClasses(serverState.query, className)
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
	if !serverState.IsReady() {
		fmt.Println("等待远程代码仓库连接...")
		fmt.Println("请使用 MCP 客户端调用 remote_code_audit 工具来提供远程仓库地址")
	}

	select {}
}
