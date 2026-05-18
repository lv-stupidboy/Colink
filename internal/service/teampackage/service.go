// 文件路径: isdp/internal/service/teampackage/service.go
package teampackage

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/configgen"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ConfigGenerator 配置生成器接口（简化依赖）
type ConfigGenerator interface {
	GenerateSync(ctx context.Context, agentRoleID uuid.UUID) error
}

// Service 团队包业务服务
type Service struct {
	workflowRepo            *repo.WorkflowTemplateRepository
	agentRepo               *repo.AgentConfigRepository
	baseAgentRepo           *repo.BaseAgentRepository // 基础Agent Repository
	skillRepo               *repo.SkillRepository
	commandRepo             *repo.CommandRepository
	subagentRepo            *repo.SubagentRepository
	ruleRepo                *repo.RuleRepository
	settingsRepo            *repo.SettingsRepository
	agentSkillBindingRepo   *repo.AgentSkillBindingRepository
	agentCommandBindingRepo *repo.AgentCommandBindingRepository
	agentSubagentBindingRepo *repo.AgentSubagentBindingRepository
	agentRuleBindingRepo    *repo.AgentRuleBindingRepository
	agentSettingsBindingRepo *repo.AgentSettingsBindingRepository
	commandSkillBindingRepo *repo.CommandSkillBindingRepository
	subagentSkillBindingRepo *repo.SubagentSkillBindingRepository
	skillStoragePath        string
	subagentStoragePath     string
	commandStoragePath      string
	ruleStoragePath         string
	settingsStoragePath     string
	logger                  *zap.Logger
	// 自动配置生成器（可选，用于导入角色后自动生成配置）
	autoGenerator           ConfigGenerator
}

// NewService 创建 TeamPackage Service
func NewService(
	workflowRepo *repo.WorkflowTemplateRepository,
	agentRepo *repo.AgentConfigRepository,
	baseAgentRepo *repo.BaseAgentRepository,
	skillRepo *repo.SkillRepository,
	commandRepo *repo.CommandRepository,
	subagentRepo *repo.SubagentRepository,
	ruleRepo *repo.RuleRepository,
	settingsRepo *repo.SettingsRepository,
	agentSkillBindingRepo *repo.AgentSkillBindingRepository,
	agentCommandBindingRepo *repo.AgentCommandBindingRepository,
	agentSubagentBindingRepo *repo.AgentSubagentBindingRepository,
	agentRuleBindingRepo *repo.AgentRuleBindingRepository,
	agentSettingsBindingRepo *repo.AgentSettingsBindingRepository,
	commandSkillBindingRepo *repo.CommandSkillBindingRepository,
	subagentSkillBindingRepo *repo.SubagentSkillBindingRepository,
	skillStoragePath string,
	subagentStoragePath string,
	commandStoragePath string,
	ruleStoragePath string,
	settingsStoragePath string,
	logger *zap.Logger,
) *Service {
	return &Service{
		workflowRepo:            workflowRepo,
		agentRepo:               agentRepo,
		baseAgentRepo:           baseAgentRepo,
		skillRepo:               skillRepo,
		commandRepo:             commandRepo,
		subagentRepo:            subagentRepo,
		ruleRepo:                ruleRepo,
		settingsRepo:            settingsRepo,
		agentSkillBindingRepo:   agentSkillBindingRepo,
		agentCommandBindingRepo: agentCommandBindingRepo,
		agentSubagentBindingRepo: agentSubagentBindingRepo,
		agentRuleBindingRepo:    agentRuleBindingRepo,
		agentSettingsBindingRepo: agentSettingsBindingRepo,
		commandSkillBindingRepo: commandSkillBindingRepo,
		subagentSkillBindingRepo: subagentSkillBindingRepo,
		skillStoragePath:        skillStoragePath,
		subagentStoragePath:     subagentStoragePath,
		commandStoragePath:      commandStoragePath,
		ruleStoragePath:         ruleStoragePath,
		settingsStoragePath:     settingsStoragePath,
		logger:                  logger,
	}
}

// SetAutoGenerator 设置自动配置生成器
// 导入角色后会调用此生成器自动生成配置
func (s *Service) SetAutoGenerator(generator ConfigGenerator) {
	s.autoGenerator = generator
}

// 确保 configgen.AutoGenerator 实现 ConfigGenerator 接口
var _ ConfigGenerator = (*configgen.AutoGenerator)(nil)

// Export 导出团队包
func (s *Service) Export(ctx context.Context, workflowID string) ([]byte, string, error) {
	now := time.Now()

	// 解析 workflow ID
	wfID, err := uuid.Parse(workflowID)
	if err != nil {
		return nil, "", fmt.Errorf("无效的工作流ID: %w", err)
	}

	// 获取工作流详情
	workflow, err := s.workflowRepo.FindByID(ctx, wfID)
	if err != nil {
		return nil, "", fmt.Errorf("获取工作流失败: %w", err)
	}

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "team-package-export-*")
	if err != nil {
		return nil, "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 解析 AgentIDs
	var agentIDs []string
	if len(workflow.AgentIDs) > 0 {
		if err := json.Unmarshal(workflow.AgentIDs, &agentIDs); err != nil {
			s.logger.Warn("解析 AgentIDs 失败", zap.Error(err))
			agentIDs = []string{}
		}
	}

	// 解析 Transitions
	var transitions []model.Transition
	if len(workflow.Transitions) > 0 {
		if err := json.Unmarshal(workflow.Transitions, &transitions); err != nil {
			s.logger.Warn("解析 Transitions 失败", zap.Error(err))
			transitions = []model.Transition{}
		}
	}

	// 解析 Checkpoints
	var checkpoints []string
	if len(workflow.Checkpoints) > 0 {
		if err := json.Unmarshal(workflow.Checkpoints, &checkpoints); err != nil {
			s.logger.Warn("解析 Checkpoints 失败", zap.Error(err))
			checkpoints = []string{}
		}
	}

	// 构建 manifest
	manifest := &model.TeamPackageManifest{
		ExportedAt: now.Format(time.RFC3339),
		Workflow: model.TeamPackageWorkflow{
			ID:            workflow.ID.String(),
			Name:          workflow.Name,
			Description:   workflow.Description,
			AgentIDs:      agentIDs,
			Transitions:   transitions,
			Checkpoints:   checkpoints,
			EstimatedTime: workflow.EstimatedTime,
			IsSystem:      workflow.IsSystem,
			IsDefault:     workflow.IsDefault,
		},
		Roles:  []model.TeamPackageRole{},
		Assets: model.TeamPackageAssets{},
	}

	// 收集角色及其绑定关系
	for _, agentIDStr := range agentIDs {
		agentID, err := uuid.Parse(agentIDStr)
		if err != nil {
			s.logger.Warn("解析角色ID失败，跳过", zap.String("agentId", agentIDStr), zap.Error(err))
			continue
		}

		agent, err := s.agentRepo.FindByID(ctx, agentID)
		if err != nil {
			s.logger.Warn("获取角色失败，跳过", zap.String("agentId", agentIDStr), zap.Error(err))
			continue
		}

		// 获取角色绑定关系
		bindings := model.TeamPackageBindings{}

		// Skills
		skillIDs, err := s.agentSkillBindingRepo.FindByAgentRoleID(ctx, agentID)
		if err != nil {
			s.logger.Warn("获取角色绑定的技能失败", zap.Error(err))
		}
		for _, skillID := range skillIDs {
			skill, err := s.skillRepo.FindByID(ctx, skillID)
			if err == nil {
				bindings.Skills = append(bindings.Skills, skill.Name)
			}
		}

		// Commands
		commandIDs, err := s.agentCommandBindingRepo.FindByAgentRoleID(ctx, agentID)
		if err != nil {
			s.logger.Warn("获取角色绑定的命令失败", zap.Error(err))
		}
		for _, commandID := range commandIDs {
			command, err := s.commandRepo.FindByID(ctx, commandID)
			if err == nil {
				bindings.Commands = append(bindings.Commands, command.Name)
			}
		}

		// Subagents
		subagentIDs, err := s.agentSubagentBindingRepo.FindByAgentRoleID(ctx, agentID)
		if err != nil {
			s.logger.Warn("获取角色绑定的子代理失败", zap.Error(err))
		}
		for _, subagentID := range subagentIDs {
			subagent, err := s.subagentRepo.FindByID(ctx, subagentID)
			if err == nil {
				bindings.Subagents = append(bindings.Subagents, subagent.Name)
			}
		}

		// Rules
		ruleIDs, err := s.agentRuleBindingRepo.FindByAgentRoleID(ctx, agentID)
		if err != nil {
			s.logger.Warn("获取角色绑定的规约失败", zap.Error(err))
		}
		for _, ruleID := range ruleIDs {
			rule, err := s.ruleRepo.FindByID(ctx, ruleID)
			if err == nil {
				bindings.Rules = append(bindings.Rules, rule.Name)
			}
		}

		// Settings
		settingsIDs, err := s.agentSettingsBindingRepo.FindByAgentRoleID(ctx, agentID)
		if err != nil {
			s.logger.Warn("获取角色绑定的配置失败", zap.Error(err))
		}
		for _, settingsID := range settingsIDs {
			settingsRecord, err := s.settingsRepo.FindByID(ctx, settingsID)
			if err == nil {
				bindings.Settings = append(bindings.Settings, settingsRecord.Name)
			}
		}

		// 添加角色到 manifest
		roleData := model.TeamPackageRole{
			ID:              agent.ID.String(),
			Name:            agent.Name,
			Role:            string(agent.Role),
			Description:     agent.Description,
			SystemPrompt:    agent.SystemPrompt,
			MaxTokens:       agent.MaxTokens,
			Temperature:     agent.Temperature,
			RequiresHuman:   agent.RequiresHuman,
			MentionPatterns: agent.MentionPatterns,
			Bindings:        bindings,
		}
		// 不导出 BaseAgentID 和 BaseAgentName（用户确认不导出，避免跨客户端 UUID 不匹配问题）
		manifest.Roles = append(manifest.Roles, roleData)
	}

	// 收集所有资产文件
	// 首先收集所有涉及的资产名称
	allSkillNames := make(map[string]bool)
	allCommandNames := make(map[string]bool)
	allSubagentNames := make(map[string]bool)
	allRuleNames := make(map[string]bool)
	allSettingsNames := make(map[string]bool)

	for _, role := range manifest.Roles {
		for _, name := range role.Bindings.Skills {
			allSkillNames[name] = true
		}
		for _, name := range role.Bindings.Commands {
			allCommandNames[name] = true
		}
		for _, name := range role.Bindings.Subagents {
			allSubagentNames[name] = true
		}
		for _, name := range role.Bindings.Rules {
			allRuleNames[name] = true
		}
		for _, name := range role.Bindings.Settings {
			allSettingsNames[name] = true
		}
	}

	// 预先收集 Command/Subagent 绑定的 Skills，确保它们也被导出
	for commandName := range allCommandNames {
		command, err := s.commandRepo.FindByName(ctx, commandName)
		if err != nil {
			continue
		}
		if s.commandSkillBindingRepo != nil {
			skills, err := s.commandSkillBindingRepo.FindSkillsByCommandID(ctx, command.ID)
			if err == nil {
				for _, skill := range skills {
					allSkillNames[skill.Name] = true
				}
			}
		}
	}

	for subagentName := range allSubagentNames {
		subagent, err := s.subagentRepo.FindByName(ctx, subagentName)
		if err != nil {
			continue
		}
		if s.subagentSkillBindingRepo != nil {
			skills, err := s.subagentSkillBindingRepo.FindSkillsBySubagentID(ctx, subagent.ID)
			if err == nil {
				for _, skill := range skills {
					allSkillNames[skill.Name] = true
				}
			}
		}
	}

	// 导出 Skills
	for skillName := range allSkillNames {
		skill, err := s.skillRepo.FindByName(ctx, skillName)
		if err != nil {
			s.logger.Warn("导出技能失败，跳过", zap.String("skill", skillName), zap.Error(err))
			continue
		}

		// 复制 Skill 目录到临时目录
		skillDir := filepath.Join(s.skillStoragePath, skill.ID.String())
		targetDir := filepath.Join(tempDir, "assets", "skills", skill.Name)
		if err := copyDir(skillDir, targetDir); err != nil {
			s.logger.Warn("复制技能目录失败，跳过", zap.String("skill", skill.Name), zap.Error(err))
			continue
		}

		manifest.Assets.Skills = append(manifest.Assets.Skills, model.AssetPackageSkillItem{
			Name:            skill.Name,
			Description:     skill.Description,
			Tags:            skill.Tags,
			SupportedAgents: skill.SupportedAgents,
			IsPublic:        skill.IsPublic,
			SourceType:      skill.SourceType,
		})
	}

	// 导出 Commands
	for commandName := range allCommandNames {
		command, err := s.commandRepo.FindByName(ctx, commandName)
		if err != nil {
			s.logger.Warn("导出命令失败，跳过", zap.String("command", commandName), zap.Error(err))
			continue
		}

		// 复制 Command 文件到临时目录
		commandFile := filepath.Join(s.commandStoragePath, command.Name+".md")
		targetFile := filepath.Join(tempDir, "assets", "commands", command.Name+".md")
		if err := copyFile(commandFile, targetFile); err != nil {
			s.logger.Warn("复制命令文件失败，跳过", zap.String("command", command.Name), zap.Error(err))
			continue
		}

		// 获取绑定的 Skills
		boundSkills := []string{}
		if s.commandSkillBindingRepo != nil {
			skills, err := s.commandSkillBindingRepo.FindSkillsByCommandID(ctx, command.ID)
			if err == nil {
				for _, skill := range skills {
					boundSkills = append(boundSkills, skill.Name)
				}
			}
		}

		manifest.Assets.Commands = append(manifest.Assets.Commands, model.AssetPackageCommandItem{
			Name:        command.Name,
			Description: command.Description,
			BoundSkills: boundSkills,
		})
	}

	// 导出 Subagents
	for subagentName := range allSubagentNames {
		subagent, err := s.subagentRepo.FindByName(ctx, subagentName)
		if err != nil {
			s.logger.Warn("导出子代理失败，跳过", zap.String("subagent", subagentName), zap.Error(err))
			continue
		}

		// 复制 Subagent 文件到临时目录
		subagentFile := filepath.Join(s.subagentStoragePath, subagent.Name+".md")
		targetFile := filepath.Join(tempDir, "assets", "subagents", subagent.Name+".md")
		if err := copyFile(subagentFile, targetFile); err != nil {
			s.logger.Warn("复制子代理文件失败，跳过", zap.String("subagent", subagent.Name), zap.Error(err))
			continue
		}

		// 获取绑定的 Skills
		boundSkills := []string{}
		if s.subagentSkillBindingRepo != nil {
			skills, err := s.subagentSkillBindingRepo.FindSkillsBySubagentID(ctx, subagent.ID)
			if err == nil {
				for _, skill := range skills {
					boundSkills = append(boundSkills, skill.Name)
				}
			}
		}

		manifest.Assets.Subagents = append(manifest.Assets.Subagents, model.AssetPackageSubagentItem{
			Name:        subagent.Name,
			Description: subagent.Description,
			BoundSkills: boundSkills,
		})
	}

	// 导出 Rules
	for ruleName := range allRuleNames {
		rule, err := s.ruleRepo.FindByName(ctx, ruleName)
		if err != nil {
			s.logger.Warn("导出规约失败，跳过", zap.String("rule", ruleName), zap.Error(err))
			continue
		}

		// 复制 Rule 文件到临时目录
		ruleFile := filepath.Join(s.ruleStoragePath, rule.Name+".md")
		targetFile := filepath.Join(tempDir, "assets", "rules", rule.Name+".md")
		if err := copyFile(ruleFile, targetFile); err != nil {
			s.logger.Warn("复制规约文件失败，跳过", zap.String("rule", rule.Name), zap.Error(err))
			continue
		}

		manifest.Assets.Rules = append(manifest.Assets.Rules, model.AssetPackageRuleItem{
			Name:        rule.Name,
			Description: rule.Description,
		})
	}

	// 导出 Settings
	for settingsName := range allSettingsNames {
		settingsRecord, err := s.settingsRepo.FindByName(ctx, settingsName)
		if err != nil {
			s.logger.Warn("导出配置失败，跳过", zap.String("settings", settingsName), zap.Error(err))
			continue
		}

		// 复制 Settings 目录到临时目录
		settingsDir := settingsRecord.DirectoryPath
		targetDir := filepath.Join(tempDir, "assets", "settings", settingsRecord.Name)
		if settingsDir != "" {
			if err := copyDir(settingsDir, targetDir); err != nil {
				s.logger.Warn("复制配置目录失败，跳过", zap.String("settings", settingsRecord.Name), zap.Error(err))
				continue
			}
		}

		manifest.Assets.Settings = append(manifest.Assets.Settings, model.AssetPackageSettingsItem{
			Name:        settingsRecord.Name,
			Description: settingsRecord.Description,
		})
	}

	// 写入 manifest.json
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("生成 manifest 失败: %w", err)
	}
	manifestPath := filepath.Join(tempDir, "manifest.json")
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return nil, "", fmt.Errorf("写入 manifest 失败: %w", err)
	}

	// 创建 ZIP 文件
	zipData, err := createZip(tempDir)
	if err != nil {
		return nil, "", fmt.Errorf("创建 ZIP 失败: %w", err)
	}

	// 生成文件名: team-{workflow-name}-{timestamp}.zip
	timestamp := now.Format("20060102-150405")
	filename := fmt.Sprintf("team-%s-%s.zip", workflow.Name, timestamp)

	s.logger.Info("导出团队包成功",
		zap.String("filename", filename),
		zap.String("workflow", workflow.Name),
		zap.Int("roles", len(manifest.Roles)),
		zap.Int("skills", len(manifest.Assets.Skills)),
		zap.Int("commands", len(manifest.Assets.Commands)),
		zap.Int("subagents", len(manifest.Assets.Subagents)),
		zap.Int("rules", len(manifest.Assets.Rules)),
		zap.Int("settings", len(manifest.Assets.Settings)))

	return zipData, filename, nil
}

// ImportPreview 导入预览
func (s *Service) ImportPreview(ctx context.Context, zipData []byte) (*model.TeamPackagePreview, error) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "team-package-preview-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 解压 ZIP
	if err := extractZip(bytes.NewReader(zipData), tempDir); err != nil {
		return nil, fmt.Errorf("解压 ZIP 失败: %w", err)
	}

	// 解析 manifest
	manifestPath := filepath.Join(tempDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("读取 manifest 失败: %w", err)
	}
	var manifest model.TeamPackageManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("解析 manifest 失败: %w", err)
	}

	preview := &model.TeamPackagePreview{
		Workflow: model.TeamPackagePreviewWorkflow{
			Name:   manifest.Workflow.Name,
			Exists: false,
		},
		Roles:  []model.TeamPackagePreviewRole{},
		Assets: model.TeamPackagePreviewAssets{
			Skills:    []model.TeamPackagePreviewAsset{},
			Commands:  []model.TeamPackagePreviewAsset{},
			Subagents: []model.TeamPackagePreviewAsset{},
			Rules:     []model.TeamPackagePreviewAsset{},
			Settings:  []model.TeamPackagePreviewAsset{},
		},
	}

	// 检查工作流是否已存在
	workflows, err := s.workflowRepo.FindAll(ctx)
	if err != nil {
		s.logger.Warn("获取工作流列表失败", zap.Error(err))
	} else {
		for _, wf := range workflows {
			if wf.Name == manifest.Workflow.Name {
				preview.Workflow.Exists = true
				break
			}
		}
	}

	// 检查角色是否已存在（按ID匹配，因为角色名称允许重复）
	for _, role := range manifest.Roles {
		previewRole := model.TeamPackagePreviewRole{
			Name:   role.Name,
			Exists: false,
		}
		// 按ID检查角色是否存在
		roleID, err := uuid.Parse(role.ID)
		if err == nil {
			existing, err := s.agentRepo.FindByID(ctx, roleID)
			if err == nil && existing != nil {
				previewRole.Exists = true
				previewRole.LocalID = existing.ID.String()
			}
		}
		preview.Roles = append(preview.Roles, previewRole)
	}

	// 检查资产是否已存在
	for _, skill := range manifest.Assets.Skills {
		exists := false
		existing, err := s.skillRepo.FindByName(ctx, skill.Name)
		if err == nil && existing != nil {
			exists = true
		}
		preview.Assets.Skills = append(preview.Assets.Skills, model.TeamPackagePreviewAsset{
			Name:   skill.Name,
			Exists: exists,
		})
	}

	for _, command := range manifest.Assets.Commands {
		exists := false
		existing, err := s.commandRepo.FindByName(ctx, command.Name)
		if err == nil && existing != nil {
			exists = true
		}
		preview.Assets.Commands = append(preview.Assets.Commands, model.TeamPackagePreviewAsset{
			Name:   command.Name,
			Exists: exists,
		})
	}

	for _, subagent := range manifest.Assets.Subagents {
		exists := false
		existing, err := s.subagentRepo.FindByName(ctx, subagent.Name)
		if err == nil && existing != nil {
			exists = true
		}
		preview.Assets.Subagents = append(preview.Assets.Subagents, model.TeamPackagePreviewAsset{
			Name:   subagent.Name,
			Exists: exists,
		})
	}

	for _, rule := range manifest.Assets.Rules {
		exists := false
		existing, err := s.ruleRepo.FindByName(ctx, rule.Name)
		if err == nil && existing != nil {
			exists = true
		}
		preview.Assets.Rules = append(preview.Assets.Rules, model.TeamPackagePreviewAsset{
			Name:   rule.Name,
			Exists: exists,
		})
	}

	for _, settings := range manifest.Assets.Settings {
		exists := false
		existing, err := s.settingsRepo.FindByName(ctx, settings.Name)
		if err == nil && existing != nil {
			exists = true
		}
		preview.Assets.Settings = append(preview.Assets.Settings, model.TeamPackagePreviewAsset{
			Name:   settings.Name,
			Exists: exists,
		})
	}

	s.logger.Info("团队包预览完成",
		zap.String("workflow", manifest.Workflow.Name),
		zap.Bool("workflowExists", preview.Workflow.Exists),
		zap.Int("roles", len(preview.Roles)))

	return preview, nil
}

// ImportConfirm 确认导入
func (s *Service) ImportConfirm(ctx context.Context, zipData []byte, confirm *model.TeamPackageImportConfirm) (*model.ImportResult, error) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "team-package-import-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 解压 ZIP
	if err := extractZip(bytes.NewReader(zipData), tempDir); err != nil {
		return nil, fmt.Errorf("解压 ZIP 失败: %w", err)
	}

	// 解析 manifest
	manifestPath := filepath.Join(tempDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("读取 manifest 失败: %w", err)
	}
	var manifest model.TeamPackageManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("解析 manifest 失败: %w", err)
	}

	s.logger.Info("解析团队包 manifest 完成",
		zap.Int("roles", len(manifest.Roles)),
		zap.Int("skills", len(manifest.Assets.Skills)),
		zap.Int("commands", len(manifest.Assets.Commands)),
		zap.Int("subagents", len(manifest.Assets.Subagents)))

	result := &model.ImportResult{
		Success: 0,
		Skipped: 0,
		Failed:  0,
		Details: make([]model.ImportDetail, 0),
	}

	// 资产名称到ID的映射
	skillNameToID := make(map[string]uuid.UUID)
	commandNameToID := make(map[string]uuid.UUID)
	subagentNameToID := make(map[string]uuid.UUID)
	ruleNameToID := make(map[string]uuid.UUID)
	settingsNameToID := make(map[string]uuid.UUID)
	roleNameToID := make(map[string]uuid.UUID)
	// 原始角色ID到新ID的映射（用于更新workflow的agentIds）
	originalRoleIDToNewID := make(map[string]uuid.UUID)

	// 导入资产
	// 导入 Skills
	for _, skillItem := range manifest.Assets.Skills {
		action := getAssetAction(confirm.AssetActions, "skill", skillItem.Name)
		if action == "skip" {
			// 获取已存在的 ID
			existing, _ := s.skillRepo.FindByName(ctx, skillItem.Name)
			if existing != nil {
				skillNameToID[skillItem.Name] = existing.ID
			}
			result.Skipped++
			result.Details = append(result.Details, model.ImportDetail{
				AssetType: "skill",
				Name:      skillItem.Name,
				Status:    "skipped",
				Message:   "用户选择跳过",
			})
			continue
		}

		id, detail := s.importSkill(ctx, tempDir, skillItem, action == "overwrite")
		result.Details = append(result.Details, detail)
		switch detail.Status {
		case "success":
			result.Success++
			skillNameToID[skillItem.Name] = id
		case "skipped":
			result.Skipped++
			existing, _ := s.skillRepo.FindByName(ctx, skillItem.Name)
			if existing != nil {
				skillNameToID[skillItem.Name] = existing.ID
			}
		case "failed":
			result.Failed++
		}
	}

	// 导入 Commands
	for _, commandItem := range manifest.Assets.Commands {
		action := getAssetAction(confirm.AssetActions, "command", commandItem.Name)
		if action == "skip" {
			existing, _ := s.commandRepo.FindByName(ctx, commandItem.Name)
			if existing != nil {
				commandNameToID[commandItem.Name] = existing.ID
				// 用户选择跳过时也要更新绑定关系
				s.bindSkillsToCommand(ctx, existing.ID, commandItem.BoundSkills, skillNameToID)
			}
			result.Skipped++
			result.Details = append(result.Details, model.ImportDetail{
				AssetType: "command",
				Name:      commandItem.Name,
				Status:    "skipped",
				Message:   "用户选择跳过",
			})
			continue
		}

		id, detail := s.importCommand(ctx, tempDir, commandItem, action == "overwrite")
		result.Details = append(result.Details, detail)
		switch detail.Status {
		case "success":
			result.Success++
			commandNameToID[commandItem.Name] = id
			s.bindSkillsToCommand(ctx, id, commandItem.BoundSkills, skillNameToID)
		case "skipped":
			result.Skipped++
			existing, _ := s.commandRepo.FindByName(ctx, commandItem.Name)
			if existing != nil {
				commandNameToID[commandItem.Name] = existing.ID
				// 被跳过的 Command 也要更新绑定关系
				s.bindSkillsToCommand(ctx, existing.ID, commandItem.BoundSkills, skillNameToID)
			}
		case "failed":
			result.Failed++
		}
	}

	// 导入 Subagents
	for _, subagentItem := range manifest.Assets.Subagents {
		action := getAssetAction(confirm.AssetActions, "subagent", subagentItem.Name)
		if action == "skip" {
			existing, _ := s.subagentRepo.FindByName(ctx, subagentItem.Name)
			if existing != nil {
				subagentNameToID[subagentItem.Name] = existing.ID
			}
				// 用户选择跳过时也要更新绑定关系
				s.bindSkillsToSubagent(ctx, existing.ID, subagentItem.BoundSkills, skillNameToID)
			result.Skipped++
			result.Details = append(result.Details, model.ImportDetail{
				AssetType: "subagent",
				Name:      subagentItem.Name,
				Status:    "skipped",
				Message:   "用户选择跳过",
			})
			continue
		}

		id, detail := s.importSubagent(ctx, tempDir, subagentItem, action == "overwrite")
		result.Details = append(result.Details, detail)
		switch detail.Status {
		case "success":
			result.Success++
			subagentNameToID[subagentItem.Name] = id
			// 绑定 Skills
			s.bindSkillsToSubagent(ctx, id, subagentItem.BoundSkills, skillNameToID)
		case "skipped":
			result.Skipped++
			existing, _ := s.subagentRepo.FindByName(ctx, subagentItem.Name)
			if existing != nil {
				subagentNameToID[subagentItem.Name] = existing.ID
				// 被跳过的 Subagent 也要更新绑定关系
				s.bindSkillsToSubagent(ctx, existing.ID, subagentItem.BoundSkills, skillNameToID)
			}
		case "failed":
			result.Failed++
		}
	}

	// 导入 Rules
	for _, ruleItem := range manifest.Assets.Rules {
		action := getAssetAction(confirm.AssetActions, "rule", ruleItem.Name)
		if action == "skip" {
			existing, _ := s.ruleRepo.FindByName(ctx, ruleItem.Name)
			if existing != nil {
				ruleNameToID[ruleItem.Name] = existing.ID
			}
			result.Skipped++
			result.Details = append(result.Details, model.ImportDetail{
				AssetType: "rule",
				Name:      ruleItem.Name,
				Status:    "skipped",
				Message:   "用户选择跳过",
			})
			continue
		}

		id, detail := s.importRule(ctx, tempDir, ruleItem, action == "overwrite")
		result.Details = append(result.Details, detail)
		switch detail.Status {
		case "success":
			result.Success++
			ruleNameToID[ruleItem.Name] = id
		case "skipped":
			result.Skipped++
			existing, _ := s.ruleRepo.FindByName(ctx, ruleItem.Name)
			if existing != nil {
				ruleNameToID[ruleItem.Name] = existing.ID
			}
		case "failed":
			result.Failed++
		}
	}

	// 导入 Settings
	for _, settingsItem := range manifest.Assets.Settings {
		action := getAssetAction(confirm.AssetActions, "settings", settingsItem.Name)
		if action == "skip" {
			existing, _ := s.settingsRepo.FindByName(ctx, settingsItem.Name)
			if existing != nil {
				settingsNameToID[settingsItem.Name] = existing.ID
			}
			result.Skipped++
			result.Details = append(result.Details, model.ImportDetail{
				AssetType: "settings",
				Name:      settingsItem.Name,
				Status:    "skipped",
				Message:   "用户选择跳过",
			})
			continue
		}

		id, detail := s.importSettings(ctx, tempDir, settingsItem, action == "overwrite")
		result.Details = append(result.Details, detail)
		switch detail.Status {
		case "success":
			result.Success++
			settingsNameToID[settingsItem.Name] = id
		case "skipped":
			result.Skipped++
			existing, _ := s.settingsRepo.FindByName(ctx, settingsItem.Name)
			if existing != nil {
				settingsNameToID[settingsItem.Name] = existing.ID
			}
		case "failed":
			result.Failed++
		}
	}

	// 导入角色
	for _, roleItem := range manifest.Roles {
		action := getRoleAction(confirm.RoleActions, roleItem.Name)
		if action == "skip" {
			// 按名称查找已存在的角色
			agents, _ := s.agentRepo.List(ctx)
			for _, agent := range agents {
				if agent.Name == roleItem.Name {
					roleNameToID[roleItem.Name] = agent.ID
					// 也更新 originalRoleIDToNewID 映射（如果有有效原始ID）
					if _, err := uuid.Parse(roleItem.ID); err == nil {
						originalRoleIDToNewID[roleItem.ID] = agent.ID
					}
					break
				}
			}
			result.Skipped++
			result.Details = append(result.Details, model.ImportDetail{
				AssetType: "role",
				Name:      roleItem.Name,
				Status:    "skipped",
				Message:   "用户选择跳过",
			})
			continue
		}

		id, originalIDStr, detail := s.importRole(ctx, roleItem, action == "overwrite")
		result.Details = append(result.Details, detail)
		switch detail.Status {
		case "success":
			result.Success++
			roleNameToID[roleItem.Name] = id
			originalRoleIDToNewID[originalIDStr] = id
		case "skipped":
			result.Skipped++
			// 查找已存在的角色
			agents, _ := s.agentRepo.List(ctx)
			for _, agent := range agents {
				if agent.Name == roleItem.Name {
					roleNameToID[roleItem.Name] = agent.ID
					originalRoleIDToNewID[originalIDStr] = agent.ID
					break
				}
			}
		case "failed":
			result.Failed++
		}
	}

	// 恢复绑定关系
	for _, roleItem := range manifest.Roles {
		roleID, ok := roleNameToID[roleItem.Name]
		if !ok {
			s.logger.Warn("角色ID未找到，无法恢复绑定", zap.String("role", roleItem.Name))
			continue
		}

		// 绑定 Skills
		for _, skillName := range roleItem.Bindings.Skills {
			skillID, ok := skillNameToID[skillName]
			if !ok {
				s.logger.Warn("技能ID未找到，无法绑定", zap.String("skill", skillName))
				continue
			}
			if err := s.createAgentSkillBinding(ctx, roleID, skillID); err != nil {
				s.logger.Warn("绑定技能失败", zap.Error(err))
			}
		}

		// 绑定 Commands
		for _, commandName := range roleItem.Bindings.Commands {
			commandID, ok := commandNameToID[commandName]
			if !ok {
				s.logger.Warn("命令ID未找到，无法绑定", zap.String("command", commandName))
				continue
			}
			if err := s.createAgentCommandBinding(ctx, roleID, commandID); err != nil {
				s.logger.Warn("绑定命令失败", zap.Error(err))
			}
		}

		// 绑定 Subagents
		for _, subagentName := range roleItem.Bindings.Subagents {
			subagentID, ok := subagentNameToID[subagentName]
			if !ok {
				s.logger.Warn("子代理ID未找到，无法绑定", zap.String("subagent", subagentName))
				continue
			}
			if err := s.createAgentSubagentBinding(ctx, roleID, subagentID); err != nil {
				s.logger.Warn("绑定子代理失败", zap.Error(err))
			}
		}

		// 绑定 Rules
		for _, ruleName := range roleItem.Bindings.Rules {
			ruleID, ok := ruleNameToID[ruleName]
			if !ok {
				s.logger.Warn("规约ID未找到，无法绑定", zap.String("rule", ruleName))
				continue
			}
			if err := s.createAgentRuleBinding(ctx, roleID, ruleID); err != nil {
				s.logger.Warn("绑定规约失败", zap.Error(err))
			}
		}

		// 绑定 Settings
		for _, settingsName := range roleItem.Bindings.Settings {
			settingsID, ok := settingsNameToID[settingsName]
			if !ok {
				s.logger.Warn("配置ID未找到，无法绑定", zap.String("settings", settingsName))
				continue
			}
			if err := s.createAgentSettingsBinding(ctx, roleID, settingsID); err != nil {
				s.logger.Warn("绑定配置失败", zap.Error(err))
			}
		}

			// 绑定完成后自动生成配置
			if s.autoGenerator != nil {
				genResult := model.ConfigGenResult{
					AgentID:   roleID.String(),
					AgentName: roleItem.Name,
				}
				if err := s.autoGenerator.GenerateSync(ctx, roleID); err != nil {
					genResult.Status = "failed"
					genResult.Message = err.Error()
					s.logger.Warn("自动生成角色配置失败",
						zap.String("roleID", roleID.String()),
						zap.Error(err))
				} else {
					genResult.Status = "success"
					genResult.Message = "配置生成成功"
					s.logger.Info("自动生成角色配置成功",
						zap.String("roleID", roleID.String()))
				}
				result.ConfigGenResults = append(result.ConfigGenResults, genResult)
			}
		}

	// 导入工作流
	if confirm.WorkflowAction != "skip" {
		_, detail := s.importWorkflow(ctx, manifest.Workflow, originalRoleIDToNewID)
		result.Details = append(result.Details, detail)
		switch detail.Status {
		case "success":
			result.Success++
		case "skipped":
			result.Skipped++
		case "failed":
			result.Failed++
		}
	} else {
		result.Skipped++
		result.Details = append(result.Details, model.ImportDetail{
			AssetType: "workflow",
			Name:      manifest.Workflow.Name,
			Status:    "skipped",
			Message:   "用户选择跳过",
		})
	}

	s.logger.Info("导入团队包完成",
		zap.Int("success", result.Success),
		zap.Int("skipped", result.Skipped),
		zap.Int("failed", result.Failed))

	return result, nil
}

// ========== 导入辅助方法 ==========

// importSkill 导入单个 Skill
// 覆盖模式下保留现有 ID，只更新内容和属性，避免断开其他团队的绑定关系
// 使用备份恢复机制确保原子性：失败时恢复原状态
func (s *Service) importSkill(ctx context.Context, tempDir string, item model.AssetPackageSkillItem, overwrite bool) (uuid.UUID, model.ImportDetail) {
	detail := model.ImportDetail{
		AssetType: "skill",
		Name:      item.Name,
	}

	// 检查是否已存在相同名称的 Skill
	existing, err := s.skillRepo.FindByName(ctx, item.Name)
	if err == nil && existing != nil {
		if !overwrite {
			detail.Status = "skipped"
			detail.Message = "已存在相同名称的 Skill"
			return existing.ID, detail
		}
		// 覆盖模式：保留现有 ID，只更新内容和属性
		// 这样不会断开其他团队对该 skill 的绑定关系

		targetDir := filepath.Join(s.skillStoragePath, existing.ID.String())
		srcDir := filepath.Join(tempDir, "assets", "skills", item.Name)

		// 创建备份目录（确保原子性）
		backupDir := filepath.Join(s.skillStoragePath, existing.ID.String()+"_backup")
		if err := copyDir(targetDir, backupDir); err != nil && !os.IsNotExist(err) {
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("创建备份目录失败: %v", err)
			return uuid.Nil, detail
		}

		// 清空目标目录并复制新内容
		if err := os.RemoveAll(targetDir); err != nil && !os.IsNotExist(err) {
			os.RemoveAll(backupDir) // 清理备份
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("清空旧 Skill 目录失败: %v", err)
			return uuid.Nil, detail
		}

		if err := copyDir(srcDir, targetDir); err != nil {
			// 失败时恢复备份
			if restoreErr := copyDir(backupDir, targetDir); restoreErr != nil {
				s.logger.Error("恢复备份失败", zap.Error(restoreErr), zap.String("backupDir", backupDir))
			}
			os.RemoveAll(backupDir)
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("复制 Skill 目录失败: %v", err)
			return uuid.Nil, detail
		}

		// 更新 Skill 属性（保留原 ID）
		existing.Description = item.Description
		existing.Tags = item.Tags
		existing.SupportedAgents = item.SupportedAgents
		existing.IsPublic = item.IsPublic
		existing.SourceType = item.SourceType

		if err := s.skillRepo.Update(ctx, existing); err != nil {
			// 数据库更新失败时恢复备份
			if restoreErr := copyDir(backupDir, targetDir); restoreErr != nil {
				s.logger.Error("恢复备份失败", zap.Error(restoreErr), zap.String("backupDir", backupDir))
			}
			os.RemoveAll(backupDir)
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("更新 Skill 记录失败: %v", err)
			return uuid.Nil, detail
		}

		// 成功后删除备份
		os.RemoveAll(backupDir)

		detail.Status = "success"
		detail.ID = existing.ID.String()
		detail.Message = "已覆盖更新现有 Skill（保留原 ID）"
		return existing.ID, detail
	}

	// 创建新 Skill（不存在同名 Skill）
	now := time.Now()
	skill := &model.Skill{
		ID:              uuid.New(),
		Name:            item.Name,
		Description:     item.Description,
		Tags:            item.Tags,
		SourceType:      model.SkillSourcePersonal,
		Status:          model.SkillStatusActive,
		IsPublic:        item.IsPublic,
		UseCount:        0,
		SupportedAgents: item.SupportedAgents,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// 复制 Skill 目录（使用 skill.ID 作为目录名）
	srcDir := filepath.Join(tempDir, "assets", "skills", item.Name)
	targetDir := filepath.Join(s.skillStoragePath, skill.ID.String())
	if err := copyDir(srcDir, targetDir); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制 Skill 目录失败: %v", err)
		return uuid.Nil, detail
	}

	if err := s.skillRepo.Create(ctx, skill); err != nil {
		os.RemoveAll(targetDir)
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("创建 Skill 记录失败: %v", err)
		return uuid.Nil, detail
	}

	detail.Status = "success"
	detail.ID = skill.ID.String()
	return skill.ID, detail
}

// importCommand 导入单个 Command
// 覆盖模式下保留现有 ID，只更新内容（文件）和属性，避免断开 AgentCommandBinding
func (s *Service) importCommand(ctx context.Context, tempDir string, item model.AssetPackageCommandItem, overwrite bool) (uuid.UUID, model.ImportDetail) {
	detail := model.ImportDetail{
		AssetType: "command",
		Name:      item.Name,
	}

	// 检查是否已存在
	existing, err := s.commandRepo.FindByName(ctx, item.Name)
	if err == nil && existing != nil {
		if !overwrite {
			detail.Status = "skipped"
			detail.Message = "已存在相同名称的 Command"
			return existing.ID, detail
		}
		// 覆盖模式：保留现有 ID，只更新文件内容和属性

		srcFile := filepath.Join(tempDir, "assets", "commands", item.Name+".md")
		targetFile := filepath.Join(s.commandStoragePath, item.Name+".md")

		// 创建备份（确保原子性）
		backupFile := filepath.Join(s.commandStoragePath, item.Name+".md_backup")
		if err := copyFile(targetFile, backupFile); err != nil && !os.IsNotExist(err) {
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("创建备份文件失败: %v", err)
			return uuid.Nil, detail
		}

		// 复制新文件内容
		if err := copyFile(srcFile, targetFile); err != nil {
			// 失败时恢复备份
			if restoreErr := copyFile(backupFile, targetFile); restoreErr != nil {
				s.logger.Error("恢复备份失败", zap.Error(restoreErr), zap.String("backupFile", backupFile))
			}
			os.Remove(backupFile)
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("复制 Command 文件失败: %v", err)
			return uuid.Nil, detail
		}

		// 更新属性（保留原 ID）
		existing.Description = item.Description

		if err := s.commandRepo.Update(ctx, existing); err != nil {
			// 数据库更新失败时恢复备份
			if restoreErr := copyFile(backupFile, targetFile); restoreErr != nil {
				s.logger.Error("恢复备份失败", zap.Error(restoreErr), zap.String("backupFile", backupFile))
			}
			os.Remove(backupFile)
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("更新 Command 记录失败: %v", err)
			return uuid.Nil, detail
		}

		// 成功后删除备份
		os.Remove(backupFile)

		detail.Status = "success"
		detail.ID = existing.ID.String()
		detail.Message = "已覆盖更新现有 Command（保留原 ID）"
		return existing.ID, detail
	}

	// 创建新 Command（不存在同名 Command）
	srcFile := filepath.Join(tempDir, "assets", "commands", item.Name+".md")
	targetFile := filepath.Join(s.commandStoragePath, item.Name+".md")
	if err := copyFile(srcFile, targetFile); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制 Command 文件失败: %v", err)
		return uuid.Nil, detail
	}

	now := time.Now()
	command := &model.Command{
		ID:          uuid.New(),
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.commandRepo.Create(ctx, command); err != nil {
		os.Remove(targetFile)
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("创建 Command 记录失败: %v", err)
		return uuid.Nil, detail
	}

	detail.Status = "success"
	detail.ID = command.ID.String()
	return command.ID, detail
}

// importSubagent 导入单个 Subagent
// 覆盖模式下保留现有 ID，只更新内容（文件）和属性，避免断开 AgentSubagentBinding
func (s *Service) importSubagent(ctx context.Context, tempDir string, item model.AssetPackageSubagentItem, overwrite bool) (uuid.UUID, model.ImportDetail) {
	detail := model.ImportDetail{
		AssetType: "subagent",
		Name:      item.Name,
	}

	// 检查是否已存在
	existing, err := s.subagentRepo.FindByName(ctx, item.Name)
	if err == nil && existing != nil {
		if !overwrite {
			detail.Status = "skipped"
			detail.Message = "已存在相同名称的 Subagent"
			return existing.ID, detail
		}
		// 覆盖模式：保留现有 ID，只更新文件内容和属性

		srcFile := filepath.Join(tempDir, "assets", "subagents", item.Name+".md")
		targetFile := filepath.Join(s.subagentStoragePath, item.Name+".md")

		// 创建备份（确保原子性）
		backupFile := filepath.Join(s.subagentStoragePath, item.Name+".md_backup")
		if err := copyFile(targetFile, backupFile); err != nil && !os.IsNotExist(err) {
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("创建备份文件失败: %v", err)
			return uuid.Nil, detail
		}

		// 复制新文件内容
		if err := copyFile(srcFile, targetFile); err != nil {
			// 失败时恢复备份
			if restoreErr := copyFile(backupFile, targetFile); restoreErr != nil {
				s.logger.Error("恢复备份失败", zap.Error(restoreErr), zap.String("backupFile", backupFile))
			}
			os.Remove(backupFile)
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("复制 Subagent 文件失败: %v", err)
			return uuid.Nil, detail
		}

		// 更新属性（保留原 ID）
		existing.Description = item.Description

		if err := s.subagentRepo.Update(ctx, existing); err != nil {
			// 数据库更新失败时恢复备份
			if restoreErr := copyFile(backupFile, targetFile); restoreErr != nil {
				s.logger.Error("恢复备份失败", zap.Error(restoreErr), zap.String("backupFile", backupFile))
			}
			os.Remove(backupFile)
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("更新 Subagent 记录失败: %v", err)
			return uuid.Nil, detail
		}

		// 成功后删除备份
		os.Remove(backupFile)

		detail.Status = "success"
		detail.ID = existing.ID.String()
		detail.Message = "已覆盖更新现有 Subagent（保留原 ID）"
		return existing.ID, detail
	}

	// 创建新 Subagent（不存在同名 Subagent）
	srcFile := filepath.Join(tempDir, "assets", "subagents", item.Name+".md")
	targetFile := filepath.Join(s.subagentStoragePath, item.Name+".md")
	if err := copyFile(srcFile, targetFile); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制 Subagent 文件失败: %v", err)
		return uuid.Nil, detail
	}

	now := time.Now()
	subagent := &model.Subagent{
		ID:          uuid.New(),
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.subagentRepo.Create(ctx, subagent); err != nil {
		os.Remove(targetFile)
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("创建 Subagent 记录失败: %v", err)
		return uuid.Nil, detail
	}

	detail.Status = "success"
	detail.ID = subagent.ID.String()
	return subagent.ID, detail
}

// importRule 导入单个 Rule
// 覆盖模式下保留现有 ID，只更新内容（文件）和属性，避免断开 AgentRuleBinding
func (s *Service) importRule(ctx context.Context, tempDir string, item model.AssetPackageRuleItem, overwrite bool) (uuid.UUID, model.ImportDetail) {
	detail := model.ImportDetail{
		AssetType: "rule",
		Name:      item.Name,
	}

	// 检查是否已存在
	existing, err := s.ruleRepo.FindByName(ctx, item.Name)
	if err == nil && existing != nil {
		if !overwrite {
			detail.Status = "skipped"
			detail.Message = "已存在相同名称的 Rule"
			return existing.ID, detail
		}
		// 覆盖模式：保留现有 ID，只更新文件内容和属性

		srcFile := filepath.Join(tempDir, "assets", "rules", item.Name+".md")
		targetFile := filepath.Join(s.ruleStoragePath, item.Name+".md")

		// 创建备份（确保原子性）
		backupFile := filepath.Join(s.ruleStoragePath, item.Name+".md_backup")
		if err := copyFile(targetFile, backupFile); err != nil && !os.IsNotExist(err) {
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("创建备份文件失败: %v", err)
			return uuid.Nil, detail
		}

		// 复制新文件内容
		if err := copyFile(srcFile, targetFile); err != nil {
			// 失败时恢复备份
			if restoreErr := copyFile(backupFile, targetFile); restoreErr != nil {
				s.logger.Error("恢复备份失败", zap.Error(restoreErr), zap.String("backupFile", backupFile))
			}
			os.Remove(backupFile)
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("复制 Rule 文件失败: %v", err)
			return uuid.Nil, detail
		}

		// 更新属性（保留原 ID）
		existing.Description = item.Description

		if err := s.ruleRepo.Update(ctx, existing); err != nil {
			// 数据库更新失败时恢复备份
			if restoreErr := copyFile(backupFile, targetFile); restoreErr != nil {
				s.logger.Error("恢复备份失败", zap.Error(restoreErr), zap.String("backupFile", backupFile))
			}
			os.Remove(backupFile)
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("更新 Rule 记录失败: %v", err)
			return uuid.Nil, detail
		}

		// 成功后删除备份
		os.Remove(backupFile)

		detail.Status = "success"
		detail.ID = existing.ID.String()
		detail.Message = "已覆盖更新现有 Rule（保留原 ID）"
		return existing.ID, detail
	}

	// 创建新 Rule（不存在同名 Rule）
	srcFile := filepath.Join(tempDir, "assets", "rules", item.Name+".md")
	targetFile := filepath.Join(s.ruleStoragePath, item.Name+".md")
	if err := copyFile(srcFile, targetFile); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制 Rule 文件失败: %v", err)
		return uuid.Nil, detail
	}

	now := time.Now()
	rule := &model.Rule{
		ID:          uuid.New(),
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.ruleRepo.Create(ctx, rule); err != nil {
		os.Remove(targetFile)
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("创建 Rule 记录失败: %v", err)
		return uuid.Nil, detail
	}

	detail.Status = "success"
	detail.ID = rule.ID.String()
	return rule.ID, detail
}

// importSettings 导入单个 Settings
// 覆盖模式下保留现有 ID，只更新目录内容和属性，避免断开 AgentSettingsBinding
func (s *Service) importSettings(ctx context.Context, tempDir string, item model.AssetPackageSettingsItem, overwrite bool) (uuid.UUID, model.ImportDetail) {
	detail := model.ImportDetail{
		AssetType: "settings",
		Name:      item.Name,
	}

	// 检查是否已存在
	existing, err := s.settingsRepo.FindByName(ctx, item.Name)
	if err == nil && existing != nil {
		if !overwrite {
			detail.Status = "skipped"
			detail.Message = "已存在相同名称的 Settings"
			return existing.ID, detail
		}
		// 覆盖模式：保留现有 ID，只更新目录内容和属性

		srcDir := filepath.Join(tempDir, "assets", "settings", item.Name)
		targetDir := filepath.Join(s.settingsStoragePath, item.Name)

		// 创建备份（确保原子性）
		backupDir := filepath.Join(s.settingsStoragePath, item.Name+"_backup")
		if existing.DirectoryPath != "" {
			if err := copyDir(existing.DirectoryPath, backupDir); err != nil && !os.IsNotExist(err) {
				detail.Status = "failed"
				detail.Message = fmt.Sprintf("创建备份目录失败: %v", err)
				return uuid.Nil, detail
			}
		}

		// 清空目标目录并复制新内容
		if existing.DirectoryPath != "" {
			if err := os.RemoveAll(existing.DirectoryPath); err != nil && !os.IsNotExist(err) {
				os.RemoveAll(backupDir) // 清理备份
				detail.Status = "failed"
				detail.Message = fmt.Sprintf("清空旧 Settings 目录失败: %v", err)
				return uuid.Nil, detail
			}
		}

		if err := copyDir(srcDir, targetDir); err != nil {
			// 失败时恢复备份
			if restoreErr := copyDir(backupDir, targetDir); restoreErr != nil {
				s.logger.Error("恢复备份失败", zap.Error(restoreErr), zap.String("backupDir", backupDir))
			}
			os.RemoveAll(backupDir)
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("复制 Settings 目录失败: %v", err)
			return uuid.Nil, detail
		}

		// 更新属性（保留原 ID）
		existing.Description = item.Description
		existing.DirectoryPath = targetDir

		if err := s.settingsRepo.Update(ctx, existing); err != nil {
			// 数据库更新失败时恢复备份
			if restoreErr := copyDir(backupDir, existing.DirectoryPath); restoreErr != nil {
				s.logger.Error("恢复备份失败", zap.Error(restoreErr), zap.String("backupDir", backupDir))
			}
			os.RemoveAll(backupDir)
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("更新 Settings 记录失败: %v", err)
			return uuid.Nil, detail
		}

		// 成功后删除备份
		os.RemoveAll(backupDir)

		detail.Status = "success"
		detail.ID = existing.ID.String()
		detail.Message = "已覆盖更新现有 Settings（保留原 ID）"
		return existing.ID, detail
	} else if err != nil && !strings.Contains(err.Error(), "not found") {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("检查配置名称失败: %v", err)
		return uuid.Nil, detail
	}

	// 创建新 Settings（不存在同名 Settings）
	srcDir := filepath.Join(tempDir, "assets", "settings", item.Name)
	targetDir := filepath.Join(s.settingsStoragePath, item.Name)
	if err := copyDir(srcDir, targetDir); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制 Settings 目录失败: %v", err)
		return uuid.Nil, detail
	}

	now := time.Now()
	settingsRecord := &model.Settings{
		ID:            uuid.New(),
		Name:          item.Name,
		Description:   item.Description,
		DirectoryPath: targetDir,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.settingsRepo.Create(ctx, settingsRecord); err != nil {
		os.RemoveAll(targetDir)
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("创建 Settings 记录失败: %v", err)
		return uuid.Nil, detail
	}

	detail.Status = "success"
	detail.ID = settingsRecord.ID.String()
	return settingsRecord.ID, detail
}

// importRole 导入角色
	// 返回值: 新ID, 原始ID字符串, 导入详情
	// 当原始ID无效时，生成新的UUID，同时返回原始ID字符串用于映射更新
	func (s *Service) importRole(ctx context.Context, role model.TeamPackageRole, overwrite bool) (uuid.UUID, string, model.ImportDetail) {
		detail := model.ImportDetail{
			AssetType: "role",
			Name:      role.Name,
		}

		// 尝试解析原始角色ID
		originalIDStr := role.ID
		originalID, err := uuid.Parse(originalIDStr)
		var roleID uuid.UUID
		var existing *model.AgentRoleConfig // 用于记录已存在的角色（覆盖时保留 BaseAgentID）

		if err != nil {
			// 原始ID不是有效UUID，生成新的UUID
			roleID = uuid.New()
			s.logger.Info("角色ID不是有效UUID，生成新ID",
				zap.String("originalID", originalIDStr),
				zap.String("newID", roleID.String()),
				zap.String("roleName", role.Name))
		} else {
			roleID = originalID
		}

		// 如果原始ID是有效UUID，按ID检查角色是否已存在
		if err == nil {
			var existErr error
			existing, existErr = s.agentRepo.FindByID(ctx, originalID)
			if existErr == nil && existing != nil {
				if !overwrite {
					detail.Status = "skipped"
					detail.Message = "已存在相同ID的 Role"
					return existing.ID, originalIDStr, detail
				}
				// 覆盖模式：先删除旧角色及其绑定关系
				if err := s.deleteRoleBindings(ctx, existing.ID); err != nil {
					s.logger.Warn("删除旧角色绑定关系失败", zap.Error(err))
				}
				if err := s.agentRepo.Delete(ctx, existing.ID); err != nil {
					detail.Status = "failed"
					detail.Message = fmt.Sprintf("删除旧 Role 记录失败: %v", err)
					return uuid.Nil, originalIDStr, detail
				}
			}
		}

		// 创建角色
		// BaseAgentID 处理策略：
		// - 覆盖已存在角色：始终保留本地原有的 BaseAgentID（包括空值）
		// - 新建角色：使用系统默认基础 Agent，如果没有默认则保持为空
		var baseAgentID uuid.UUID
		if existing != nil {
			// 覆盖模式：始终保留原有的 BaseAgentID（包括空值）
			baseAgentID = existing.BaseAgentID
			s.logger.Info("覆盖角色，保留原有 BaseAgentID",
				zap.String("roleID", roleID.String()),
				zap.Bool("baseAgentIDEmpty", baseAgentID == uuid.Nil))
		} else if s.baseAgentRepo != nil {
			// 新建角色：尝试获取系统默认基础Agent
			defaultAgent, defaultErr := s.baseAgentRepo.FindDefault(ctx)
			if defaultErr == nil && defaultAgent != nil {
				baseAgentID = defaultAgent.ID
				s.logger.Info("新建角色，使用系统默认 BaseAgent",
					zap.String("roleID", roleID.String()),
					zap.String("baseAgentID", baseAgentID.String()),
					zap.String("baseAgentName", defaultAgent.Name))
			} else {
				// 没有默认基础 Agent，保持为空
				baseAgentID = uuid.Nil
				s.logger.Info("新建角色，无默认 BaseAgent，保持为空",
					zap.String("roleID", roleID.String()))
			}
		} else {
			// baseAgentRepo 为空，无法获取默认基础 Agent，保持为空
			baseAgentID = uuid.Nil
			s.logger.Warn("新建角色，baseAgentRepo 为 nil，无法获取默认 BaseAgent",
				zap.String("roleID", roleID.String()))
		}

		now := time.Now()
		agentConfig := &model.AgentRoleConfig{
			ID:              roleID,
			Name:            role.Name,
			Role:            model.AgentRole(role.Role),
			BaseAgentID:     baseAgentID,
			Description:     role.Description,
			SystemPrompt:    role.SystemPrompt,
			MaxTokens:       role.MaxTokens,
			Temperature:     role.Temperature,
			RequiresHuman:   role.RequiresHuman,
			IsDefault:       false,
			IsSystem:        false,
			MentionPatterns: role.MentionPatterns,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if err := s.agentRepo.Create(ctx, agentConfig); err != nil {
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("创建 Role 记录失败: %v", err)
			return uuid.Nil, originalIDStr, detail
		}

		detail.Status = "success"
		detail.ID = roleID.String()
		if _, err := uuid.Parse(originalIDStr); err == nil {
			detail.Message = "角色导入成功"
		} else {
			detail.Message = fmt.Sprintf("角色导入成功（原ID无效，已生成新ID: %s）", roleID.String())
		}
		return roleID, originalIDStr, detail
	}

func (s *Service) importWorkflow(ctx context.Context, wf model.TeamPackageWorkflow, originalRoleIDToNewID map[string]uuid.UUID) (uuid.UUID, model.ImportDetail) {
	detail := model.ImportDetail{
		AssetType: "workflow",
		Name:      wf.Name,
	}

	// 检查是否已存在
	workflows, err := s.workflowRepo.FindAll(ctx)
	if err == nil {
		for _, existing := range workflows {
			if existing.Name == wf.Name {
				detail.Status = "skipped"
				detail.Message = "已存在相同名称的 Team"
				detail.ID = existing.ID.String() // 设置已存在的 workflow ID
				return existing.ID, detail
			}
		}
	}

	// 更新 agentIds 映射：将原始ID替换为导入后的新ID
	updatedAgentIDs := make([]string, len(wf.AgentIDs))
	for i, originalID := range wf.AgentIDs {
		if newID, ok := originalRoleIDToNewID[originalID]; ok {
			updatedAgentIDs[i] = newID.String()
			s.logger.Info("更新 workflow agentId",
				zap.String("originalID", originalID),
				zap.String("newID", newID.String()))
		} else {
			// 如果没有映射，保留原始ID（可能是有效UUID）
			updatedAgentIDs[i] = originalID
		}
	}

	// 更新 transitions 中的 ID 映射
	updatedTransitions := make([]model.Transition, len(wf.Transitions))
	for i, t := range wf.Transitions {
		updatedTransitions[i] = model.Transition{
			Type:        t.Type,
			TriggerHint: t.TriggerHint,
			WaitFor:     t.WaitFor,
		}
		// 更新 fromAgentId
		if newID, ok := originalRoleIDToNewID[t.FromAgentID]; ok {
			updatedTransitions[i].FromAgentID = newID.String()
		} else {
			updatedTransitions[i].FromAgentID = t.FromAgentID
		}
		// 更新 toAgentId
		if newID, ok := originalRoleIDToNewID[t.ToAgentID]; ok {
			updatedTransitions[i].ToAgentID = newID.String()
		} else {
			updatedTransitions[i].ToAgentID = t.ToAgentID
		}
	}

	agentIDsJSON, _ := json.Marshal(updatedAgentIDs)
	transitionsJSON, _ := json.Marshal(updatedTransitions)
	checkpointsJSON, _ := json.Marshal(wf.Checkpoints)

	now := time.Now()
	workflow := &model.WorkflowTemplate{
		ID:            uuid.New(),
		Name:          wf.Name,
		Description:   wf.Description,
		AgentIDs:      agentIDsJSON,
		Transitions:   transitionsJSON,
		Checkpoints:   checkpointsJSON,
		EstimatedTime: wf.EstimatedTime,
		IsSystem:      false,
		IsDefault:     false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.workflowRepo.Create(ctx, workflow); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("创建 Team 记录失败: %v", err)
		return uuid.Nil, detail
	}

	detail.Status = "success"
	detail.ID = workflow.ID.String()
	return workflow.ID, detail
	}


// deleteRoleBindings 删除角色的所有绑定关系
func (s *Service) deleteRoleBindings(ctx context.Context, agentRoleID uuid.UUID) error {
	// 删除 Skill 绑定
	if err := s.agentSkillBindingRepo.DeleteByAgentRoleID(ctx, agentRoleID); err != nil {
		return err
	}
	// 删除 Command 绑定
	if err := s.agentCommandBindingRepo.DeleteByAgentRoleID(ctx, agentRoleID); err != nil {
		return err
	}
	// 删除 Subagent 绑定
	if err := s.agentSubagentBindingRepo.DeleteByAgentRoleID(ctx, agentRoleID); err != nil {
		return err
	}
	// 删除 Rule 绑定
	if err := s.agentRuleBindingRepo.DeleteByAgentRoleID(ctx, agentRoleID); err != nil {
		return err
	}
	// 删除 Settings 绑定
	if err := s.agentSettingsBindingRepo.DeleteByAgentRoleID(ctx, agentRoleID); err != nil {
		return err
	}
	return nil
}

func (s *Service) createAgentSkillBinding(ctx context.Context, agentRoleID, skillID uuid.UUID) error {
	// 检查是否已存在
	exists, err := s.agentSkillBindingRepo.ExistsBinding(ctx, agentRoleID, skillID)
	if err != nil || exists {
		return err
	}

	binding := &model.AgentSkillBinding{
		ID:          uuid.New(),
		AgentRoleID: agentRoleID,
		SkillID:     skillID,
		CreatedAt:   time.Now(),
	}
	return s.agentSkillBindingRepo.Create(ctx, binding)
}

func (s *Service) createAgentCommandBinding(ctx context.Context, agentRoleID, commandID uuid.UUID) error {
	exists, err := s.agentCommandBindingRepo.ExistsBinding(ctx, agentRoleID, commandID)
	if err != nil || exists {
		return err
	}

	binding := &model.AgentCommandBinding{
		ID:          uuid.New(),
		AgentRoleID: agentRoleID,
		CommandID:   commandID,
		CreatedAt:   time.Now(),
	}
	return s.agentCommandBindingRepo.Create(ctx, binding)
}

func (s *Service) createAgentSubagentBinding(ctx context.Context, agentRoleID, subagentID uuid.UUID) error {
	exists, err := s.agentSubagentBindingRepo.ExistsBinding(ctx, agentRoleID, subagentID)
	if err != nil || exists {
		return err
	}

	binding := &model.AgentSubagentBinding{
		ID:          uuid.New(),
		AgentRoleID: agentRoleID,
		SubagentID:  subagentID,
		CreatedAt:   time.Now(),
	}
	return s.agentSubagentBindingRepo.Create(ctx, binding)
}

func (s *Service) createAgentRuleBinding(ctx context.Context, agentRoleID, ruleID uuid.UUID) error {
	exists, err := s.agentRuleBindingRepo.ExistsBinding(ctx, agentRoleID, ruleID)
	if err != nil || exists {
		return err
	}

	binding := &model.AgentRuleBinding{
		ID:          uuid.New(),
		AgentRoleID: agentRoleID,
		RuleID:      ruleID,
		CreatedAt:   time.Now(),
	}
	return s.agentRuleBindingRepo.Create(ctx, binding)
}

func (s *Service) createAgentSettingsBinding(ctx context.Context, agentRoleID, settingsID uuid.UUID) error {
	exists, err := s.agentSettingsBindingRepo.ExistsBinding(ctx, agentRoleID, settingsID)
	if err != nil || exists {
		return err
	}

	binding := &model.AgentSettingsBinding{
		ID:          uuid.New(),
		AgentRoleID: agentRoleID,
		SettingsID:  settingsID,
		CreatedAt:   time.Now(),
	}
	return s.agentSettingsBindingRepo.Create(ctx, binding)
}

// ========== 辅助函数 ==========

// getAssetAction 获取资产操作策略
func getAssetAction(actions []model.TeamPackageAssetAction, assetType, name string) string {
	for _, action := range actions {
		if action.AssetType == assetType && action.Name == name {
			return action.Action
		}
	}
	// 默认：如果存在则跳过，不存在则覆盖（创建）
	return "overwrite"
}

// getRoleAction 获取角色操作策略
func getRoleAction(actions []model.TeamPackageRoleAction, name string) string {
	for _, action := range actions {
		if action.Name == name {
			return action.Action
		}
	}
	return "overwrite"
}

// copyDir 复制目录
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return errors.New("源路径不是目录")
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if srcInfo.IsDir() {
		return errors.New("源路径是目录")
	}

	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}

// createZip 创建 ZIP 文件
func createZip(srcDir string) ([]byte, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			_, err = zipWriter.Create(relPath + "/")
			return err
		}

		zipEntry, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(zipEntry, file)
		return err
	})

	if err != nil {
		return nil, err
	}

	if err := zipWriter.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// extractZip 解压 ZIP 文件（包含安全保护措施）
func extractZip(zipReader io.Reader, dstDir string) error {
	zipData, err := io.ReadAll(zipReader)
	if err != nil {
		return err
	}

	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return err
	}

	const (
		maxTotalSize = int64(500 * 1024 * 1024) // 500MB
		maxFileCount = 1000
		maxFileSize  = int64(100 * 1024 * 1024) // 100MB
	)

	var totalSize int64
	fileCount := 0
	cleanDstDir := filepath.Clean(dstDir)

	for _, file := range reader.File {
		fileCount++
		if fileCount > maxFileCount {
			return fmt.Errorf("ZIP 文件数量超过限制 (最大 %d 个文件)", maxFileCount)
		}

		fileInfo := file.FileInfo()
		fileSize := fileInfo.Size()
		if fileSize > maxFileSize {
			return fmt.Errorf("文件 %s 超过大小限制 (最大 %d MB)", file.Name, maxFileSize/1024/1024)
		}

		totalSize += fileSize
		if totalSize > maxTotalSize {
			return fmt.Errorf("ZIP 解压总大小超过限制 (最大 %d MB)", maxTotalSize/1024/1024)
		}

		dstPath := filepath.Join(dstDir, file.Name)
		cleanPath := filepath.Clean(dstPath)
		if !strings.HasPrefix(cleanPath, cleanDstDir+string(filepath.Separator)) {
			if cleanPath != cleanDstDir {
				return fmt.Errorf("检测到路径遍历攻击: %s", file.Name)
			}
		}

		if fileInfo.IsDir() {
			if err := os.MkdirAll(cleanPath, 0755); err != nil {
				return fmt.Errorf("创建目录失败: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(cleanPath), 0755); err != nil {
			return fmt.Errorf("创建父目录失败: %w", err)
		}

		srcFile, err := file.Open()
		if err != nil {
			return fmt.Errorf("打开 ZIP 条目失败: %w", err)
		}

		dstFile, err := os.Create(cleanPath)
		if err != nil {
			srcFile.Close()
			return fmt.Errorf("创建目标文件失败: %w", err)
		}

		_, err = io.CopyN(dstFile, srcFile, maxFileSize+1)
		dstFile.Close()
		srcFile.Close()

		if err != nil {
			if err == io.EOF {
				continue
			}
			return fmt.Errorf("解压文件失败: %w", err)
		}
	}

	return nil
}
// bindSkillsToCommand 绑定 Skills 到 Command
func (s *Service) bindSkillsToCommand(ctx context.Context, commandID uuid.UUID, skillNames []string, skillNameToID map[string]uuid.UUID) {
	if s.commandSkillBindingRepo == nil {
		s.logger.Warn("commandSkillBindingRepo 为 nil，无法绑定技能到命令")
		return
	}
	if len(skillNames) == 0 {
		return
	}
	for _, skillName := range skillNames {
		skillID, ok := skillNameToID[skillName]
		if !ok {
			s.logger.Warn("绑定技能到命令失败，技能不存在", zap.String("skill", skillName))
			continue
		}

		// 检查是否已存在绑定
		exists, err := s.commandSkillBindingRepo.ExistsBinding(ctx, commandID, skillID)
		if err == nil && exists {
			continue
		}

		binding := &model.CommandSkillBinding{
			ID:        uuid.New(),
			CommandID: commandID,
			SkillID:   skillID,
			CreatedAt: time.Now(),
		}
		if err := s.commandSkillBindingRepo.Create(ctx, binding); err != nil {
			s.logger.Warn("创建命令技能绑定失败", zap.Error(err))
		}
	}
}

// bindSkillsToSubagent 绑定 Skills 到 Subagent
func (s *Service) bindSkillsToSubagent(ctx context.Context, subagentID uuid.UUID, skillNames []string, skillNameToID map[string]uuid.UUID) {
	if s.subagentSkillBindingRepo == nil {
		s.logger.Warn("subagentSkillBindingRepo 为 nil，无法绑定技能到子代理")
		return
	}
	if len(skillNames) == 0 {
		return
	}
	for _, skillName := range skillNames {
		skillID, ok := skillNameToID[skillName]
		if !ok {
			s.logger.Warn("绑定技能到子代理失败，技能不存在", zap.String("skill", skillName))
			continue
		}

		// 检查是否已存在绑定
		exists, err := s.subagentSkillBindingRepo.ExistsBinding(ctx, subagentID, skillID)
		if err == nil && exists {
			continue
		}

		binding := &model.SubagentSkillBinding{
			ID:         uuid.New(),
			SubagentID: subagentID,
			SkillID:    skillID,
			CreatedAt:  time.Now(),
		}
		if err := s.subagentSkillBindingRepo.Create(ctx, binding); err != nil {
			s.logger.Warn("创建子代理技能绑定失败", zap.Error(err))
		}
	}
}
