package utils

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ProjectDetector 项目目录检测器
type ProjectDetector struct {
	projectsDir string
}

// NewProjectDetector 创建项目检测器
func NewProjectDetector(projectsDir string) *ProjectDetector {
	return &ProjectDetector{
		projectsDir: projectsDir,
	}
}

// AutoDetectProject 自动检测最新的项目目录
func (pd *ProjectDetector) AutoDetectProject() string {
	// 确保项目目录存在
	if err := os.MkdirAll(pd.projectsDir, 0755); err != nil {
		return ""
	}

	entries, err := os.ReadDir(pd.projectsDir)
	if err != nil {
		return ""
	}

	// 收集所有项目目录及其修改时间
	type projectInfo struct {
		path    string
		modTime time.Time
	}

	var projects []projectInfo

	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			projectPath := filepath.Join(pd.projectsDir, entry.Name())

			// 获取目录修改时间
			info, err := entry.Info()
			if err != nil {
				continue
			}

			// 检查是否是有效的项目目录（包含代码文件）
			if pd.isValidProject(projectPath) {
				projects = append(projects, projectInfo{
					path:    projectPath,
					modTime: info.ModTime(),
				})
			}
		}
	}

	if len(projects) == 0 {
		return ""
	}

	// 按修改时间排序（最新的在前）
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].modTime.After(projects[j].modTime)
	})

	return projects[0].path
}

// isValidProject 检查是否是有效的项目目录
func (pd *ProjectDetector) isValidProject(projectPath string) bool {
	// 常见项目标识文件
	projectIndicators := []string{
		"pom.xml",          // Maven
		"build.gradle",     // Gradle
		"package.json",     // Node.js
		"setup.py",         // Python
		"go.mod",           // Go
		"requirements.txt", // Python
		"src/",             // 源码目录
		"main/",            // 主目录
		"java/",            // Java代码
		"py/",              // Python代码
	}

	// 检查是否存在项目标识文件或目录
	for _, indicator := range projectIndicators {
		indicatorPath := filepath.Join(projectPath, indicator)
		if _, err := os.Stat(indicatorPath); err == nil {
			return true
		}
	}

	// 检查是否包含源代码文件
	codeExtensions := []string{".java", ".go", ".py", ".js", ".ts", ".c", ".cpp", ".h"}
	if pd.containsCodeFiles(projectPath, codeExtensions) {
		return true
	}

	return false
}

// containsCodeFiles 检查目录是否包含代码文件
func (pd *ProjectDetector) containsCodeFiles(dirPath string, extensions []string) bool {
	var found bool
	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || found {
			return nil
		}

		if !info.IsDir() {
			ext := filepath.Ext(path)
			for _, targetExt := range extensions {
				if ext == targetExt {
					found = true
					return filepath.SkipDir
				}
			}
		}
		return nil
	})

	return found
}

// CleanupOldProjects 清理旧项目
func (pd *ProjectDetector) CleanupOldProjects(maxAge time.Duration) {
	entries, err := os.ReadDir(pd.projectsDir)
	if err != nil {
		return
	}

	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			if now.Sub(info.ModTime()) > maxAge {
				oldProjectPath := filepath.Join(pd.projectsDir, entry.Name())
				os.RemoveAll(oldProjectPath)
			}
		}
	}
}
