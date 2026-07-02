package agent

import (
	"errors"
	"testing"
)

// TestIsResumeFallbackError_ClaudeSpecificPatterns
// 验证 S1W2 新增的 Claude / ACP 特有 fallback 模式
func TestIsResumeFallbackError_ClaudeSpecificPatterns(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"unrelated error", errors.New("connection refused"), false},

		// Claude CLI --resume 报错
		{"claude no conversation", errors.New("CLI error: No conversation found with session ID abc-123"), true},
		{"claude conversation not found lowercase", errors.New("conversation not found in transcript"), true},
		{"claude unable to resume", errors.New("Unable to resume the specified session"), true},

		// ClaudeThinkingRescue 触发点
		{"invalid signature thinking block", errors.New("Error: Invalid signature in thinking block detected"), true},

		// 保持向后兼容
		{"legacy session not found", errors.New("session not found"), true},
		{"legacy resource not found (ACP)", errors.New("resource not found: session/load"), true},
		{"legacy pipe closed", errors.New("Pipe is being closed"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isResumeFallbackError(tc.err)
			if got != tc.want {
				t.Fatalf("isResumeFallbackError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
