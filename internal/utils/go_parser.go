package utils

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

// GoParser 实现 Go 语言的 AST 解析
type GoParser struct{}

func (p *GoParser) Language() string {
	return "go"
}

func (p *GoParser) ParseFile(filePath string) ([]UniversalASTNode, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.AllErrors)
	if err != nil {
		return nil, err
	}

	var nodes []UniversalASTNode
	packageName := file.Name.Name

	// 遍历 AST 节点
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		pos := fset.Position(n.Pos())
		endPos := fset.Position(n.End())
		id := fmt.Sprintf("%s:%d:%d", filePath, pos.Line, pos.Column)

		switch node := n.(type) {
		case *ast.FuncDecl:
			if node.Name != nil {
				nodes = append(nodes, UniversalASTNode{
					ID:        id,
					Language:  "go",
					Type:      "Function",
					Name:      node.Name.Name,
					File:      filePath,
					Package:   packageName,
					StartLine: pos.Line,
					EndLine:   endPos.Line,
					Metadata:  map[string]string{},
				})
			}

		case *ast.TypeSpec:
			if node.Name != nil {
				nodes = append(nodes, UniversalASTNode{
					ID:        id,
					Language:  "go",
					Type:      "Type",
					Name:      node.Name.Name,
					File:      filePath,
					Package:   packageName,
					StartLine: pos.Line,
					EndLine:   endPos.Line,
					Metadata:  map[string]string{},
				})
			}

		case *ast.CallExpr:
			if ident, ok := node.Fun.(*ast.Ident); ok {
				nodes = append(nodes, UniversalASTNode{
					ID:        id,
					Language:  "go",
					Type:      "FunctionCall",
					Name:      ident.Name,
					File:      filePath,
					Package:   packageName,
					StartLine: pos.Line,
					EndLine:   endPos.Line,
					Metadata:  map[string]string{},
				})
			}
		}
		return true
	})

	return nodes, nil
}
