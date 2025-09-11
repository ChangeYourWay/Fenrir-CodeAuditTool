package main

import (
	"Fenrir-CodeAuditTool/configs"
	"Fenrir-CodeAuditTool/internal/utils"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// UploadService 处理文件上传
type UploadService struct {
	config     *configs.Config
	zipHandler *utils.ZipHandler
}

// NewUploadService 创建上传服务
func NewUploadService(config *configs.Config) *UploadService {
	return &UploadService{
		config:     config,
		zipHandler: utils.NewZipHandler(config.FileUpload.UploadDir),
	}
}

// UploadRequest 上传请求结构
type UploadRequest struct {
	FileName string `json:"fileName"`
	Data     string `json:"data"` // base64编码的文件内容
}

// UploadResponse 上传响应结构
type UploadResponse struct {
	Success    bool   `json:"success"`
	ProjectID  string `json:"projectId,omitempty"`
	Message    string `json:"message"`
	ProjectDir string `json:"projectDir,omitempty"`
}

// HandleUpload 处理文件上传
func (us *UploadService) HandleUpload(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var uploadReq UploadRequest

	// 解析请求参数
	if request.Params.Arguments != nil {
		if args, ok := request.Params.Arguments.(map[string]any); ok {
			// 文件名
			if v, exists := args["fileName"]; exists && v != nil {
				if s, ok := v.(string); ok {
					uploadReq.FileName = s
				} else {
					uploadReq.FileName = fmt.Sprint(v)
				}
			}

			// 文件数据（base64编码）
			if v, exists := args["data"]; exists && v != nil {
				if s, ok := v.(string); ok {
					uploadReq.Data = s
				} else {
					uploadReq.Data = fmt.Sprint(v)
				}
			}
		}
	}

	// 验证必要参数
	if uploadReq.FileName == "" {
		return us.createErrorResponse("文件名不能为空"), nil
	}
	if uploadReq.Data == "" {
		return us.createErrorResponse("文件内容不能为空"), nil
	}

	// 处理文件上传
	response, err := us.processUpload(uploadReq)
	if err != nil {
		return us.createErrorResponse(fmt.Sprintf("处理上传失败: %v", err)), nil
	}

	// 转换为JSON
	jsonData, err := json.Marshal(response)
	if err != nil {
		return us.createErrorResponse(fmt.Sprintf("JSON序列化失败: %v", err)), nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}, nil
}

// processUpload 处理上传逻辑
func (us *UploadService) processUpload(req UploadRequest) (*UploadResponse, error) {
	// 检查文件类型
	if !us.isValidFileType(req.FileName) {
		return &UploadResponse{
			Success: false,
			Message: "不支持的文件类型，仅支持ZIP格式",
		}, nil
	}

	// 检查文件大小
	if us.isFileTooLarge(req.Data) {
		return &UploadResponse{
			Success: false,
			Message: fmt.Sprintf("文件大小超过限制 (%dMB)", us.config.FileUpload.MaxFileSize/1024/1024),
		}, nil
	}

	// 创建临时文件
	tempFile, err := us.createTempFile(req.Data)
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %v", err)
	}
	defer func() {
		if err := os.Remove(tempFile); err != nil {
			log.Printf("删除临时文件失败: %v", err)
		}
	}()

	// 创建项目目录
	projectID := fmt.Sprintf("project_%d", time.Now().UnixNano())
	projectDir := filepath.Join(us.config.FileUpload.ProjectsDir, projectID)

	// 确保项目目录存在
	if err := os.MkdirAll(us.config.FileUpload.ProjectsDir, 0755); err != nil {
		return nil, fmt.Errorf("创建项目目录失败: %v", err)
	}

	// 解压ZIP文件
	projectRoot, err := us.zipHandler.ExtractZip(tempFile, projectDir)
	if err != nil {
		return &UploadResponse{
			Success: false,
			Message: fmt.Sprintf("解压文件失败: %v", err),
		}, nil
	}

	// 验证解压后的项目是否有效
	projectDetector := utils.NewProjectDetector(us.config.FileUpload.ProjectsDir)
	if !projectDetector.IsValidProject(projectRoot) {
		// 清理无效项目
		os.RemoveAll(projectDir)
		return &UploadResponse{
			Success: false,
			Message: "解压后的目录不是有效的代码项目，请检查ZIP文件内容",
		}, nil
	}

	// 自动更新配置指向新项目
	us.config.CodeAudit.RepositoryPath = projectRoot
	log.Printf("项目已上传并设置为当前分析目录: %s", projectRoot)

	return &UploadResponse{
		Success:    true,
		ProjectID:  projectID,
		Message:    "文件上传并解压成功，已自动设置为分析目录",
		ProjectDir: projectRoot,
	}, nil
}

// isValidFileType 验证文件类型
func (us *UploadService) isValidFileType(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	allowed := []string{".zip", ".jar"}
	for _, allowedExt := range allowed {
		if ext == allowedExt {
			return true
		}
	}
	return false
}

// isFileTooLarge 检查文件大小
func (us *UploadService) isFileTooLarge(base64Data string) bool {
	// Base64编码的数据大小约为原始数据的4/3
	estimatedSize := int64(len(base64Data)) * 3 / 4
	return estimatedSize > us.config.FileUpload.MaxFileSize
}

// createTempFile 创建临时文件
func (us *UploadService) createTempFile(base64Data string) (string, error) {
	// 解码base64数据
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("base64解码失败: %v", err)
	}

	// 确保临时目录存在
	if err := os.MkdirAll(us.config.FileUpload.UploadDir, 0755); err != nil {
		return "", fmt.Errorf("创建上传目录失败: %v", err)
	}

	// 创建临时文件
	tempFile := filepath.Join(us.config.FileUpload.UploadDir, fmt.Sprintf("upload_%d.zip", time.Now().UnixNano()))

	file, err := os.Create(tempFile)
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %v", err)
	}
	defer file.Close()

	// 写入数据
	if _, err := file.Write(data); err != nil {
		return "", fmt.Errorf("写入文件失败: %v", err)
	}

	return tempFile, nil
}

// createErrorResponse 创建错误响应
func (us *UploadService) createErrorResponse(message string) *mcp.CallToolResult {
	errorResponse := UploadResponse{
		Success: false,
		Message: message,
	}

	jsonData, err := json.Marshal(errorResponse)
	if err != nil {
		// 如果JSON序列化失败，返回纯文本错误
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

// RegisterUploadTools 注册上传工具
func (us *UploadService) RegisterUploadTools(s *server.MCPServer) {
	uploadTool := mcp.NewTool("upload_project",
		mcp.WithDescription("上传ZIP格式的代码项目，自动解压并准备分析。支持最大100MB的ZIP文件。"),
		mcp.WithString("fileName",
			mcp.Required(),
			mcp.Description("上传的文件名，必须为ZIP格式（如：myproject.zip）")),
		mcp.WithString("data",
			mcp.Required(),
			mcp.Description("Base64编码的文件内容")),
	)

	s.AddTool(uploadTool, us.HandleUpload)
}

// CleanupUploads 清理上传的临时文件
func (us *UploadService) CleanupUploads() {
	// 清理临时上传文件（超过1小时）
	us.cleanupOldFiles(us.config.FileUpload.UploadDir, time.Hour)

	// 清理旧项目（超过配置的超时时间）
	if us.config.FileUpload.AutoCleanup {
		timeout := time.Duration(us.config.FileUpload.CleanupTimeout) * time.Second
		us.cleanupOldFiles(us.config.FileUpload.ProjectsDir, timeout)
	}
}

// cleanupOldFiles 清理旧文件
func (us *UploadService) cleanupOldFiles(dirPath string, maxAge time.Duration) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		log.Printf("清理目录失败: %s, 错误: %v", dirPath, err)
		return
	}

	now := time.Now()
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if now.Sub(info.ModTime()) > maxAge {
			oldFilePath := filepath.Join(dirPath, entry.Name())
			if err := os.RemoveAll(oldFilePath); err != nil {
				log.Printf("删除旧文件失败: %s, 错误: %v", oldFilePath, err)
			} else {
				log.Printf("已清理旧文件: %s", oldFilePath)
			}
		}
	}
}
