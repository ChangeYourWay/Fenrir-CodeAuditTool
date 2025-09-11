package main

import (
	"Fenrir-CodeAuditTool/configs"
	"Fenrir-CodeAuditTool/internal/utils"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RemoteDownloadService 远程下载服务
type RemoteDownloadService struct {
	config         *configs.Config
	zipHandler     *utils.ZipHandler
	sessionManager *SessionManager
	downloadSem    chan struct{} // 信号量控制并发数
}

// SessionManager 会话管理器
type SessionManager struct {
	sessions    map[string]*UserSession
	mu          sync.RWMutex
	cleanupTick *time.Ticker
}

// UserSession 用户会话
type UserSession struct {
	SessionID  string
	ProjectDir string
	CreatedAt  time.Time
	LastAccess time.Time
	IsActive   bool
}

// DownloadRequest 下载请求结构
type DownloadRequest struct {
	URL         string `json:"url"`
	ProjectName string `json:"projectName,omitempty"`
}

// DownloadResponse 下载响应结构
type DownloadResponse struct {
	Success      bool   `json:"success"`
	SessionID    string `json:"sessionId,omitempty"`
	ProjectDir   string `json:"projectDir,omitempty"`
	Message      string `json:"message"`
	FileSize     int64  `json:"fileSize,omitempty"`
	DownloadTime int64  `json:"downloadTime,omitempty"` // 毫秒
}

// NewRemoteDownloadService 创建远程下载服务
func NewRemoteDownloadService(config *configs.Config) *RemoteDownloadService {
	service := &RemoteDownloadService{
		config:      config,
		downloadSem: make(chan struct{}, 5), // 最大5个并发下载
	}

	// 初始化信号量
	for i := 0; i < 5; i++ {
		service.downloadSem <- struct{}{}
	}

	// 初始化会话管理器
	service.sessionManager = NewSessionManager()

	return service
}

// HandleRemoteDownload 处理远程下载请求
//func (rds *RemoteDownloadService) HandleRemoteDownload(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
//	// ... 其他代码 ...
//
//	// 使用配置的超时时间
//	select {
//	case response := <-resultChan:
//		// ... 处理响应 ...
//	case err := <-errorChan:
//		// ... 处理错误 ...
//	case <-time.After(rds.config.GetDownloadTimeout()):
//		return rds.createErrorResponse("下载处理超时，请检查网络连接或文件大小"), nil
//	}
//}

// cleanupRoutine 清理过期会话的协程
func (sm *SessionManager) cleanupRoutine() {
	for range sm.cleanupTick.C {
		sm.cleanupExpiredSessions(24 * time.Hour) // 可以使用配置的超时时间
	}
}

// NewSessionManager 创建会话管理器
//func NewSessionManager() *SessionManager {
//	sm := &SessionManager{
//		sessions:    make(map[string]*UserSession),
//		cleanupTick: time.NewTicker(5 * time.Minute), // 每5分钟清理一次
//	}
//
//	// 启动清理协程
//	go sm.cleanupRoutine()
//
//	return sm
//}

// cleanupRoutine 清理过期会话的协程
//func (sm *SessionManager) cleanupRoutine() {
//	for range sm.cleanupTick.C {
//		sm.cleanupExpiredSessions(24 * time.Hour) // 清理24小时前的会话
//	}
//}

// CreateSession 创建新会话
func (sm *SessionManager) CreateSession(projectDir string) *UserSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sessionID := generateSessionID()
	session := &UserSession{
		SessionID:  sessionID,
		ProjectDir: projectDir,
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
		IsActive:   true,
	}

	sm.sessions[sessionID] = session
	log.Printf("创建新会话: %s, 项目目录: %s", sessionID, projectDir)
	return session
}

// GetSession 获取会话
func (sm *SessionManager) GetSession(sessionID string) (*UserSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if exists {
		session.LastAccess = time.Now()
	}
	return session, exists
}

// cleanupExpiredSessions 清理过期会话
func (sm *SessionManager) cleanupExpiredSessions(maxAge time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for sessionID, session := range sm.sessions {
		if now.Sub(session.LastAccess) > maxAge {
			// 清理项目文件
			if err := os.RemoveAll(session.ProjectDir); err != nil {
				log.Printf("清理会话项目失败: %s, 错误: %v", session.ProjectDir, err)
			}
			delete(sm.sessions, sessionID)
			log.Printf("已清理过期会话: %s", sessionID)
		}
	}
}

// generateSessionID 生成会话ID
func generateSessionID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("session_%d", time.Now().UnixNano())
	}
	return "session_" + hex.EncodeToString(bytes)
}

// HandleRemoteDownload 处理远程下载请求
func (rds *RemoteDownloadService) HandleRemoteDownload(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var downloadReq DownloadRequest

	// 解析请求参数
	if request.Params.Arguments != nil {
		if args, ok := request.Params.Arguments.(map[string]any); ok {
			// URL
			if v, exists := args["url"]; exists && v != nil {
				if s, ok := v.(string); ok {
					downloadReq.URL = s
				} else {
					downloadReq.URL = fmt.Sprint(v)
				}
			}

			// 项目名称
			if v, exists := args["projectName"]; exists && v != nil {
				if s, ok := v.(string); ok {
					downloadReq.ProjectName = s
				} else {
					downloadReq.ProjectName = fmt.Sprint(v)
				}
			}
		}
	}

	// 验证URL
	if downloadReq.URL == "" {
		return rds.createErrorResponse("下载URL不能为空"), nil
	}

	if !isValidURL(downloadReq.URL) {
		return rds.createErrorResponse("无效的URL格式，仅支持HTTP/HTTPS协议"), nil
	}

	// 处理下载（使用goroutine避免阻塞）
	resultChan := make(chan *DownloadResponse, 1)
	errorChan := make(chan error, 1)

	go func() {
		// 获取下载许可（控制并发）
		<-rds.downloadSem
		defer func() { rds.downloadSem <- struct{}{} }()

		response, err := rds.processDownload(downloadReq)
		if err != nil {
			errorChan <- err
			return
		}
		resultChan <- response
	}()

	// 等待结果（带超时）
	select {
	case response := <-resultChan:
		jsonData, err := json.Marshal(response)
		if err != nil {
			return rds.createErrorResponse("响应序列化失败"), nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: string(jsonData),
				},
			},
		}, nil

	case err := <-errorChan:
		return rds.createErrorResponse(err.Error()), nil

	case <-time.After(10 * time.Minute): // 10分钟超时
		return rds.createErrorResponse("下载处理超时，请检查网络连接或文件大小"), nil
	}
}

// processDownload 处理下载逻辑
func (rds *RemoteDownloadService) processDownload(req DownloadRequest) (*DownloadResponse, error) {
	startTime := time.Now()
	log.Printf("开始处理下载请求: %s", req.URL)

	// 创建项目目录
	projectName := req.ProjectName
	if projectName == "" {
		projectName = generateProjectNameFromURL(req.URL)
	}

	//projectDir := filepath.Join(rds.config.FileUpload.ProjectsDir, projectName)
	//if err := os.MkdirAll(projectDir, 0755); err != nil {
	//	return nil, fmt.Errorf("创建项目目录失败: %v", err)
	//}

	// 下载文件
	tempFile, fileSize, err := rds.downloadFile(req.URL, projectDir)
	if err != nil {
		os.RemoveAll(projectDir) // 清理失败的项目
		return nil, err
	}
	defer os.Remove(tempFile)

	log.Printf("文件下载完成: %s, 大小: %d bytes", tempFile, fileSize)

	// 解压文件
	projectRoot, err := rds.zipHandler.ExtractZip(tempFile, projectDir)
	if err != nil {
		os.RemoveAll(projectDir)
		return nil, fmt.Errorf("解压文件失败: %v", err)
	}

	log.Printf("文件解压完成: %s", projectRoot)

	// 验证项目有效性
	projectDetector := utils.NewProjectDetector(rds.config.FileUpload.ProjectsDir)
	if !projectDetector.IsValidProject(projectRoot) {
		os.RemoveAll(projectDir)
		return nil, fmt.Errorf("下载的项目不是有效的代码项目，请确保ZIP文件包含源代码")
	}

	// 创建用户会话
	session := rds.sessionManager.CreateSession(projectRoot)

	downloadTime := time.Since(startTime).Milliseconds()

	log.Printf("下载处理完成: 会话ID=%s, 耗时=%dms", session.SessionID, downloadTime)

	return &DownloadResponse{
		Success:      true,
		SessionID:    session.SessionID,
		ProjectDir:   projectRoot,
		Message:      "项目下载解压成功，已准备好进行分析",
		FileSize:     fileSize,
		DownloadTime: downloadTime,
	}, nil
}

// downloadFile 下载文件
func (rds *RemoteDownloadService) downloadFile(urlStr, targetDir string) (string, int64, error) {
	log.Printf("开始下载: %s", urlStr)

	// 创建HTTP客户端（带超时）
	client := &http.Client{
		Timeout: 15 * time.Minute,
		Transport: &http.Transport{
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}

	// 发送请求
	resp, err := client.Get(urlStr)
	if err != nil {
		return "", 0, fmt.Errorf("下载请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("下载失败，HTTP状态码: %d", resp.StatusCode)
	}

	// 检查Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/zip") &&
		!strings.Contains(contentType, "application/octet-stream") {
		log.Printf("警告: 非标准Content-Type: %s", contentType)
	}

	// 创建临时文件
	tempFile := filepath.Join(targetDir, "download_temp.zip")
	file, err := os.Create(tempFile)
	if err != nil {
		return "", 0, fmt.Errorf("创建临时文件失败: %v", err)
	}
	defer file.Close()

	// 下载文件内容（带进度日志）
	var totalSize int64
	if resp.ContentLength > 0 {
		totalSize = resp.ContentLength
		log.Printf("文件大小: %d bytes", totalSize)
	}

	buffer := make([]byte, 32*1024) // 32KB缓冲区
	var downloaded int64
	startTime := time.Now()

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := file.Write(buffer[:n]); writeErr != nil {
				return "", 0, fmt.Errorf("写入文件失败: %v", writeErr)
			}
			downloaded += int64(n)

			// 每10MB记录一次进度
			if downloaded%(10*1024*1024) == 0 {
				elapsed := time.Since(startTime).Seconds()
				speed := float64(downloaded) / elapsed / 1024 / 1024 // MB/s
				log.Printf("下载进度: %.1fMB/%.1fMB (%.1f MB/s)",
					float64(downloaded)/1024/1024,
					float64(totalSize)/1024/1024,
					speed)
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return "", 0, fmt.Errorf("读取下载内容失败: %v", err)
		}
	}

	log.Printf("下载完成: %s, 总大小: %d bytes", tempFile, downloaded)
	return tempFile, downloaded, nil
}

// isValidURL 验证URL格式
func isValidURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// generateProjectNameFromURL 从URL生成项目名称
func generateProjectNameFromURL(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Sprintf("project_%d", time.Now().Unix())
	}

	path := u.Path
	if path == "" || path == "/" {
		return fmt.Sprintf("project_%d", time.Now().Unix())
	}

	// 从路径中提取文件名
	if strings.Contains(path, "/") {
		parts := strings.Split(path, "/")
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] != "" && parts[i] != "/" {
				filename := parts[i]

				// 移除文件扩展名
				if strings.Contains(filename, ".") {
					filename = strings.Split(filename, ".")[0]
				}

				// 清理特殊字符
				filename = strings.ReplaceAll(filename, " ", "_")
				filename = strings.ReplaceAll(filename, "+", "_")
				filename = strings.ToLower(filename)

				if filename != "" {
					return fmt.Sprintf("%s_%d", filename, time.Now().Unix())
				}
			}
		}
	}

	return fmt.Sprintf("project_%d", time.Now().Unix())
}

// createErrorResponse 创建错误响应
func (rds *RemoteDownloadService) createErrorResponse(message string) *mcp.CallToolResult {
	errorResponse := DownloadResponse{
		Success: false,
		Message: message,
	}

	jsonData, err := json.Marshal(errorResponse)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf(`{"success":false,"message":"%s"}`, message),
				},
			},
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}
}

// RegisterDownloadTools 注册下载工具
func (rds *RemoteDownloadService) RegisterDownloadTools(s *server.MCPServer) {
	downloadTool := mcp.NewTool("download_project",
		mcp.WithDescription("从远程URL下载代码项目ZIP文件，自动解压并准备分析。支持HTTP/HTTPS链接，最大支持100MB文件。"),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("项目ZIP文件的下载URL（支持HTTP/HTTPS），例如：https://github.com/user/repo/archive/main.zip")),
		mcp.WithString("projectName",
			mcp.Optional(),
			mcp.Description("可选的项目名称，用于标识会话。如果未提供，将从URL自动生成")),
	)

	s.AddTool(downloadTool, rds.HandleRemoteDownload)
}

// GetProjectBySession 通过会话ID获取项目目录
func (rds *RemoteDownloadService) GetProjectBySession(sessionID string) (string, error) {
	session, exists := rds.sessionManager.GetSession(sessionID)
	if !exists {
		return "", fmt.Errorf("会话不存在或已过期")
	}
	return session.ProjectDir, nil
}

// CleanupAllSessions 清理所有会话
func (rds *RemoteDownloadService) CleanupAllSessions() {
	rds.sessionManager.mu.Lock()
	defer rds.sessionManager.mu.Unlock()

	for sessionID, session := range rds.sessionManager.sessions {
		if err := os.RemoveAll(session.ProjectDir); err != nil {
			log.Printf("清理项目目录失败: %s, 错误: %v", session.ProjectDir, err)
		}
		delete(rds.sessionManager.sessions, sessionID)
		log.Printf("已清理会话: %s", sessionID)
	}
}

// GetActiveSessionsCount 获取活跃会话数量
func (rds *RemoteDownloadService) GetActiveSessionsCount() int {
	rds.sessionManager.mu.RLock()
	defer rds.sessionManager.mu.RUnlock()
	return len(rds.sessionManager.sessions)
}
