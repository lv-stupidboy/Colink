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
	packageRepo          *repo.AssetPackageRepository
	skillRepo            *repo.SkillRepository
	commandRepo          *repo.CommandRepository
	subagentRepo         *repo.SubagentRepository
	ruleRepo             *repo.RuleRepository
	settingsRepo         *repo.SettingsRepository
	settingsService      *settings.Service
	commandSkillBinding  *repo.CommandSkillBindingRepository
	subagentSkillBinding *repo.SubagentSkillBindingRepository
	skillStoragePath     string
	subagentStoragePath  string
	commandStoragePath   string
	ruleStoragePath      string
	settingsStoragePath  string
	logger               *zap.Logger
}

// NewService 创建 AssetPackage Service
func NewService(
	packageRepo *repo.AssetPackageRepository,
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
		packageRepo:          packageRepo,
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

// List 列出资产包
func (s *Service) List(ctx context.Context, query *model.AssetPackageListQuery) ([]*model.AssetPackage, int64, error) {
	return s.packageRepo.List(ctx, query)
}

// GetByID 根据ID获取资产包
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*model.AssetPackage, error) {
	return s.packageRepo.FindByID(ctx, id)
}

// Delete 删除资产包
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.packageRepo.Delete(ctx, id)
}

// Export 导出资产包
func (s *Service) Export(ctx context.Context, req *model.ExportAssetPackageRequest) ([]byte, string, error) {
	// 生成完整版本号: v{version}-{YYYYMMDD}-{HHMMSS}
	now := time.Now()
	fullVersion := fmt.Sprintf("v%s-%s-%s", req.Version, now.Format("20060102"), now.Format("150456"))

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "asset-package-export-*")
	if err != nil {
		return nil, "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 构建 manifest
	manifest := &model.AssetPackageManifest{
		Name:        req.Name,
		Version:     fullVersion,
		ExportedAt:  now.Format(time.RFC3339),
		Description: req.Description,
		Assets:      model.AssetPackageAssetsList{},
	}

	// 导出 Skills
	for _, skillID := range req.SkillIDs {
		skill, err := s.skillRepo.FindByID(ctx, skillID)
		if err != nil {
			s.logger.Warn("导出技能失败，跳过", zap.String("skillId", skillID.String()), zap.Error(err))
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
			Name:    skill.Name,
			Version: skill.Version,
		})
	}

	// 导出 Commands
	for _, commandID := range req.CommandIDs {
		command, err := s.commandRepo.FindByID(ctx, commandID)
		if err != nil {
			s.logger.Warn("导出命令失败，跳过", zap.String("commandId", commandID.String()), zap.Error(err))
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
			s.logger.Warn("获取命令绑定的技能失败", zap.String("commandId", commandID.String()), zap.Error(err))
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
			Version:     command.Version,
			BoundSkills: boundSkills,
		})
	}

	// 导出 Subagents
	for _, subagentID := range req.SubagentIDs {
		subagent, err := s.subagentRepo.FindByID(ctx, subagentID)
		if err != nil {
			s.logger.Warn("导出子代理失败，跳过", zap.String("subagentId", subagentID.String()), zap.Error(err))
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
			s.logger.Warn("获取子代理绑定的技能失败", zap.String("subagentId", subagentID.String()), zap.Error(err))
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
			Version:     subagent.Version,
			BoundSkills: boundSkills,
		})
	}

	// 导出 Rules
	for _, ruleID := range req.RuleIDs {
		rule, err := s.ruleRepo.FindByID(ctx, ruleID)
		if err != nil {
			s.logger.Warn("导出规约失败，跳过", zap.String("ruleId", ruleID.String()), zap.Error(err))
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
			Name:    rule.Name,
			Version: rule.Version,
		})
	}

	// 导出 Settings
	for _, settingsID := range req.SettingsIDs {
		settingsRecord, err := s.settingsRepo.FindByID(ctx, settingsID)
		if err != nil {
			s.logger.Warn("导出配置失败，跳过", zap.String("settingsId", settingsID.String()), zap.Error(err))
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

	// 创建 AssetPackage 记录
	pkg := &model.AssetPackage{
		ID:          uuid.New(),
		Name:        req.Name,
		Version:     fullVersion,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.packageRepo.Create(ctx, pkg); err != nil {
		return nil, "", fmt.Errorf("保存资产包记录失败: %w", err)
	}

	// 生成文件名
	filename := fmt.Sprintf("%s-%s.zip", req.Name, fullVersion)

	s.logger.Info("导出资产包成功",
		zap.String("id", pkg.ID.String()),
		zap.String("name", pkg.Name),
		zap.String("version", pkg.Version),
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

	// 创建 AssetPackage 记录
	now := time.Now()
	pkg := &model.AssetPackage{
		ID:          uuid.New(),
		Name:        manifest.Name,
		Version:     manifest.Version,
		Description: manifest.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.packageRepo.Create(ctx, pkg); err != nil {
		return nil, fmt.Errorf("保存资产包记录失败: %w", err)
	}

	result := &model.ImportResult{
		PackageName: manifest.Name,
		PackageID:   pkg.ID,
		Success:     0,
		Skipped:     0,
		Failed:      0,
		Details:     make([]model.ImportDetail, 0),
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
			existing, _ := s.skillRepo.FindByNameAndVersion(ctx, skillItem.Name, skillItem.Version)
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
		zap.String("id", pkg.ID.String()),
		zap.String("name", pkg.Name),
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
		Version:   item.Version,
	}

	// 检查是否已存在相同名称和版本的 Skill
	existing, err := s.skillRepo.FindByNameAndVersion(ctx, item.Name, item.Version)
	if err == nil && existing != nil {
		detail.Status = "skipped"
		detail.Message = "已存在相同名称和版本的技能"
		return uuid.Nil, detail
	}

	// 复制 Skill 目录
	srcDir := filepath.Join(tempDir, "skills", item.Name)
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
		Version:         item.Version,
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
		Version:   item.Version,
	}

	// 检查是否已存在相同名称和版本的 Command
	existing, err := s.commandRepo.FindByNameAndVersion(ctx, item.Name, item.Version)
	if err == nil && existing != nil {
		detail.Status = "skipped"
		detail.Message = "已存在相同名称和版本的命令"
		return uuid.Nil, detail
	}

	// 复制 Command 文件
	srcFile := filepath.Join(tempDir, "commands", item.Name+".md")
	targetFile := filepath.Join(s.commandStoragePath, item.Name+".md")
	if err := copyFile(srcFile, targetFile); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制命令文件失败: %v", err)
		return uuid.Nil, detail
	}

	// 创建 Command 记录
	now := time.Now()
	command := &model.Command{
		ID:          uuid.New(),
		Name:        item.Name,
		Version:     item.Version,
		CreatedAt:   now,
		UpdatedAt:   now,
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
		Version:   item.Version,
	}

	// 检查是否已存在相同名称和版本的 Subagent
	existing, err := s.subagentRepo.FindByNameAndVersion(ctx, item.Name, item.Version)
	if err == nil && existing != nil {
		detail.Status = "skipped"
		detail.Message = "已存在相同名称和版本的子代理"
		return uuid.Nil, detail
	}

	// 复制 Subagent 文件
	srcFile := filepath.Join(tempDir, "subagents", item.Name+".md")
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
		Version:   item.Version,
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
		Version:   item.Version,
	}

	// 检查是否已存在相同名称和版本的 Rule
	existing, err := s.ruleRepo.FindByNameAndVersion(ctx, item.Name, item.Version)
	if err == nil && existing != nil {
		detail.Status = "skipped"
		detail.Message = "已存在相同名称和版本的规约"
		return uuid.Nil, detail
	}

	// 复制 Rule 文件
	srcFile := filepath.Join(tempDir, "rules", item.Name+".md")
	targetFile := filepath.Join(s.ruleStoragePath, item.Name+".md")
	if err := copyFile(srcFile, targetFile); err != nil {
		detail.Status = "failed"
		detail.Message = fmt.Sprintf("复制规约文件失败: %v", err)
		return uuid.Nil, detail
	}

	// 创建 Rule 记录
	now := time.Now()
	rule := &model.Rule{
		ID:          uuid.New(),
		Name:        item.Name,
		Version:     item.Version,
		Visibility:  model.RuleVisibilityPublic,
		CreatedAt:   now,
		UpdatedAt:   now,
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
		Version:   "",
	}

	// 检查是否已存在相同名称的 Settings（Settings 没有 Version）
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
	srcDir := filepath.Join(tempDir, "settings", item.Name)
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
		Version:       "1.0.0",
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
func extractZip(zipReader io.Reader, dstDir string) error {
	zipData, err := io.ReadAll(zipReader)
	if err != nil {
		return err
	}

	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return err
	}

	for _, file := range reader.File {
		dstPath := filepath.Join(dstDir, file.Name)

		// 使用 file.Mode().IsDir() 或 FileInfo() 来判断是否为目录
		fileInfo := file.FileInfo()
		if fileInfo.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
			continue
		}

		// 创建目标目录
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return err
		}

		// 打开 ZIP 条目
		srcFile, err := file.Open()
		if err != nil {
			return err
		}

		// 创建目标文件
		dstFile, err := os.Create(dstPath)
		if err != nil {
			srcFile.Close()
			return err
		}

		// 复制内容
		_, err = io.Copy(dstFile, srcFile)
		dstFile.Close()
		srcFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}