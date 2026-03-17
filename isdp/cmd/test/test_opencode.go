package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
)

func main() {
	fmt.Println("=== OpenCode Agent 测试 ===")
	fmt.Println()

	// 从环境变量获取模型名，如果没有则使用默认值
	modelName := os.Getenv("OPENCODE_MODEL")
	if modelName == "" {
		modelName = "alibaba-cn/qwen3-coder-plus" // 使用 Qwen Coder 模型
	}

	// 创建 BaseAgent 配置
	baseAgent := &model.BaseAgent{
		ID:             [16]byte{},
		Name:           "Test OpenCode",
		Type:           model.BaseAgentTypeOpenCode,
		ApiURL:         os.Getenv("OPENCODE_API_URL"),
		ApiToken:       os.Getenv("OPENCODE_API_KEY"),
		DefaultModel:   modelName,
		CliPath:        "opencode",
		MaxTokens:      4096,
		TimeoutMinutes: 30,
	}

	// 创建适配器
	adapter := agent.NewOpenCodeAdapter(baseAgent)
	fmt.Printf("适配器已创建：%T\n", adapter)

	// 检查健康状态
	ctx := context.Background()
	fmt.Println("检查 OpenCode CLI 健康状态...")
	if err := adapter.CheckHealth(ctx); err != nil {
		fmt.Printf("健康检查失败：%v\n", err)
		fmt.Println("提示：请确保 opencode CLI 已安装并添加到 PATH")
		return
	}
	fmt.Println("健康检查通过")
	fmt.Println()

	// 创建测试配置
	config := &model.AgentRoleConfig{
		ID:          [16]byte{},
		Name:        "Test Agent",
		Role:        model.AgentRoleDeveloper,
		Description: "测试 Agent",
		SystemPrompt: "你是一个专业的软件开发助手。请用简洁的中文回答问题。",
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	// 创建上下文层
	layers := &agent.ContextLayers{
		Layer0: "你是一个专业的软件开发助手。请用简洁的中文回答问题。",
		Layer1: "",
		Layer2: "",
		Layer3: fmt.Sprintf("测试时间：%s", time.Now().Format("2006-01-02 15:04:05")),
	}

	// 测试输入
	input := "请列出当前目录下的所有 Go 文件"

	// 获取当前工作目录
	workDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("获取工作目录失败：%v\n", err)
		return
	}
	// 切换到项目根目录
	if wd, err := os.Getwd(); err == nil {
		fmt.Printf("当前工作目录：%s\n", wd)
	}

	fmt.Println()
	fmt.Println("=== 测试 ExecuteWithStream（流式执行） ===")
	fmt.Println()

	// 测试流式执行
	var outputBuffer string
	err = adapter.ExecuteWithStream(ctx, config, baseAgent, layers, input, workDir, func(chunk string) {
		fmt.Printf("[CHUNK] %s", chunk)
		outputBuffer += chunk
	})

	if err != nil {
		fmt.Printf("\n执行失败：%v\n", err)
	} else {
		fmt.Println()
		fmt.Println("=== 执行完成 ===")
		fmt.Printf("输出长度：%d 字节\n", len(outputBuffer))
		if len(outputBuffer) > 0 {
			fmt.Println("\n--- 完整输出 ---")
			fmt.Println(outputBuffer)
		}
	}

	fmt.Println()
	fmt.Println("=== 测试 Execute（普通执行） ===")
	fmt.Println()

	// 测试普通执行
	output, err := adapter.Execute(ctx, config, baseAgent, layers, input, workDir)
	if err != nil {
		fmt.Printf("执行失败：%v\n", err)
	} else {
		fmt.Println("执行完成")
		fmt.Printf("输出长度：%d 字节\n", len(output))
		if len(output) > 0 {
			fmt.Println("\n--- 完整输出 ---")
			fmt.Println(output)
		}
	}
}
