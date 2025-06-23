package main

import (
	"fmt"
	"io/ioutil"
	"log"
)

// loadPrompt 从指定路径读取 prompt 文件内容
func loadPrompt(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func main() {
	// 1. 启动前先加载 prompt 文件
	prompt, err := loadPrompt("prompts/CodeGetPrompts.txt")
	if err != nil {
		log.Fatalf("加载 prompt 文件失败：%v", err)
	}
	fmt.Println("成功加载 prompt：")
	fmt.Println(prompt)

}
