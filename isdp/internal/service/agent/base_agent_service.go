package agent

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// BaseAgentService 基础Agent服务
type BaseAgentService struct {
	repo    *repo.BaseAgentRepository
	cache   map[uuid.UUID]*model.BaseAgent
	cacheMu sync.RWMutex
}

// NewBaseAgentService 创建基础Agent服务
func NewBaseAgentService(repo *repo.BaseAgentRepository) *BaseAgentService {
	return &BaseAgentService{
		repo:  repo,
		cache: make(map[uuid.UUID]*model.BaseAgent),
	}
}

// GetByID 根据ID获取基础Agent
func (s *BaseAgentService) GetByID(ctx context.Context, id uuid.UUID) (*model.BaseAgent, error) {
	s.cacheMu.RLock()
	if agent, ok := s.cache[id]; ok {
		s.cacheMu.RUnlock()
		return s.sanitizeAgent(agent), nil
	}
	s.cacheMu.RUnlock()

	agent, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cache[id] = agent
	s.cacheMu.Unlock()

	return s.sanitizeAgent(agent), nil
}

// GetByType 根据类型获取基础Agent
func (s *BaseAgentService) GetByType(ctx context.Context, agentType model.BaseAgentType) ([]*model.BaseAgent, error) {
	agents, err := s.repo.FindByType(ctx, agentType)
	if err != nil {
		return nil, err
	}
	return s.sanitizeAgents(agents), nil
}

// List 列出所有基础Agent
func (s *BaseAgentService) List(ctx context.Context) ([]*model.BaseAgent, error) {
	agents, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	return s.sanitizeAgents(agents), nil
}

// ListActive 列出所有启用的基础Agent
func (s *BaseAgentService) ListActive(ctx context.Context) ([]*model.BaseAgent, error) {
	agents, err := s.repo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	return s.sanitizeAgents(agents), nil
}

// Create 创建基础Agent
func (s *BaseAgentService) Create(ctx context.Context, req *model.CreateBaseAgentRequest) (*model.BaseAgent, error) {
	// 设置默认值
	cliPath := req.CliPath
	if cliPath == "" {
		if req.Type == model.BaseAgentTypeClaudeCode {
			cliPath = "claude"
		} else if req.Type == model.BaseAgentTypeOpenCode {
			cliPath = "opencode"
		}
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	timeoutMinutes := req.TimeoutMinutes
	if timeoutMinutes == 0 {
		timeoutMinutes = 30
	}

	agent := &model.BaseAgent{
		ID:            uuid.New(),
		Name:          req.Name,
		Type:          req.Type,
		ApiURL:        req.ApiURL,
		ApiToken:      req.ApiToken,
		DefaultModel:  req.DefaultModel,
		CliPath:       cliPath,
		GitBashPath:   req.GitBashPath,
		MaxTokens:     maxTokens,
		TimeoutMinutes: timeoutMinutes,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.repo.Create(ctx, agent); err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cache[agent.ID] = agent
	s.cacheMu.Unlock()

	return s.sanitizeAgent(agent), nil
}

// Update 更新基础Agent
func (s *BaseAgentService) Update(ctx context.Context, id uuid.UUID, req *model.UpdateBaseAgentRequest) (*model.BaseAgent, error) {
	agent, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 只更新非零值字段
	if req.Name != "" {
		agent.Name = req.Name
	}
	if req.Type != "" {
		agent.Type = req.Type
	}
	if req.ApiURL != "" {
		agent.ApiURL = req.ApiURL
	}
	if req.ApiToken != "" {
		agent.ApiToken = req.ApiToken
	}
	if req.DefaultModel != "" {
		agent.DefaultModel = req.DefaultModel
	}
	if req.CliPath != "" {
		agent.CliPath = req.CliPath
	}
	// GitBashPath 可以为空字符串（清除配置），所以需要单独判断
	agent.GitBashPath = req.GitBashPath
	if req.MaxTokens != 0 {
		agent.MaxTokens = req.MaxTokens
	}
	if req.TimeoutMinutes != 0 {
		agent.TimeoutMinutes = req.TimeoutMinutes
	}
	// IsActive 是布尔值，需要特殊处理 - 只有显式设置才更新
	// 这里我们通过检查请求中是否有设置来决定

	if err := s.repo.Update(ctx, agent); err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cache[id] = agent
	s.cacheMu.Unlock()

	return s.sanitizeAgent(agent), nil
}

// Delete 删除基础Agent
func (s *BaseAgentService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	s.cacheMu.Lock()
	delete(s.cache, id)
	s.cacheMu.Unlock()

	return nil
}

// TestConnection 测试基础Agent连接
func (s *BaseAgentService) TestConnection(ctx context.Context, id uuid.UUID) error {
	agent, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// 根据类型创建适配器并测试
	adapter := NewAdapter(agent)
	if adapter == nil {
		return errors.New("unsupported agent type")
	}

	return adapter.CheckHealth(ctx)
}

// GetTypes 获取支持的基础Agent类型
func (s *BaseAgentService) GetTypes() []model.BaseAgentTypeInfo {
	return []model.BaseAgentTypeInfo{
		{
			Type:        model.BaseAgentTypeClaudeCode,
			Name:        "Claude Code",
			Description: "Anthropic Claude CLI - 使用 claude 命令行工具",
		},
		{
			Type:        model.BaseAgentTypeOpenCode,
			Name:        "OpenCode",
			Description: "OpenCode CLI - 开源AI编程助手",
		},
	}
}

// sanitizeAgent 清理敏感信息
func (s *BaseAgentService) sanitizeAgent(agent *model.BaseAgent) *model.BaseAgent {
	// 复制一份，避免修改缓存
	result := *agent
	result.ApiToken = "" // 不返回API Token
	return &result
}

// sanitizeAgents 批量清理敏感信息
func (s *BaseAgentService) sanitizeAgents(agents []*model.BaseAgent) []*model.BaseAgent {
	result := make([]*model.BaseAgent, len(agents))
	for i, agent := range agents {
		result[i] = s.sanitizeAgent(agent)
	}
	return result
}

// InitDefaultAgents 初始化默认基础Agent
func (s *BaseAgentService) InitDefaultAgents(ctx context.Context) error {
	// 检查是否已存在
	agents, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	if len(agents) > 0 {
		return nil // 已有数据，跳过初始化
	}

	// 创建默认的Claude Code Agent
	defaultClaude := &model.BaseAgent{
		ID:            uuid.New(),
		Name:          "Claude Sonnet",
		Type:          model.BaseAgentTypeClaudeCode,
		ApiURL:        "https://api.anthropic.com",
		DefaultModel:  "claude-sonnet-4-6",
		CliPath:       "claude",
		MaxTokens:     4096,
		TimeoutMinutes: 30,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.repo.Create(ctx, defaultClaude); err != nil {
		return err
	}

	s.cacheMu.Lock()
	s.cache[defaultClaude.ID] = defaultClaude
	s.cacheMu.Unlock()

	return nil
}

var (
	ErrBaseAgentNotFound = errors.New("base agent not found")
)