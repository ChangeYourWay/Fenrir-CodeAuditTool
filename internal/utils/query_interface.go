package utils

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// QueryEngine 提供强大的查询功能
type QueryEngine struct {
	index *ASTIndex
}

// NewQueryEngine 创建查询引擎
func NewQueryEngine(index *ASTIndex) *QueryEngine {
	return &QueryEngine{index: index}
}

// GetAllNodes 获取索引中的所有节点
func (e *QueryEngine) GetAllNodes() []UniversalASTNode {
	var nodes []UniversalASTNode
	for _, node := range e.index.index {
		nodes = append(nodes, node)
	}
	return nodes
}

// FindByType 按类型查找节点
func (e *QueryEngine) FindByType(nodeType string) []UniversalASTNode {
	return e.index.FindNodes(func(node UniversalASTNode) bool {
		return node.Type == nodeType
	})
}

// FindByName 按名称查找节点
func (e *QueryEngine) FindByName(name string) []UniversalASTNode {
	return e.index.FindNodes(func(node UniversalASTNode) bool {
		return node.Name == name
	})
}

// FindByPackage 按包名查找节点
func (e *QueryEngine) FindByPackage(pkg string) []UniversalASTNode {
	return e.index.FindNodes(func(node UniversalASTNode) bool {
		return node.Package == pkg
	})
}

// FindByLanguage 按语言查找节点
func (e *QueryEngine) FindByLanguage(language string) []UniversalASTNode {
	return e.index.FindNodes(func(node UniversalASTNode) bool {
		return node.Language == language
	})
}

// GetCodeSnippet 获取代码片段
func (e *QueryEngine) GetCodeSnippet(node UniversalASTNode, contextLines int) (string, error) {
	// 打印调试信息
	fmt.Printf("Getting code snippet for node: %s (Type: %s, Lines: %d-%d)\n",
		node.Name, node.Type, node.StartLine, node.EndLine)

	file, err := os.Open(node.File)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %v", node.File, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	currentLine := 1
	var lines []string
	startLine := node.StartLine - contextLines
	endLine := node.EndLine + contextLines

	if startLine < 1 {
		startLine = 1
	}

	// 打印调试信息
	fmt.Printf("Reading lines %d to %d from file %s\n", startLine, endLine, node.File)

	for scanner.Scan() {
		if currentLine >= startLine && currentLine <= endLine {
			lines = append(lines, scanner.Text())
		}
		currentLine++
		if currentLine > endLine {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file: %v", err)
	}

	// 如果没有找到任何行，返回错误
	if len(lines) == 0 {
		return "", fmt.Errorf("no lines found in range %d-%d", startLine, endLine)
	}

	// 打印调试信息
	fmt.Printf("Found %d lines of code\n", len(lines))

	return strings.Join(lines, "\n"), nil
}

// VisualizeCallGraph 可视化调用关系
func (e *QueryEngine) VisualizeCallGraph(startNode UniversalASTNode, depth int) string {
	var builder strings.Builder
	visited := make(map[string]bool)
	e.traverseCallGraph(&builder, startNode, 0, depth, visited)
	return builder.String()
}

func (e *QueryEngine) traverseCallGraph(builder *strings.Builder, node UniversalASTNode, level, maxDepth int, visited map[string]bool) {
	if visited[node.ID] || level > maxDepth {
		return
	}
	visited[node.ID] = true

	indent := strings.Repeat("  ", level)
	builder.WriteString(fmt.Sprintf("%s[%s] %s.%s (%s:%d)\n",
		indent, node.Language, node.Package, node.Name,
		filepath.Base(node.File), node.StartLine))

	// 查找调用关系 (简化实现)
	called := e.index.FindNodes(func(n UniversalASTNode) bool {
		// 实际项目中需要更精确的关系匹配
		return n.Type == "FunctionCall" && n.Name == node.Name && n.ID != node.ID
	})

	for _, call := range called {
		e.traverseCallGraph(builder, call, level+1, maxDepth, visited)
	}
}
