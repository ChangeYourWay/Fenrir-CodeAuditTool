# 项目目录结构

## 概述

本项目采用清晰的目录结构，将不同类型的文件分别组织到相应的目录中，便于维护和管理。

## 目录结构

```
Fenrir-CodeAuditTool/
├── application/                 # 应用程序
│   ├── client/                 # 客户端程序
│   │   └── main.go
│   └── server/                 # 服务器程序
│       └── main.go
├── configs/                    # Go配置代码
│   └── config.go              # 配置加载和管理逻辑
├── resources/                  # 配置文件资源
│   └── config.yaml            # 应用配置文件
├── internal/                   # 内部包
│   ├── deepseek/              # DeepSeek相关
│   │   └── deepseekConfig.go
│   └── utils/                 # 工具类
│       ├── ast_common.go      # AST通用结构
│       ├── ast_builder.go     # AST构建服务
│       ├── ast_persistence.go # AST持久化管理
│       ├── class_utils.go     # 类工具
│       ├── go_parser.go       # Go语言解析器
│       ├── java_parser.go     # Java语言解析器
│       ├── method_utils.go    # 方法工具
│       ├── parser_manager.go  # 解析器管理器
│       ├── query_interface.go # 查询接口
│       └── search_handler.go  # 搜索处理器
├── tools/                      # 工具程序
│   └── cache_manager.go       # 缓存管理工具
├── test/                       # 测试程序
│   ├── main.go                # 测试主程序
│   └── ast_builder_test.go    # AST构建测试
├── prompts/                    # 提示词
│   └── test.txt
├── cache/                      # 缓存目录（运行时生成）
│   └── *.json                 # AST缓存文件
├── go.mod                      # Go模块文件
├── go.sum                      # Go依赖校验文件
├── fenrir-server.exe          # 编译后的服务器程序
├── README.md                   # 项目说明
├── AST_CACHE_README.md         # AST缓存功能说明
├── DIRECTORY_STRUCTURE.md      # 目录结构说明（本文件）
└── .gitignore                  # Git忽略文件
```

## 目录说明

### application/
应用程序的主要代码，包含客户端和服务器端程序。

### configs/
Go语言的配置管理代码，负责配置文件的加载、解析和管理。

### resources/
静态资源文件，主要是配置文件（YAML、JSON等）。

### internal/
内部包，不对外暴露的代码。
- `deepseek/`: DeepSeek API相关代码
- `utils/`: 工具类，包含AST解析、查询、缓存等功能

### tools/
独立的工具程序，如缓存管理工具。

### test/
测试程序和测试用例。

### prompts/
提示词文件，用于AI模型。

### cache/
运行时生成的缓存文件目录，包含AST索引缓存。

## 文件命名规范

### 配置文件
- Go配置代码：`configs/config.go`
- YAML配置文件：`resources/config.yaml`

### 缓存文件
- 格式：`{仓库名}_{构建时间}_ast_index.json`
- 示例：`apache-sling-cms-1.1.8_20231201_143022_ast_index.json`

### 工具程序
- 缓存管理：`tools/cache_manager.go`
- 测试程序：`test/ast_builder_test.go`

## 配置加载

### 默认配置
```go
config, err := configs.LoadDefaultConfig()
```

### 指定配置文件
```go
config, err := configs.LoadConfig("path/to/config.yaml")
```

## 缓存管理

### 列出缓存文件
```bash
go run tools/cache_manager.go -list
```

### 清除缓存
```bash
go run tools/cache_manager.go -clear
```

### 清理旧缓存
```bash
go run tools/cache_manager.go -cleanup
```

## 开发建议

1. **配置管理**: 将Go配置代码放在 `configs/` 目录，将配置文件放在 `resources/` 目录
2. **工具程序**: 独立的工具程序放在 `tools/` 目录
3. **测试代码**: 测试程序放在 `test/` 目录
4. **缓存文件**: 运行时生成的文件放在 `cache/` 目录
5. **文档**: 项目文档放在根目录，便于查找

## 注意事项

1. `internal/` 目录下的代码仅供内部使用，不应被外部包导入
2. `resources/` 目录下的配置文件会被打包到程序中
3. `cache/` 目录会在运行时自动创建
4. 工具程序可以独立编译和运行 