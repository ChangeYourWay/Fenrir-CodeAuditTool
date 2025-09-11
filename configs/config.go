package configs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 应用配置结构
type Config struct {
	Deepseek struct {
		APIKey string `yaml:"api_key"`
	} `yaml:"deepseek"`

	CodeAudit struct {
		RepositoryPath string `yaml:"repository_path"`
		ASTCache       struct {
			Enabled          bool   `yaml:"enabled"`
			CacheDir         string `yaml:"cache_dir"`
			RebuildOnStartup bool   `yaml:"rebuild_on_startup"`
		} `yaml:"ast_cache"`
	} `yaml:"code_audit"`

	// 新增远程仓库配置
	RemoteRepository struct {
		Enabled    bool   `yaml:"enabled"`
		Type       string `yaml:"type"` // "zip", "git", "local"
		URL        string `yaml:"url"`
		Branch     string `yaml:"branch"`
		TargetPath string `yaml:"target_path"`
		AutoClean  bool   `yaml:"auto_clean"`
	} `yaml:"remote_repository"`
}

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	// 如果没有指定配置文件路径，使用默认路径
	if configPath == "" {
		configPath = "resources/config.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// LoadDefaultConfig 加载默认配置文件
func LoadDefaultConfig() (*Config, error) {
	return LoadConfig("resources/config.yaml")
}

// GenerateCacheFileName 生成缓存文件名
func (c *Config) GenerateCacheFileName() string {
	// 获取仓库名称，并将空格替换为下划线
	repoName := filepath.Base(c.CodeAudit.RepositoryPath)
	repoName = replaceSpaceWithUnderscore(repoName)
	// 生成缓存文件名（无时间信息）
	cacheFileName := fmt.Sprintf("%s_ast_index.json", repoName)
	return cacheFileName
}

// replaceSpaceWithUnderscore 将字符串中的空格替换为下划线
func replaceSpaceWithUnderscore(s string) string {
	return strings.ReplaceAll(s, " ", "_")
}

// GetCacheFilePath 获取完整的缓存文件路径
func (c *Config) GetCacheFilePath() string {
	cacheDir := c.CodeAudit.ASTCache.CacheDir
	if cacheDir == "" {
		cacheDir = "./cache"
	}
	cacheFileName := c.GenerateCacheFileName()
	return filepath.Join(cacheDir, cacheFileName)
}

// GetLatestCacheFile 获取最新的缓存文件路径（现在只会有一个）
func (c *Config) GetLatestCacheFile() (string, error) {
	cacheDir := c.CodeAudit.ASTCache.CacheDir
	if cacheDir == "" {
		cacheDir = "./cache"
	}
	repoName := filepath.Base(c.CodeAudit.RepositoryPath)
	repoName = replaceSpaceWithUnderscore(repoName)
	pattern := filepath.Join(cacheDir, repoName+"_ast_index.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", nil // 没有找到缓存文件
	}
	return matches[0], nil // 只会有一个
}
