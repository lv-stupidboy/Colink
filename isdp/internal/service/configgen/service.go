package configgen

import (
	"context"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service 配置生成服务
type Service struct {
	downloader  *Downloader
	projectRepo *repo.ProjectRepository
	agentRepo   *repo.AgentConfigRepository
	skillRepo   *repo.SkillRepository
	bindingRepo *repo.AgentSkillBindingRepository
	logger      *zap.Logger
}

// NewService 创建配置生成服务
func NewService(
	projectRepo *repo.ProjectRepository,
	agentRepo *repo.AgentConfigRepository,
	skillRepo *repo.SkillRepository,
	bindingRepo *repo.AgentSkillBindingRepository,
	logger *zap.Logger,
) *Service {
	return &Service{
		downloader:  NewDownloader(logger),
		projectRepo: projectRepo,
		agentRepo:   agentRepo,
		skillRepo:   skillRepo,
		bindingRepo: bindingRepo,
		logger:      logger,
	}
}

// GenerateConfigRequest 配置生成请求
type GenerateConfigRequest struct {
	ProjectID     string `json:"project_id"`
	BaseAgentType string `json:"base_agent_type"` // claude_code | open_code
	CleanExisting bool   `json:"clean_existing"`  // 是否清理现有配置
}

// GenerateConfigResult 配置生成结果
type GenerateConfigResult struct {
	ProjectID    string           `json:"project_id"`
	TargetDir    string           `json:"target_dir"`
	SkillsCount  int              `json:"skills_count"`
	RulesCount   int              `json:"rules_count"`
	Results      []DownloadResult `json:"results"`
	AgentRoles   []string         `json:"agent_roles"`
}

// GenerateConfig 生成项目配置
func (s *Service) GenerateConfig(ctx context.Context, req *GenerateConfigRequest) (*GenerateConfigResult, error) {
	// 解析项目 ID
	projectUUID, err := uuid.Parse(req.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("无效的项目 ID: %w", err)
	}

	// 验证项目存在
	project, err := s.projectRepo.FindByID(ctx, projectUUID)
	if err != nil {
		return nil, fmt.Errorf("项目不存在: %w", err)
	}

	// 确定目标目录
	projectPath := project.LocalPath
	if projectPath == "" {
		return nil, fmt.Errorf("项目路径未配置")
	}

	targetDir := s.getConfigDir(projectPath, req.BaseAgentType)

	s.logger.Info("开始生成配置",
		zap.String("project_id", req.ProjectID),
		zap.String("base_agent_type", req.BaseAgentType),
		zap.String("target_dir", targetDir))

	// 清理现有配置（可选）
	if req.CleanExisting {
		if err := s.downloader.CleanConfigDir(targetDir); err != nil {
			s.logger.Warn("清理配置目录失败", zap.Error(err))
		}
	}

	// 获取项目的所有 AgentRole
	// 注意：当前项目模型没有直接关联 AgentRole，这里使用全局 AgentRole
	// 后续可以扩展为项目级 AgentRole
	agentConfigs, err := s.agentRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取 AgentRole 失败: %w", err)
	}

	// 收集所有绑定的 Skill
	skillMap := make(map[string]*model.Skill) // 使用 map 去重
	agentRoleNames := make([]string, 0)

	for _, agent := range agentConfigs {
		// 获取 AgentRole 绑定的 Skill
		skillIDs, err := s.bindingRepo.FindByAgentRoleID(ctx, agent.ID)
		if err != nil {
			s.logger.Warn("获取 AgentRole 绑定的 Skill 失败",
				zap.String("agent_id", agent.ID.String()),
				zap.Error(err))
			continue
		}

		if len(skillIDs) > 0 {
			agentRoleNames = append(agentRoleNames, agent.Name)
		}

		for _, skillID := range skillIDs {
			if _, exists := skillMap[skillID.String()]; !exists {
				skill, err := s.skillRepo.FindByID(ctx, skillID)
				if err != nil {
					s.logger.Warn("获取 Skill 失败",
						zap.String("skill_id", skillID.String()),
						zap.Error(err))
					continue
				}
				skillMap[skillID.String()] = skill
			}
		}
	}

	// 转换为列表
	skills := make([]*model.Skill, 0, len(skillMap))
	for _, skill := range skillMap {
		skills = append(skills, skill)
	}

	if len(skills) == 0 {
		return &GenerateConfigResult{
			ProjectID:   req.ProjectID,
			TargetDir:   targetDir,
			SkillsCount: 0,
			RulesCount:  0,
			Results:     []DownloadResult{},
			AgentRoles:  agentRoleNames,
		}, nil
	}

	// 下载所有 Skill
	results := s.downloader.DownloadSkills(ctx, skills, req.BaseAgentType, targetDir)

	// 统计
	skillsCount := 0
	rulesCount := 0
	for _, skill := range skills {
		if skill.Type == model.SkillTypeRule {
			rulesCount++
		} else {
			skillsCount++
		}
	}

	// 更新使用次数
	for _, skill := range skills {
		if err := s.skillRepo.IncrementUseCount(ctx, skill.ID); err != nil {
			s.logger.Warn("更新 Skill 使用次数失败",
				zap.String("skill_id", skill.ID.String()),
				zap.Error(err))
		}
	}

	s.logger.Info("配置生成完成",
		zap.String("project_id", req.ProjectID),
		zap.Int("skills_count", skillsCount),
		zap.Int("rules_count", rulesCount))

	return &GenerateConfigResult{
		ProjectID:   req.ProjectID,
		TargetDir:   targetDir,
		SkillsCount: skillsCount,
		RulesCount:  rulesCount,
		Results:     results,
		AgentRoles:  agentRoleNames,
	}, nil
}

// getConfigDir 获取配置目录路径
func (s *Service) getConfigDir(projectPath, baseAgentType string) string {
	switch baseAgentType {
	case "claude_code":
		return projectPath + "/.claude"
	case "open_code":
		return projectPath + "/.opencode"
	default:
		return projectPath + "/.claude"
	}
}