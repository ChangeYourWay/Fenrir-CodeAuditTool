package main

import (
	"Fenrir-CodeAuditTool/configs"
	"Fenrir-CodeAuditTool/internal/utils"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
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

	//className := "com.example.vulnerablejava.realm.UserRealm"
	//methodName := ""
	//fieldName := ""
	//
	//// 调用统一搜索函数
	//results, err := utils.UnifiedSearch(query, className, methodName, fieldName)
	//if err != nil {
	//	return
	//}
	//
	//// 格式化结果
	//resultStr := utils.FormatSearchResults(results)

	//// 打印结果
	//fmt.Println(resultStr)

	superClasses := utils.GetAllSuperClasses(query, "org.apache.sling.auth.core.impl.LoginServlet")

	subClasses := utils.GetAllSubClasses(query, "org.apache.sling.api.servlets.SlingSafeMethodsServlet")

	fmt.Printf("Superclasses of %s:\n", "org.apache.sling.auth.core.impl.LoginServlet")
	for _, superClass := range superClasses {
		fmt.Printf("Superclass: %s\n", superClass)
	}
	fmt.Printf("Subclasses of %s:\n", "org.apache.sling.api.servlets.SlingSafeMethodsServlet")
	for _, subClass := range subClasses {
		fmt.Printf("Subclass: %s\n", subClass)
	}

}
