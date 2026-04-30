package configgen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/agent"
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
	settingsRepo             *repo.SettingsRepository
	agentSettingsBindingRepo *repo.AgentSettingsBindingRepository
	skillStoragePath         string
	subagentStoragePath      string
	commandStoragePath       string
	ruleStoragePath          string
	dataDir                  string
	logger                   *zap.Logger
	// 缓存失效回调（配置生成后需要刷新 ConfigService 的缓存）
	onCacheInvalidate func(agentRoleID uuid.UUID)
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
	settingsRepo *repo.SettingsRepository,
	agentSettingsBindingRepo *repo.AgentSettingsBindingRepository,
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
		settingsRepo:             settingsRepo,
		agentSettingsBindingRepo: agentSettingsBindingRepo,
		skillStoragePath:         skillStoragePath,
		subagentStoragePath:      subagentStoragePath,
		commandStoragePath:       commandStoragePath,
		ruleStoragePath:          ruleStoragePath,
		dataDir:                  dataDir,
		logger:                   logger,
	}
}

// SetCacheInvalidateCallback 设置缓存失效回调
// 配置生成后会调用此回调通知 ConfigService 刷新缓存
func (s *Service) SetCacheInvalidateCallback(callback func(agentRoleID uuid.UUID)) {
	s.onCacheInvalidate = callback
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
	SettingsCount  int       `json:"settingsCount"`
	GeneratedAt    time.Time `json:"generatedAt"`
}

// PreviewAgentConfigResult Agent配置预览结果
type PreviewAgentConfigResult struct {
	AgentID        string             `json:"agentId"`
	AgentName      string             `json:"agentName"`
	Skills         []PreviewAssetItem `json:"skills"`
	Commands       []PreviewAssetItem `json:"commands"`
	Subagents      []PreviewAssetItem `json:"subagents"`
	Rules          []PreviewAssetItem `json:"rules"`
	Settings       []PreviewAssetItem `json:"settings"`
	SkillsCount    int                `json:"skillsCount"`
	CommandsCount  int                `json:"commandsCount"`
	SubagentsCount int                `json:"subagentsCount"`
	RulesCount     int                `json:"rulesCount"`
	SettingsCount  int                `json:"settingsCount"`
}

// PreviewAssetItem 预览资产项
type PreviewAssetItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
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
	agentRole, err := s.agentRepo.FindByID(ctx, req.AgentRoleID)
	if err != nil {
		return nil, fmt.Errorf("Agent角色不存在: %w", err)
	}

	// 2. 确定配置目录: {dataDir}/{agentID}/
	configPath := filepath.Join(s.dataDir, req.AgentRoleID.String())

	// 转换为绝对路径
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("获取配置目录绝对路径失败: %w", err)
	}
	configPath = absConfigPath

	s.logger.Info("开始生成Agent配置",
		zap.String("agent_id", req.AgentRoleID.String()),
		zap.String("agent_name", agentRole.Name),
		zap.String("config_path", configPath))

	// 3. 获取并过滤资产（根据 SupportedAgents）
	filteredAssets := s.getFilteredAssets(ctx, req.AgentRoleID, req.BaseAgentType)

	// 4. 获取对应Agent的ConfigGenerator
	generator := agent.CreateConfigGenerator(
		model.BaseAgentType(req.BaseAgentType),
		s.skillStoragePath,
		s.subagentStoragePath,
		s.commandStoragePath,
		s.ruleStoragePath,
		s.logger,
	)
	if generator == nil {
		return nil, fmt.Errorf("没有配置生成器支持Agent类型: %s", req.BaseAgentType)
	}

	// 5. 调用ConfigGenerator生成配置
	result, err := generator.GenerateConfig(ctx, &agent.ConfigGenerateRequest{
		AgentRoleID:   req.AgentRoleID,
		BaseAgentType: req.BaseAgentType,
		ConfigPath:    configPath,
		Skills:        filteredAssets.Skills,
		Commands:      filteredAssets.Commands,
		Subagents:     filteredAssets.Subagents,
		Rules:         filteredAssets.Rules,
		Settings:      filteredAssets.Settings,
		CleanExisting: req.CleanExisting,
	})
	if err != nil {
		return nil, fmt.Errorf("配置生成失败: %w", err)
	}

	// 6. 更新Agent配置生成时间
	if err := s.agentRepo.UpdateConfigGeneratedAt(ctx, req.AgentRoleID, configPath); err != nil {
		s.logger.Warn("更新配置生成时间失败", zap.Error(err))
	}

	// 7. 通知 ConfigService 刷新缓存（确保 ConfigPath 更新生效）
	if s.onCacheInvalidate != nil {
		s.onCacheInvalidate(req.AgentRoleID)
	}

	s.logger.Info("Agent配置生成完成",
		zap.String("agent_id", req.AgentRoleID.String()),
		zap.Int("skills_count", result.SkillsCount),
		zap.Int("subagents_count", result.SubagentsCount),
		zap.Int("commands_count", result.CommandsCount),
		zap.Int("rules_count", result.RulesCount),
		zap.Int("settings_count", result.SettingsCount))

	return &GenerateAgentConfigResult{
		AgentID:        req.AgentRoleID.String(),
		ConfigPath:     configPath,
		SkillsCount:    result.SkillsCount,
		SubagentsCount: result.SubagentsCount,
		CommandsCount:  result.CommandsCount,
		RulesCount:     result.RulesCount,
		SettingsCount:  result.SettingsCount,
		GeneratedAt:    time.Now(),
	}, nil
}

// FilteredAssets 过滤后的资产集合
type FilteredAssets struct {
	Skills    []*model.Skill
	Commands  []*model.Command
	Subagents []*model.Subagent
	Rules     []*model.Rule
	Settings  []*model.Settings
}

// getFilteredAssets 获取并过滤资产（根据 SupportedAgents 向后兼容）
func (s *Service) getFilteredAssets(ctx context.Context, agentRoleID uuid.UUID, agentType string) *FilteredAssets {
	assets := &FilteredAssets{
		Skills:    []*model.Skill{},
		Commands:  []*model.Command{},
		Subagents: []*model.Subagent{},
		Rules:     []*model.Rule{},
		Settings:  []*model.Settings{},
	}

	// 用于收集所有 Skill（去重）
	skillMap := make(map[uuid.UUID]*model.Skill)

	// 1. 获取直接绑定的技能（过滤）
	directSkillIDs, err := s.bindingRepo.FindByAgentRoleID(ctx, agentRoleID)
	if err != nil {
		s.logger.Warn("获取绑定的Skill失败", zap.Error(err))
	}
	for _, skillID := range directSkillIDs {
		skill, err := s.skillRepo.FindByID(ctx, skillID)
		if err != nil {
			s.logger.Warn("获取Skill失败", zap.String("skill_id", skillID.String()), zap.Error(err))
			continue
		}
		if matchesAgentType(skill.SupportedAgents, agentType) {
			skillMap[skillID] = skill
		}
	}

	// 2. 获取绑定的Commands及其关联Skills（过滤）
	commandIDs, err := s.agentCommandBindingRepo.FindByAgentRoleID(ctx, agentRoleID)
	if err != nil {
		s.logger.Warn("获取绑定的Command失败", zap.Error(err))
	}
	for _, commandID := range commandIDs {
		command, err := s.commandRepo.FindByID(ctx, commandID)
		if err != nil {
			s.logger.Warn("获取Command失败", zap.String("command_id", commandID.String()), zap.Error(err))
			continue
		}
		if matchesAgentType(command.SupportedAgents, agentType) {
			assets.Commands = append(assets.Commands, command)
		}

		// 收集Command关联的Skill（过滤）
		commandSkillIDs, err := s.commandSkillBindingRepo.FindByCommandID(ctx, commandID)
		if err != nil {
			s.logger.Warn("获取Command关联的Skill失败", zap.Error(err))
			continue
		}
		for _, skillID := range commandSkillIDs {
			if _, exists := skillMap[skillID]; !exists {
				skill, err := s.skillRepo.FindByID(ctx, skillID)
				if err != nil {
					s.logger.Warn("获取Skill失败", zap.String("skill_id", skillID.String()), zap.Error(err))
					continue
				}
				if matchesAgentType(skill.SupportedAgents, agentType) {
					skillMap[skillID] = skill
				}
			}
		}
	}

	// 3. 获取绑定的Subagents及其关联Skills（过滤）
	subagents, err := s.agentSubagentBindingRepo.FindSubagentsByAgentRoleID(ctx, agentRoleID)
	if err != nil {
		s.logger.Warn("获取绑定的Subagent失败", zap.Error(err))
	}
	for _, subagent := range subagents {
		if matchesAgentType(subagent.SupportedAgents, agentType) {
			assets.Subagents = append(assets.Subagents, subagent)
		}

		// 收集Subagent关联的Skill（过滤）
		subagentSkillIDs, err := s.subagentSkillBindingRepo.FindBySubagentID(ctx, subagent.ID)
		if err != nil {
			s.logger.Warn("获取Subagent关联的Skill失败", zap.Error(err))
			continue
		}
		for _, skillID := range subagentSkillIDs {
			if _, exists := skillMap[skillID]; !exists {
				skill, err := s.skillRepo.FindByID(ctx, skillID)
				if err != nil {
					s.logger.Warn("获取Skill失败", zap.String("skill_id", skillID.String()), zap.Error(err))
					continue
				}
				if matchesAgentType(skill.SupportedAgents, agentType) {
					skillMap[skillID] = skill
				}
			}
		}
	}

	// 4. 获取绑定的Rules（过滤）
	ruleIDs, err := s.agentRuleBindingRepo.FindByAgentRoleID(ctx, agentRoleID)
	if err != nil {
		s.logger.Warn("获取绑定的Rule失败", zap.Error(err))
	}
	for _, ruleID := range ruleIDs {
		rule, err := s.ruleRepo.FindByID(ctx, ruleID)
		if err != nil {
			s.logger.Warn("获取Rule失败", zap.String("rule_id", ruleID.String()), zap.Error(err))
			continue
		}
		if matchesAgentType(rule.SupportedAgents, agentType) {
			assets.Rules = append(assets.Rules, rule)
		}
	}

	// 5. 获取绑定的Settings（过滤）
	settingsIDs, err := s.agentSettingsBindingRepo.FindByAgentRoleID(ctx, agentRoleID)
	if err != nil {
		s.logger.Warn("获取绑定的Settings失败", zap.Error(err))
	}
	for _, settingsID := range settingsIDs {
		settings, err := s.settingsRepo.FindByID(ctx, settingsID)
		if err != nil {
			s.logger.Warn("获取Settings失败", zap.String("settings_id", settingsID.String()), zap.Error(err))
			continue
		}
		if matchesAgentType(settings.SupportedAgents, agentType) {
			assets.Settings = append(assets.Settings, settings)
		}
	}

	// 6. 转换skillMap为列表
	for _, skill := range skillMap {
		assets.Skills = append(assets.Skills, skill)
	}

	return assets
}

// matchesAgentType 检查资产是否支持指定的Agent类型（向后兼容）
func matchesAgentType(supportedAgents []string, agentType string) bool {
	// 空数组向后兼容：默认只支持 claude_code
	if len(supportedAgents) == 0 {
		return agentType == "claude_code"
	}
	// 非空数组：检查是否包含指定类型
	for _, a := range supportedAgents {
		if a == agentType {
			return true
		}
	}
	return false
}

// getConfigDir 获取配置目录路径
func (s *Service) getConfigDir(projectPath, baseAgentType string) string {
	return filepath.Join(projectPath, s.getConfigDirName(baseAgentType))
}

// getConfigDirName 获取配置目录名称
func (s *Service) getConfigDirName(baseAgentType string) string {
	return agent.GetConfigDir(model.BaseAgentType(baseAgentType))
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

// copySettingsDirectory 复制Settings目录内容到目标目录
// 源目录: {settings.DirectoryPath}
// 目标目录: {targetDir}（直接复制到configPath根目录，与skills/、agents/并列）
func (s *Service) copySettingsDirectory(settings *model.Settings, targetDir string) error {
	// 检查Settings目录路径是否存在
	if settings.DirectoryPath == "" {
		return fmt.Errorf("Settings目录路径为空")
	}

	// 源目录
	sourceDir := settings.DirectoryPath
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return fmt.Errorf("Settings源目录不存在: %s", sourceDir)
	}

	// 直接复制目录内容到目标目录（不创建settings/{name}/子目录）
	s.logger.Info("复制Settings内容到配置目录",
		zap.String("settings", settings.Name),
		zap.String("source", sourceDir),
		zap.String("target", targetDir))

	// 递归复制目录内容
	return s.copyDirContents(sourceDir, targetDir)
}

// copyDirContents 递归复制目录内容
func (s *Service) copyDirContents(srcDir, destDir string) error {
	// 读取源目录
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("读取源目录失败: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		destPath := filepath.Join(destDir, entry.Name())

		if entry.IsDir() {
			// 递归复制子目录
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return fmt.Errorf("创建子目录失败: %w", err)
			}
			if err := s.copyDirContents(srcPath, destPath); err != nil {
				return err
			}
		} else {
			// 复制文件
			content, err := os.ReadFile(srcPath)
			if err != nil {
				return fmt.Errorf("读取文件失败: %w", err)
			}
			if err := os.WriteFile(destPath, content, 0644); err != nil {
				return fmt.Errorf("写入文件失败: %w", err)
			}
		}
	}

	return nil
}

// PreviewAgentConfig 预览Agent配置（生成前）
func (s *Service) PreviewAgentConfig(ctx context.Context, agentRoleID uuid.UUID) (*PreviewAgentConfigResult, error) {
	// 1. 获取Agent角色
	agent, err := s.agentRepo.FindByID(ctx, agentRoleID)
	if err != nil {
		return nil, fmt.Errorf("Agent角色不存在: %w", err)
	}

	result := &PreviewAgentConfigResult{
		AgentID:   agentRoleID.String(),
		AgentName: agent.Name,
		Skills:    []PreviewAssetItem{},
		Commands:  []PreviewAssetItem{},
		Subagents: []PreviewAssetItem{},
		Rules:     []PreviewAssetItem{},
		Settings:  []PreviewAssetItem{},
	}

	// 用于收集所有 Skill（去重）
	skillMap := make(map[uuid.UUID]*PreviewAssetItem)

	// 2. 获取直接绑定的技能
	directSkillIDs, err := s.bindingRepo.FindByAgentRoleID(ctx, agentRoleID)
	if err != nil {
		s.logger.Warn("获取绑定的Skill失败", zap.Error(err))
	}
	for _, skillID := range directSkillIDs {
		skill, err := s.skillRepo.FindByID(ctx, skillID)
		if err != nil {
			s.logger.Warn("获取Skill失败", zap.String("skill_id", skillID.String()), zap.Error(err))
			continue
		}
		skillMap[skillID] = &PreviewAssetItem{
			ID:          skillID.String(),
			Name:        skill.Name,
			Description: skill.Description,
		}
	}

	// 3. 获取绑定的Commands及其关联Skills
	commandIDs, err := s.agentCommandBindingRepo.FindByAgentRoleID(ctx, agentRoleID)
	if err != nil {
		s.logger.Warn("获取绑定的Command失败", zap.Error(err))
	}
	for _, commandID := range commandIDs {
		command, err := s.commandRepo.FindByID(ctx, commandID)
		if err != nil {
			s.logger.Warn("获取Command失败", zap.String("command_id", commandID.String()), zap.Error(err))
			continue
		}
		result.Commands = append(result.Commands, PreviewAssetItem{
			ID:          commandID.String(),
			Name:        command.Name,
			Description: command.Description,
		})

		// 收集Command关联的Skill
		commandSkillIDs, err := s.commandSkillBindingRepo.FindByCommandID(ctx, commandID)
		if err != nil {
			s.logger.Warn("获取Command关联的Skill失败", zap.Error(err))
			continue
		}
		for _, skillID := range commandSkillIDs {
			if _, exists := skillMap[skillID]; !exists {
				skill, err := s.skillRepo.FindByID(ctx, skillID)
				if err != nil {
					s.logger.Warn("获取Skill失败", zap.String("skill_id", skillID.String()), zap.Error(err))
					continue
				}
				skillMap[skillID] = &PreviewAssetItem{
					ID:          skillID.String(),
					Name:        skill.Name,
					Description: skill.Description,
				}
			}
		}
	}

	// 4. 获取绑定的Subagents及其关联Skills
	subagents, err := s.agentSubagentBindingRepo.FindSubagentsByAgentRoleID(ctx, agentRoleID)
	if err != nil {
		s.logger.Warn("获取绑定的Subagent失败", zap.Error(err))
	}
	for _, subagent := range subagents {
		result.Subagents = append(result.Subagents, PreviewAssetItem{
			ID:          subagent.ID.String(),
			Name:        subagent.Name,
			Description: subagent.Description,
		})

		// 收集Subagent关联的Skill
		subagentSkillIDs, err := s.subagentSkillBindingRepo.FindBySubagentID(ctx, subagent.ID)
		if err != nil {
			s.logger.Warn("获取Subagent关联的Skill失败", zap.Error(err))
			continue
		}
		for _, skillID := range subagentSkillIDs {
			if _, exists := skillMap[skillID]; !exists {
				skill, err := s.skillRepo.FindByID(ctx, skillID)
				if err != nil {
					s.logger.Warn("获取Skill失败", zap.String("skill_id", skillID.String()), zap.Error(err))
					continue
				}
				skillMap[skillID] = &PreviewAssetItem{
					ID:          skillID.String(),
					Name:        skill.Name,
					Description: skill.Description,
				}
			}
		}
	}

	// 5. 获取绑定的Rules
	ruleIDs, err := s.agentRuleBindingRepo.FindByAgentRoleID(ctx, agentRoleID)
	if err != nil {
		s.logger.Warn("获取绑定的Rule失败", zap.Error(err))
	}
	for _, ruleID := range ruleIDs {
		rule, err := s.ruleRepo.FindByID(ctx, ruleID)
		if err != nil {
			s.logger.Warn("获取Rule失败", zap.String("rule_id", ruleID.String()), zap.Error(err))
			continue
		}
		result.Rules = append(result.Rules, PreviewAssetItem{
			ID:          ruleID.String(),
			Name:        rule.Name,
			Description: rule.Description,
		})
	}

	// 6. 获取绑定的Settings
	settingsIDs, err := s.agentSettingsBindingRepo.FindByAgentRoleID(ctx, agentRoleID)
	if err != nil {
		s.logger.Warn("获取绑定的Settings失败", zap.Error(err))
	}
	for _, settingsID := range settingsIDs {
		settings, err := s.settingsRepo.FindByID(ctx, settingsID)
		if err != nil {
			s.logger.Warn("获取Settings失败", zap.String("settings_id", settingsID.String()), zap.Error(err))
			continue
		}
		result.Settings = append(result.Settings, PreviewAssetItem{
			ID:          settingsID.String(),
			Name:        settings.Name,
			Description: settings.Description,
		})
	}

	// 7. 转换skillMap为列表
	for _, skill := range skillMap {
		result.Skills = append(result.Skills, *skill)
	}

	// 8. 设置计数
	result.SkillsCount = len(result.Skills)
	result.CommandsCount = len(result.Commands)
	result.SubagentsCount = len(result.Subagents)
	result.RulesCount = len(result.Rules)
	result.SettingsCount = len(result.Settings)

	return result, nil
}