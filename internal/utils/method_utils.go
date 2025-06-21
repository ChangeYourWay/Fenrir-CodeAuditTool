package utils

import (
	"fmt"
	"strings"
)

// ParseMethodSignature 解析方法签名为方法名和参数列表
func ParseMethodSignature(methodSignature string) (string, []string) {
	// 如果方法签名不包含括号，直接返回
	if !strings.Contains(methodSignature, "(") {
		return methodSignature, nil
	}

	// 分离方法名和参数部分
	parts := strings.Split(methodSignature, "(")
	methodName := parts[0]

	// 处理参数部分
	paramsStr := strings.TrimSuffix(parts[1], ")")
	if paramsStr == "" {
		return methodName, nil
	}

	// 分割参数
	var params []string
	currentParam := ""
	bracketCount := 0

	for _, char := range paramsStr {
		switch char {
		case '<':
			bracketCount++
			currentParam += string(char)
		case '>':
			bracketCount--
			currentParam += string(char)
		case ',':
			if bracketCount == 0 {
				// 处理参数类型和形参名称
				paramParts := strings.Fields(strings.TrimSpace(currentParam))
				if len(paramParts) > 0 {
					// 只保留参数类型，去掉形参名称
					paramType := strings.Join(paramParts[:len(paramParts)-1], " ")
					params = append(params, strings.TrimSpace(paramType))
				}
				currentParam = ""
			} else {
				currentParam += string(char)
			}
		default:
			currentParam += string(char)
		}
	}

	if currentParam != "" {
		// 处理最后一个参数
		paramParts := strings.Fields(strings.TrimSpace(currentParam))
		if len(paramParts) > 0 {
			paramType := strings.Join(paramParts[:len(paramParts)-1], " ")
			params = append(params, strings.TrimSpace(paramType))
		}
	}

	// 打印调试信息
	fmt.Printf("Parsed method signature: %s -> %s, params: %v\n", methodSignature, methodName, params)

	return methodName, params
}

// IsMatchingMethod 检查方法是否匹配指定的方法签名
func IsMatchingMethod(node UniversalASTNode, methodSignature string) bool {
	// 解析方法签名
	targetName, targetParams := ParseMethodSignature(methodSignature)

	// 打印调试信息
	fmt.Printf("Checking method: %s against signature: %s\n", node.Name, methodSignature)
	fmt.Printf("Target name: %s, Target params: %v\n", targetName, targetParams)
	fmt.Printf("Node name: %s, Node params: %v\n", node.Name, node.MethodParams)

	// 检查方法名是否匹配
	if node.Name != targetName {
		return false
	}

	// 如果没有参数要求，直接返回true
	if len(targetParams) == 0 {
		return true
	}

	// 检查参数数量是否匹配
	if len(node.MethodParams) != len(targetParams) {
		return false
	}

	// 检查每个参数是否匹配
	for i := range targetParams {
		nodeParam := strings.TrimSpace(node.MethodParams[i])
		targetParam := strings.TrimSpace(targetParams[i])

		// 打印参数比较信息
		fmt.Printf("Comparing params: %s vs %s\n", nodeParam, targetParam)

		if !strings.EqualFold(nodeParam, targetParam) {
			return false
		}
	}

	return true
}
