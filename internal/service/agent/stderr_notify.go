// internal/service/agent/stderr_notify.go
// 从 CLI stderr 中识别用户需要感知的提示（限流、重试、错误等），
// 转为 ChunkTypeError 推送到前端作瞬时状态展示。
package agent

import "strings"

// StderrNotifyPatterns 需要推送前端的 stderr 关键词（大小写不敏感）。
//
// 覆盖来源：
//   - Anthropic API：rate_limit_error / overloaded_error / request_too_large /
//     usage limit reached / Retry-After
//   - OpenAI / 通用 HTTP：429 / 503 / 502 / 500 / quota / context length / token limit
//   - CLI 自身：retrying / retry attempt / connection reset|refused
//   - 中文兜底（部分 CLI / 代理网关用中文打印提示）
var StderrNotifyPatterns = []string{
	// Anthropic API 错误类型
	"rate limit",
	"rate_limit_error",
	"rate_limit",
	"overloaded_error",
	"overloaded",
	"usage limit",
	"usage_limit",
	"request_too_large",
	"too many requests",
	// HTTP 状态
	"429",
	"500",
	"502",
	"503",
	"internal server error",
	"bad gateway",
	"service unavailable",
	"gateway timeout",
	"504",
	// 重试 / 退避相关
	"retrying",
	"retry-after",
	"retry after",
	"retry attempt",
	"attempt",
	// 配额 / token
	"quota",
	"token limit",
	"context length",
	"context_length_exceeded",
	"max tokens",
	// API 通用错误
	"api error",
	"api_error",
	// 网络层
	"timeout",
	"connection reset",
	"connection refused",
	"econnreset",
	"econnrefused",
	"etimedout",
	// 中文兜底
	"请稍后",
	"等待重试",
	"重试中",
	"限流",
	"配额",
	"已达上限",
}

// ShouldNotifyStderr 判断 stderr 行是否应该通知前端
func ShouldNotifyStderr(line string) bool {
	lower := strings.ToLower(line)
	for _, p := range StderrNotifyPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}