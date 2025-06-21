package utils

import (
	"fmt"
	"os"
	"strings"

	// 使用第三方 Java 解析器
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
)

// JavaParser 实现 Java 语言的 AST 解析
type JavaParser struct{}

func (p *JavaParser) Language() string {
	return "java"
}

func (p *JavaParser) ParseFile(filePath string) ([]UniversalASTNode, error) {
	// 读取 Java 文件内容
	code, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// 使用 Tree-sitter 解析 Java 代码
	parser := sitter.NewParser()
	parser.SetLanguage(java.GetLanguage())
	tree := parser.Parse(nil, code)

	defer tree.Close()

	root := tree.RootNode()
	var nodes []UniversalASTNode

	// 首先提取包名
	packageName := p.extractPackageName(root, code)

	// 收集 import 语句
	importMap := map[string]string{}
	importStar := []string{}
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		if child != nil && child.Type() == "import_declaration" {
			importPath := ""
			for j := 0; j < int(child.ChildCount()); j++ {
				idNode := child.Child(j)
				if idNode != nil && (idNode.Type() == "scoped_identifier" || idNode.Type() == "identifier") {
					importPath = idNode.Content(code)
				}
			}
			if strings.HasSuffix(importPath, ".*") {
				importStar = append(importStar, strings.TrimSuffix(importPath, ".*"))
			} else if importPath != "" {
				parts := strings.Split(importPath, ".")
				simpleName := parts[len(parts)-1]
				importMap[simpleName] = importPath
			}
		}
	}

	// 遍历 AST 提取关键信息
	p.traverseNode(root, filePath, code, packageName, importMap, importStar, &nodes)

	return nodes, nil
}

// 提取包名 - 在文件级别提取，只执行一次
func (p *JavaParser) extractPackageName(root *sitter.Node, code []byte) string {
	// 查找包声明节点
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		if child == nil {
			continue
		}

		// 检查是否是包声明
		if child.Type() == "package_declaration" {
			// 查找包名标识符
			for j := 0; j < int(child.ChildCount()); j++ {
				grandChild := child.Child(j)
				if grandChild == nil {
					continue
				}

				switch grandChild.Type() {
				case "scoped_identifier":
					// 多段包名，如 com.example.test
					return grandChild.Content(code)
				case "identifier":
					// 单段包名，如 test
					return grandChild.Content(code)
				}
			}
		}
	}

	// 如果没有找到包声明，返回默认包名
	return "default.package"
}

func (p *JavaParser) traverseNode(node *sitter.Node, filePath string, code []byte, packageName string, importMap map[string]string, importStar []string, nodes *[]UniversalASTNode) {
	// 处理不同类型的节点
	switch node.Type() {
	case "class_declaration", "interface_declaration", "annotation_type_declaration":
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			className := nameNode.Content(code)
			id := fmt.Sprintf("%s:%s:%d", filePath, className, node.StartByte())

			// 调试：打印 class_declaration 的所有子节点
			fmt.Printf("=== 分析类: %s ===\n", className)
			fmt.Printf("类节点类型: %s, 子节点数: %d\n", node.Type(), node.ChildCount())
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child != nil {
					fmt.Printf("  子节点 %d: type=%s, content=%s\n",
						i, child.Type(), child.Content(code))
				}
			}

			// 创建类节点
			classNode := UniversalASTNode{
				ID:        id,
				Language:  "java",
				Type:      "Class",
				Name:      className,
				File:      filePath,
				Package:   packageName,
				StartLine: int(node.StartPoint().Row),
				EndLine:   int(node.EndPoint().Row),
				Fields:    make([]FieldInfo, 0),
				Metadata:  map[string]string{},
			}

			// 获取类体节点
			bodyNode := node.ChildByFieldName("body")
			if bodyNode != nil {
				// 收集类中的字段信息
				p.collectClassFields(bodyNode, code, &classNode)
			}

			// 获取所有父类和接口
			var superClassNames []string
			// 1. 直接用 ChildByFieldName 拿 extends 对应的那棵子树
			if extendsNode := node.ChildByFieldName("superclass"); extendsNode != nil {
				// 通常 extendsNode 下的命名子节点才是真正的类型标识，如 Foo 或 com.Bar
				for i := 0; i < int(extendsNode.NamedChildCount()); i++ {
					typeNode := extendsNode.NamedChild(i)
					superClassNames = append(superClassNames, p.extractTypeName(typeNode, code))
				}
			}

			// 2. 拿 implements 对应的接口列表，注意 field name 要写对
			if impls := node.ChildByFieldName("interfaces"); impls != nil {
				for i := 0; i < int(impls.NamedChildCount()); i++ {
					typeNode := impls.NamedChild(i)
					superClassNames = append(superClassNames, p.extractTypeName(typeNode, code))
				}
			}

			// 推断全限定名
			var superFullNames []string
			var superClassRefs []ClassRef
			for _, sc := range superClassNames {
				fq := ""
				if v, ok := importMap[sc]; ok {
					fq = v
				} else if len(importStar) > 0 {
					for _, pkg := range importStar {
						candidate := pkg + "." + sc
						fq = candidate
						break
					}
				}
				if fq == "" && packageName != "" {
					fq = packageName + "." + sc
				}
				if fq == "" {
					fq = sc // 兜底：直接用类名
				}
				superFullNames = append(superFullNames, fq)
				// 新增：填充 SuperClasses 字段
				pkg, name := "", fq
				if idx := strings.LastIndex(fq, "."); idx != -1 {
					pkg = fq[:idx]
					name = fq[idx+1:]
				}
				superClassRefs = append(superClassRefs, ClassRef{Package: pkg, Name: name})
			}
			classNode.Metadata["superClasses"] = strings.Join(superFullNames, ",")
			classNode.SuperClasses = superClassRefs

			// 打印调试信息
			fmt.Printf("Found class: %s.%s (Fields: %d)\n", packageName, className, len(classNode.Fields))
			for _, field := range classNode.Fields {
				fmt.Printf("  Field: %s (Type: %s, Lines: %d-%d)\n",
					field.Name, field.Type, field.StartLine, field.EndLine)
			}

			*nodes = append(*nodes, classNode)
		}
	case "method_declaration", "constructor_declaration", "annotation_type_element_declaration":
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			methodName := nameNode.Content(code)
			id := fmt.Sprintf("%s:%s:%d", filePath, methodName, node.StartByte())

			// 获取方法参数
			var methodParams []string
			parametersNode := node.ChildByFieldName("parameters")
			if parametersNode != nil {
				for i := 0; i < int(parametersNode.ChildCount()); i++ {
					paramNode := parametersNode.Child(i)
					if paramNode != nil && paramNode.Type() == "formal_parameter" {
						typeNode := paramNode.ChildByFieldName("type")
						if typeNode != nil {
							// 获取完整的参数类型，包括泛型信息
							paramType := typeNode.Content(code)
							// 处理泛型类型
							if strings.Contains(paramType, "?") {
								// 保留泛型信息
								methodParams = append(methodParams, paramType)
							} else {
								methodParams = append(methodParams, paramType)
							}
						}
					}
				}
			}

			// 获取返回类型
			returnType := ""
			if node.Type() == "annotation_type_element_declaration" {
				typeNode := node.ChildByFieldName("type")
				if typeNode != nil {
					returnType = typeNode.Content(code)
				}
			} else {
				returnTypeNode := node.ChildByFieldName("return_type")
				if returnTypeNode != nil {
					returnType = returnTypeNode.Content(code)
				}
			}

			// 打印调试信息
			fmt.Printf("Found method: %s.%s (Return: %s, Params: %v)\n",
				packageName, methodName, returnType, methodParams)

			*nodes = append(*nodes, UniversalASTNode{
				ID:           id,
				Language:     "java",
				Type:         "Method",
				Name:         methodName,
				File:         filePath,
				Package:      packageName,
				StartLine:    int(node.StartPoint().Row),
				EndLine:      int(node.EndPoint().Row),
				MethodParams: methodParams,
				Metadata: map[string]string{
					"returnType": returnType,
				},
			})
		}
	case "method_invocation":
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			methodName := nameNode.Content(code)
			id := fmt.Sprintf("%s:%s:%d", filePath, methodName, node.StartByte())
			*nodes = append(*nodes, UniversalASTNode{
				ID:        id,
				Language:  "java",
				Type:      "MethodCall",
				Name:      methodName,
				File:      filePath,
				Package:   packageName,
				StartLine: int(node.StartPoint().Row),
				EndLine:   int(node.EndPoint().Row),
			})
		}
	}

	// 递归遍历子节点
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			p.traverseNode(child, filePath, code, packageName, importMap, importStar, nodes)
		}
	}
}

// collectClassFields 收集类中的字段信息
func (p *JavaParser) collectClassFields(classNode *sitter.Node, code []byte, node *UniversalASTNode) {
	// 遍历类体的所有子节点
	for i := 0; i < int(classNode.ChildCount()); i++ {
		child := classNode.Child(i)
		if child == nil {
			continue
		}

		// 检查是否是字段声明或注解元素声明
		if child.Type() == "field_declaration" || child.Type() == "annotation_type_element_declaration" {
			// 获取字段修饰符
			modifiers := make([]string, 0)
			for j := 0; j < int(child.ChildCount()); j++ {
				modNode := child.Child(j)
				if modNode != nil && modNode.Type() == "modifiers" {
					for k := 0; k < int(modNode.ChildCount()); k++ {
						mod := modNode.Child(k)
						if mod != nil {
							modifiers = append(modifiers, mod.Content(code))
						}
					}
				}
			}

			// 获取字段类型
			typeNode := child.ChildByFieldName("type")
			fieldType := ""
			if typeNode != nil {
				fieldType = typeNode.Content(code)
			}

			// 处理字段声明（可能包含多个变量）
			for j := 0; j < int(child.ChildCount()); j++ {
				declNode := child.Child(j)
				if declNode != nil {
					var nameNode *sitter.Node

					// 根据节点类型获取名称节点
					if declNode.Type() == "variable_declarator" {
						nameNode = declNode.ChildByFieldName("name")
					} else if declNode.Type() == "annotation_type_element_declaration" {
						nameNode = declNode.ChildByFieldName("name")
					}

					if nameNode != nil {
						fieldName := nameNode.Content(code)

						// 获取字段的起始和结束行
						startLine := int(declNode.StartPoint().Row)
						endLine := int(declNode.EndPoint().Row)

						// 如果字段声明跨越多行，调整结束行
						if endLine < startLine {
							endLine = startLine
						}

						// 创建字段信息
						fieldInfo := FieldInfo{
							Name:      fieldName,
							Type:      fieldType,
							StartLine: startLine,
							EndLine:   endLine,
							Modifiers: modifiers,
							Metadata: map[string]string{
								"fullType": fieldType, // 保存完整的类型信息
							},
						}

						// 添加到类的字段列表中
						node.Fields = append(node.Fields, fieldInfo)

						// 打印调试信息
						fmt.Printf("Found field: %s (Type: %s, Lines: %d-%d, Modifiers: %v)\n",
							fieldName, fieldType, startLine, endLine, modifiers)
					}
				}
			}
		}
	}
}

// extractTypeName 提取类型名
func (p *JavaParser) extractTypeName(typeNode *sitter.Node, code []byte) string {
	if typeNode == nil {
		return ""
	}
	// 处理带包名的类型 (如 java.util.List)
	if typeNode.Type() == "scoped_identifier" {
		var parts []string
		for i := 0; i < int(typeNode.ChildCount()); i++ {
			child := typeNode.Child(i)
			if child != nil && child.Type() == "identifier" {
				parts = append(parts, child.Content(code))
			}
		}
		return strings.Join(parts, ".")
	}
	return typeNode.Content(code)
}

// 新增泛型参数提取方法
func (p *JavaParser) extractTypeArguments(argsNode *sitter.Node, code []byte) string {
	var typeArgs []string
	for i := 0; i < int(argsNode.ChildCount()); i++ {
		argNode := argsNode.Child(i)
		if argNode != nil && argNode.Type() == "type_argument" {
			typeArg := p.extractTypeName(argNode.Child(0), code)
			typeArgs = append(typeArgs, typeArg)
		}
	}
	return strings.Join(typeArgs, ", ")
}

// FillSubClasses 反向推断所有类节点的子类，并写入它们的 SubClasses 字段
func FillSubClasses(m *ParserManager) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. 构建全限定名 -> 节点ID 映射
	classMap := make(map[string]string)
	for id, node := range m.index.index {
		if node.Type == "Class" && node.FullClassName != "" {
			classMap[node.FullClassName] = id
		}
	}

	// 2. 遍历所有类节点，为其父类填充子类信息
	for _, node := range m.index.index {
		if node.Type != "Class" {
			continue
		}

		for _, superClassRef := range node.SuperClasses {
			superClassName := superClassRef.Package + "." + superClassRef.Name
			if parentID, ok := classMap[superClassName]; ok {
				// 获取父节点的指针以进行修改
				if parentNode, exists := m.index.index[parentID]; exists {
					// 避免重复添加
					found := false
					for _, subClass := range parentNode.SubClasses {
						if subClass.Name == node.Name && subClass.Package == node.Package {
							found = true
							break
						}
					}
					if !found {
						parentNode.SubClasses = append(parentNode.SubClasses, ClassRef{
							Package: node.Package,
							Name:    node.Name,
						})
						// 将修改后的节点写回map
						m.index.index[parentID] = parentNode
					}
				}
			}
		}
	}
}
