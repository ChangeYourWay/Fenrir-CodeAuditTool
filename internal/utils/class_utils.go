package utils

import "strings"

// 解析全限定类名
func ParseFullClassName(fullName string) (pkgPath, className string, isInner bool) {
	// 处理内部类表示法
	if strings.Contains(fullName, "$") {
		fullName = strings.ReplaceAll(fullName, "$", ".")
		isInner = true
	}

	parts := strings.Split(fullName, ".")
	if len(parts) == 0 {
		return "", "", false
	}

	// 分离包路径和类名
	className = parts[len(parts)-1]
	if len(parts) > 1 {
		pkgPath = strings.Join(parts[:len(parts)-1], ".")
	}

	return pkgPath, className, isInner
}

// 检查节点是否匹配全限定类名
func IsMatchingClass(node UniversalASTNode, fullClassName string) bool {
	targetPkg, targetName, targetIsInner := ParseFullClassName(fullClassName)

	// 对于类节点，使用FullClassName或Package+Name
	if node.Type == "Class" {
		var nodePkg string
		var nodeFullName string

		// 如果FullClassName已设置，使用它
		if node.FullClassName != "" {
			nodeFullName = node.FullClassName
			nodePkg, _, _ = ParseFullClassName(node.FullClassName)
		} else {
			// 否则使用Package字段
			nodePkg = node.Package
			if node.Package != "" {
				nodeFullName = node.Package + "." + node.Name
			} else {
				nodeFullName = node.Name
			}
		}

		// 检查包路径匹配
		if targetPkg != "" && nodePkg != targetPkg {
			return false
		}

		// 检查类名匹配（支持内部类）
		if targetIsInner {
			// 对于内部类，检查完整类名是否匹配
			return nodeFullName == fullClassName
		}

		// 对于普通类，检查类名是否匹配
		return node.Name == targetName
	}

	// 对于非类节点，不应该直接匹配类名
	return false
}

// 检查节点是否属于指定的类（通过文件路径和行号范围）
func IsNodeInClass(node UniversalASTNode, className string, allNodes map[string]UniversalASTNode) bool {
	// 首先找到目标类
	var targetClass UniversalASTNode
	found := false

	for _, n := range allNodes {
		if n.Type == "Class" && IsMatchingClass(n, className) {
			targetClass = n
			found = true
			break
		}
	}

	if !found {
		return false
	}

	// 检查节点是否在目标类的范围内
	return node.File == targetClass.File &&
		node.StartLine >= targetClass.StartLine &&
		node.EndLine <= targetClass.EndLine
}
