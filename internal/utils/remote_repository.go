package utils

import (
	"Fenrir-CodeAuditTool/configs"
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RemoteRepositoryManager 远程仓库管理器
type RemoteRepositoryManager struct {
	config *configs.Config
}

// NewRemoteRepositoryManager 创建远程仓库管理器
func NewRemoteRepositoryManager(config *configs.Config) *RemoteRepositoryManager {
	return &RemoteRepositoryManager{
		config: config,
	}
}

// DownloadAndPrepare 下载并准备远程仓库
func (m *RemoteRepositoryManager) DownloadAndPrepare() (string, error) {
	repoConfig := m.config.RemoteRepository

	switch repoConfig.Type {
	case "zip":
		return m.downloadAndExtractZip(repoConfig.URL, repoConfig.TargetPath)
	case "git":
		return m.cloneGitRepository(repoConfig.URL, repoConfig.Branch, repoConfig.TargetPath)
	case "local":
		return repoConfig.TargetPath, nil
	default:
		return "", fmt.Errorf("不支持的仓库类型: %s", repoConfig.Type)
	}
}

// downloadAndExtractZip 下载并解压ZIP文件
func (m *RemoteRepositoryManager) downloadAndExtractZip(url, targetPath string) (string, error) {
	// 清理目标目录
	if err := os.RemoveAll(targetPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("清理目录失败: %v", err)
	}

	// 创建临时目录
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %v", err)
	}

	// 下载文件
	zipPath := filepath.Join(targetPath, "download.zip")
	if err := m.downloadFile(url, zipPath); err != nil {
		return "", err
	}

	// 解压文件
	if err := m.extractZip(zipPath, targetPath); err != nil {
		return "", err
	}

	// 删除zip文件
	os.Remove(zipPath)

	return targetPath, nil
}

// downloadFile 下载文件
func (m *RemoteRepositoryManager) downloadFile(url, filepath string) error {
	fmt.Printf("开始下载: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("下载失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	fmt.Printf("下载完成: %s\n", filepath)
	return nil
}

// extractZip 解压ZIP文件
func (m *RemoteRepositoryManager) extractZip(zipPath, targetPath string) error {
	fmt.Printf("开始解压: %s\n", zipPath)

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开zip文件失败: %v", err)
	}
	defer r.Close()

	for _, f := range r.File {
		// 防止路径遍历攻击
		fpath := filepath.Join(targetPath, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(targetPath)+string(os.PathSeparator)) {
			return fmt.Errorf("非法文件路径: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, 0755); err != nil {
				return fmt.Errorf("创建目录失败: %v", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return fmt.Errorf("创建父目录失败: %v", err)
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("创建文件失败: %v", err)
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("打开zip内文件失败: %v", err)
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			return fmt.Errorf("解压文件失败: %v", err)
		}
	}

	fmt.Printf("解压完成: %s\n", targetPath)
	return nil
}

// cloneGitRepository 克隆Git仓库
func (m *RemoteRepositoryManager) cloneGitRepository(url, branch, targetPath string) (string, error) {
	fmt.Printf("开始克隆Git仓库: %s (分支: %s)\n", url, branch)

	// 清理目标目录
	if err := os.RemoveAll(targetPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("清理目录失败: %v", err)
	}

	// 构建git命令
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, url, targetPath)

	cmd := exec.Command("git", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git克隆失败: %v, 输出: %s", err, string(output))
	}

	fmt.Printf("Git克隆完成: %s\n", targetPath)
	return targetPath, nil
}

// Cleanup 清理临时文件
func (m *RemoteRepositoryManager) Cleanup() error {
	if m.config.RemoteRepository.AutoClean {
		fmt.Printf("清理临时目录: %s\n", m.config.RemoteRepository.TargetPath)
		return os.RemoveAll(m.config.RemoteRepository.TargetPath)
	}
	return nil
}
