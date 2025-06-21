package utils

import "strings"

// ClassRef 表示类的引用（包名+类名）
type ClassRef struct {
	Package string `json:"package"`
	Name    string `json:"name"`
}

// Relation 表示节点间的关系
type Relation struct {
	TargetID string `json:"target_id"` // 目标节点ID
	Type     string `json:"type"`      // 关系类型 (calls, references, etc.)
}

// UniversalASTNode 表示通用的 AST 节点
type UniversalASTNode struct {
	ID        string            `json:"id"`        // 全局唯一标识符
	Language  string            `json:"language"`  // 语言类型 (java, go, etc.)
	Type      string            `json:"type"`      // 节点类型 (FuncDecl, Class, etc.)
	Name      string            `json:"name"`      // 节点名称
	File      string            `json:"file"`      // 文件路径
	Package   string            `json:"package"`   // 包名/命名空间
	StartLine int               `json:"startLine"` // 起始行号
	EndLine   int               `json:"endLine"`   // 结束行号
	Metadata  map[string]string `json:"metadata"`  // 语言特定元数据
	Relations []Relation        `json:"relations"` // 节点关系

	// 新增字段
	FullClassName string   `json:"fullClassName"` // 全限定类名（包名+类名）
	MethodParams  []string `json:"methodParams"`  // 方法参数类型列表
	IsInnerClass  bool     `json:"isInnerClass"`  // 是否是内部类
	OuterClass    string   `json:"outerClass"`    // 外部类名（内部类独有的属性）

	// 新增：类节点的字段信息
	Fields []FieldInfo `json:"fields"` // 类中定义的字段列表

	// 新增：类节点的父类和子类
	SuperClasses []ClassRef `json:"superClasses"` // 所有父类
	SubClasses   []ClassRef `json:"subClasses"`   // 所有子类
}

// FieldInfo 表示类中的字段信息
type FieldInfo struct {
	Name      string            `json:"name"`      // 字段名
	Type      string            `json:"type"`      // 字段类型
	StartLine int               `json:"startLine"` // 字段起始行
	EndLine   int               `json:"endLine"`   // 字段结束行
	Modifiers []string          `json:"modifiers"` // 字段修饰符（public, private, static等）
	Metadata  map[string]string `json:"metadata"`  // 字段的其他元数据
}

// ASTParser 通用 AST 解析器接口
type ASTParser interface {
	ParseFile(filePath string) ([]UniversalASTNode, error)
	Language() string
}

// ASTIndex 统一索引结构
type ASTIndex struct {
	index map[string]UniversalASTNode // ID -> Node
}

// NewASTIndex 创建新索引
func NewASTIndex() *ASTIndex {
	return &ASTIndex{
		index: make(map[string]UniversalASTNode),
	}
}

// AddNode 添加节点到索引
func (i *ASTIndex) AddNode(node UniversalASTNode) {
	// 自动生成全限定类名
	if node.Type == "Class" {
		if node.Package != "" {
			node.FullClassName = node.Package + "." + node.Name
		} else {
			node.FullClassName = node.Name
		}
	}

	// 处理内部类标识
	if strings.Contains(node.Name, "$") {
		node.IsInnerClass = true
		parts := strings.Split(node.Name, "$")
		node.OuterClass = parts[0]
		node.Name = parts[len(parts)-1]
	}

	i.index[node.ID] = node
}

// GetNode 获取节点
func (i *ASTIndex) GetNode(id string) (UniversalASTNode, bool) {
	node, exists := i.index[id]
	return node, exists
}

// FindNodes 查找节点
func (i *ASTIndex) FindNodes(filter func(UniversalASTNode) bool) []UniversalASTNode {
	var results []UniversalASTNode
	for _, node := range i.index {
		if filter(node) {
			results = append(results, node)
		}
	}
	return results
}
