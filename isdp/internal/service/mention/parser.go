package mention

import (
	"context"
		"fmt"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/parser"
	"github.com/anthropic/isdp/internal/repo"
)

// PatternEntry mention 模式条目（支持多 Agent 匹配）
type PatternEntry struct {
	Pattern  string   // mention 模式（如 "@architect", "@架构师"）
	AgentIDs []string // 匹配的所有 Agent ID 列表（博弈场景支持）
}

// PatternRegistry mention 模式注册表
// 支持动态从数据库加载，一个 pattern 可以匹配多个 Agent
type PatternRegistry struct {
	repo   *repo.AgentConfigRepository

	// pattern -> CatIDs 映射（全局缓存）
	// 一个 pattern 可能匹配多个 Agent（博弈场景）
	patterns map[string][]string
	mu       sync.RWMutex

	// 缓存刷新
	lastRefresh time.Time
	ttl         time.Duration
}

// NewPatternRegistry 创建 mention 模式注册表
func NewPatternRegistry(repo *repo.AgentConfigRepository) *PatternRegistry {
	return &PatternRegistry{
		repo:     repo,
		patterns: make(map[string][]string),
		ttl:      5 * time.Minute, // 缓存 5 分钟
	}
}

// Refresh 从数据库刷新 mention patterns
func (r *PatternRegistry) Refresh(ctx context.Context) error {
	agents, err := r.repo.List(ctx)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 清空旧数据
	r.patterns = make(map[string][]string)

	// 构建 pattern -> AgentIDs 映射
	// 使用 Agent.ID 作为唯一标识，而不是 role
	for _, agent := range agents {
		agentID := agent.ID.String()
		for _, pattern := range agent.MentionPatterns {
			patternLower := strings.ToLower(pattern)
			// 追加到该 pattern 的 AgentID 列表
			r.patterns[patternLower] = append(r.patterns[patternLower], agentID)
		}
	}

	r.lastRefresh = time.Now()
	return nil
}

// GetPatternsForAgents 获取指定 Agent 列表的 mention patterns
// 重要：只返回在 allowedAgents 范围内的 patterns，支持博弈场景
func (r *PatternRegistry) GetPatternsForAgents(ctx context.Context, allowedAgents []*model.AgentRoleConfig) ([]PatternEntry, error) {
	// 先确保全局缓存是最新的
	r.mu.RLock()
	needRefresh := time.Since(r.lastRefresh) > r.ttl || len(r.patterns) == 0
	r.mu.RUnlock()

	if needRefresh {
		if err := r.Refresh(ctx); err != nil {
			return nil, err
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// DEBUG: 打印全局 patterns 数量
	fmt.Printf("[DEBUG] GetPatternsForAgents: 全局 patterns 数量=%d\n", len(r.patterns))

	// 构建 allowedAgents 的 ID 集合
	allowedIDs := make(map[string]bool)
	for _, agent := range allowedAgents {
		allowedIDs[agent.ID.String()] = true
	}

	// DEBUG: 打印 allowedAgents
	var allowedIDStrs []string
	for id := range allowedIDs {
		allowedIDStrs = append(allowedIDStrs, id[:8])
	}
	fmt.Printf("[DEBUG] GetPatternsForAgents: allowedIDs=%v\n", allowedIDStrs)

	// 为每个 pattern 只保留在 allowedAgents 范围内的 AgentIDs
	entries := make([]PatternEntry, 0)
	for pattern, agentIDs := range r.patterns {
		filteredAgentIDs := make([]string, 0)
		for _, agentID := range agentIDs {
			if allowedIDs[agentID] {
				filteredAgentIDs = append(filteredAgentIDs, agentID)
			}
		}
		// 只有在范围内有匹配时才添加
		if len(filteredAgentIDs) > 0 {
			entries = append(entries, PatternEntry{
				Pattern:  pattern,
				AgentIDs: filteredAgentIDs,
			})
		}
	}

	return entries, nil
}

// GetPatterns 获取所有 mention patterns（用于向后兼容）
// 返回 PatternEntry 列表，支持一个 pattern 匹配多个 Agent
func (r *PatternRegistry) GetPatterns(ctx context.Context) ([]PatternEntry, error) {
	// 检查是否需要刷新
	r.mu.RLock()
	needRefresh := time.Since(r.lastRefresh) > r.ttl || len(r.patterns) == 0
	r.mu.RUnlock()

	if needRefresh {
		if err := r.Refresh(ctx); err != nil {
			return nil, err
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]PatternEntry, 0, len(r.patterns))
	for pattern, agentIDs := range r.patterns {
		entries = append(entries, PatternEntry{
			Pattern:  pattern,
			AgentIDs: agentIDs,
		})
	}

	return entries, nil
}

// Lookup 根据 pattern 查找匹配的 AgentIDs
// 支持一个 pattern 返回多个 Agent（博弈场景）
func (r *PatternRegistry) Lookup(ctx context.Context, pattern string) ([]string, error) {
	entries, err := r.GetPatterns(ctx)
	if err != nil {
		return nil, err
	}

	patternLower := strings.ToLower(pattern)
	for _, entry := range entries {
		if entry.Pattern == patternLower {
			return entry.AgentIDs, nil
		}
	}

	return nil, nil
}

// Parser mention 解析器
// 支持动态从数据库加载 mention patterns
type Parser struct {
	registry *PatternRegistry
}

// NewParser 创建 mention 解析器
func NewParser(registry *PatternRegistry) *Parser {
	return &Parser{
		registry: registry,
	}
}

// Parse 解析行首 @mention
// 使用数据库中的动态 patterns
// 注意：此方法返回所有匹配的 AgentIDs，调用方需自行过滤
func (p *Parser) Parse(ctx context.Context, text string, currentAgentID string) ([]string, error) {
	entries, err := p.registry.GetPatterns(ctx)
	if err != nil {
		return nil, err
	}

	// 转换为 parser.MentionPattern
	patterns := make([]parser.MentionPattern, 0, len(entries))
	for _, entry := range entries {
		patterns = append(patterns, parser.MentionPattern{
			Pattern:  entry.Pattern,
			AgentIDs: entry.AgentIDs,
		})
	}

	return parser.ParseA2AMentions(text, currentAgentID, patterns), nil
}

// ParseForAgents 解析行首 @mention，限制在指定 Agent 范围内
// 重要：用于博弈场景，只返回在 allowedAgents 范围内匹配的 AgentIDs
func (p *Parser) ParseForAgents(ctx context.Context, text string, currentAgentID string, allowedAgents []*model.AgentRoleConfig) ([]string, error) {
	entries, err := p.registry.GetPatternsForAgents(ctx, allowedAgents)
	if err != nil {
		return nil, err
	}

	// DEBUG: 打印获取到的 patterns
	fmt.Printf("[DEBUG] ParseForAgents: entries 数量=%d\n", len(entries))
	for i, e := range entries {
		if i < 10 { // 只打印前10个
			fmt.Printf("[DEBUG]   pattern=%s, agentIDs=%v\n", e.Pattern, e.AgentIDs)
		}
	}

	// 转换为 parser.MentionPattern
	patterns := make([]parser.MentionPattern, 0, len(entries))
	for _, entry := range entries {
		patterns = append(patterns, parser.MentionPattern{
			Pattern:  entry.Pattern,
			AgentIDs: entry.AgentIDs,
		})
	}

	result := parser.ParseA2AMentions(text, currentAgentID, patterns)
	fmt.Printf("[DEBUG] ParseForAgents: 解析结果=%v\n", result)
	return result, nil
}

// ParseMulti 解析行首 @mention，支持博弈场景
// 返回每个匹配 pattern 对应的 AgentID 列表
func (p *Parser) ParseMulti(ctx context.Context, text string, currentAgentID string) ([][]string, error) {
	entries, err := p.registry.GetPatterns(ctx)
	if err != nil {
		return nil, err
	}

	// 转换为 parser.MentionPattern
	patterns := make([]parser.MentionPattern, 0, len(entries))
	for _, entry := range entries {
		patterns = append(patterns, parser.MentionPattern{
			Pattern:  entry.Pattern,
			AgentIDs: entry.AgentIDs,
		})
	}

	return parser.ParseA2AMentionsMulti(text, currentAgentID, patterns), nil
}

// ParseMultiForAgents 解析行首 @mention，限制在指定 Agent 范围内，支持博弈场景
// 返回每个匹配 pattern 对应的 AgentID 列表（已过滤到 allowedAgents 范围）
func (p *Parser) ParseMultiForAgents(ctx context.Context, text string, currentAgentID string, allowedAgents []*model.AgentRoleConfig) ([][]string, error) {
	entries, err := p.registry.GetPatternsForAgents(ctx, allowedAgents)
	if err != nil {
		return nil, err
	}

	// 转换为 parser.MentionPattern
	patterns := make([]parser.MentionPattern, 0, len(entries))
	for _, entry := range entries {
		patterns = append(patterns, parser.MentionPattern{
			Pattern:  entry.Pattern,
			AgentIDs: entry.AgentIDs,
		})
	}

	return parser.ParseA2AMentionsMulti(text, currentAgentID, patterns), nil
}