// internal/service/agent/stderr_notify.go
// 从 CLI stderr 中识别用户需要感知的提示（限流、重试、错误等），
// 转为 ChunkTypeError 推送到前端作瞬时状态展示。
package agent

import "strings"

// StderrNotifyPatterns 需要推送前端的 stderr 关键词（大小写不敏感）
var StderrNotifyPatterns = []string{
	"rate limit",
	"too many requests",
	"retrying",
	"429",
	"api error",
	"attempt",
	"timeout",
	"connection reset",
	"connection refused",
	"service unavailable",
	"503",
	"internal server error",
	"500",
	"bad gateway",
	"502",
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