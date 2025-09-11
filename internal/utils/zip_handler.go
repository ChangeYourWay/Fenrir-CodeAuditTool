package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ZipHandler 处理ZIP文件解压
type ZipHandler struct {
	tempDir string
}

// NewZipHandler 创建ZIP处理器
func NewZipHandler(tempDir string) *ZipHandler {
	return &ZipHandler{
		tempDir: tempDir,
	}
}

// ExtractZip 解压ZIP文件（跨平台兼容）
func (zh *ZipHandler) ExtractZip(zipFile, targetDir string) (string, error) {
	// 创建目标目录
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %v", err)
	}

	// 打开ZIP文件
	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return "", fmt.Errorf("打开ZIP文件失败: %v", err)
	}
	defer reader.Close()

	// 解压所有文件
	for _, file := range reader.File {
		err := zh.extractFile(file, targetDir)
		if err != nil {
			return "", err
		}
	}

	// 查找可能的项目根目录
	projectRoot := zh.findProjectRoot(targetDir)
	return projectRoot, nil
}

// extractFile 解压单个文件（处理跨平台路径问题）
func (zh *ZipHandler) extractFile(file *zip.File, targetDir string) error {
	// 安全处理文件名，防止路径遍历攻击
	safePath := filepath.Join(targetDir, zh.sanitizeFileName(file.Name))

	// 创建目录（如果是目录条目）
	if file.FileInfo().IsDir() {
		return os.MkdirAll(safePath, 0755)
	}

	// 确保父目录存在
	if err := os.MkdirAll(filepath.Dir(safePath), 0755); err != nil {
		return err
	}

	// 创建目标文件
	outFile, err := os.OpenFile(safePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	// 打开ZIP中的文件
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// 复制文件内容
	_, err = io.Copy(outFile, rc)
	return err
}

// sanitizeFileName 安全处理文件名，防止路径遍历
func (zh *ZipHandler) sanitizeFileName(name string) string {
	// 处理Windows和Unix路径分隔符
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimPrefix(name, "../")

	// 清理路径
	return filepath.Clean(name)
}

// findProjectRoot 查找项目根目录（智能识别）
func (zh *ZipHandler) findProjectRoot(baseDir string) string {
	// 常见项目根目录标识文件
	indicators := []string{
		"pom.xml",       // Maven
		"build.gradle",  // Gradle
		"package.json",  // Node.js
		"setup.py",      // Python
		"go.mod",        // Go
		"src/main/java", // Java源码目录
		"src/main/go",   // Go源码目录
	}

	// 检查当前目录
	for _, indicator := range indicators {
		if zh.fileExists(filepath.Join(baseDir, indicator)) {
			return baseDir
		}
	}

	// 检查子目录
	entries, err := os.ReadDir(baseDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				subDir := filepath.Join(baseDir, entry.Name())
				for _, indicator := range indicators {
					if zh.fileExists(filepath.Join(subDir, indicator)) {
						return subDir
					}
				}
			}
		}
	}

	return baseDir // 默认返回基础目录
}

func (zh *ZipHandler) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetPlatformTempDir 获取平台相关的临时目录
func GetPlatformTempDir() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("TEMP")
	}
	return "/tmp"
}

// Cleanup 清理临时文件
func (zh *ZipHandler) Cleanup(path string) error {
	return os.RemoveAll(path)
}
