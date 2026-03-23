package configgen

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service 配置生成服务
type Service struct {
	downloader               *Downloader
	projectRepo              *repo.ProjectRepository
	agentRepo                *repo.AgentConfigRepository
	skillRepo                *repo.SkillRepository
	bindingRepo              *repo.AgentSkillBindingRepository
	subagentRepo             *repo.SubagentRepository
	agentSubagentBindingRepo *repo.AgentSubagentBindingRepository
	commandRepo              *repo.CommandRepository
	ruleRepo                 *repo.RuleRepository
	agentCommandBindingRepo  *repo.AgentCommandBindingRepository
	agentRuleBindingRepo     *repo.AgentRuleBindingRepository
	commandSkillBindingRepo  *repo.CommandSkillBindingRepository
	subagentSkillBindingRepo *repo.SubagentSkillBindingRepository
	skillStoragePath         string
	subagentStoragePath      string
	commandStoragePath       string
	ruleStoragePath          string
	dataDir                  string
	logger                   *zap.Logger
}

// NewService 创建配置生成服务
func NewService(
	projectRepo *repo.ProjectRepository,
	agentRepo *repo.AgentConfigRepository,
	skillRepo *repo.SkillRepository,
	bindingRepo *repo.AgentSkillBindingRepository,
	subagentRepo *repo.SubagentRepository,
	agentSubagentBindingRepo *repo.AgentSubagentBindingRepository,
	commandRepo *repo.CommandRepository,
	ruleRepo *repo.RuleRepository,
	agentCommandBindingRepo *repo.AgentCommandBindingRepository,
	agentRuleBindingRepo *repo.AgentRuleBindingRepository,
	commandSkillBindingRepo *repo.CommandSkillBindingRepository,
	subagentSkillBindingRepo *repo.SubagentSkillBindingRepository,
	skillStoragePath string,
	subagentStoragePath string,
	commandStoragePath string,
	ruleStoragePath string,
	dataDir string,
	logger *zap.Logger,
) *Service {
	return &Service{
		downloader:               NewDownloader(skillStoragePath, subagentStoragePath, logger),
		projectRepo:              projectRepo,
		agentRepo:                agentRepo,
		skillRepo:                skillRepo,
		bindingRepo:              bindingRepo,
		subagentRepo:             subagentRepo,
		agentSubagentBindingRepo: agentSubagentBindingRepo,
		commandRepo:              commandRepo,
		ruleRepo:                 ruleRepo,
		agentCommandBindingRepo:  agentCommandBindingRepo,
		agentRuleBindingRepo:     agentRuleBindingRepo,
		commandSkillBindingRepo:  commandSkillBindingRepo,
		subagentSkillBindingRepo: subagentSkillBindingRepo,
		skillStoragePath:         skillStoragePath,
		subagentStoragePath:      subagentStoragePath,
		commandStoragePath:       commandStoragePath,
		ruleStoragePath:          ruleStoragePath,
		dataDir:                  dataDir,
		logger:                   logger,
	}
}

// GenerateConfigRequest 配置生成请求（项目级，保留兼容）
type GenerateConfigRequest struct {
	ProjectID     string `json:"project_id"`
	BaseAgentType string `json:"base_agent_type"` // claude_code | open_code
	CleanExisting bool   `json:"clean_existing"`  // 是否清理现有配置
}

// GenerateConfigResult 配置生成结果（项目级，保留兼容）
type GenerateConfigResult struct {
	ProjectID   string           `json:"project_id"`
	TargetDir   string           `json:"target_dir"`
	SkillsCount int              `json:"skills_count"`
	Results     []DownloadResult `json:"results"`
	AgentRoles  []string         `json:"agent_roles"`
}

// GenerateAgentConfigRequest Agent配置生成请求（Agent级）
type GenerateAgentConfigRequest struct {
	AgentRoleID   uuid.UUID `json:"agentRoleId"`
	BaseAgentType string    `json:"baseAgentType"` // claude_code | open_code
	CleanExisting bool      `json:"cleanExisting"`
}

// GenerateAgentConfigResult Agent配置生成结果
type GenerateAgentConfigResult struct {
	AgentID        string    `json:"agentId"`
	ConfigPath     string    `json:"configPath"`
	SkillsCount    int       `json:"skillsCount"`
	SubagentsCount int       `json:"subagentsCount"`
	CommandsCount  int       `json:"commandsCount"`
	RulesCount     int       `json:"rulesCount"`
	GeneratedAt    time.Time `json:"generatedAt"`
}

// GenerateConfig 生成项目配置（项目级，保留兼容）
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
			Results:     []DownloadResult{},
			AgentRoles:  agentRoleNames,
		}, nil
	}

	// 下载所有 Skill
	results := s.downloader.DownloadSkills(ctx, skills, req.BaseAgentType, targetDir)

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
		zap.Int("skills_count", len(skills)))

	return &GenerateConfigResult{
		ProjectID:   req.ProjectID,
		TargetDir:   targetDir,
		SkillsCount: len(skills),
		Results:     results,
		AgentRoles:  agentRoleNames,
	}, nil
}

// GenerateAgentConfig 为单个Agent角色生成配置（Agent级）
func (s *Service) GenerateAgentConfig(ctx context.Context, req *GenerateAgentConfigRequest) (*GenerateAgentConfigResult, error) {
	// 1. 获取Agent角色
	agent, err := s.agentRepo.FindByID(ctx, req.AgentRoleID)
	if err != nil {
		return nil, fmt.Errorf("Agent角色不存在: %w", err)
	}

	// 2. 确定配置目录: {dataDir}/agents/{agentID}/.claude
	configPath := filepath.Join(s.dataDir, "agents", req.AgentRoleID.String(), s.getConfigDirName(req.BaseAgentType))

	s.logger.Info("开始生成Agent配置",
		zap.String("agent_id", req.AgentRoleID.String()),
		zap.String("agent_name", agent.Name),
		zap.String("config_path", configPath))

	// 3. 清理现有配置（可选）
	if req.CleanExisting {
		if err := os.RemoveAll(configPath); err != nil && !os.IsNotExist(err) {
			s.logger.Warn("清理配置目录失败", zap.Error(err))
		}
	}

	// 4. 创建目录结构: skills/, agents/, commands/, rules/
	skillsDir := filepath.Join(configPath, "skills")
	agentsDir := filepath.Join(configPath, "agents")
	commandsDir := filepath.Join(configPath, "commands")
	rulesDir := filepath.Join(configPath, "rules")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return nil, fmt.Errorf("创建skills目录失败: %w", err)
	}
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return nil, fmt.Errorf("创建agents目录失败: %w", err)
	}
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return nil, fmt.Errorf("创建commands目录失败: %w", err)
	}
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return nil, fmt.Errorf("创建rules目录失败: %w", err)
	}

	// 5. 生成 settings.json
	if err := s.generateSettingsJSON(agent, configPath, req.BaseAgentType); err != nil {
		s.logger.Warn("生成settings.json失败", zap.Error(err))
	}

	// 6. 生成 CLAUDE.md
	if err := s.generateCLAUDEMd(agent, configPath); err != nil {
		s.logger.Warn("生成CLAUDE.md失败", zap.Error(err))
	}

	// 用于收集所有 Skill（去重）
	skillMap := make(map[uuid.UUID]*model.Skill)

	// 7. 复制绑定的技能文件到 skills/（直接绑定的）
	directSkillIDs, err := s.bindingRepo.FindByAgentRoleID(ctx, req.AgentRoleID)
	if err != nil {
		s.logger.Warn("获取绑定的Skill失败", zap.Error(err))
	}
	for _, skillID := range directSkillIDs {
		if _, exists := skillMap[skillID]; !exists {
			skill, err := s.skillRepo.FindByID(ctx, skillID)
			if err != nil {
				s.logger.Warn("获取Skill失败",
					zap.String("skill_id", skillID.String()),
					zap.Error(err))
				continue
			}
			skillMap[skillID] = skill
		}
	}

	// 8. 复制绑定的Command文件到 commands/，并收集关联的Skill
	commandsCount := 0
	commandIDs, err := s.agentCommandBindingRepo.FindByAgentRoleID(ctx, req.AgentRoleID)
	if err != nil {
		s.logger.Warn("获取绑定的Command失败", zap.Error(err))
	}
	for _, commandID := range commandIDs {
		command, err := s.commandRepo.FindByID(ctx, commandID)
		if err != nil {
			s.logger.Warn("获取Command失败",
				zap.String("command_id", commandID.String()),
				zap.Error(err))
			continue
		}
		// 复制Command文件
		if err := s.copyCommandFile(command, commandsDir); err != nil {
			s.logger.Warn("复制Command文件失败",
				zap.String("command", command.Name),
				zap.Error(err))
			continue
		}
		commandsCount++

		// 收集Command关联的Skill
		commandSkillIDs, err := s.commandSkillBindingRepo.FindByCommandID(ctx, commandID)
		if err != nil {
			s.logger.Warn("获取Command关联的Skill失败",
				zap.String("command_id", commandID.String()),
				zap.Error(err))
			continue
		}
		for _, skillID := range commandSkillIDs {
			if _, exists := skillMap[skillID]; !exists {
				skill, err := s.skillRepo.FindByID(ctx, skillID)
				if err != nil {
					s.logger.Warn("获取Skill失败",
						zap.String("skill_id", skillID.String()),
						zap.Error(err))
					continue
				}
				skillMap[skillID] = skill
			}
		}
	}

	// 9. 复制绑定的Subagent文件到 agents/，并收集关联的Skill
	subagentsCount := 0
	subagents, err := s.agentSubagentBindingRepo.FindSubagentsByAgentRoleID(ctx, req.AgentRoleID)
	if err != nil {
		s.logger.Warn("获取绑定的Subagent失败", zap.Error(err))
	}
	for _, subagent := range subagents {
		if err := s.downloader.CopySubagentToDir(subagent, agentsDir); err != nil {
			s.logger.Warn("复制Subagent文件失败",
				zap.String("subagent", subagent.Name),
				zap.Error(err))
			continue
		}
		subagentsCount++

		// 收集Subagent关联的Skill
		subagentSkillIDs, err := s.subagentSkillBindingRepo.FindBySubagentID(ctx, subagent.ID)
		if err != nil {
			s.logger.Warn("获取Subagent关联的Skill失败",
				zap.String("subagent_id", subagent.ID.String()),
				zap.Error(err))
			continue
		}
		for _, skillID := range subagentSkillIDs {
			if _, exists := skillMap[skillID]; !exists {
				skill, err := s.skillRepo.FindByID(ctx, skillID)
				if err != nil {
					s.logger.Warn("获取Skill失败",
						zap.String("skill_id", skillID.String()),
						zap.Error(err))
					continue
				}
				skillMap[skillID] = skill
			}
		}
	}

	// 10. 复制绑定的Rule文件到 rules/
	rulesCount := 0
	ruleIDs, err := s.agentRuleBindingRepo.FindByAgentRoleID(ctx, req.AgentRoleID)
	if err != nil {
		s.logger.Warn("获取绑定的Rule失败", zap.Error(err))
	}
	for _, ruleID := range ruleIDs {
		rule, err := s.ruleRepo.FindByID(ctx, ruleID)
		if err != nil {
			s.logger.Warn("获取Rule失败",
				zap.String("rule_id", ruleID.String()),
				zap.Error(err))
			continue
		}
		// 复制Rule文件
		if err := s.copyRuleFile(rule, rulesDir); err != nil {
			s.logger.Warn("复制Rule文件失败",
				zap.String("rule", rule.Name),
				zap.Error(err))
			continue
		}
		rulesCount++
	}

	// 11. 复制所有收集到的Skill文件（去重后）
	skillsCount := 0
	for _, skill := range skillMap {
		if _, err := s.downloader.DownloadSkill(ctx, skill, req.BaseAgentType, configPath); err != nil {
			s.logger.Warn("复制Skill文件失败",
				zap.String("skill", skill.Name),
				zap.Error(err))
			continue
		}
		skillsCount++
	}

	// 12. 更新Agent配置生成时间
	if err := s.agentRepo.UpdateConfigGeneratedAt(ctx, req.AgentRoleID, configPath); err != nil {
		s.logger.Warn("更新配置生成时间失败", zap.Error(err))
	}

	s.logger.Info("Agent配置生成完成",
		zap.String("agent_id", req.AgentRoleID.String()),
		zap.Int("skills_count", skillsCount),
		zap.Int("subagents_count", subagentsCount),
		zap.Int("commands_count", commandsCount),
		zap.Int("rules_count", rulesCount))

	return &GenerateAgentConfigResult{
		AgentID:        req.AgentRoleID.String(),
		ConfigPath:     configPath,
		SkillsCount:    skillsCount,
		SubagentsCount: subagentsCount,
		CommandsCount:  commandsCount,
		RulesCount:     rulesCount,
		GeneratedAt:    time.Now(),
	}, nil
}

// generateSettingsJSON 生成settings.json文件
func (s *Service) generateSettingsJSON(agent *model.AgentRoleConfig, configPath, baseAgentType string) error {
	// 构建settings结构
	settings := map[string]interface{}{
		"permissions": map[string]interface{}{
			"allow": []string{},
			"deny":  []string{},
		},
	}

	// 如果有路由配置，添加相关配置
	if len(agent.RoutingConfig.CanRouteTo) > 0 || len(agent.RoutingConfig.RouteOnSignal) > 0 {
		settings["routing"] = map[string]interface{}{
			"can_route_to":     agent.RoutingConfig.CanRouteTo,
			"route_on_signal":  agent.RoutingConfig.RouteOnSignal,
		}
	}

	// 写入文件
	filePath := filepath.Join(configPath, "settings.json")
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化settings失败: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// generateCLAUDEMd 生成CLAUDE.md文件
func (s *Service) generateCLAUDEMd(agent *model.AgentRoleConfig, configPath string) error {
	// 构建内容
	var content string
	if agent.SystemPrompt != "" {
		content = agent.SystemPrompt
	} else {
		// 默认模板
		content = fmt.Sprintf("# %s\n\n%s", agent.Name, agent.Description)
	}

	// 写入文件
	filePath := filepath.Join(configPath, "CLAUDE.md")
	return os.WriteFile(filePath, []byte(content), 0644)
}

// getConfigDir 获取配置目录路径
func (s *Service) getConfigDir(projectPath, baseAgentType string) string {
	return filepath.Join(projectPath, s.getConfigDirName(baseAgentType))
}

// getConfigDirName 获取配置目录名称
func (s *Service) getConfigDirName(baseAgentType string) string {
	switch baseAgentType {
	case "claude_code":
		return ".claude"
	case "open_code":
		return ".opencode"
	default:
		return ".claude"
	}
}

// copyCommandFile 复制Command文件到目标目录
func (s *Service) copyCommandFile(command *model.Command, targetDir string) error {
	// 源文件路径: {commandStoragePath}/{name}.md
	sourcePath := filepath.Join(s.commandStoragePath, command.Name+".md")
	targetPath := filepath.Join(targetDir, command.Name+".md")

	// 读取源文件
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("读取Command文件失败: %w", err)
	}

	// 写入目标文件
	return os.WriteFile(targetPath, content, 0644)
}

// copyRuleFile 复制Rule文件到目标目录
func (s *Service) copyRuleFile(rule *model.Rule, targetDir string) error {
	// 源文件路径: {ruleStoragePath}/{name}.md
	sourcePath := filepath.Join(s.ruleStoragePath, rule.Name+".md")
	targetPath := filepath.Join(targetDir, rule.Name+".md")

	// 读取源文件
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("读取Rule文件失败: %w", err)
	}

	// 写入目标文件
	return os.WriteFile(targetPath, content, 0644)
}