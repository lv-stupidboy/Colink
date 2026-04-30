package skill

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
	skillRepo            *repo.SkillRepository
	bindingRepo          *repo.AgentSkillBindingRepository
	agentRepo            *repo.AgentConfigRepository
	subagentSkillBinding *repo.SubagentSkillBindingRepository
	commandSkillBinding  *repo.CommandSkillBindingRepository
	subagentRepo         *repo.SubagentRepository
	commandRepo          *repo.CommandRepository
	storagePath          string
	logger               *zap.Logger
}

// NewService 创建Skill Service
func NewService(
	skillRepo *repo.SkillRepository,
	bindingRepo *repo.AgentSkillBindingRepository,
	agentRepo *repo.AgentConfigRepository,
	subagentSkillBinding *repo.SubagentSkillBindingRepository,
	commandSkillBinding *repo.CommandSkillBindingRepository,
	subagentRepo *repo.SubagentRepository,
	commandRepo *repo.CommandRepository,
	storagePath string,
	logger *zap.Logger,
) *Service {
	return &Service{
		skillRepo:            skillRepo,
		bindingRepo:          bindingRepo,
		agentRepo:            agentRepo,
		subagentSkillBinding: subagentSkillBinding,
		commandSkillBinding:  commandSkillBinding,
		subagentRepo:         subagentRepo,
		commandRepo:          commandRepo,
		storagePath:          storagePath,
		logger:               logger,
	}
}

// Create 创建Skill
func (s *Service) Create(ctx context.Context, req *model.CreateSkillRequest) (*model.Skill, error) {
	// 允许 skill 重名，不再检查名称唯一性
	// Skills from different sources (personal, team packages, system) may have the same name

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
	// 先获取技能信息（用于删除文件）
	skillRecord, err := s.skillRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("技能不存在: %w", err)
	}

	// 检查是否有Agent绑定，获取绑定的Agent名称
	agentRoleIDs, err := s.bindingRepo.FindBySkillID(ctx, id)
	if err != nil {
		return fmt.Errorf("检查Agent绑定关系失败: %w", err)
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
		return fmt.Errorf("无法删除技能：该技能已被以下Agent绑定：%s", strings.Join(agentNames, "、"))
	}

	// 检查是否有Subagent绑定，获取绑定的子代理名称
	if s.subagentSkillBinding != nil {
		subagentIDs, err := s.subagentSkillBinding.FindBySkillID(ctx, id)
		if err != nil {
			return fmt.Errorf("检查Subagent绑定关系失败: %w", err)
		}
		if len(subagentIDs) > 0 {
			// 获取子代理名称列表
			subagentNames := make([]string, 0, len(subagentIDs))
			for _, subagentID := range subagentIDs {
				if s.subagentRepo != nil {
					subagent, err := s.subagentRepo.FindByID(ctx, subagentID)
					if err == nil {
						subagentNames = append(subagentNames, subagent.Name)
						continue
					}
				}
				subagentNames = append(subagentNames, subagentID.String()[:8])
			}
			return fmt.Errorf("无法删除技能：该技能已被以下子代理绑定：%s", strings.Join(subagentNames, "、"))
		}
	}

	// 检查是否有Command绑定，获取绑定的命令名称
	if s.commandSkillBinding != nil {
		commandIDs, err := s.commandSkillBinding.FindBySkillID(ctx, id)
		if err != nil {
			return fmt.Errorf("检查Command绑定关系失败: %w", err)
		}
		if len(commandIDs) > 0 {
			// 获取命令名称列表
			commandNames := make([]string, 0, len(commandIDs))
			for _, commandID := range commandIDs {
				if s.commandRepo != nil {
					command, err := s.commandRepo.FindByID(ctx, commandID)
					if err == nil {
						commandNames = append(commandNames, command.Name)
						continue
					}
				}
				commandNames = append(commandNames, commandID.String()[:8])
			}
			return fmt.Errorf("无法删除技能：该技能已被以下命令绑定：%s", strings.Join(commandNames, "、"))
		}
	}

	// 删除数据库记录
	if err := s.skillRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除技能失败: %w", err)
	}

	// 删除对应的技能目录
	if s.storagePath != "" && skillRecord != nil {
		skillDir := filepath.Join(s.storagePath, skillRecord.Name)
		if _, err := os.Stat(skillDir); err == nil {
			if err := os.RemoveAll(skillDir); err != nil {
				s.logger.Warn("删除技能目录失败", zap.String("path", skillDir), zap.Error(err))
			} else {
				s.logger.Info("删除技能目录成功", zap.String("path", skillDir))
			}
		}
	}

	s.logger.Info("删除技能成功", zap.String("id", id.String()), zap.String("name", skillRecord.Name))
	return nil
}

// BindSkills 绑定Skills到Agent（全量替换）
func (s *Service) BindSkills(ctx context.Context, agentRoleID uuid.UUID, skillIDs []uuid.UUID) error {
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

	// 先删除所有现有绑定
	if err := s.bindingRepo.DeleteByAgentRoleID(ctx, agentRoleID); err != nil {
		return fmt.Errorf("清理旧绑定失败: %w", err)
	}

	// 创建新的绑定
	for _, skillID := range skillIDs {
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