package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ErrCommandNameExists 命令名称已存在错误
var ErrCommandNameExists = fmt.Errorf("command name already exists")

// Service Command业务服务
type Service struct {
	repo             *repo.CommandRepository
	skillBindingRepo *repo.CommandSkillBindingRepository
	agentBindingRepo *repo.AgentCommandBindingRepository
	agentRepo        *repo.AgentConfigRepository
	skillRepo        *repo.SkillRepository
	storagePath      string
	logger           *zap.Logger
}

// NewService 创建Command Service
func NewService(
	commandRepo *repo.CommandRepository,
	skillBindingRepo *repo.CommandSkillBindingRepository,
	agentBindingRepo *repo.AgentCommandBindingRepository,
	agentRepo *repo.AgentConfigRepository,
	skillRepo *repo.SkillRepository,
	storagePath string,
	logger *zap.Logger,
) *Service {
	return &Service{
		repo:             commandRepo,
		skillBindingRepo: skillBindingRepo,
		agentBindingRepo: agentBindingRepo,
		agentRepo:        agentRepo,
		skillRepo:        skillRepo,
		storagePath:      storagePath,
		logger:           logger,
	}
}

// Create 创建Command
func (s *Service) Create(ctx context.Context, req *model.CreateCommandRequest) (*model.Command, error) {
	// 检查名称格式
	if !isValidName(req.Name) {
		return nil, errors.New("名称只能包含小写字母、数字和中划线，且必须以字母开头")
	}

	// 检查名称是否重复
	existing, err := s.repo.FindByName(ctx, req.Name)
	if err == nil && existing != nil {
		return nil, ErrCommandNameExists
	}

	command := &model.Command{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, command); err != nil {
		return nil, fmt.Errorf("创建命令失败: %w", err)
	}

	s.logger.Info("创建命令成功",
		zap.String("id", command.ID.String()),
		zap.String("name", command.Name),
	)

	return command, nil
}

// Get 根据ID获取Command
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Command, error) {
	return s.repo.FindByID(ctx, id)
}

// GetByName 根据名称获取Command
func (s *Service) GetByName(ctx context.Context, name string) (*model.Command, error) {
	return s.repo.FindByName(ctx, name)
}

// List 列出Commands
func (s *Service) List(ctx context.Context, query *model.CommandListQuery) ([]*model.Command, int64, error) {
	return s.repo.List(ctx, query)
}

// Update 更新Command
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateCommandRequest) (*model.Command, error) {
	command, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("命令不存在: %w", err)
	}

	if req.Description != "" {
		command.Description = req.Description
	}
	command.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, command); err != nil {
		return nil, fmt.Errorf("更新命令失败: %w", err)
	}

	s.logger.Info("更新命令成功",
		zap.String("id", command.ID.String()),
		zap.String("name", command.Name),
	)

	return command, nil
}

// Delete 删除Command
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// 先获取命令信息（用于删除文件）
	command, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("命令不存在: %w", err)
	}

	// 检查是否有Agent绑定，获取绑定的Agent名称
	agentRoleIDs, err := s.agentBindingRepo.FindByCommandID(ctx, id)
	if err != nil {
		return fmt.Errorf("检查绑定关系失败: %w", err)
	}
	if len(agentRoleIDs) > 0 {
		// 获取Agent名称列表
		agentNames := make([]string, 0, len(agentRoleIDs))
		for _, agentID := range agentRoleIDs {
			agent, err := s.agentRepo.FindByID(ctx, agentID)
			if err == nil {
				agentNames = append(agentNames, agent.Name)
			}
		}
		return fmt.Errorf("无法删除命令：该命令已被以下Agent绑定：%s", strings.Join(agentNames, "、"))
	}

	// 删除技能绑定
	if err := s.skillBindingRepo.DeleteByCommandID(ctx, id); err != nil {
		s.logger.Warn("删除技能绑定失败", zap.Error(err))
	}

	// 删除数据库记录
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除命令失败: %w", err)
	}

	// 删除对应的文件
	if s.storagePath != "" && command != nil {
		filePath := filepath.Join(s.storagePath, command.Name+".md")
		if _, err := os.Stat(filePath); err == nil {
			if err := os.Remove(filePath); err != nil {
				s.logger.Warn("删除命令文件失败", zap.String("path", filePath), zap.Error(err))
			} else {
				s.logger.Info("删除命令文件成功", zap.String("path", filePath))
			}
		}
	}

	s.logger.Info("删除命令成功", zap.String("id", id.String()), zap.String("name", command.Name))
	return nil
}

// BindSkills 绑定技能到Command（全量替换）
func (s *Service) BindSkills(ctx context.Context, commandID uuid.UUID, skillIDs []uuid.UUID) error {
	// 验证Command是否存在
	_, err := s.repo.FindByID(ctx, commandID)
	if err != nil {
		return fmt.Errorf("命令不存在: %w", err)
	}

	// 验证所有Skill存在
	for _, skillID := range skillIDs {
		_, err := s.skillRepo.FindByID(ctx, skillID)
		if err != nil {
			return fmt.Errorf("技能 %s 不存在: %w", skillID.String(), err)
		}
	}

	// 先删除所有现有绑定
	if err := s.skillBindingRepo.DeleteByCommandID(ctx, commandID); err != nil {
		return fmt.Errorf("清理旧绑定失败: %w", err)
	}

	// 创建新的绑定
	for _, skillID := range skillIDs {
		binding := &model.CommandSkillBinding{
			ID:        uuid.New(),
			CommandID: commandID,
			SkillID:   skillID,
			CreatedAt: time.Now(),
		}
		if err := s.skillBindingRepo.Create(ctx, binding); err != nil {
			return err
		}
	}

	s.logger.Info("绑定技能到Command成功",
		zap.String("command_id", commandID.String()),
		zap.Int("skill_count", len(skillIDs)),
	)

	return nil
}

// GetSkills 获取Command绑定的技能
func (s *Service) GetSkills(ctx context.Context, commandID uuid.UUID) ([]*model.Skill, error) {
	return s.skillBindingRepo.FindSkillsByCommandID(ctx, commandID)
}

// UnbindSkill 解绑技能
func (s *Service) UnbindSkill(ctx context.Context, commandID, skillID uuid.UUID) error {
	exists, err := s.skillBindingRepo.ExistsBinding(ctx, commandID, skillID)
	if err != nil {
		return fmt.Errorf("检查绑定关系失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("绑定关系不存在")
	}

	if err := s.skillBindingRepo.DeleteBinding(ctx, commandID, skillID); err != nil {
		return fmt.Errorf("解除绑定失败: %w", err)
	}

	s.logger.Info("解除技能绑定成功",
		zap.String("command_id", commandID.String()),
		zap.String("skill_id", skillID.String()),
	)

	return nil
}

// BindCommandsToAgent 绑定Commands到Agent（全量替换）
func (s *Service) BindCommandsToAgent(ctx context.Context, agentRoleID uuid.UUID, commandIDs []uuid.UUID) error {
	// 验证Agent是否存在
	_, err := s.agentRepo.FindByID(ctx, agentRoleID)
	if err != nil {
		return fmt.Errorf("Agent角色不存在: %w", err)
	}

	// 验证所有Command存在
	for _, commandID := range commandIDs {
		_, err := s.repo.FindByID(ctx, commandID)
		if err != nil {
			return fmt.Errorf("命令 %s 不存在: %w", commandID.String(), err)
		}
	}

	// 先删除所有现有绑定
	if err := s.agentBindingRepo.DeleteByAgentRoleID(ctx, agentRoleID); err != nil {
		return fmt.Errorf("清理旧绑定失败: %w", err)
	}

	// 创建新的绑定
	for _, commandID := range commandIDs {
		binding := &model.AgentCommandBinding{
			ID:          uuid.New(),
			AgentRoleID: agentRoleID,
			CommandID:   commandID,
			CreatedAt:   time.Now(),
		}
		if err := s.agentBindingRepo.Create(ctx, binding); err != nil {
			return err
		}
	}

	s.logger.Info("绑定命令到Agent成功",
		zap.String("agent_role_id", agentRoleID.String()),
		zap.Int("command_count", len(commandIDs)),
	)

	return nil
}

// GetAgentCommands 获取Agent绑定的所有Commands
func (s *Service) GetAgentCommands(ctx context.Context, agentRoleID uuid.UUID) ([]*model.Command, error) {
	return s.agentBindingRepo.FindCommandsByAgentRoleID(ctx, agentRoleID)
}

// UnbindCommandFromAgent 解除Command绑定
func (s *Service) UnbindCommandFromAgent(ctx context.Context, agentRoleID, commandID uuid.UUID) error {
	// 检查绑定是否存在
	exists, err := s.agentBindingRepo.ExistsBinding(ctx, agentRoleID, commandID)
	if err != nil {
		return fmt.Errorf("检查绑定关系失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("绑定关系不存在")
	}

	if err := s.agentBindingRepo.DeleteBinding(ctx, agentRoleID, commandID); err != nil {
		return fmt.Errorf("解除绑定失败: %w", err)
	}

	s.logger.Info("解除命令绑定成功",
		zap.String("agent_role_id", agentRoleID.String()),
		zap.String("command_id", commandID.String()),
	)

	return nil
}

// isValidName 校验名称格式
func isValidName(name string) bool {
	if len(name) == 0 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-z][a-z0-9-]*$`, name)
	return matched
}

// GetStoragePath 获取存储路径
func (s *Service) GetStoragePath() string {
	return s.storagePath
}