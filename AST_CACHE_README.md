# AST缓存功能说明

## 概述

本次改进实现了AST语法树的持久化存储和配置化管理，解决了每次启动服务器都需要重新构建AST的问题。新增了基于代码仓库名称和构建时间的智能缓存文件命名系统。

## 主要改进

### 1. 配置文件管理
- 代码仓库路径现在通过 `resources/config.yaml` 配置文件管理
- 支持AST缓存相关配置
- 配置文件结构清晰，Go配置代码在 `configs` 目录，YAML配置文件在 `resources` 目录

### 2. AST持久化
- AST语法树可以保存到本地文件
- 服务器启动时可以选择从缓存加载或重新构建
- 支持缓存文件的自动创建和管理

### 3. 智能缓存文件命名
- 缓存文件名格式：`{仓库名}_{构建时间}_ast_index.json`
- 示例：`apache-sling-cms-1.1.8_20231201_143022_ast_index.json`
- 自动查找最新的缓存文件进行加载
- 支持多个缓存文件的管理

### 4. 代码解耦
- AST构建逻辑与服务器启动代码完全解耦
- 创建了独立的AST构建服务

## 配置文件说明

```yaml
deepseek:
  api_key: "your-api-key"

# 代码审计配置
code_audit:
  # 代码仓库路径
  repository_path: "D:\\CodeAudit\\Apache Sling CMS 1.1.8\\apache-sling-cms-1.1.8"
  
  # AST缓存配置
  ast_cache:
    # 是否启用AST缓存
    enabled: true
    # AST缓存目录
    cache_dir: "./cache"
    # 是否在启动时重新构建AST（当enabled为true时，此选项生效）
    rebuild_on_startup: false
```

### 配置参数说明

- `repository_path`: 指定要分析的代码仓库路径
- `enabled`: 是否启用AST缓存功能
- `cache_dir`: AST缓存文件的存储目录
- `rebuild_on_startup`: 是否在每次启动时重新构建AST

## 缓存文件命名规则

### 文件命名格式
```
{仓库名}_{构建时间}_ast_index.json
```

### 示例
- `apache-sling-cms-1.1.8_20231201_143022_ast_index.json`
- `spring-boot_20231201_150530_ast_index.json`
- `my-project_20231202_091245_ast_index.json`

### 时间格式
- 格式：`YYYYMMDD_HHMMSS`
- 示例：`20231201_143022` 表示 2023年12月1日 14:30:22

## 使用场景

### 场景1：首次运行
- 设置 `enabled: true`, `rebuild_on_startup: false`
- 系统会自动构建AST并保存到缓存文件
- 后续启动时会从缓存文件加载，大幅提升启动速度

### 场景2：代码更新后
- 设置 `enabled: true`, `rebuild_on_startup: true`
- 系统会重新构建AST并更新缓存文件
- 构建完成后可以改回 `rebuild_on_startup: false`

### 场景3：禁用缓存
- 设置 `enabled: false`
- 系统每次启动都会重新构建AST
- 适用于开发调试阶段

### 场景4：多项目管理
- 不同项目会自动生成不同的缓存文件
- 系统会根据仓库名称自动匹配对应的缓存文件
- 支持同时管理多个项目的AST缓存

## 缓存管理工具

新增了专门的缓存管理工具 `tools/cache_manager.go`，提供以下功能：

### 功能列表
- **列出缓存文件**: `cache_manager -list`
- **清除所有缓存**: `cache_manager -clear`
- **显示缓存信息**: `cache_manager -info`
- **清理旧缓存**: `cache_manager -cleanup`

### 使用示例
```bash
# 列出所有缓存文件
go run tools/cache_manager.go -list

# 清除所有缓存文件
go run tools/cache_manager.go -clear

# 清理旧缓存文件（保留最新的3个）
go run tools/cache_manager.go -cleanup

# 显示缓存信息
go run tools/cache_manager.go -info
```

## 文件结构

```
Fenrir-CodeAuditTool/
├── configs/                    # Go配置代码
│   └── config.go              # 配置加载和管理
├── resources/                  # 配置文件资源
│   └── config.yaml            # 应用配置文件
├── internal/utils/             # 工具类
│   ├── ast_persistence.go     # AST持久化管理
│   ├── ast_builder.go         # AST构建服务
│   └── ...
├── tools/                      # 工具程序
│   └── cache_manager.go       # 缓存管理工具
├── test/                       # 测试程序
│   └── ast_builder_test.go    # AST构建测试
└── application/server/         # 服务器程序
    └── main.go                # 服务器主程序
```

## 新增文件说明

### 1. `configs/config.go`
- 配置文件加载和管理
- 支持YAML格式的配置文件解析
- 智能缓存文件命名生成
- 最新缓存文件查找
- 默认从 `resources/config.yaml` 加载配置

### 2. `resources/config.yaml`
- 应用配置文件
- 支持缓存目录配置
- 代码仓库路径配置

### 3. `internal/utils/ast_persistence.go`
- AST索引的序列化和反序列化
- 缓存文件的读写操作
- 支持元数据存储（构建时间、节点数量等）
- 智能缓存文件管理

### 4. `internal/utils/ast_builder.go`
- AST构建服务的核心逻辑
- 支持构建或加载AST的选择
- 统计信息的生成和打印
- 缓存信息展示

### 5. `tools/cache_manager.go`
- 缓存文件管理工具
- 支持查看、清理、管理缓存文件
- 提供友好的命令行界面

### 6. `test/ast_builder_test.go`
- AST构建服务的测试程序
- 可以独立运行验证功能

## 缓存文件结构

缓存文件采用JSON格式，包含以下结构：

```json
{
  "metadata": {
    "repository_path": "D:\\CodeAudit\\Apache Sling CMS 1.1.8\\apache-sling-cms-1.1.8",
    "build_time": "2023-12-01T14:30:22Z",
    "node_count": 12345,
    "cache_version": "1.0"
  },
  "nodes": {
    "node_id_1": {
      "id": "node_id_1",
      "language": "java",
      "type": "Class",
      "name": "MyClass",
      "file": "src/main/java/com/example/MyClass.java",
      "package": "com.example",
      "startLine": 10,
      "endLine": 50,
      // ... 其他节点信息
    }
    // ... 更多节点
  }
}
```

## 性能提升

- **首次启动**: 需要构建AST，时间较长
- **后续启动**: 从缓存加载，启动时间大幅缩短
- **缓存文件大小**: 根据代码库大小而定，通常几MB到几十MB
- **多项目支持**: 不同项目独立缓存，互不干扰

## 注意事项

1. 缓存文件会占用磁盘空间，请确保有足够的存储空间
2. 代码更新后需要重新构建AST才能反映最新变化
3. 缓存文件格式为JSON，包含元数据信息
4. 如果缓存文件损坏，系统会自动重新构建AST
5. 支持多个缓存文件共存，系统会自动选择最新的
6. 可以使用缓存管理工具清理不需要的缓存文件

## 测试方法

运行测试程序验证功能：

```bash
cd test
go run ast_builder_test.go
```

或者直接启动服务器：

```bash
cd application/server
go run main.go
```

使用缓存管理工具：

```bash
cd tools
go run cache_manager.go -list
```

## 配置加载方式

### 默认配置加载
```go
config, err := configs.LoadDefaultConfig()
```

### 指定配置文件路径
```go
config, err := configs.LoadConfig("path/to/config.yaml")
```

### 配置文件优先级
1. 如果指定了配置文件路径，使用指定路径
2. 如果未指定路径，默认使用 `resources/config.yaml`
3. 如果配置文件不存在，返回错误 