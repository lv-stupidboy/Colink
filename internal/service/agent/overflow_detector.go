package agent

import (
	"strings"
	"unicode/utf8"
)

const (
	MAX_CONSECUTIVE_FAILURES = 3
)

// CircuitBreakerSession Circuit Breaker 检查所需的会话接口
// 用于解耦 agent 和 a2a 包，避免循环导入
type CircuitBreakerSession interface {
	GetConsecutiveRestoreFailures() int
}

// IsContextWindowOverflowError detects context window overflow errors
func IsContextWindowOverflowError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	overflowPatterns := []string{
		"context too large",      // 复用现有（isResumeFallbackError L1512）
		"ran out of room",        // 新增
		"context window",         // 新增
		"token limit",            // 新增
		"exceeds token limit",    // 新增
	}
	for _, pattern := range overflowPatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}
	return false
}

// estimateCJKTokens 估算 CJK 文本的 Token 数量
// CJK 字符通常占用 1.5-2.0 tokens
func estimateCJKTokens(content string) int {
	runeCount := utf8.RuneCountInString(content)
	return int(float64(runeCount) * 1.5)
}

// ShouldSealOnOverflow 判断是否应该 seal session
func ShouldSealOnOverflow(consecutiveFailures int) bool {
	return consecutiveFailures >= MAX_CONSECUTIVE_FAILURES
}

// CheckCircuitBreaker 检查 Circuit Breaker 是否应该触发
func CheckCircuitBreaker(session CircuitBreakerSession) bool {
	if session == nil {
		return false
	}
	return ShouldSealOnOverflow(session.GetConsecutiveRestoreFailures())
}