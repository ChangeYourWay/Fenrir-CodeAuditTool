package main

import (
	"Fenrir-CodeAuditTool/configs"
	"fmt"
	"log"
	"testing"

	"Fenrir-CodeAuditTool/internal/utils"
)

func TestName(t *testing.T) {
	// 加载配置文件
	config, err := configs.LoadDefaultConfig()
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	fmt.Println("=== AST构建服务测试 ===")
	fmt.Printf("代码仓库路径: %s\n", config.CodeAudit.RepositoryPath)
	fmt.Printf("AST缓存启用: %t\n", config.CodeAudit.ASTCache.Enabled)
	fmt.Printf("缓存目录: %s\n", config.CodeAudit.ASTCache.CacheDir)
	fmt.Printf("启动时重新构建: %t\n", config.CodeAudit.ASTCache.RebuildOnStartup)

	// 创建AST构建服务
	astService := utils.NewASTBuilderService(config)

	// 显示缓存信息
	fmt.Println("\n=== 缓存信息 ===")
	astService.GetCacheInfo()

	// 测试构建或加载AST
	fmt.Println("\n=== 开始构建或加载AST ===")
	index, err := astService.BuildOrLoadAST()
	if err != nil {
		log.Fatalf("构建或加载AST失败: %v", err)
	}

	// 打印统计信息
	fmt.Println("\n=== AST统计信息 ===")
	err = astService.PrintStatistics(index)
	if err != nil {
		log.Printf("打印统计信息失败: %v", err)
	}

	// 测试查询引擎
	fmt.Println("\n=== 测试查询引擎 ===")
	query := utils.NewQueryEngine(index)

	// 获取所有节点
	allNodes := query.GetAllNodes()
	fmt.Printf("总节点数: %d\n", len(allNodes))

	// 按语言统计
	javaNodes := query.FindByLanguage("java")
	goNodes := query.FindByLanguage("go")
	fmt.Printf("Java节点数: %d\n", len(javaNodes))
	fmt.Printf("Go节点数: %d\n", len(goNodes))

	// 按类型统计
	classNodes := query.FindByType("Class")
	methodNodes := query.FindByType("Method")
	fmt.Printf("类节点数: %d\n", len(classNodes))
	fmt.Printf("方法节点数: %d\n", len(methodNodes))

	// 显示更新后的缓存信息
	fmt.Println("\n=== 更新后的缓存信息 ===")
	astService.GetCacheInfo()

	fmt.Println("\n=== 测试完成 ===")
}
