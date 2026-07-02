package agent

import (
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// 常量定义
const (
	DefaultAvgTurnTokens  = 10_000 // 默认平均每轮 Token 消耗
	DefaultContextWindow  = 200_000 // 默认上下文窗口大小
	// clowder-ai 对齐的默认配置
	DefaultMaxTotalTokens   = 2000  // 最大总 token（参考 clowder-ai ContextAssembler）
	DefaultMaxContentLength = 1500  // 单条消息最大长度
	DefaultMaxMessages      = 20    // 最大消息数
	SafetyBuffer            = 200   // 安全缓冲

	// A2A Handoff 预算控制
	DefaultHandoffMaxTokens = 800 // 五件套交接块最大 token 数

	// Token 预算阈值分级
	WarnThreshold   = 0.75  // 警告阈值：建议采取行动
	ActionThreshold = 0.85  // 行动阈值：必须采取行动
	TurnBudget      = 12000 // 单轮 Token 预算
)

// StrategyAction 表示根据 Token 预算状态应采取的行动类型
type StrategyAction int

const (
	ActionNone StrategyAction = iota // 无需行动
	ActionWarn                        // 警告：建议压缩上下文
	ActionSeal                        // 行动：必须 seal session
)

// TokenBudgetInfo Token 预算信息（用于 A2AChainContext）
type TokenBudgetInfo struct {
	MaxTokens       int `json:"maxTokens"`       // 最大 token
	UsedTokens      int `json:"usedTokens"`      // 已用 token
	RemainingTokens int `json:"remainingTokens"` // 剩余 token
}

// ActiveParticipant 活跃参与者（参考 clowder-ai activeParticipants）
type ActiveParticipant struct {
	AgentID      string `json:"agentId"`      // Agent ID
	LastActiveAt int64  `json:"lastActiveAt"` // 最后活动时间
	MessageCount int    `json:"messageCount"` // 消息数量
}

// TokenBudgetManager Token 预算管理器
// 注意：应作为 ExecutionService 的成员而非全局单例，避免跨 invocation 状态污染
// usageCache 按 invocationID 缓存，不跨 invocation 共享
// avgTurnTokensCache 按 threadID 缓存，用于动态统计
type TokenBudgetManager struct {
	contextWindowSizes map[string]int64        // 模型 -> 上下文窗口大小 (fallback)
	usageCache         map[uuid.UUID]*TokenUsage // invocationID -> 最新 Usage
	avgTurnTokensCache map[uuid.UUID]int64      // threadID -> 动态统计的平均消耗
	mu                 sync.RWMutex
}

// NewTokenBudgetManager 创建管理器
func NewTokenBudgetManager() *TokenBudgetManager {
	return &TokenBudgetManager{
		contextWindowSizes: defaultContextWindowSizes(),
		usageCache:         make(map[uuid.UUID]*TokenUsage),
		avgTurnTokensCache: make(map[uuid.UUID]int64),
	}
}

// defaultContextWindowSizes 默认上下文窗口大小表（参考 clowder-ai）
func defaultContextWindowSizes() map[string]int64 {
	return map[string]int64{
		"claude-opus-4-6":    200_000,
		"claude-sonnet-4-5":  200_000,
		"claude-haiku-4-5":   200_000,
		"claude-sonnet-4-6":  200_000, // 添加 4.6 版本
		"gpt-5.3":            128_000,
		"gpt-5.2":            128_000,
		"o3":                 200_000,
		"o4-mini":            200_000,
	}
}

// GetAvgTurnTokens 动态计算平均 Token 消耗
// 改进：使用最近 invocation 的平均值，而非硬编码 10K
func (m *TokenBudgetManager) GetAvgTurnTokens(threadID uuid.UUID) int64 {
	m.mu.RLock()
	avg, ok := m.avgTurnTokensCache[threadID]
	m.mu.RUnlock()
	if ok && avg > 0 {
		return avg
	}
	return DefaultAvgTurnTokens // fallback 默认值
}

// UpdateAvgTurnTokens 更新动态平均消耗统计
func (m *TokenBudgetManager) UpdateAvgTurnTokens(threadID uuid.UUID, turnTokens int64) {
	m.mu.Lock()
	m.avgTurnTokensCache[threadID] = turnTokens // 简化版：单次更新，后续可改为滑动平均
	m.mu.Unlock()
}

// UpdateUsageFromCLI 从 CLI Usage 报告更新缓存
func (m *TokenBudgetManager) UpdateUsageFromCLI(invocationID uuid.UUID, usage *TokenUsage) {
	m.mu.Lock()
	m.usageCache[invocationID] = usage
	m.mu.Unlock()
}

// GetUsage 获取缓存的 Usage 信息
func (m *TokenBudgetManager) GetUsage(invocationID uuid.UUID) *TokenUsage {
	m.mu.RLock()
	usage := m.usageCache[invocationID]
	m.mu.RUnlock()
	return usage
}

// GetRemainingBudget 获取剩余 Token 预算
// 计算：contextWindow - inputTokens - outputTokens
func (m *TokenBudgetManager) GetRemainingBudget(model string, usage *TokenUsage) int64 {
	windowSize := m.GetContextWindowSize(model)
	used := usage.InputTokens + usage.OutputTokens
	return windowSize - used
}

// FillRatio returns the ratio of used tokens to window size (0.0-1.0)
func (m *TokenBudgetManager) FillRatio(model string, usage *TokenUsage) float64 {
	remaining := m.GetRemainingBudget(model, usage)
	windowSize := m.GetContextWindowSize(model)
	used := windowSize - remaining
	return float64(used) / float64(windowSize)
}

// GetContextWindowSize 获取模型上下文窗口大小
func (m *TokenBudgetManager) GetContextWindowSize(model string) int64 {
	// 优先从 usageCache 获取实际值（CLI 报告）- 暂未实现，使用 fallback
	// fallback 到硬编码表
	if size, ok := m.contextWindowSizes[model]; ok {
		return size
	}
	// 前缀匹配
	for key, size := range m.contextWindowSizes {
		if strings.HasPrefix(model, key) {
			return size
		}
	}
	return DefaultContextWindow // 默认值
}

// ClearUsageCache 清除 Usage 缓存（用于 invocation 结束时清理）
func (m *TokenBudgetManager) ClearUsageCache(invocationID uuid.UUID) {
	m.mu.Lock()
	delete(m.usageCache, invocationID)
	m.mu.Unlock()
}

// ClearAvgTurnTokensCache 清除平均消耗缓存
func (m *TokenBudgetManager) ClearAvgTurnTokensCache(threadID uuid.UUID) {
	m.mu.Lock()
	delete(m.avgTurnTokensCache, threadID)
	m.mu.Unlock()
}

// GetAllCachedUsages 获取所有缓存的 Usage（用于调试）
func (m *TokenBudgetManager) GetAllCachedUsages() map[uuid.UUID]*TokenUsage {
	m.mu.RLock()
	result := make(map[uuid.UUID]*TokenUsage, len(m.usageCache))
	for k, v := range m.usageCache {
		result[k] = v
	}
	m.mu.RUnlock()
	return result
}

// ========== clowder-ai 对齐的 Token 预算函数 ==========

// EstimateTokens 估算文本 token 数
// 简化版：字符/4（参考 clowder-ai estimateTokens）
// 对于中文等非拉丁字符，实际 token 数可能更高，但这是合理的近似
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	// 简化估算：每个 token 约 4 个字符
	// clowder-ai 使用更精确的估算，但我们采用简化版本以避免外部依赖
	return len(text) / 4
}

// ConstrainHandoffBudget 对交接块内容进行 token 预算裁剪
func ConstrainHandoffBudget(handoff string, maxTokens int) string {
	estimated := EstimateTokens(handoff)
	if estimated <= maxTokens {
		return handoff
	}
	charLimit := maxTokens * 4
	return TruncateHeadTail(handoff, charLimit)
}

// TruncateHeadTail 首尾截断保留核心内容
// 参考 clowder-ai ContextAssembler.ts truncateHeadTail()
// Head 40%，Tail 60%（结论和请求通常在末尾）
func TruncateHeadTail(content string, limit int) string {
	if len(content) <= limit {
		return content
	}

	dropped := len(content) - limit
	marker := "\n\n[...truncated " + formatDropped(dropped) + " chars...]\n\n"
	available := limit - len(marker)

	if available <= 0 {
		// 极限情况：仅保留开头
		return content[:limit]
	}

	headSize := available * 40 / 100 // 40% 给开头
	tailSize := available - headSize // 60% 给结尾

	return content[:headSize] + marker + content[len(content)-tailSize:]
}

// formatDropped 格式化截断数量
func formatDropped(n int) string {
	if n >= 1000 {
		return strconv.Itoa(n/1000) + "k"
	}
	return strconv.Itoa(n)
}

// BudgetForContext 计算上下文可用 Token 预算
// 参考 clowder-ai route-serial.ts effectiveContextBudget 计算
// maxTotalTokens - systemTokens - promptTokens - safetyBuffer
func (m *TokenBudgetManager) BudgetForContext(systemTokens, promptTokens int, maxTotalTokens int) int {
	if maxTotalTokens <= 0 {
		maxTotalTokens = DefaultMaxTotalTokens
	}

	// 可用预算 = 最大 token - 系统部分 - 用户输入 - 安全缓冲
	available := maxTotalTokens - systemTokens - promptTokens - SafetyBuffer

	if available < 0 {
		return 0
	}

	// 不超过 maxContentLength
	if available > DefaultMaxContentLength {
		available = DefaultMaxContentLength
	}

	return available
}

// CreateTokenBudgetInfo 创建 Token 预算信息
func CreateTokenBudgetInfo(maxTokens, usedTokens int) *TokenBudgetInfo {
	return &TokenBudgetInfo{
		MaxTokens:       maxTokens,
		UsedTokens:      usedTokens,
		RemainingTokens: maxTokens - usedTokens,
	}
}

// EstimateTokenBudget 估算当前上下文的 Token 预算信息
func EstimateTokenBudget(systemParts []string, prompt string, maxTokens int) *TokenBudgetInfo {
	// 计算系统部分 token
	var systemTokens int
	for _, part := range systemParts {
		systemTokens += EstimateTokens(part)
	}

	promptTokens := EstimateTokens(prompt)
	usedTokens := systemTokens + promptTokens

	return CreateTokenBudgetInfo(maxTokens, usedTokens)
}

// ShouldTakeAction 根据 Token 预算状态判断应采取的行动
// fillRatio: 已用 token 占上下文窗口的比例 (0.0-1.0)
// remainingTokens: 剩余可用 token 数
// 返回值: 应采取的行动类型 (ActionNone, ActionWarn, ActionSeal)
func ShouldTakeAction(fillRatio float64, remainingTokens int) StrategyAction {
	// 优先检查行动阈值或剩余预算不足
	if fillRatio >= ActionThreshold || remainingTokens < TurnBudget {
		return ActionSeal
	}
	// 检查警告阈值
	if fillRatio >= WarnThreshold {
		return ActionWarn
	}
	// 无需行动
	return ActionNone
}