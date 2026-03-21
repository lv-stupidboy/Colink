package configgen

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"go.uber.org/zap"
)

// Downloader Skill 文件下载器
// 从本地技能存储目录复制技能文件到目标项目目录
type Downloader struct {
	skillStoragePath string
	maxRetries       int
	logger           *zap.Logger
}

// NewDownloader 创建下载器
func NewDownloader(skillStoragePath string, logger *zap.Logger) *Downloader {
	return &Downloader{
		skillStoragePath: skillStoragePath,
		maxRetries:       3,
		logger:           logger,
	}
}

// DownloadResult 下载结果
type DownloadResult struct {
	SkillID   string
	SkillName string
	FilePath  string
	Error     error
}

// DownloadSkill 下载 Skill 文件到指定目录
// 从本地技能存储目录复制技能文件到目标项目目录
// agentType: "claude_code" 或 "open_code"
// targetDir: 目标目录（如 .claude/ 或 .opencode/）
func (d *Downloader) DownloadSkill(ctx context.Context, skill *model.Skill, agentType, targetDir string) (string, error) {
	// 确定源文件路径（本地存储的技能 zip 包）
	sourcePath := filepath.Join(d.skillStoragePath, skill.Name+".zip")
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return "", fmt.Errorf("技能文件不存在: %s", skill.Name)
	}

	// 确定目标文件路径
	fileName := d.getFileName(skill, agentType)
	targetPath := filepath.Join(targetDir, "skills", fileName)

	// 创建目标目录
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	// 复制文件
	if err := d.copyFileWithRetry(ctx, sourcePath, targetPath); err != nil {
		return "", fmt.Errorf("复制失败: %w", err)
	}

	d.logger.Info("Skill 复制完成",
		zap.String("skill", skill.Name),
		zap.String("source", sourcePath),
		zap.String("target", targetPath))

	return targetPath, nil
}

// DownloadSkills 批量下载 Skills
func (d *Downloader) DownloadSkills(ctx context.Context, skills []*model.Skill, agentType, targetDir string) []DownloadResult {
	results := make([]DownloadResult, 0, len(skills))

	for _, skill := range skills {
		filePath, err := d.DownloadSkill(ctx, skill, agentType, targetDir)
		results = append(results, DownloadResult{
			SkillID:   skill.ID.String(),
			SkillName: skill.Name,
			FilePath:  filePath,
			Error:     err,
		})
	}

	return results
}

// getFileName 获取保存的文件名
func (d *Downloader) getFileName(skill *model.Skill, agentType string) string {
	// 使用 skill 名称作为文件名
	// OpenCode 可能使用 .ts 扩展名，但这里统一使用 .md
	return skill.Name + ".md"
}

// copyFileWithRetry 带重试的文件复制
func (d *Downloader) copyFileWithRetry(ctx context.Context, sourcePath, targetPath string) error {
	var lastErr error

	for i := 0; i < d.maxRetries; i++ {
		if err := d.copyFileOnce(ctx, sourcePath, targetPath); err != nil {
			lastErr = err
			d.logger.Warn("复制重试",
				zap.String("source", sourcePath),
				zap.Int("attempt", i+1),
				zap.Error(err))

			// 短暂等待后重试
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second * time.Duration(i+1)):
				continue
			}
		}

		return nil
	}

	return fmt.Errorf("复制失败，已重试 %d 次: %w", d.maxRetries, lastErr)
}

// copyFileOnce 执行一次文件复制
func (d *Downloader) copyFileOnce(ctx context.Context, sourcePath, targetPath string) error {
	// 打开源文件
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer sourceFile.Close()

	// 创建临时文件
	tempPath := targetPath + ".tmp"
	targetFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer targetFile.Close()

	// 复制内容
	_, err = io.Copy(targetFile, sourceFile)
	if err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("复制内容失败: %w", err)
	}

	// 确保数据写入磁盘
	if err := targetFile.Sync(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("同步文件失败: %w", err)
	}

	// 原子性重命名
	if err := os.Rename(tempPath, targetPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("重命名文件失败: %w", err)
	}

	return nil
}

// CleanConfigDir 清理配置目录
func (d *Downloader) CleanConfigDir(targetDir string) error {
	skillsDir := filepath.Join(targetDir, "skills")
	rulesDir := filepath.Join(targetDir, "rules")

	// 删除 skills 目录
	if _, err := os.Stat(skillsDir); err == nil {
		if err := os.RemoveAll(skillsDir); err != nil {
			return fmt.Errorf("删除 skills 目录失败: %w", err)
		}
	}

	// 删除 rules 目录
	if _, err := os.Stat(rulesDir); err == nil {
		if err := os.RemoveAll(rulesDir); err != nil {
			return fmt.Errorf("删除 rules 目录失败: %w", err)
		}
	}

	return nil
}