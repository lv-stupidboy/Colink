// internal/service/agent/plugins/open_code/config_generator_impl.go
// OpenCode AssetConfigGenerator implementation
package open_code

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"go.uber.org/zap"
)

// OpenCodeConfigGenerator OpenCode 配置生成器
type OpenCodeConfigGenerator struct {
	skillStoragePath    string
	subagentStoragePath string
	commandStoragePath  string
	ruleStoragePath     string
	logger              *zap.Logger
}

// NewOpenCodeConfigGenerator 创建 OpenCode 配置生成器
func NewOpenCodeConfigGenerator(
	skillStoragePath string,
	subagentStoragePath string,
	commandStoragePath string,
	ruleStoragePath string,
	logger *zap.Logger,
) agent.AssetConfigGenerator {
	return &OpenCodeConfigGenerator{
		skillStoragePath:    skillStoragePath,
		subagentStoragePath: subagentStoragePath,
		commandStoragePath:  commandStoragePath,
		ruleStoragePath:     ruleStoragePath,
		logger:              logger,
	}
}

// GenerateConfig 生成 OpenCode 配置
func (g *OpenCodeConfigGenerator) GenerateConfig(ctx context.Context, req *agent.ConfigGenerateRequest) (*agent.ConfigGenerateResult, error) {
	configPath := req.ConfigPath

	g.logger.Info("开始生成OpenCode配置",
		zap.String("agent_role_id", req.AgentRoleID.String()),
		zap.String("config_path", configPath))

	// 清理现有配置（可选）
	if req.CleanExisting {
		if err := os.RemoveAll(configPath); err != nil && !os.IsNotExist(err) {
			g.logger.Warn("清理配置目录失败", zap.Error(err))
		}
	}

	// 创建目录结构: skills/, agents/, commands/, rules/
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

	// 复制 Settings 内容到 configPath 根目录（与 skills/、agents/ 并列）
	settingsCount := 0
	for _, settings := range req.Settings {
		if err := g.copySettingsDirectory(settings, configPath); err != nil {
			g.logger.Warn("复制Settings目录失败",
				zap.String("settings", settings.Name),
				zap.Error(err))
			continue
		}
		settingsCount++
	}

	// 复制 Skills
	skillsCount := 0
	for _, skill := range req.Skills {
		if err := g.copySkill(skill, configPath); err != nil {
			g.logger.Warn("复制Skill失败",
				zap.String("skill", skill.Name),
				zap.Error(err))
			continue
		}
		skillsCount++
	}

	// 复制 Commands
	commandsCount := 0
	for _, command := range req.Commands {
		if err := g.copyCommand(command, commandsDir); err != nil {
			g.logger.Warn("复制Command失败",
				zap.String("command", command.Name),
				zap.Error(err))
			continue
		}
		commandsCount++
	}

	// 复制 Subagents
	subagentsCount := 0
	for _, subagent := range req.Subagents {
		if err := g.copySubagent(subagent, agentsDir); err != nil {
			g.logger.Warn("复制Subagent失败",
				zap.String("subagent", subagent.Name),
				zap.Error(err))
			continue
		}
		subagentsCount++
	}

	// 复制 Rules
	rulesCount := 0
	for _, rule := range req.Rules {
		if err := g.copyRule(rule, rulesDir); err != nil {
			g.logger.Warn("复制Rule失败",
				zap.String("rule", rule.Name),
				zap.Error(err))
			continue
		}
		rulesCount++
	}

	g.logger.Info("OpenCode配置生成完成",
		zap.Int("skills_count", skillsCount),
		zap.Int("commands_count", commandsCount),
		zap.Int("subagents_count", subagentsCount),
		zap.Int("rules_count", rulesCount),
		zap.Int("settings_count", settingsCount))

	return &agent.ConfigGenerateResult{
		ConfigPath:     configPath,
		SkillsCount:    skillsCount,
		CommandsCount:  commandsCount,
		SubagentsCount: subagentsCount,
		RulesCount:     rulesCount,
		SettingsCount:  settingsCount,
	}, nil
}

// PreviewConfig 预览配置内容
func (g *OpenCodeConfigGenerator) PreviewConfig(ctx context.Context, req *agent.ConfigPreviewRequest) (*agent.ConfigPreviewResult, error) {
	// 预览逻辑（可选实现）
	return &agent.ConfigPreviewResult{
		Files: []agent.ConfigPreviewFile{},
	}, nil
}

// copySkill 复制技能目录到目标配置目录
func (g *OpenCodeConfigGenerator) copySkill(skill *model.Skill, targetDir string) error {
	// 技能源目录: {skillStoragePath}/{skillName}/
	sourceDir := filepath.Join(g.skillStoragePath, skill.ID.String())

	// 检查目录是否存在
	if stat, err := os.Stat(sourceDir); err != nil || !stat.IsDir() {
		return fmt.Errorf("技能目录不存在: %s", skill.Name)
	}

	// 目标目录: {targetDir}/skills/{skillName}/
	targetSkillDir := filepath.Join(targetDir, "skills", skill.Name)

	return g.copyDir(sourceDir, targetSkillDir)
}

// copyCommand 复制命令文件到目标目录
func (g *OpenCodeConfigGenerator) copyCommand(command *model.Command, targetDir string) error {
	// 源文件路径: {commandStoragePath}/{name}.md
	sourcePath := filepath.Join(g.commandStoragePath, command.Name+".md")
	targetPath := filepath.Join(targetDir, command.Name+".md")

	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("读取Command文件失败: %w", err)
	}

	return os.WriteFile(targetPath, content, 0644)
}

// copySubagent 复制子代理文件到目标目录
func (g *OpenCodeConfigGenerator) copySubagent(subagent *model.Subagent, targetDir string) error {
	// 文件名: {name}.md
	filename := strings.ReplaceAll(subagent.Name, " ", "-") + ".md"
	targetPath := filepath.Join(targetDir, filename)

	// 优先从存储目录读取文件
	sourcePath := filepath.Join(g.subagentStoragePath, filename)
	if content, err := os.ReadFile(sourcePath); err == nil {
		return os.WriteFile(targetPath, content, 0644)
	}

	// 存储目录没有文件，使用数据库中的 content（兼容旧数据）
	g.logger.Warn("子代理文件不在存储目录，使用数据库内容",
		zap.String("subagent", subagent.Name))

	content := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n%s",
		subagent.Name, subagent.Description, subagent.Content)

	return os.WriteFile(targetPath, []byte(content), 0644)
}

// copyRule 复制规则文件到目标目录
func (g *OpenCodeConfigGenerator) copyRule(rule *model.Rule, targetDir string) error {
	// 源文件路径: {ruleStoragePath}/{name}.md
	sourcePath := filepath.Join(g.ruleStoragePath, rule.Name+".md")
	targetPath := filepath.Join(targetDir, rule.Name+".md")

	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("读取Rule文件失败: %w", err)
	}

	return os.WriteFile(targetPath, content, 0644)
}

// copySettingsDirectory 复制Settings目录内容到目标目录
func (g *OpenCodeConfigGenerator) copySettingsDirectory(settings *model.Settings, targetDir string) error {
	if settings.DirectoryPath == "" {
		return fmt.Errorf("Settings目录路径为空")
	}

	sourceDir := settings.DirectoryPath
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return fmt.Errorf("Settings源目录不存在: %s", sourceDir)
	}

	g.logger.Info("复制Settings内容到配置目录",
		zap.String("settings", settings.Name),
		zap.String("source", sourceDir),
		zap.String("target", targetDir))

	return g.copyDirContents(sourceDir, targetDir)
}

// copyDir 复制目录（如果目标存在则先清理）
func (g *OpenCodeConfigGenerator) copyDir(sourceDir, targetDir string) error {
	// 如果目标目录已存在，先删除
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return fmt.Errorf("清理目标目录失败: %w", err)
		}
	}

	// 创建目标目录
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	return g.copyDirContents(sourceDir, targetDir)
}

// copyDirContents 递归复制目录内容
func (g *OpenCodeConfigGenerator) copyDirContents(srcDir, destDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("读取源目录失败: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		destPath := filepath.Join(destDir, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return fmt.Errorf("创建子目录失败: %w", err)
			}
			if err := g.copyDirContents(srcPath, destPath); err != nil {
				return err
			}
		} else {
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

// copyFile 执行一次文件复制
func (g *OpenCodeConfigGenerator) copyFile(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer targetFile.Close()

	_, err = io.Copy(targetFile, sourceFile)
	if err != nil {
		os.Remove(targetPath)
		return fmt.Errorf("复制内容失败: %w", err)
	}

	return targetFile.Sync()
}
