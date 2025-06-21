package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"Fenrir-CodeAuditTool/configs"
)

// ASTPersistenceManager AST持久化管理器
type ASTPersistenceManager struct {
	config *configs.Config
}

// NewASTPersistenceManager 创建AST持久化管理器
func NewASTPersistenceManager(config *configs.Config) *ASTPersistenceManager {
	return &ASTPersistenceManager{
		config: config,
	}
}

// SaveASTIndex 保存AST索引到文件
func (pm *ASTPersistenceManager) SaveASTIndex(index *ASTIndex) error {
	cacheFilePath := pm.config.GetCacheFilePath()

	// 确保缓存目录存在
	cacheDir := filepath.Dir(cacheFilePath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("创建缓存目录失败: %v", err)
	}

	// 将索引转换为可序列化的格式
	data := make(map[string]UniversalASTNode)
	for id, node := range index.index {
		data[id] = node
	}

	// 添加元数据
	metadata := map[string]interface{}{
		"repository_path": pm.config.CodeAudit.RepositoryPath,
		"build_time":      time.Now().Format(time.RFC3339),
		"node_count":      len(data),
		"cache_version":   "1.0",
	}

	// 创建完整的缓存数据结构
	cacheData := map[string]interface{}{
		"metadata": metadata,
		"nodes":    data,
	}

	// 序列化为JSON
	jsonData, err := json.MarshalIndent(cacheData, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化AST索引失败: %v", err)
	}

	// 写入文件
	err = os.WriteFile(cacheFilePath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("写入AST缓存文件失败: %v", err)
	}

	fmt.Printf("AST索引已保存到: %s\n", cacheFilePath)
	return nil
}

// LoadASTIndex 从文件加载AST索引
func (pm *ASTPersistenceManager) LoadASTIndex() (*ASTIndex, error) {
	// 获取最新的缓存文件
	cacheFilePath, err := pm.config.GetLatestCacheFile()
	if err != nil {
		return nil, fmt.Errorf("查找缓存文件失败: %v", err)
	}

	if cacheFilePath == "" {
		return nil, fmt.Errorf("未找到匹配的AST缓存文件")
	}

	// 检查缓存文件是否存在
	if _, err := os.Stat(cacheFilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("AST缓存文件不存在: %s", cacheFilePath)
	}

	// 读取文件
	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return nil, fmt.Errorf("读取AST缓存文件失败: %v", err)
	}

	// 反序列化JSON
	var cacheData map[string]interface{}
	err = json.Unmarshal(data, &cacheData)
	if err != nil {
		return nil, fmt.Errorf("反序列化AST索引失败: %v", err)
	}

	// 提取元数据
	if metadata, exists := cacheData["metadata"]; exists {
		if metaMap, ok := metadata.(map[string]interface{}); ok {
			if buildTime, exists := metaMap["build_time"]; exists {
				fmt.Printf("加载缓存文件: %s (构建时间: %s)\n", cacheFilePath, buildTime)
			}
		}
	}

	// 提取节点数据
	nodesData, exists := cacheData["nodes"]
	if !exists {
		return nil, fmt.Errorf("缓存文件中缺少节点数据")
	}

	// 将节点数据转换为map
	nodesJSON, err := json.Marshal(nodesData)
	if err != nil {
		return nil, fmt.Errorf("处理节点数据失败: %v", err)
	}

	var nodeMap map[string]UniversalASTNode
	err = json.Unmarshal(nodesJSON, &nodeMap)
	if err != nil {
		return nil, fmt.Errorf("反序列化节点数据失败: %v", err)
	}

	// 创建新的索引并填充数据
	index := NewASTIndex()
	for id, node := range nodeMap {
		index.index[id] = node
	}

	return index, nil
}

// CacheExists 检查缓存文件是否存在
func (pm *ASTPersistenceManager) CacheExists() bool {
	cacheFilePath, err := pm.config.GetLatestCacheFile()
	if err != nil {
		return false
	}

	if cacheFilePath == "" {
		return false
	}

	_, err = os.Stat(cacheFilePath)
	return err == nil
}

// ClearCache 清除缓存文件
func (pm *ASTPersistenceManager) ClearCache() error {
	cacheDir := pm.config.CodeAudit.ASTCache.CacheDir
	if cacheDir == "" {
		cacheDir = "./cache"
	}

	// 获取仓库名称，并将空格替换为下划线
	repoName := filepath.Base(pm.config.CodeAudit.RepositoryPath)
	repoName = replaceSpaceWithUnderscore(repoName)

	// 匹配新规则下的缓存文件
	pattern := filepath.Join(cacheDir, repoName+"_ast_index.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	// 删除所有匹配的缓存文件（理论上只会有一个）
	for _, file := range matches {
		err := os.Remove(file)
		if err != nil {
			fmt.Printf("删除缓存文件失败: %s, 错误: %v\n", file, err)
		} else {
			fmt.Printf("已删除缓存文件: %s\n", file)
		}
	}

	return nil
}

// ListCacheFiles 列出所有缓存文件
func (pm *ASTPersistenceManager) ListCacheFiles() ([]string, error) {
	cacheDir := pm.config.CodeAudit.ASTCache.CacheDir
	if cacheDir == "" {
		cacheDir = "./cache"
	}

	// 获取仓库名称，并将空格替换为下划线
	repoName := filepath.Base(pm.config.CodeAudit.RepositoryPath)
	repoName = replaceSpaceWithUnderscore(repoName)

	// 匹配新规则下的缓存文件
	pattern := filepath.Join(cacheDir, repoName+"_ast_index.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	return matches, nil
}

// replaceSpaceWithUnderscore 将字符串中的空格替换为下划线
func replaceSpaceWithUnderscore(s string) string {
	return strings.ReplaceAll(s, " ", "_")
}
