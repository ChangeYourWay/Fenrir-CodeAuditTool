package main

import (
	"Fenrir-CodeAuditTool/configs"
	"Fenrir-CodeAuditTool/internal/utils"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	// 定义命令行参数
	var (
		configPath = flag.String("config", "resources/config.yaml", "配置文件路径")
		list       = flag.Bool("list", false, "列出所有缓存文件")
		clear      = flag.Bool("clear", false, "清除所有缓存文件")
		info       = flag.Bool("info", false, "显示缓存信息")
		cleanup    = flag.Bool("cleanup", false, "清理旧缓存文件（保留最新的3个）")
	)
	flag.Parse()

	// 加载配置文件
	config, err := configs.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 创建AST构建服务
	astService := utils.NewASTBuilderService(config)

	// 根据参数执行相应操作
	if *list {
		listCacheFiles(astService)
	} else if *clear {
		clearCacheFiles(astService)
	} else if *info {
		showCacheInfo(astService)
	} else if *cleanup {
		cleanupOldCacheFiles(astService)
	} else {
		// 默认显示帮助信息
		showHelp()
	}
}

// listCacheFiles 列出所有缓存文件
func listCacheFiles(astService *utils.ASTBuilderService) {
	fmt.Println("=== 缓存文件列表 ===")

	files, err := astService.ListCacheFiles()
	if err != nil {
		log.Fatalf("获取缓存文件列表失败: %v", err)
	}

	if len(files) == 0 {
		fmt.Println("没有找到缓存文件")
		return
	}

	fmt.Printf("找到 %d 个缓存文件:\n\n", len(files))

	for i, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			fmt.Printf("  %d. %s (无法获取文件信息)\n", i+1, filepath.Base(file))
			continue
		}

		size := info.Size()
		modTime := info.ModTime()

		fmt.Printf("  %d. %s\n", i+1, filepath.Base(file))
		fmt.Printf("      路径: %s\n", file)
		fmt.Printf("      大小: %s\n", formatFileSize(size))
		fmt.Printf("      修改时间: %s\n", modTime.Format("2006-01-02 15:04:05"))
		fmt.Printf("      年龄: %s\n", formatAge(modTime))
		fmt.Println()
	}
}

// clearCacheFiles 清除所有缓存文件
func clearCacheFiles(astService *utils.ASTBuilderService) {
	fmt.Println("=== 清除缓存文件 ===")

	files, err := astService.ListCacheFiles()
	if err != nil {
		log.Fatalf("获取缓存文件列表失败: %v", err)
	}

	if len(files) == 0 {
		fmt.Println("没有找到需要清除的缓存文件")
		return
	}

	fmt.Printf("找到 %d 个缓存文件，确认要删除吗？(y/N): ", len(files))

	var response string
	fmt.Scanln(&response)

	if response != "y" && response != "Y" {
		fmt.Println("操作已取消")
		return
	}

	err = astService.ClearCache()
	if err != nil {
		log.Fatalf("清除缓存文件失败: %v", err)
	}

	fmt.Println("所有缓存文件已清除")
}

// showCacheInfo 显示缓存信息
func showCacheInfo(astService *utils.ASTBuilderService) {
	astService.GetCacheInfo()
}

// cleanupOldCacheFiles 清理旧缓存文件
func cleanupOldCacheFiles(astService *utils.ASTBuilderService) {
	fmt.Println("=== 清理旧缓存文件 ===")

	files, err := astService.ListCacheFiles()
	if err != nil {
		log.Fatalf("获取缓存文件列表失败: %v", err)
	}

	if len(files) <= 3 {
		fmt.Printf("只有 %d 个缓存文件，无需清理\n", len(files))
		return
	}

	// 按修改时间排序
	type fileInfo struct {
		path    string
		modTime time.Time
	}

	var fileInfos []fileInfo
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		fileInfos = append(fileInfos, fileInfo{
			path:    file,
			modTime: info.ModTime(),
		})
	}

	// 按时间排序（最新的在前）
	for i := 0; i < len(fileInfos)-1; i++ {
		for j := i + 1; j < len(fileInfos); j++ {
			if fileInfos[i].modTime.Before(fileInfos[j].modTime) {
				fileInfos[i], fileInfos[j] = fileInfos[j], fileInfos[i]
			}
		}
	}

	// 保留最新的3个，删除其余的
	toDelete := fileInfos[3:]

	if len(toDelete) == 0 {
		fmt.Println("没有需要清理的旧文件")
		return
	}

	fmt.Printf("将删除 %d 个旧缓存文件:\n", len(toDelete))
	for _, file := range toDelete {
		fmt.Printf("  - %s (修改时间: %s)\n", filepath.Base(file.path), file.modTime.Format("2006-01-02 15:04:05"))
	}

	fmt.Print("确认删除吗？(y/N): ")
	var response string
	fmt.Scanln(&response)

	if response != "y" && response != "Y" {
		fmt.Println("操作已取消")
		return
	}

	// 删除旧文件
	for _, file := range toDelete {
		err := os.Remove(file.path)
		if err != nil {
			fmt.Printf("删除文件失败: %s, 错误: %v\n", filepath.Base(file.path), err)
		} else {
			fmt.Printf("已删除: %s\n", filepath.Base(file.path))
		}
	}

	fmt.Printf("清理完成，保留了 %d 个最新的缓存文件\n", 3)
}

// showHelp 显示帮助信息
func showHelp() {
	fmt.Println("AST缓存管理工具")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  cache_manager [选项]")
	fmt.Println()
	fmt.Println("选项:")
	fmt.Println("  -config string")
	fmt.Println("        配置文件路径 (默认: resources/config.yaml)")
	fmt.Println("  -list")
	fmt.Println("        列出所有缓存文件")
	fmt.Println("  -clear")
	fmt.Println("        清除所有缓存文件")
	fmt.Println("  -info")
	fmt.Println("        显示缓存信息")
	fmt.Println("  -cleanup")
	fmt.Println("        清理旧缓存文件（保留最新的3个）")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  cache_manager -list")
	fmt.Println("  cache_manager -clear")
	fmt.Println("  cache_manager -cleanup")
}

// formatFileSize 格式化文件大小
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// formatAge 格式化文件年龄
func formatAge(modTime time.Time) string {
	duration := time.Since(modTime)

	if duration < time.Minute {
		return "刚刚"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%d分钟前", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%d小时前", hours)
	} else {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%d天前", days)
	}
}
