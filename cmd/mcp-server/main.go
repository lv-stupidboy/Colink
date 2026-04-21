package main

import (
	"fmt"
	"os"

	mcpserver "github.com/anthropic/isdp/internal/mcp-server"
)

func main() {
	// 从环境变量获取配置
	apiURL := os.Getenv("ISDP_API_URL")
	invocationID := os.Getenv("ISDP_INVOCATION_ID")
	callbackToken := os.Getenv("ISDP_CALLBACK_TOKEN")

	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	if invocationID == "" || callbackToken == "" {
		fmt.Fprintln(os.Stderr, "Error: ISDP_INVOCATION_ID and ISDP_CALLBACK_TOKEN must be set")
		os.Exit(1)
	}

	// 启动 MCP Server
	server := mcpserver.NewServer(apiURL, invocationID, callbackToken)
	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running MCP server: %v\n", err)
		os.Exit(1)
	}
}