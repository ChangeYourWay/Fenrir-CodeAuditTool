package main

import (
	"Fenrir-CodeAuditTool/configs"
	"context"
	"encoding/json"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"log"
	"os"
	"path/filepath"
)

type ProjectService struct {
	config *configs.Config
}

type DirInfo struct {
	Name     string    `json:"name"`
	Children []DirInfo `json:"children,omitempty"`
}

type FileNode struct {
	Name     string     `json:"name"`
	IsDir    bool       `json:"isDir"`
	Children []FileNode `json:"children,omitempty"`
}

// 遍历目录
func walkDir(path string) (DirInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return DirInfo{}, err
	}

	// 如果不是目录，直接跳过，不返回节点
	if !info.IsDir() {
		return DirInfo{}, nil
	}

	node := DirInfo{
		Name: info.Name(),
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return node, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			childPath := filepath.Join(path, entry.Name())
			childNode, err := walkDir(childPath)
			if err != nil {
				// 如果遇到错误可跳过
				continue
			}
			node.Children = append(node.Children, childNode)
		}
	}

	return node, nil
}

func (ps *ProjectService) GetProjectDirTree(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	dir, err := walkDir(ps.config.CodeAudit.RepositoryPath + "\\src\\main")
	if err != nil {
		return nil, err
	}
	ps.LogCall("getProjectDirTree", "")
	marshal, err := json.Marshal(dir)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: string(marshal),
			},
		},
	}, nil
}

func (ps *ProjectService) ListFiles(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var path string
	if args, ok := request.Params.Arguments.(map[string]any); ok {
		path = args["path"].(string)
	}
	basePath := ps.config.CodeAudit.RepositoryPath
	absPath := filepath.Join(basePath, path)
	ps.LogCall("listFiles", path)
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}
	var nodes []FileNode
	for _, entry := range entries {
		nodes = append(nodes, FileNode{
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
		})
	}
	res, err := json.Marshal(nodes)
	if err != nil {
		return nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: string(res),
			},
		},
	}, nil
}

func (ps *ProjectService) ReadFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var path string

	if args, ok := request.Params.Arguments.(map[string]any); ok {
		path = args["path"].(string)
	}
	basePath := ps.config.CodeAudit.RepositoryPath
	absPath := filepath.Join(basePath, path)
	data, err := os.ReadFile(absPath)
	ps.LogCall("readFile", absPath)
	if err != nil {
		return nil, err
	}
	if len(data) > 60000 {
		return nil, fmt.Errorf("file is too large")
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: string(data),
			},
		},
	}, nil
}

func (ps *ProjectService) LogCall(name, arg string) {
	log.Printf("Call [%s ] - %s", name, arg)
}

func (ps *ProjectService) RegisterMcpServer(s *server.MCPServer) {
	getDirTree := mcp.NewTool("project_getDirTree", mcp.WithDescription("获取项目对应的java源代码目录结构（/src/main/java）(只包括了，省略了文件且不递归子目录)"))
	s.AddTool(getDirTree, ps.GetProjectDirTree)

	listFiles := mcp.NewTool("project_listFiles",
		mcp.WithDescription("获取项目中对应目录(相对路径)下文件和目录列表"),
		mcp.WithString("path", mcp.Required(), mcp.Description("项目中的相对路径,为'/'时代表项目根路径")),
	)
	readFile := mcp.NewTool("project_readFile",
		mcp.WithDescription("读取项目中文件内容"),
		mcp.WithString("path", mcp.Required(), mcp.Description("项目中文件的相对路径,不能为空")),
	)
	s.AddTool(listFiles, ps.ListFiles)
	s.AddTool(readFile, ps.ReadFile)
}
