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
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service 团队包业务服务
type Service struct {
	workflowRepo            *repo.WorkflowTemplateRepository
	agentRepo               *repo.AgentConfigRepository
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
}

// NewService 创建 TeamPackage Service
func NewService(
	workflowRepo *repo.WorkflowTemplateRepository,
	agentRepo *repo.AgentConfigRepository,
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
		manifest.Roles = append(manifest.Roles, model.TeamPackageRole{
			ID:              agent.ID.String(),
			Name:            agent.Name,
			Role:            string(agent.Role),
			Description:     agent.Description,
			SystemPrompt:    agent.SystemPrompt,
			MaxTokens:       agent.MaxTokens,
			Temperature:     agent.Temperature,
			MentionPatterns: agent.MentionPatterns,
			Bindings:        bindings,
		})
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
		skillDir := filepath.Join(s.skillStoragePath, skill.Name)
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

	// 检查角色是否已存在
	for _, role := range manifest.Roles {
		previewRole := model.TeamPackagePreviewRole{
			Name:   role.Name,
			Exists: false,
		}
		agents, err := s.agentRepo.List(ctx)
		if err != nil {
			s.logger.Warn("获取角色列表失败", zap.Error(err))
		} else {
			for _, agent := range agents {
				if agent.Name == role.Name {
					previewRole.Exists = true
					previewRole.LocalID = agent.ID.String()
					break
				}
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
			// 绑定 Skills
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
			existing, _ := s.agentRepo.FindByID(ctx, uuid.MustParse(roleItem.ID))
			if existing != nil {
				roleNameToID[roleItem.Name] = existing.ID
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

		id, detail := s.importRole(ctx, roleItem, action == "overwrite")
		result.Details = append(result.Details, detail)
		switch detail.Status {
		case "success":
			result.Success++
			roleNameToID[roleItem.Name] = id
		case "skipped":
			result.Skipped++
			// 查找已存在的角色
			agents, _ := s.agentRepo.List(ctx)
			for _, agent := range agents {
				if agent.Name == roleItem.Name {
					roleNameToID[roleItem.Name] = agent.ID
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
	}

	// 导入工作流
	if confirm.WorkflowAction != "skip" {
		_, detail := s.importWorkflow(ctx, manifest.Workflow, roleNameToID)
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
			return uuid.Nil, detail
		}
		// 覆盖模式：先删除旧记录和文件
		oldDir := filepath.Join(s.skillStoragePath, existing.Name)
		if err := s.skillRepo.Delete(ctx, existing.ID); err != nil {
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("删除旧 Skill 记录失败: %v", err)
			return uuid.Nil, detail
		}
		os.RemoveAll(oldDir) // 删除旧文件目录
	}

	// 复制 Skill 目录
	srcDir := filepath.Join(tempDir, "assets", "skills", item.Name)
	targetDir := filepath.Join(s.skillStoragePath, item.Name)
	if err := copyDir(srcDir, targetDir); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制 Skill 目录失败: %v", err)
		return uuid.Nil, detail
	}

	// 创建 Skill 记录
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

	if err := s.skillRepo.Create(ctx, skill); err != nil {
		os.RemoveAll(targetDir)
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("创建 Skill 记录失败: %v", err)
		return uuid.Nil, detail
	}

	detail.Status = "success"
	return skill.ID, detail
}

// importCommand 导入单个 Command
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
			return uuid.Nil, detail
		}
		// 覆盖模式：先删除旧记录和文件
		oldFile := filepath.Join(s.commandStoragePath, existing.Name+".md")
		if err := s.commandRepo.Delete(ctx, existing.ID); err != nil {
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("删除旧 Command 记录失败: %v", err)
			return uuid.Nil, detail
		}
		os.Remove(oldFile) // 删除旧文件
	}

	// 复制文件
	srcFile := filepath.Join(tempDir, "assets", "commands", item.Name+".md")
	targetFile := filepath.Join(s.commandStoragePath, item.Name+".md")
	if err := copyFile(srcFile, targetFile); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制 Command 文件失败: %v", err)
		return uuid.Nil, detail
	}

	// 创建记录
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
	return command.ID, detail
}

// importSubagent 导入单个 Subagent
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
			return uuid.Nil, detail
		}
		// 覆盖模式：先删除旧记录和文件
		oldFile := filepath.Join(s.subagentStoragePath, existing.Name+".md")
		if err := s.subagentRepo.Delete(ctx, existing.ID); err != nil {
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("删除旧 Subagent 记录失败: %v", err)
			return uuid.Nil, detail
		}
		os.Remove(oldFile) // 删除旧文件
	}

	// 复制文件
	srcFile := filepath.Join(tempDir, "assets", "subagents", item.Name+".md")
	targetFile := filepath.Join(s.subagentStoragePath, item.Name+".md")
	if err := copyFile(srcFile, targetFile); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制 Subagent 文件失败: %v", err)
		return uuid.Nil, detail
	}

	// 创建记录
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
	return subagent.ID, detail
}

// importRule 导入单个 Rule
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
			return uuid.Nil, detail
		}
		// 覆盖模式：先删除旧记录和文件
		oldFile := filepath.Join(s.ruleStoragePath, existing.Name+".md")
		if err := s.ruleRepo.Delete(ctx, existing.ID); err != nil {
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("删除旧 Rule 记录失败: %v", err)
			return uuid.Nil, detail
		}
		os.Remove(oldFile) // 删除旧文件
	}

	// 复制文件
	srcFile := filepath.Join(tempDir, "assets", "rules", item.Name+".md")
	targetFile := filepath.Join(s.ruleStoragePath, item.Name+".md")
	if err := copyFile(srcFile, targetFile); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制 Rule 文件失败: %v", err)
		return uuid.Nil, detail
	}

	// 创建记录
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
	return rule.ID, detail
}

// importSettings 导入单个 Settings
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
			return uuid.Nil, detail
		}
		// 覆盖模式：先删除旧记录和目录
		if existing.DirectoryPath != "" {
			os.RemoveAll(existing.DirectoryPath)
		}
		if err := s.settingsRepo.Delete(ctx, existing.ID); err != nil {
			detail.Status = "failed"
			detail.Message = fmt.Sprintf("删除旧 Settings 记录失败: %v", err)
			return uuid.Nil, detail
		}
	} else if err != nil && !strings.Contains(err.Error(), "not found") {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("检查配置名称失败: %v", err)
		return uuid.Nil, detail
	}

	// 复制目录
	srcDir := filepath.Join(tempDir, "assets", "settings", item.Name)
	targetDir := filepath.Join(s.settingsStoragePath, item.Name)
	if err := copyDir(srcDir, targetDir); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制 Settings 目录失败: %v", err)
		return uuid.Nil, detail
	}

	// 创建记录
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
	return settingsRecord.ID, detail
}

// importRole 导入角色
func (s *Service) importRole(ctx context.Context, role model.TeamPackageRole, overwrite bool) (uuid.UUID, model.ImportDetail) {
	detail := model.ImportDetail{
		AssetType: "role",
		Name:      role.Name,
	}

	// 解析原始角色ID
	originalID, err := uuid.Parse(role.ID)
	if err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("无效的角色ID: %v", err)
		return uuid.Nil, detail
	}

	// 检查是否已存在
	agents, err := s.agentRepo.List(ctx)
	if err == nil {
		for _, agent := range agents {
			if agent.Name == role.Name {
				if !overwrite {
					detail.Status = "skipped"
					detail.Message = "已存在相同名称的 Role"
					return uuid.Nil, detail
				}
				// 覆盖模式：先删除旧角色及其绑定关系
				if err := s.deleteRoleBindings(ctx, agent.ID); err != nil {
					s.logger.Warn("删除旧角色绑定关系失败", zap.Error(err))
				}
				if err := s.agentRepo.Delete(ctx, agent.ID); err != nil {
					detail.Status = "failed"
					detail.Message = fmt.Sprintf("删除旧 Role 记录失败: %v", err)
					return uuid.Nil, detail
				}
				break
			}
		}
	}

	// 创建角色，使用原始ID
	now := time.Now()
	agentConfig := &model.AgentRoleConfig{
		ID:           originalID,
		Name:         role.Name,
		Role:         model.AgentRole(role.Role),
		Description:  role.Description,
		SystemPrompt: role.SystemPrompt,
		MaxTokens:    role.MaxTokens,
		Temperature:  role.Temperature,
		IsDefault:    false,
		IsSystem:     false,
		MentionPatterns: role.MentionPatterns,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.agentRepo.Create(ctx, agentConfig); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("创建 Role 记录失败: %v", err)
		return uuid.Nil, detail
	}

	detail.Status = "success"
	return agentConfig.ID, detail
}

// importWorkflow 导入工作流
func (s *Service) importWorkflow(ctx context.Context, wf model.TeamPackageWorkflow, roleNameToID map[string]uuid.UUID) (uuid.UUID, model.ImportDetail) {
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
				return uuid.Nil, detail
			}
		}
	}

	// 由于角色导入时保留了原始ID，直接使用manifest中的agentIds
	agentIDsJSON, _ := json.Marshal(wf.AgentIDs)
	transitionsJSON, _ := json.Marshal(wf.Transitions)
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
	return workflow.ID, detail
}

// ========== 绑定关系创建辅助方法 ==========

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
		if err != nil || exists {
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
		if err != nil || exists {
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
