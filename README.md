# Fenrir 代码审计工具

Fenrir 是一个强大的代码审计工具，专注于代码结构分析和安全漏洞检测。它支持多种编程语言，提供统一的代码结构表示和查询接口。

## 功能特性

### 1. 多语言支持
- Java
- Go
- Python
- JavaScript/TypeScript
- 更多语言支持正在开发中...

### 2. 代码结构分析
- 类、接口、方法、字段的提取
- 包/模块结构分析
- 方法调用关系分析
- 继承关系分析

### 3. 统一查询接口
- 基于类型的查询
- 基于名称的查询
- 基于位置的查询
- 代码片段获取
- 调用图可视化

### 4. 安全分析
- 漏洞模式识别
- 安全规则检查
- 依赖分析
- 敏感数据流追踪

## 安装说明

### 系统要求
- Go 1.16 或更高版本
- 支持的操作系统：Windows、Linux、macOS

### 安装步骤

1. 克隆仓库
```bash
git clone https://github.com/yourusername/Fenrir-CodeAuditTool.git
cd Fenrir-CodeAuditTool
```

2. 安装依赖
```bash
go mod download
```

3. 编译项目
```bash
go build -o fenrir
```

## 使用方法

### 1. 基本使用

```go
// 创建解析器管理器
manager := NewParserManager()

// 构建代码索引
err := manager.BuildIndexFromDir("./your/project/path")
if err != nil {
    log.Fatal(err)
}

// 创建查询引擎
engine := NewQueryEngine(manager.GetIndex())

// 查询示例
nodes := engine.FindByType("Method")
```

### 2. 代码结构查询

```go
// 查找特定类型的方法
methods := engine.FindByType("Method")

// 查找特定名称的类
classes := engine.FindByName("UserService")

// 获取代码片段
snippet, err := engine.GetCodeSnippet(node, 5) // 获取前后5行上下文
```

### 3. 调用图分析

```go
// 生成调用图
graph := engine.GenerateCallGraph("com.example.UserService")

// 导出为 DOT 格式
err := graph.ExportDOT("callgraph.dot")
```

## 项目结构

```
Fenrir-CodeAuditTool/
├── cmd/                    # 命令行工具
├── internal/               # 内部包
│   ├── ast/               # AST 相关
│   ├── parser/            # 语言解析器
│   ├── query/             # 查询引擎
│   └── utils/             # 工具函数
├── pkg/                   # 公共包
├── test/                  # 测试文件
└── docs/                  # 文档
```

## 贡献指南

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

## 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

## 联系方式

- 项目维护者：[Your Name]
- 邮箱：[your.email@example.com]
- 项目链接：[https://github.com/yourusername/Fenrir-CodeAuditTool]

## 致谢

- [Tree-sitter](https://tree-sitter.github.io/tree-sitter/) - 用于代码解析
- [Go AST](https://golang.org/pkg/go/ast/) - Go 语言 AST 支持
- [Java Parser](https://github.com/smacker/go-tree-sitter-java) - Java 语言解析支持 