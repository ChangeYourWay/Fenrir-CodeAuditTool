package utils

import (
	"fmt"
	"strings"
)

// 查询类节点并返回源代码片段（支持全限定类名和内部类）
func SearchClassOnly(query *QueryEngine, className string) ([]string, error) {
	var results []string
	for _, node := range query.index.index {
		if node.Type == "Class" && IsMatchingClass(node, className) {
			snippet, err := query.GetCodeSnippet(node, 100) // 0 表示不扩展上下文行
			if err == nil {
				results = append(results, snippet)
			}
		}
	}
	return results, nil
}

// 查询类中的方法并返回源代码片段（支持方法签名）
func SearchClassMethod(query *QueryEngine, className, methodName string) ([]string, error) {
	var targetClasses []UniversalASTNode
	for _, node := range query.index.index {
		if node.Type == "Class" && IsMatchingClass(node, className) {
			targetClasses = append(targetClasses, node)
			fmt.Printf("Found target class: %s.%s\n", node.Package, node.Name)
		}
	}

	var results []string
	for _, class := range targetClasses {
		for _, node := range query.index.index {
			// 检查方法是否在目标类范围内
			if node.Type == "Method" &&
				node.File == class.File &&
				node.StartLine >= class.StartLine &&
				node.EndLine <= class.EndLine {

				// 打印调试信息
				fmt.Printf("Checking method: %s.%s (Return: %s, Params: %v)\n",
					node.Package, node.Name, node.Metadata["returnType"], node.MethodParams)

				// 检查方法签名匹配
				if IsMatchingMethod(node, methodName) {
					fmt.Printf("Found matching method: %s.%s\n", node.Package, node.Name)
					snippet, err := query.GetCodeSnippet(node, 1) // 添加1行上下文
					if err != nil {
						fmt.Printf("Error getting code snippet: %v\n", err)
						continue
					}
					results = append(results, snippet)
				}
			}
		}
	}
	return results, nil
}

// 查询类中的字段并返回源代码片段
func SearchClassField(query *QueryEngine, className, fieldName string) ([]string, error) {
	var targetClasses []UniversalASTNode

	// 首先尝试精确匹配
	for _, node := range query.index.index {
		if node.Type == "Class" && IsMatchingClass(node, className) {
			targetClasses = append(targetClasses, node)
		}
	}

	// 如果没有找到目标类，尝试更宽松的匹配
	if len(targetClasses) == 0 {
		_, targetClassName, _ := ParseFullClassName(className)
		for _, node := range query.index.index {
			if node.Type == "Class" && node.Name == targetClassName {
				targetClasses = append(targetClasses, node)
			}
		}
	}

	var results []string
	for _, class := range targetClasses {
		// 在类的字段列表中查找目标字段
		for _, field := range class.Fields {
			if field.Name == fieldName {
				// 创建临时节点用于获取代码片段
				fieldNode := UniversalASTNode{
					ID:        fmt.Sprintf("%s:%s:%d", class.File, field.Name, field.StartLine),
					Language:  class.Language,
					Type:      "Field",
					Name:      field.Name,
					File:      class.File,
					Package:   class.Package,
					StartLine: field.StartLine,
					EndLine:   field.EndLine,
					Metadata: map[string]string{
						"fieldType": field.Type,
					},
				}

				// 获取字段的代码片段
				snippet, err := query.GetCodeSnippet(fieldNode, 1)
				if err == nil {
					results = append(results, snippet)
				}
			}
		}
	}

	return results, nil
}

// 增强版查询（支持嵌套类和内部类）
func searchInClassScope(query *QueryEngine, className, targetName, targetType string) ([]string, error) {
	var classes []UniversalASTNode

	// 首先尝试精确匹配
	for _, node := range query.index.index {
		if node.Type == "Class" && IsMatchingClass(node, className) {
			classes = append(classes, node)
		}
	}

	// 如果没有找到目标类，尝试更宽松的匹配
	if len(classes) == 0 {
		_, targetClassName, _ := ParseFullClassName(className)
		for _, node := range query.index.index {
			if node.Type == "Class" && node.Name == targetClassName {
				classes = append(classes, node)
			}
		}
	}

	var results []string
	var findInClass func(class UniversalASTNode)

	findInClass = func(class UniversalASTNode) {
		// 查找目标元素（方法或字段）
		for _, node := range query.index.index {
			if node.File != class.File ||
				node.StartLine < class.StartLine ||
				node.EndLine > class.EndLine {
				continue
			}

			if node.Type == targetType {
				// 特殊处理方法签名
				if targetType == "Method" {
					if IsMatchingMethod(node, targetName) {
						snippet, err := query.GetCodeSnippet(node, 1)
						if err == nil {
							results = append(results, snippet)
						}
					}
				} else {
					// 字段直接匹配
					if node.Name == targetName {
						snippet, err := query.GetCodeSnippet(node, 1)
						if err == nil {
							results = append(results, snippet)
						}
					}
				}
			}
		}

		// 递归查找嵌套类
		for _, node := range query.index.index {
			if node.Type == "Class" &&
				node.File == class.File &&
				node.StartLine > class.StartLine &&
				node.EndLine < class.EndLine {
				findInClass(node)
			}
		}
	}

	for _, class := range classes {
		findInClass(class)
	}
	return results, nil
}

// 使用增强版查询方法并返回源代码片段
func EnhancedSearchClassMethod(query *QueryEngine, className, methodName string) ([]string, error) {
	return searchInClassScope(query, className, methodName, "Method")
}

func EnhancedSearchClassField(query *QueryEngine, className, fieldName string) ([]string, error) {
	var results []string
	seen := make(map[string]struct{}) // 用于去重

	// 遍历所有节点
	for _, node := range query.GetAllNodes() {
		// 检查是否是目标类
		if node.Type == "Class" && IsMatchingClass(node, className) {
			fmt.Printf("Found matching class: %s.%s\n", node.Package, node.Name)

			// 在类的字段列表中查找目标字段
			for _, field := range node.Fields {
				if field.Name == fieldName {
					fmt.Printf("Found matching field: %s (Type: %s, Lines: %d-%d)\n",
						field.Name, field.Type, field.StartLine, field.EndLine)

					// 创建临时节点用于获取代码片段
					fieldNode := UniversalASTNode{
						ID:        fmt.Sprintf("%s:%s:%d", node.File, field.Name, field.StartLine),
						Language:  node.Language,
						Type:      "Field",
						Name:      field.Name,
						File:      node.File,
						Package:   node.Package,
						StartLine: field.StartLine,
						EndLine:   field.EndLine,
						Metadata: map[string]string{
							"fieldType": field.Type,
						},
					}

					// 获取字段的代码片段
					snippet, err := query.GetCodeSnippet(fieldNode, 1) // 添加1行上下文
					if err != nil {
						fmt.Printf("Error getting code snippet: %v\n", err)
						continue
					}

					// 以代码片段内容为唯一性依据去重
					if _, ok := seen[snippet]; !ok {
						result := fmt.Sprintf("Field: %s (Type: %s)\n%s",
							field.Name, field.Type, snippet)
						results = append(results, result)
						seen[snippet] = struct{}{}
					}
				}
			}
		}
	}

	return results, nil
}

// 智能搜索方法（根据输入自动选择简单或增强搜索）
func SmartSearchClassMethod(query *QueryEngine, className, methodName string) ([]string, error) {
	// 如果方法名包含括号，使用增强匹配（支持方法签名）
	if strings.Contains(methodName, "(") {
		return EnhancedSearchClassMethod(query, className, methodName)
	}

	// 否则使用简单搜索（仅方法名）
	return SearchClassMethod(query, className, methodName)
}

// 格式化搜索结果函数
func FormatSearchResults(results []string) string {
	if len(results) == 0 {
		return "未找到匹配结果"
	}

	var builder strings.Builder
	for i, res := range results {
		builder.WriteString(fmt.Sprintf("==== 结果 %d ====\n%s\n\n", i+1, res))
	}
	return builder.String()
}

// 新增：统一搜索入口函数
func UnifiedSearch(query *QueryEngine, className, methodName, fieldName string) ([]string, error) {
	// 验证参数
	if className == "" {
		return nil, fmt.Errorf("className is required")
	}
	if methodName != "" && fieldName != "" {
		return nil, fmt.Errorf("cannot specify both methodName and fieldName at the same time")
	}

	switch {
	case methodName != "":
		return SmartSearchClassMethod(query, className, methodName)
	case fieldName != "":
		return EnhancedSearchClassField(query, className, fieldName)
	default:
		return SearchClassOnly(query, className)
	}
}

// 查询接口：查找所有父类（递归获取所有父类）
func GetAllSuperClasses(query *QueryEngine, className string) [][]ClassRef {
	var results [][]ClassRef
	for _, node := range query.index.index {
		if node.Type == "Class" && (node.FullClassName == className || node.Name == className) {
			// 递归收集所有父类
			allSuperClasses := collectAllSuperClasses(query, node.FullClassName, make(map[string]bool))
			results = append(results, allSuperClasses)
		}
	}
	return results
}

// 递归收集所有父类（包括间接父类）
func collectAllSuperClasses(query *QueryEngine, className string, visited map[string]bool) []ClassRef {
	var allSuperClasses []ClassRef

	// 查找当前类
	var currentNode UniversalASTNode
	found := false
	for _, node := range query.index.index {
		if node.Type == "Class" && node.FullClassName == className {
			currentNode = node
			found = true
			break
		}
	}

	if !found {
		return allSuperClasses
	}

	// 添加直接父类
	for _, superClass := range currentNode.SuperClasses {
		superClassName := superClass.Package + "." + superClass.Name
		if !visited[superClassName] {
			visited[superClassName] = true
			allSuperClasses = append(allSuperClasses, superClass)
			// 递归获取父类的父类
			parentSupers := collectAllSuperClasses(query, superClassName, visited)
			allSuperClasses = append(allSuperClasses, parentSupers...)
		}
	}

	return allSuperClasses
}

// 查询接口：查找所有子类
func GetAllSubClasses(query *QueryEngine, className string) [][]ClassRef {
	var results [][]ClassRef
	for _, node := range query.index.index {
		if node.Type == "Class" && (node.FullClassName == className || node.Name == className) {
			results = append(results, node.SubClasses)
		}
	}
	return results
}
