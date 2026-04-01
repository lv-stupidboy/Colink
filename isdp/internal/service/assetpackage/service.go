// 文件路径: isdp/internal/service/assetpackage/service.go
package assetpackage

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
	"github.com/anthropic/isdp/internal/service/settings"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service 资产包业务服务
type Service struct {
	skillRepo            *repo.SkillRepository
	commandRepo          *repo.CommandRepository
	subagentRepo         *repo.SubagentRepository
	ruleRepo            *repo.RuleRepository
	settingsRepo        *repo.SettingsRepository
	settingsService     *settings.Service
	commandSkillBinding *repo.CommandSkillBindingRepository
	subagentSkillBinding *repo.SubagentSkillBindingRepository
	skillStoragePath    string
	subagentStoragePath string
	commandStoragePath  string
	ruleStoragePath     string
	settingsStoragePath string
	logger              *zap.Logger
}

// NewService 创建 AssetPackage Service
func NewService(
	skillRepo *repo.SkillRepository,
	commandRepo *repo.CommandRepository,
	subagentRepo *repo.SubagentRepository,
	ruleRepo *repo.RuleRepository,
	settingsRepo *repo.SettingsRepository,
	settingsService *settings.Service,
	commandSkillBinding *repo.CommandSkillBindingRepository,
	subagentSkillBinding *repo.SubagentSkillBindingRepository,
	skillStoragePath string,
	subagentStoragePath string,
	commandStoragePath string,
	ruleStoragePath string,
	settingsStoragePath string,
	logger *zap.Logger,
) *Service {
	return &Service{
		skillRepo:            skillRepo,
		commandRepo:          commandRepo,
		subagentRepo:         subagentRepo,
		ruleRepo:             ruleRepo,
		settingsRepo:         settingsRepo,
		settingsService:      settingsService,
		commandSkillBinding:  commandSkillBinding,
		subagentSkillBinding: subagentSkillBinding,
		skillStoragePath:     skillStoragePath,
		subagentStoragePath:  subagentStoragePath,
		commandStoragePath:   commandStoragePath,
		ruleStoragePath:      ruleStoragePath,
		settingsStoragePath:  settingsStoragePath,
		logger:               logger,
	}
}

// Export 导出资产包
func (s *Service) Export(ctx context.Context, req *model.ExportAssetPackageRequest) ([]byte, string, error) {
	now := time.Now()

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "asset-package-export-*")
	if err != nil {
		return nil, "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 构建 manifest
	manifest := &model.AssetPackageManifest{
		ExportedAt: now.Format(time.RFC3339),
		Assets:     model.AssetPackageAssetsList{},
	}

	// 导出 Skills
	for _, skillIDStr := range req.SkillIDs {
		skillID, err := uuid.Parse(skillIDStr)
		if err != nil {
			s.logger.Warn("解析技能ID失败，跳过", zap.String("skillId", skillIDStr), zap.Error(err))
			continue
		}

		skill, err := s.skillRepo.FindByID(ctx, skillID)
		if err != nil {
			s.logger.Warn("导出技能失败，跳过", zap.String("skillId", skillIDStr), zap.Error(err))
			continue
		}

		// 复制 Skill 目录到临时目录
		skillDir := filepath.Join(s.skillStoragePath, skill.Name)
		targetDir := filepath.Join(tempDir, "skills", skill.Name)
		if err := copyDir(skillDir, targetDir); err != nil {
			s.logger.Warn("复制技能目录失败，跳过", zap.String("skill", skill.Name), zap.Error(err))
			continue
		}

		manifest.Assets.Skills = append(manifest.Assets.Skills, model.AssetPackageSkillItem{
			Name: skill.Name,
			File: skill.Name,
		})
	}

	// 导出 Commands
	for _, commandIDStr := range req.CommandIDs {
		commandID, err := uuid.Parse(commandIDStr)
		if err != nil {
			s.logger.Warn("解析命令ID失败，跳过", zap.String("commandId", commandIDStr), zap.Error(err))
			continue
		}

		command, err := s.commandRepo.FindByID(ctx, commandID)
		if err != nil {
			s.logger.Warn("导出命令失败，跳过", zap.String("commandId", commandIDStr), zap.Error(err))
			continue
		}

		// 复制 Command 文件到临时目录
		commandFile := filepath.Join(s.commandStoragePath, command.Name+".md")
		targetFile := filepath.Join(tempDir, "commands", command.Name+".md")
		if err := copyFile(commandFile, targetFile); err != nil {
			s.logger.Warn("复制命令文件失败，跳过", zap.String("command", command.Name), zap.Error(err))
			continue
		}

		// 获取 Command 绑定的 Skills
		boundSkillIDs, err := s.commandSkillBinding.FindByCommandID(ctx, commandID)
		if err != nil {
			s.logger.Warn("获取命令绑定的技能失败", zap.String("commandId", commandIDStr), zap.Error(err))
		}
		boundSkills := make([]string, 0, len(boundSkillIDs))
		for _, skillID := range boundSkillIDs {
			skill, err := s.skillRepo.FindByID(ctx, skillID)
			if err == nil {
				boundSkills = append(boundSkills, skill.Name)
			}
		}

		manifest.Assets.Commands = append(manifest.Assets.Commands, model.AssetPackageCommandItem{
			Name:        command.Name,
			File:        command.Name,
			BoundSkills: boundSkills,
		})
	}

	// 导出 Subagents
	for _, subagentIDStr := range req.SubagentIDs {
		subagentID, err := uuid.Parse(subagentIDStr)
		if err != nil {
			s.logger.Warn("解析子代理ID失败，跳过", zap.String("subagentId", subagentIDStr), zap.Error(err))
			continue
		}

		subagent, err := s.subagentRepo.FindByID(ctx, subagentID)
		if err != nil {
			s.logger.Warn("导出子代理失败，跳过", zap.String("subagentId", subagentIDStr), zap.Error(err))
			continue
		}

		// 复制 Subagent 文件到临时目录
		subagentFile := filepath.Join(s.subagentStoragePath, subagent.Name+".md")
		targetFile := filepath.Join(tempDir, "subagents", subagent.Name+".md")
		if err := copyFile(subagentFile, targetFile); err != nil {
			s.logger.Warn("复制子代理文件失败，跳过", zap.String("subagent", subagent.Name), zap.Error(err))
			continue
		}

		// 获取 Subagent 绑定的 Skills
		boundSkillIDs, err := s.subagentSkillBinding.FindBySubagentID(ctx, subagentID)
		if err != nil {
			s.logger.Warn("获取子代理绑定的技能失败", zap.String("subagentId", subagentIDStr), zap.Error(err))
		}
		boundSkills := make([]string, 0, len(boundSkillIDs))
		for _, skillID := range boundSkillIDs {
			skill, err := s.skillRepo.FindByID(ctx, skillID)
			if err == nil {
				boundSkills = append(boundSkills, skill.Name)
			}
		}

		manifest.Assets.Subagents = append(manifest.Assets.Subagents, model.AssetPackageSubagentItem{
			Name:        subagent.Name,
			File:        subagent.Name,
			BoundSkills: boundSkills,
		})
	}

	// 导出 Rules
	for _, ruleIDStr := range req.RuleIDs {
		ruleID, err := uuid.Parse(ruleIDStr)
		if err != nil {
			s.logger.Warn("解析规约ID失败，跳过", zap.String("ruleId", ruleIDStr), zap.Error(err))
			continue
		}

		rule, err := s.ruleRepo.FindByID(ctx, ruleID)
		if err != nil {
			s.logger.Warn("导出规约失败，跳过", zap.String("ruleId", ruleIDStr), zap.Error(err))
			continue
		}

		// 复制 Rule 文件到临时目录
		ruleFile := filepath.Join(s.ruleStoragePath, rule.Name+".md")
		targetFile := filepath.Join(tempDir, "rules", rule.Name+".md")
		if err := copyFile(ruleFile, targetFile); err != nil {
			s.logger.Warn("复制规约文件失败，跳过", zap.String("rule", rule.Name), zap.Error(err))
			continue
		}

		manifest.Assets.Rules = append(manifest.Assets.Rules, model.AssetPackageRuleItem{
			Name: rule.Name,
			File: rule.Name,
		})
	}

	// 导出 Settings
	for _, settingsIDStr := range req.SettingsIDs {
		settingsID, err := uuid.Parse(settingsIDStr)
		if err != nil {
			s.logger.Warn("解析配置ID失败，跳过", zap.String("settingsId", settingsIDStr), zap.Error(err))
			continue
		}

		settingsRecord, err := s.settingsRepo.FindByID(ctx, settingsID)
		if err != nil {
			s.logger.Warn("导出配置失败，跳过", zap.String("settingsId", settingsIDStr), zap.Error(err))
			continue
		}

		// 复制 Settings 目录到临时目录
		settingsDir := settingsRecord.DirectoryPath
		targetDir := filepath.Join(tempDir, "settings", settingsRecord.Name)
		if settingsDir != "" {
			if err := copyDir(settingsDir, targetDir); err != nil {
				s.logger.Warn("复制配置目录失败，跳过", zap.String("settings", settingsRecord.Name), zap.Error(err))
				continue
			}
		}

		manifest.Assets.Settings = append(manifest.Assets.Settings, model.AssetPackageSettingsItem{
			Name: settingsRecord.Name,
			Dir:  settingsRecord.Name,
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

	// 生成文件名: asset-{timestamp}.zip
	timestamp := now.Format("20060102-150405")
	filename := fmt.Sprintf("asset-%s.zip", timestamp)

	s.logger.Info("导出资产包成功",
		zap.String("filename", filename),
		zap.Int("skills", len(manifest.Assets.Skills)),
		zap.Int("commands", len(manifest.Assets.Commands)),
		zap.Int("subagents", len(manifest.Assets.Subagents)),
		zap.Int("rules", len(manifest.Assets.Rules)),
		zap.Int("settings", len(manifest.Assets.Settings)))

	return zipData, filename, nil
}

// Import 导入资产包
func (s *Service) Import(ctx context.Context, zipData []byte) (*model.ImportResult, error) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "asset-package-import-*")
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
	var manifest model.AssetPackageManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("解析 manifest 失败: %w", err)
	}

	result := &model.ImportResult{
		Success: 0,
		Skipped: 0,
		Failed:  0,
		Details: make([]model.ImportDetail, 0),
	}

	// 导入 Skills
	skillNameToID := make(map[string]uuid.UUID)
	for _, skillItem := range manifest.Assets.Skills {
		id, detail := s.importSkill(ctx, tempDir, skillItem)
		result.Details = append(result.Details, detail)
		switch detail.Status {
		case "success":
			result.Success++
			skillNameToID[skillItem.Name] = id
		case "skipped":
			result.Skipped++
			// 获取已存在的 Skill ID
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
		id, detail := s.importCommand(ctx, tempDir, commandItem, skillNameToID)
		result.Details = append(result.Details, detail)
		switch detail.Status {
		case "success":
			result.Success++
			// 绑定 Skills
			s.bindSkillsToCommand(ctx, id, commandItem.BoundSkills, skillNameToID)
		case "skipped":
			result.Skipped++
		case "failed":
			result.Failed++
		}
	}

	// 导入 Subagents
	for _, subagentItem := range manifest.Assets.Subagents {
		id, detail := s.importSubagent(ctx, tempDir, subagentItem, skillNameToID)
		result.Details = append(result.Details, detail)
		switch detail.Status {
		case "success":
			result.Success++
			// 绑定 Skills
			s.bindSkillsToSubagent(ctx, id, subagentItem.BoundSkills, skillNameToID)
		case "skipped":
			result.Skipped++
		case "failed":
			result.Failed++
		}
	}

	// 导入 Rules
	for _, ruleItem := range manifest.Assets.Rules {
		_, detail := s.importRule(ctx, tempDir, ruleItem)
		result.Details = append(result.Details, detail)
		switch detail.Status {
		case "success":
			result.Success++
		case "skipped":
			result.Skipped++
		case "failed":
			result.Failed++
		}
	}

	// 导入 Settings
	for _, settingsItem := range manifest.Assets.Settings {
		_, detail := s.importSettings(ctx, tempDir, settingsItem)
		result.Details = append(result.Details, detail)
		switch detail.Status {
		case "success":
			result.Success++
		case "skipped":
			result.Skipped++
		case "failed":
			result.Failed++
		}
	}

	s.logger.Info("导入资产包完成",
		zap.Int("success", result.Success),
		zap.Int("skipped", result.Skipped),
		zap.Int("failed", result.Failed))

	return result, nil
}

// importSkill 导入单个 Skill
func (s *Service) importSkill(ctx context.Context, tempDir string, item model.AssetPackageSkillItem) (uuid.UUID, model.ImportDetail) {
	detail := model.ImportDetail{
		AssetType: "skill",
		Name:      item.Name,
	}

	// 检查是否已存在相同名称的 Skill
	existing, err := s.skillRepo.FindByName(ctx, item.Name)
	if err == nil && existing != nil {
		detail.Status = "skipped"
		detail.Message = "已存在相同名称的技能"
		return uuid.Nil, detail
	}

	// 复制 Skill 目录
	srcDir := filepath.Join(tempDir, "skills", item.File)
	targetDir := filepath.Join(s.skillStoragePath, item.Name)
	if err := copyDir(srcDir, targetDir); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制技能目录失败: %v", err)
		return uuid.Nil, detail
	}

	// 创建 Skill 记录
	now := time.Now()
	skill := &model.Skill{
		ID:              uuid.New(),
		Name:            item.Name,
		SourceType:      model.SkillSourcePersonal,
		Status:          model.SkillStatusActive,
		IsPublic:        true,
		UseCount:        0,
		SupportedAgents: []string{},
		Tags:            []string{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.skillRepo.Create(ctx, skill); err != nil {
		// 回滚：删除已复制的目录
		os.RemoveAll(targetDir)
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("创建技能记录失败: %v", err)
		return uuid.Nil, detail
	}

	detail.Status = "success"
	return skill.ID, detail
}

// importCommand 导入单个 Command
func (s *Service) importCommand(ctx context.Context, tempDir string, item model.AssetPackageCommandItem, skillNameToID map[string]uuid.UUID) (uuid.UUID, model.ImportDetail) {
	detail := model.ImportDetail{
		AssetType: "command",
		Name:      item.Name,
	}

	// 检查是否已存在相同名称的 Command
	existing, err := s.commandRepo.FindByName(ctx, item.Name)
	if err == nil && existing != nil {
		detail.Status = "skipped"
		detail.Message = "已存在相同名称的命令"
		return uuid.Nil, detail
	}

	// 复制 Command 文件
	srcFile := filepath.Join(tempDir, "commands", item.File+".md")
	targetFile := filepath.Join(s.commandStoragePath, item.Name+".md")
	if err := copyFile(srcFile, targetFile); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制命令文件失败: %v", err)
		return uuid.Nil, detail
	}

	// 创建 Command 记录
	now := time.Now()
	command := &model.Command{
		ID:        uuid.New(),
		Name:      item.Name,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.commandRepo.Create(ctx, command); err != nil {
		// 回滚：删除已复制的文件
		os.Remove(targetFile)
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("创建命令记录失败: %v", err)
		return uuid.Nil, detail
	}

	detail.Status = "success"
	return command.ID, detail
}

// importSubagent 导入单个 Subagent
func (s *Service) importSubagent(ctx context.Context, tempDir string, item model.AssetPackageSubagentItem, skillNameToID map[string]uuid.UUID) (uuid.UUID, model.ImportDetail) {
	detail := model.ImportDetail{
		AssetType: "subagent",
		Name:      item.Name,
	}

	// 检查是否已存在相同名称的 Subagent
	existing, err := s.subagentRepo.FindByName(ctx, item.Name)
	if err == nil && existing != nil {
		detail.Status = "skipped"
		detail.Message = "已存在相同名称的子代理"
		return uuid.Nil, detail
	}

	// 复制 Subagent 文件
	srcFile := filepath.Join(tempDir, "subagents", item.File+".md")
	targetFile := filepath.Join(s.subagentStoragePath, item.Name+".md")
	if err := copyFile(srcFile, targetFile); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制子代理文件失败: %v", err)
		return uuid.Nil, detail
	}

	// 创建 Subagent 记录
	now := time.Now()
	subagent := &model.Subagent{
		ID:        uuid.New(),
		Name:      item.Name,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.subagentRepo.Create(ctx, subagent); err != nil {
		// 回滚：删除已复制的文件
		os.Remove(targetFile)
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("创建子代理记录失败: %v", err)
		return uuid.Nil, detail
	}

	detail.Status = "success"
	return subagent.ID, detail
}

// importRule 导入单个 Rule
func (s *Service) importRule(ctx context.Context, tempDir string, item model.AssetPackageRuleItem) (uuid.UUID, model.ImportDetail) {
	detail := model.ImportDetail{
		AssetType: "rule",
		Name:      item.Name,
	}

	// 检查是否已存在相同名称的 Rule
	existing, err := s.ruleRepo.FindByName(ctx, item.Name)
	if err == nil && existing != nil {
		detail.Status = "skipped"
		detail.Message = "已存在相同名称的规约"
		return uuid.Nil, detail
	}

	// 复制 Rule 文件
	srcFile := filepath.Join(tempDir, "rules", item.File+".md")
	targetFile := filepath.Join(s.ruleStoragePath, item.Name+".md")
	if err := copyFile(srcFile, targetFile); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制规约文件失败: %v", err)
		return uuid.Nil, detail
	}

	// 创建 Rule 记录
	now := time.Now()
	rule := &model.Rule{
		ID:         uuid.New(),
		Name:       item.Name,
		Visibility: model.RuleVisibilityPublic,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.ruleRepo.Create(ctx, rule); err != nil {
		// 回滚：删除已复制的文件
		os.Remove(targetFile)
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("创建规约记录失败: %v", err)
		return uuid.Nil, detail
	}

	detail.Status = "success"
	return rule.ID, detail
}

// importSettings 导入单个 Settings
func (s *Service) importSettings(ctx context.Context, tempDir string, item model.AssetPackageSettingsItem) (uuid.UUID, model.ImportDetail) {
	detail := model.ImportDetail{
		AssetType: "settings",
		Name:      item.Name,
	}

	// 检查是否已存在相同名称的 Settings
	existing, err := s.settingsRepo.FindByName(ctx, item.Name)
	if err == nil && existing != nil {
		detail.Status = "skipped"
		detail.Message = "已存在相同名称的配置"
		return uuid.Nil, detail
	} else if err != nil && !strings.Contains(err.Error(), "not found") {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("检查配置名称失败: %v", err)
		return uuid.Nil, detail
	}

	// 复制 Settings 目录
	srcDir := filepath.Join(tempDir, "settings", item.Dir)
	targetDir := filepath.Join(s.settingsStoragePath, item.Name)
	if err := copyDir(srcDir, targetDir); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制配置目录失败: %v", err)
		return uuid.Nil, detail
	}

	// 创建 Settings 记录
	now := time.Now()
	settingsRecord := &model.Settings{
		ID:            uuid.New(),
		Name:          item.Name,
		DirectoryPath: targetDir,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.settingsRepo.Create(ctx, settingsRecord); err != nil {
		// 回滚：删除已复制的目录
		os.RemoveAll(targetDir)
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("创建配置记录失败: %v", err)
		return uuid.Nil, detail
	}

	detail.Status = "success"
	return settingsRecord.ID, detail
}

// bindSkillsToCommand 绑定 Skills 到 Command
func (s *Service) bindSkillsToCommand(ctx context.Context, commandID uuid.UUID, skillNames []string, skillNameToID map[string]uuid.UUID) {
	for _, skillName := range skillNames {
		skillID, ok := skillNameToID[skillName]
		if !ok {
			s.logger.Warn("绑定技能到命令失败，技能不存在", zap.String("skill", skillName))
			continue
		}

		binding := &model.CommandSkillBinding{
			ID:        uuid.New(),
			CommandID: commandID,
			SkillID:   skillID,
			CreatedAt: time.Now(),
		}
		if err := s.commandSkillBinding.Create(ctx, binding); err != nil {
			s.logger.Warn("创建命令技能绑定失败", zap.Error(err))
		}
	}
}

// bindSkillsToSubagent 绑定 Skills 到 Subagent
func (s *Service) bindSkillsToSubagent(ctx context.Context, subagentID uuid.UUID, skillNames []string, skillNameToID map[string]uuid.UUID) {
	for _, skillName := range skillNames {
		skillID, ok := skillNameToID[skillName]
		if !ok {
			s.logger.Warn("绑定技能到子代理失败，技能不存在", zap.String("skill", skillName))
			continue
		}

		binding := &model.SubagentSkillBinding{
			ID:         uuid.New(),
			SubagentID: subagentID,
			SkillID:    skillID,
			CreatedAt:  time.Now(),
		}
		if err := s.subagentSkillBinding.Create(ctx, binding); err != nil {
			s.logger.Warn("创建子代理技能绑定失败", zap.Error(err))
		}
	}
}

// ========== Helper Functions ==========

// copyDir 复制目录
func copyDir(src, dst string) error {
	// 检查源目录是否存在
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return errors.New("源路径不是目录")
	}

	// 创建目标目录
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// 遍历源目录
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
	// 检查源文件是否存在
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if srcInfo.IsDir() {
		return errors.New("源路径是目录")
	}

	// 创建目标目录
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	// 打开源文件
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 创建目标文件
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// 复制内容
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// 保持权限
	return os.Chmod(dst, srcInfo.Mode())
}

// createZip 创建 ZIP 文件
func createZip(srcDir string) ([]byte, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	// 遍历目录
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 获取相对路径
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// 创建 ZIP 条目
		if info.IsDir() {
			_, err = zipWriter.Create(relPath + "/")
			return err
		}

		zipEntry, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		// 打开文件
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		// 复制内容
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

// extractZip 解压 ZIP 文件
// 包含安全保护措施：
// 1. 路径遍历防护：防止恶意路径逃逸目标目录
// 2. ZIP bomb 防护：限制文件数量和总大小
func extractZip(zipReader io.Reader, dstDir string) error {
	zipData, err := io.ReadAll(zipReader)
	if err != nil {
		return err
	}

	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return err
	}

	// ZIP bomb 防护常量
	const (
		maxTotalSize = int64(500 * 1024 * 1024) // 500MB 总大小限制
		maxFileCount = 1000                      // 最大文件数量
		maxFileSize  = int64(100 * 1024 * 1024) // 单文件最大 100MB
	)

	var totalSize int64
	fileCount := 0

	// 清理并规范化目标目录路径
	cleanDstDir := filepath.Clean(dstDir)

	for _, file := range reader.File {
		fileCount++
		if fileCount > maxFileCount {
			return fmt.Errorf("ZIP 文件数量超过限制 (最大 %d 个文件)", maxFileCount)
		}

		// 获取文件信息
		fileInfo := file.FileInfo()

		// 检查单文件大小
		fileSize := fileInfo.Size()
		if fileSize > maxFileSize {
			return fmt.Errorf("文件 %s 超过大小限制 (最大 %d MB)", file.Name, maxFileSize/1024/1024)
		}

		// 累计总大小
		totalSize += fileSize
		if totalSize > maxTotalSize {
			return fmt.Errorf("ZIP 解压总大小超过限制 (最大 %d MB)", maxTotalSize/1024/1024)
		}

		// 构建目标路径
		dstPath := filepath.Join(dstDir, file.Name)

		// 安全检查：路径遍历防护
		// 清理路径并检查是否在目标目录内
		cleanPath := filepath.Clean(dstPath)
		if !strings.HasPrefix(cleanPath, cleanDstDir+string(filepath.Separator)) {
			// 允许路径恰好等于目标目录（根目录的情况）
			if cleanPath != cleanDstDir {
				return fmt.Errorf("检测到路径遍历攻击: %s", file.Name)
			}
		}

		// 处理目录
		if fileInfo.IsDir() {
			if err := os.MkdirAll(cleanPath, 0755); err != nil {
				return fmt.Errorf("创建目录失败: %w", err)
			}
			continue
		}

		// 创建目标文件的父目录
		if err := os.MkdirAll(filepath.Dir(cleanPath), 0755); err != nil {
			return fmt.Errorf("创建父目录失败: %w", err)
		}

		// 打开 ZIP 条目
		srcFile, err := file.Open()
		if err != nil {
			return fmt.Errorf("打开 ZIP 条目失败: %w", err)
		}

		// 创建目标文件
		dstFile, err := os.Create(cleanPath)
		if err != nil {
			srcFile.Close()
			return fmt.Errorf("创建目标文件失败: %w", err)
		}

		// 复制内容（使用 LimitReader 防止解压炸弹）
		_, err = io.CopyN(dstFile, srcFile, maxFileSize+1)
		dstFile.Close()
		srcFile.Close()

		if err != nil {
			if err == io.EOF {
				// 正常结束
				continue
			}
			return fmt.Errorf("解压文件失败: %w", err)
		}
	}

	return nil
}