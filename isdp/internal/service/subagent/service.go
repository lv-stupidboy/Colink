package subagent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ErrSubagentNameExists 子代理名称已存在错误
var ErrSubagentNameExists = fmt.Errorf("subagent name already exists")

// Service Subagent业务服务
type Service struct {
	repo             *repo.SubagentRepository
	bindingRepo      *repo.AgentSubagentBindingRepository
	skillBindingRepo *repo.SubagentSkillBindingRepository
	agentRepo        *repo.AgentConfigRepository
	skillRepo        *repo.SkillRepository
	storagePath      string
	logger           *zap.Logger
}

// NewService 创建Subagent Service
func NewService(
	subagentRepo *repo.SubagentRepository,
	bindingRepo *repo.AgentSubagentBindingRepository,
	skillBindingRepo *repo.SubagentSkillBindingRepository,
	agentRepo *repo.AgentConfigRepository,
	skillRepo *repo.SkillRepository,
	storagePath string,
	logger *zap.Logger,
) *Service {
	return &Service{
		repo:             subagentRepo,
		bindingRepo:      bindingRepo,
		skillBindingRepo: skillBindingRepo,
		agentRepo:        agentRepo,
		skillRepo:        skillRepo,
		storagePath:      storagePath,
		logger:           logger,
	}
}

// getContentFilePath 获取 content 文件路径
func (s *Service) getContentFilePath(name string) string {
	return filepath.Join(s.storagePath, name+".md")
}

// readContentFromFile 从文件读取 content
func (s *Service) readContentFromFile(name string) string {
	filePath := s.getContentFilePath(name)
	content, err := os.ReadFile(filePath)
	if err != nil {
		s.logger.Debug("读取子代理文件失败，返回空内容",
			zap.String("path", filePath),
			zap.Error(err),
		)
		return ""
	}
	return string(content)
}

// writeContentToFile 将 content 写入文件
func (s *Service) writeContentToFile(name, content string) error {
	// 确保存储目录存在
	if err := os.MkdirAll(s.storagePath, 0755); err != nil {
		return fmt.Errorf("创建存储目录失败: %w", err)
	}

	filePath := s.getContentFilePath(name)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	s.logger.Debug("写入子代理文件成功", zap.String("path", filePath))
	return nil
}

// deleteContentFile 删除 content 文件
func (s *Service) deleteContentFile(name string) error {
	filePath := s.getContentFilePath(name)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // 文件不存在，无需删除
	}
	return os.Remove(filePath)
}

// populateContent 为 Subagent 填充 content（从文件读取）
func (s *Service) populateContent(subagent *model.Subagent) {
	if s.storagePath != "" && subagent != nil {
		subagent.Content = s.readContentFromFile(subagent.Name)
	}
}

// populateContentList 为 Subagent 列表填充 content
func (s *Service) populateContentList(subagents []*model.Subagent) {
	for _, subagent := range subagents {
		s.populateContent(subagent)
	}
}

// Create 创建Subagent
func (s *Service) Create(ctx context.Context, req *model.CreateSubagentRequest) (*model.Subagent, error) {
	// 检查名称是否重复
	existing, err := s.repo.FindByName(ctx, req.Name)
	if err != nil {
		// 如果不是"未找到"错误，返回实际错误
		if !strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("检查子代理名称失败: %w", err)
		}
		// 名称不存在，可以创建
	} else if existing != nil {
		return nil, ErrSubagentNameExists
	}

	subagent := &model.Subagent{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		Content:     "", // Content 不再存储到数据库
		SkillID:     req.SkillID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 写入 content 到文件
	if s.storagePath != "" && req.Content != "" {
		if err := s.writeContentToFile(req.Name, req.Content); err != nil {
			return nil, fmt.Errorf("写入子代理文件失败: %w", err)
		}
		subagent.Content = req.Content
	}

	// 创建数据库记录（不含 content）
	if err := s.repo.Create(ctx, subagent); err != nil {
		// 回滚：删除已创建的文件
		if s.storagePath != "" {
			s.deleteContentFile(req.Name)
		}
		return nil, fmt.Errorf("创建子代理失败: %w", err)
	}

	s.logger.Info("创建子代理成功",
		zap.String("id", subagent.ID.String()),
		zap.String("name", subagent.Name),
	)

	return subagent, nil
}

// Get 根据ID获取Subagent
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Subagent, error) {
	subagent, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// 从文件填充 content
	s.populateContent(subagent)
	return subagent, nil
}

// List 列出Subagents
func (s *Service) List(ctx context.Context, query *model.SubagentListQuery) ([]*model.Subagent, int64, error) {
	subagents, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	// 从文件填充 content
	s.populateContentList(subagents)
	return subagents, total, nil
}

// Update 更新Subagent
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateSubagentRequest) (*model.Subagent, error) {
	subagent, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("子代理不存在: %w", err)
	}

	// 更新字段
	if req.Description != "" {
		subagent.Description = req.Description
	}
	// 更新 content 文件
	if req.Content != "" {
		if s.storagePath != "" {
			if err := s.writeContentToFile(subagent.Name, req.Content); err != nil {
				return nil, fmt.Errorf("更新子代理文件失败: %w", err)
			}
		}
		subagent.Content = req.Content
	}
	subagent.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, subagent); err != nil {
		return nil, fmt.Errorf("更新子代理失败: %w", err)
	}

	s.logger.Info("更新子代理成功",
		zap.String("id", subagent.ID.String()),
		zap.String("name", subagent.Name),
	)

	return subagent, nil
}

// Delete 删除Subagent
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// 先获取子代理信息（用于删除文件）
	subagent, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("子代理不存在: %w", err)
	}

	// 检查是否有Agent绑定，获取绑定的Agent名称
	agentRoleIDs, err := s.bindingRepo.FindBySubagentID(ctx, id)
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
		return fmt.Errorf("无法删除子代理：该子代理已被以下Agent绑定：%s", strings.Join(agentNames, "、"))
	}

	// 删除技能绑定
	if err := s.skillBindingRepo.DeleteBySubagentID(ctx, id); err != nil {
		s.logger.Warn("删除技能绑定失败", zap.Error(err))
	}

	// 删除数据库记录
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除子代理失败: %w", err)
	}

	// 删除对应的文件
	if s.storagePath != "" && subagent != nil {
		filePath := filepath.Join(s.storagePath, subagent.Name+".md")
		if _, err := os.Stat(filePath); err == nil {
			if err := os.Remove(filePath); err != nil {
				s.logger.Warn("删除子代理文件失败", zap.String("path", filePath), zap.Error(err))
			} else {
				s.logger.Info("删除子代理文件成功", zap.String("path", filePath))
			}
		}
	}

	s.logger.Info("删除子代理成功", zap.String("id", id.String()), zap.String("name", subagent.Name))
	return nil
}

// BindSubagents 绑定Subagents到Agent（全量替换）
func (s *Service) BindSubagents(ctx context.Context, agentRoleID uuid.UUID, subagentIDs []uuid.UUID) error {
	// 验证Agent是否存在
	_, err := s.agentRepo.FindByID(ctx, agentRoleID)
	if err != nil {
		return fmt.Errorf("Agent角色不存在: %w", err)
	}

	// 验证所有Subagent存在
	for _, subagentID := range subagentIDs {
		_, err := s.repo.FindByID(ctx, subagentID)
		if err != nil {
			return fmt.Errorf("子代理 %s 不存在: %w", subagentID.String(), err)
		}
	}

	// 先删除所有现有绑定
	if err := s.bindingRepo.DeleteByAgentRoleID(ctx, agentRoleID); err != nil {
		return fmt.Errorf("清理旧绑定失败: %w", err)
	}

	// 创建新的绑定
	for _, subagentID := range subagentIDs {
		binding := &model.AgentSubagentBinding{
			ID:          uuid.New(),
			AgentRoleID: agentRoleID,
			SubagentID:  subagentID,
			CreatedAt:   time.Now(),
		}
		if err := s.bindingRepo.Create(ctx, binding); err != nil {
			return err
		}
	}

	s.logger.Info("绑定子代理到Agent成功",
		zap.String("agent_role_id", agentRoleID.String()),
		zap.Int("subagent_count", len(subagentIDs)),
	)

	return nil
}

// GetAgentSubagents 获取Agent绑定的所有Subagents
func (s *Service) GetAgentSubagents(ctx context.Context, agentRoleID uuid.UUID) ([]*model.Subagent, error) {
	return s.bindingRepo.FindSubagentsByAgentRoleID(ctx, agentRoleID)
}

// UnbindSubagent 解除Subagent绑定
func (s *Service) UnbindSubagent(ctx context.Context, agentRoleID, subagentID uuid.UUID) error {
	// 检查绑定是否存在
	exists, err := s.bindingRepo.ExistsBinding(ctx, agentRoleID, subagentID)
	if err != nil {
		return fmt.Errorf("检查绑定关系失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("绑定关系不存在")
	}

	if err := s.bindingRepo.DeleteBinding(ctx, agentRoleID, subagentID); err != nil {
		return fmt.Errorf("解除绑定失败: %w", err)
	}

	s.logger.Info("解除子代理绑定成功",
		zap.String("agent_role_id", agentRoleID.String()),
		zap.String("subagent_id", subagentID.String()),
	)

	return nil
}

// GetSkills 获取Subagent绑定的所有Skills
func (s *Service) GetSkills(ctx context.Context, subagentID uuid.UUID) ([]*model.Skill, error) {
	// 验证Subagent是否存在
	_, err := s.repo.FindByID(ctx, subagentID)
	if err != nil {
		return nil, fmt.Errorf("子代理不存在: %w", err)
	}

	return s.skillBindingRepo.FindSkillsBySubagentID(ctx, subagentID)
}

// BindSkills 绑定Skills到Subagent
func (s *Service) BindSkills(ctx context.Context, subagentID uuid.UUID, skillIDs []uuid.UUID) error {
	// 验证Subagent是否存在
	_, err := s.repo.FindByID(ctx, subagentID)
	if err != nil {
		return fmt.Errorf("子代理不存在: %w", err)
	}

	// 验证所有Skill存在
	for _, skillID := range skillIDs {
		_, err := s.skillRepo.FindByID(ctx, skillID)
		if err != nil {
			return fmt.Errorf("技能 %s 不存在: %w", skillID.String(), err)
		}
	}

	// 先删除现有绑定
	if err := s.skillBindingRepo.DeleteBySubagentID(ctx, subagentID); err != nil {
		return fmt.Errorf("清除旧绑定失败: %w", err)
	}

	// 创建新绑定
	for _, skillID := range skillIDs {
		binding := &model.SubagentSkillBinding{
			ID:         uuid.New(),
			SubagentID: subagentID,
			SkillID:    skillID,
			CreatedAt:  time.Now(),
		}
		if err := s.skillBindingRepo.Create(ctx, binding); err != nil {
			return fmt.Errorf("创建绑定失败: %w", err)
		}
	}

	s.logger.Info("绑定技能到子代理成功",
		zap.String("subagent_id", subagentID.String()),
		zap.Int("skill_count", len(skillIDs)),
	)

	return nil
}

// UnbindSkill 解除Subagent的Skill绑定
func (s *Service) UnbindSkill(ctx context.Context, subagentID, skillID uuid.UUID) error {
	// 检查绑定是否存在
	exists, err := s.skillBindingRepo.ExistsBinding(ctx, subagentID, skillID)
	if err != nil {
		return fmt.Errorf("检查绑定关系失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("绑定关系不存在")
	}

	if err := s.skillBindingRepo.DeleteBinding(ctx, subagentID, skillID); err != nil {
		return fmt.Errorf("解除绑定失败: %w", err)
	}

	s.logger.Info("解除子代理技能绑定成功",
		zap.String("subagent_id", subagentID.String()),
		zap.String("skill_id", skillID.String()),
	)

	return nil
}