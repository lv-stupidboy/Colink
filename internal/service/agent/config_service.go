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

// ConfigService Agent配置服务
type ConfigService struct {
	repo    *repo.AgentConfigRepository
	cache   map[uuid.UUID]*model.AgentRoleConfig
	cacheMu sync.RWMutex
}

// NewConfigService 创建配置服务
func NewConfigService(repo *repo.AgentConfigRepository) *ConfigService {
	return &ConfigService{
		repo:  repo,
		cache: make(map[uuid.UUID]*model.AgentRoleConfig),
	}
}

// GetByID 根据ID获取配置
func (s *ConfigService) GetByID(ctx context.Context, id uuid.UUID) (*model.AgentRoleConfig, error) {
	s.cacheMu.RLock()
	if config, ok := s.cache[id]; ok {
		s.cacheMu.RUnlock()
		return config, nil
	}
	s.cacheMu.RUnlock()

	config, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cache[id] = config
	s.cacheMu.Unlock()

	return config, nil
}

// GetByRole 根据角色获取配置
func (s *ConfigService) GetByRole(ctx context.Context, role model.AgentRole) ([]*model.AgentRoleConfig, error) {
	return s.repo.FindByRole(ctx, role)
}

// GetDefaultByRole 获取角色的默认配置
func (s *ConfigService) GetDefaultByRole(ctx context.Context, role model.AgentRole) (*model.AgentRoleConfig, error) {
	configs, err := s.repo.FindByRole(ctx, role)
	if err != nil {
		return nil, err
	}
	for _, c := range configs {
		if c.IsDefault {
			return c, nil
		}
	}
	if len(configs) > 0 {
		return configs[0], nil
	}
	return nil, ErrConfigNotFound
}

// Create 创建配置
func (s *ConfigService) Create(ctx context.Context, req *model.CreateAgentRequest) (*model.AgentRoleConfig, error) {
	// 角色必须由调用方指定，默认为 agent
	role := req.Role
	if role == "" {
		role = model.AgentRoleAgent
	}

	config := &model.AgentRoleConfig{
		ID:             uuid.New(),
		Name:           req.Name,
		Role:           role,
		BaseAgentID:    req.BaseAgentID,
		Description:    req.Description,
		SystemPrompt:   req.SystemPrompt,
		MaxTokens:      req.MaxTokens,
		Temperature:    req.Temperature,
		IsDefault:      req.IsDefault,
		MentionPatterns: req.MentionPatterns,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.repo.Create(ctx, config); err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cache[config.ID] = config
	s.cacheMu.Unlock()

	return config, nil
}

// Update 更新配置
func (s *ConfigService) Update(ctx context.Context, id uuid.UUID, req *model.CreateAgentRequest) (*model.AgentRoleConfig, error) {
	config, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 角色必须由调用方指定，默认为 agent
	role := req.Role
	if role == "" {
		role = model.AgentRoleAgent
	}

	config.Name = req.Name
	config.Role = role
	config.BaseAgentID = req.BaseAgentID
	config.Description = req.Description
	config.SystemPrompt = req.SystemPrompt
	config.MaxTokens = req.MaxTokens
	config.Temperature = req.Temperature
	config.IsDefault = req.IsDefault
	// 直接更新 mentionPatterns
	config.MentionPatterns = req.MentionPatterns
	config.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, config); err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cache[id] = config
	s.cacheMu.Unlock()

	return config, nil
}

// Delete 删除配置
func (s *ConfigService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	s.cacheMu.Lock()
	delete(s.cache, id)
	s.cacheMu.Unlock()

	return nil
}

// List 列出所有配置
func (s *ConfigService) List(ctx context.Context) ([]*model.AgentRoleConfig, error) {
	return s.repo.List(ctx)
}

var (
	ErrConfigNotFound = errors.New("agent config not found")
)