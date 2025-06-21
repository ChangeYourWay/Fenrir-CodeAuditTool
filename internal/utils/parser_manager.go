package utils

import (
	"os"
	"path/filepath"
	"sync"
)

// ParserManager 管理多种语言的解析器
type ParserManager struct {
	parsers map[string]ASTParser
	index   *ASTIndex
	mu      sync.Mutex
}

// NewParserManager 创建解析器管理器
func NewParserManager() *ParserManager {
	return &ParserManager{
		parsers: make(map[string]ASTParser),
		index:   NewASTIndex(),
	}
}

// RegisterParser 注册解析器
func (m *ParserManager) RegisterParser(parser ASTParser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.parsers[parser.Language()] = parser
}

// BuildIndexFromDir 从目录构建索引
func (m *ParserManager) BuildIndexFromDir(root string) error {
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// 根据文件扩展名确定语言
		ext := filepath.Ext(path)
		language := ""
		switch ext {
		case ".go":
			language = "go"
		case ".java":
			language = "java"
		case ".py":
			language = "python"
		default:
			return nil
		}

		// 获取对应的解析器
		parser, exists := m.parsers[language]
		if !exists {
			return nil
		}

		// 解析文件
		nodes, err := parser.ParseFile(path)
		if err != nil {
			return err
		}

		// 添加到索引
		m.mu.Lock()
		for _, node := range nodes {
			m.index.AddNode(node)
		}
		m.mu.Unlock()

		return nil
	})
	if err != nil {
		return err
	}

	// ====== 遍历完所有文件后，再填充子类关系 ======
	FillSubClasses(m)
	return nil
}

// GetIndex 获取索引
func (m *ParserManager) GetIndex() *ASTIndex {
	return m.index
}
