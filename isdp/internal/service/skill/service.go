package skill

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// BuiltInTagCategory 内置标签分类
type BuiltInTagCategory struct {
	Name  string   `json:"name"`
	Tags  []string `json:"tags"`
}

// builtInTagCategories 内置标签分类列表
var builtInTagCategories = []BuiltInTagCategory{
	{
		Name: "编程语言",
		Tags: []string{
			"Java", "Python", "JavaScript", "TypeScript", "Go", "Rust",
			"C++", "C#", "PHP", "Ruby", "Swift", "Kotlin", "Scala",
		},
	},
	{
		Name: "前端技术",
		Tags: []string{
			"React", "Vue", "Angular", "Next.js", "Nuxt", "Svelte",
			"CSS", "Tailwind", "Sass", "Webpack", "Vite",
		},
	},
	{
		Name: "后端技术",
		Tags: []string{
			"Spring", "Spring Boot", "Django", "Flask", "Express",
			"Gin", "FastAPI", "Node.js", "Microservices", "REST API",
		},
	},
	{
		Name: "数据库",
		Tags: []string{
			"MySQL", "PostgreSQL", "MongoDB", "Redis", "Elasticsearch",
			"SQLite", "Oracle", "SQL Server", "Cassandra",
		},
	},
	{
		Name: "云与DevOps",
		Tags: []string{
			"Docker", "Kubernetes", "AWS", "Azure", "GCP",
			"CI/CD", "Terraform", "Ansible", "Nginx",
		},
	},
	{
		Name: "使用场景",
		Tags: []string{
			"代码规范", "代码审查", "单元测试", "集成测试",
			"安全审计", "性能优化", "重构", "文档生成",
			"API设计", "数据库设计", "架构设计", "错误处理",
		},
	},
	{
		Name: "项目类型",
		Tags: []string{
			"Web应用", "移动应用", "微服务", "CLI工具",
			"API服务", "批处理", "实时系统", "游戏开发",
		},
	},
}

// Service Skill业务服务
type Service struct {
	skillRepo   *repo.SkillRepository
	bindingRepo *repo.AgentSkillBindingRepository
	agentRepo   *repo.AgentConfigRepository
}

// NewService 创建Skill Service
func NewService(skillRepo *repo.SkillRepository, bindingRepo *repo.AgentSkillBindingRepository, agentRepo *repo.AgentConfigRepository) *Service {
	return &Service{
		skillRepo:   skillRepo,
		bindingRepo: bindingRepo,
		agentRepo:   agentRepo,
	}
}

// Create 创建Skill
func (s *Service) Create(ctx context.Context, req *model.CreateSkillRequest) (*model.Skill, error) {
	// 检查名称是否重复
	existing, err := s.skillRepo.FindByName(ctx, req.Name)
	if err != nil {
		// 如果不是"未找到"错误，返回实际错误
		if !strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("检查技能名称失败: %w", err)
		}
		// 名称不存在，可以创建
	} else if existing != nil {
		return nil, errors.New("技能名称已存在")
	}

	// 只有 personal 类型才能设置私有
	isPublic := true
	if req.SourceType == model.SkillSourcePersonal {
		isPublic = req.IsPublic
	}

	skill := &model.Skill{
		ID:              uuid.New(),
		Name:            req.Name,
		Description:     req.Description,
		Tags:            req.Tags,
		SourceType:      req.SourceType,
		SupportedAgents: req.SupportedAgents,
		Version:         req.Version,
		IsPublic:        isPublic,
		Status:          model.SkillStatusActive,
		UseCount:        0,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.skillRepo.Create(ctx, skill); err != nil {
		return nil, fmt.Errorf("创建技能失败: %w", err)
	}

	return skill, nil
}

// GetByID 根据ID获取Skill
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*model.Skill, error) {
	return s.skillRepo.FindByID(ctx, id)
}

// GetByName 根据名称获取Skill
func (s *Service) GetByName(ctx context.Context, name string) (*model.Skill, error) {
	return s.skillRepo.FindByName(ctx, name)
}

// List 列出Skills
func (s *Service) List(ctx context.Context, query *model.SkillListQuery) ([]*model.Skill, int64, error) {
	return s.skillRepo.List(ctx, query)
}

// Update 更新Skill
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateSkillRequest) (*model.Skill, error) {
	skill, err := s.skillRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("技能不存在: %w", err)
	}

	// 更新字段
	if req.Description != "" {
		skill.Description = req.Description
	}
	if req.Tags != nil {
		skill.Tags = req.Tags
	}
	if req.SupportedAgents != nil {
		skill.SupportedAgents = req.SupportedAgents
	}
	if req.Version != "" {
		skill.Version = req.Version
	}
	if req.Status != "" {
		skill.Status = model.SkillStatus(req.Status)
	}
	// 只有 personal 类型才能设置私有
	if skill.SourceType == model.SkillSourcePersonal {
		skill.IsPublic = req.IsPublic
	}
	skill.UpdatedAt = time.Now()

	if err := s.skillRepo.Update(ctx, skill); err != nil {
		return nil, fmt.Errorf("更新技能失败: %w", err)
	}

	return skill, nil
}

// Delete 删除Skill
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// 检查是否有Agent绑定
	agentRoleIDs, err := s.bindingRepo.FindBySkillID(ctx, id)
	if err != nil {
		return fmt.Errorf("检查绑定关系失败: %w", err)
	}
	if len(agentRoleIDs) > 0 {
		return fmt.Errorf("无法删除技能：该技能已被 %d 个Agent绑定", len(agentRoleIDs))
	}

	if err := s.skillRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除技能失败: %w", err)
	}
	return nil
}

// BindSkills 绑定Skills到Agent
func (s *Service) BindSkills(ctx context.Context, agentRoleID uuid.UUID, skillIDs []uuid.UUID) error {
	// 空切片检查
	if len(skillIDs) == 0 {
		return errors.New("技能ID列表不能为空")
	}

	// 验证Agent是否存在
	_, err := s.agentRepo.FindByID(ctx, agentRoleID)
	if err != nil {
		return fmt.Errorf("Agent角色不存在: %w", err)
	}

	// 验证所有Skill存在
	for _, skillID := range skillIDs {
		_, err := s.skillRepo.FindByID(ctx, skillID)
		if err != nil {
			return fmt.Errorf("技能 %s 不存在: %w", skillID.String(), err)
		}
	}

	// 创建绑定
	for _, skillID := range skillIDs {
		// 检查是否已存在绑定
		exists, err := s.bindingRepo.ExistsBinding(ctx, agentRoleID, skillID)
		if err != nil {
			return err
		}
		if exists {
			continue // 已存在绑定，跳过
		}

		binding := &model.AgentSkillBinding{
			ID:          uuid.New(),
			AgentRoleID: agentRoleID,
			SkillID:     skillID,
			CreatedAt:   time.Now(),
		}
		if err := s.bindingRepo.Create(ctx, binding); err != nil {
			return err
		}
	}

	return nil
}

// UnbindSkill 解除Skill绑定
func (s *Service) UnbindSkill(ctx context.Context, agentRoleID, skillID uuid.UUID) error {
	return s.bindingRepo.DeleteBinding(ctx, agentRoleID, skillID)
}

// GetBoundSkills 获取Agent绑定的所有Skills
func (s *Service) GetBoundSkills(ctx context.Context, agentRoleID uuid.UUID) ([]*model.Skill, error) {
	skillIDs, err := s.bindingRepo.FindByAgentRoleID(ctx, agentRoleID)
	if err != nil {
		return nil, err
	}

	skills := make([]*model.Skill, 0, len(skillIDs))
	for _, skillID := range skillIDs {
		skill, err := s.skillRepo.FindByID(ctx, skillID)
		if err != nil {
			continue // 跳过不存在的skill
		}
		skills = append(skills, skill)
	}

	return skills, nil
}

// GetBoundAgents 获取Skill绑定的所有Agents
func (s *Service) GetBoundAgents(ctx context.Context, skillID uuid.UUID) ([]*model.AgentRoleConfig, error) {
	agentRoleIDs, err := s.bindingRepo.FindBySkillID(ctx, skillID)
	if err != nil {
		return nil, err
	}

	agents := make([]*model.AgentRoleConfig, 0, len(agentRoleIDs))
	for _, agentRoleID := range agentRoleIDs {
		agent, err := s.agentRepo.FindByID(ctx, agentRoleID)
		if err != nil {
			continue // 跳过不存在的agent
		}
		agents = append(agents, agent)
	}

	return agents, nil
}

// IncrementUse 增加使用次数
func (s *Service) IncrementUse(ctx context.Context, id uuid.UUID) error {
	return s.skillRepo.IncrementUseCount(ctx, id)
}

// GetAllTags 获取所有标签
func (s *Service) GetAllTags(ctx context.Context) ([]string, error) {
	return s.skillRepo.GetAllTags(ctx)
}

// GetBuiltInTagCategories 获取内置标签分类
func (s *Service) GetBuiltInTagCategories() []BuiltInTagCategory {
	return builtInTagCategories
}