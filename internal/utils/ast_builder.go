package utils

import (
	"Fenrir-CodeAuditTool/configs"
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

// ASTBuilderService AST构建服务
type ASTBuilderService struct {
	config      *configs.Config
	manager     *ParserManager
	persistence *ASTPersistenceManager
}

// NewASTBuilderService 创建AST构建服务
func NewASTBuilderService(config *configs.Config) *ASTBuilderService {
	// 创建解析器管理器
	manager := NewParserManager()

	// 注册解析器
	manager.RegisterParser(&GoParser{})
	manager.RegisterParser(&JavaParser{})
	// 可以添加更多语言的解析器

	// 创建持久化管理器
	persistence := NewASTPersistenceManager(config)

	return &ASTBuilderService{
		config:      config,
		manager:     manager,
		persistence: persistence,
	}
}

// BuildOrLoadAST 构建或加载AST索引
func (s *ASTBuilderService) BuildOrLoadAST() (*ASTIndex, error) {
	// 检查是否启用缓存
	if !s.config.CodeAudit.ASTCache.Enabled {
		log.Println("AST缓存已禁用，正在重新构建AST...")
		return s.buildAST()
	}

	// 检查是否需要重新构建
	if s.config.CodeAudit.ASTCache.RebuildOnStartup {
		log.Println("配置要求重新构建AST，正在构建...")
		return s.buildAST()
	}

	// 尝试从缓存加载
	if s.persistence.CacheExists() {
		log.Println("发现AST缓存文件，正在加载...")
		index, err := s.persistence.LoadASTIndex()
		if err != nil {
			log.Printf("加载AST缓存失败: %v，正在重新构建...", err)
			return s.buildAST()
		}

		// 验证加载的索引
		nodes := index.FindNodes(func(node UniversalASTNode) bool { return true })
		log.Printf("成功从缓存加载AST，节点数: %d", len(nodes))
		return index, nil
	}

	// 缓存不存在，构建新的AST
	log.Println("未发现AST缓存文件，正在构建AST...")
	return s.buildAST()
}

// buildAST 构建AST索引
func (s *ASTBuilderService) buildAST() (*ASTIndex, error) {
	repoPath := s.config.CodeAudit.RepositoryPath

	// 获取绝对路径
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("无法获取绝对路径: %v", err)
	}

	// 检查路径是否存在
	if _, err := filepath.Abs(absPath); err != nil {
		return nil, fmt.Errorf("指定的路径不存在: %s", absPath)
	}

	log.Printf("正在分析代码仓库: %s", absPath)

	// 构建索引
	err = s.manager.BuildIndexFromDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("构建代码索引失败: %v", err)
	}

	index := s.manager.GetIndex()

	// 如果启用缓存，保存到文件
	if s.config.CodeAudit.ASTCache.Enabled {
		log.Println("正在保存AST索引到缓存文件...")
		err = s.persistence.SaveASTIndex(index)
		if err != nil {
			log.Printf("保存AST缓存失败: %v", err)
			// 不返回错误，因为索引已经构建成功
		} else {
			log.Println("AST索引已保存到缓存文件")
		}
	}

	return index, nil
}

// GetQueryEngine 获取查询引擎
func (s *ASTBuilderService) GetQueryEngine() (*QueryEngine, error) {
	index, err := s.BuildOrLoadAST()
	if err != nil {
		return nil, err
	}

	return NewQueryEngine(index), nil
}

// PrintStatistics 打印统计信息
func (s *ASTBuilderService) PrintStatistics(index *ASTIndex) error {
	query := NewQueryEngine(index)
	nodes := query.GetAllNodes()

	fmt.Printf("\n代码分析完成！\n")
	fmt.Printf("总节点数：%d\n", len(nodes))

	// 按类型统计
	typeCount := make(map[string]int)
	for _, node := range nodes {
		typeCount[node.Type]++
	}

	fmt.Println("\n节点类型统计：")
	for nodeType, count := range typeCount {
		fmt.Printf("- %s: %d\n", nodeType, count)
	}

	return nil
}

// ClearCache 清除缓存
func (s *ASTBuilderService) ClearCache() error {
	return s.persistence.ClearCache()
}

// ListCacheFiles 列出缓存文件
func (s *ASTBuilderService) ListCacheFiles() ([]string, error) {
	return s.persistence.ListCacheFiles()
}

// GetCacheInfo 获取缓存信息
func (s *ASTBuilderService) GetCacheInfo() {
	fmt.Println("=== 缓存信息 ===")
	fmt.Printf("缓存目录: %s\n", s.config.CodeAudit.ASTCache.CacheDir)
	fmt.Printf("缓存启用: %t\n", s.config.CodeAudit.ASTCache.Enabled)
	fmt.Printf("启动时重新构建: %t\n", s.config.CodeAudit.ASTCache.RebuildOnStartup)

	// 生成新的缓存文件名
	newCacheFile := s.config.GenerateCacheFileName()
	fmt.Printf("下次构建的缓存文件: %s\n", newCacheFile)

	// 列出现有缓存文件
	files, err := s.ListCacheFiles()
	if err != nil {
		fmt.Printf("获取缓存文件列表失败: %v\n", err)
		return
	}

	if len(files) == 0 {
		fmt.Println("当前没有缓存文件")
	} else {
		fmt.Printf("现有缓存文件 (%d 个):\n", len(files))
		for i, file := range files {
			fmt.Printf("  %d. %s\n", i+1, filepath.Base(file))
		}
	}
}

// BuildClassHierarchy 构建类的父类和子类链
func (s *ASTBuilderService) BuildClassHierarchy(index *ASTIndex) {
	classMap := make(map[string]string) // 类名到节点ID的映射
	for id, node := range index.index {
		if node.Type == "Class" {
			classMap[node.FullClassName] = id
		}
	}

	parentToChildren := make(map[string][]string) // 父类名到子类ID列表的映射
	for id, node := range index.index {
		if node.Type == "Class" {
			fmt.Printf("处理类: %s, SuperClasses 数量: %d\n", node.FullClassName, len(node.SuperClasses))
			// 使用 SuperClasses 字段而不是 Metadata
			for _, superClass := range node.SuperClasses {
				superClassName := superClass.Package + "." + superClass.Name
				if superClassName != "" {
					parentToChildren[superClassName] = append(parentToChildren[superClassName], id)
					// 调试输出
					fmt.Printf("建立父子关系: %s -> %s\n", superClassName, node.FullClassName)
				} else {
					fmt.Printf("警告: 类 %s 的父类包名或类名为空 (Package: %s, Name: %s)\n",
						node.FullClassName, superClass.Package, superClass.Name)
				}
			}
		}
	}

	// 调试输出：检查 parentToChildren 映射
	fmt.Printf("parentToChildren 映射大小: %d\n", len(parentToChildren))
	for parent, children := range parentToChildren {
		fmt.Printf("父类 %s 有 %d 个子类\n", parent, len(children))
	}

	// 补全所有父类链（只补直接父类/接口）
	for id, node := range index.index {
		if node.Type == "Class" && node.Metadata != nil {
			tmp := index.index[id]
			if supers, ok := node.Metadata["superClasses"]; ok && supers != "" {
				for _, super := range strings.Split(supers, ",") {
					super = strings.TrimSpace(super)
					if parentID, ok := classMap[super]; ok {
						parentNode := index.index[parentID]
						tmp.SuperClasses = append(tmp.SuperClasses, ClassRef{Package: parentNode.Package, Name: parentNode.Name})
					}
				}
			}
			index.index[id] = tmp
		}
	}

	// 补全所有子类链（递归收集所有子类）
	var collectAllSubClasses func(className string, visited map[string]bool) []ClassRef
	collectAllSubClasses = func(className string, visited map[string]bool) []ClassRef {
		var result []ClassRef
		childIDs := parentToChildren[className]
		fmt.Printf("收集 %s 的子类，找到 %d 个直接子类\n", className, len(childIDs))
		for _, childID := range childIDs {
			childNode := index.index[childID]
			if visited[childNode.FullClassName] {
				continue
			}
			visited[childNode.FullClassName] = true
			result = append(result, ClassRef{Package: childNode.Package, Name: childNode.Name})
			fmt.Printf("添加子类: %s.%s\n", childNode.Package, childNode.Name)
			result = append(result, collectAllSubClasses(childNode.FullClassName, visited)...)
		}
		return result
	}
	for id, node := range index.index {
		if node.Type == "Class" {
			visited := make(map[string]bool)
			tmp := index.index[id]
			// 递归收集所有子类，并直接赋值给 SubClasses 字段
			tmp.SubClasses = collectAllSubClasses(node.FullClassName, visited)
			fmt.Printf("类 %s 最终收集到 %d 个子类\n", node.FullClassName, len(tmp.SubClasses))
			index.index[id] = tmp
		}
	}
}
