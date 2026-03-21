package configgen

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"go.uber.org/zap"
)

// Downloader Skill 文件下载器
type Downloader struct {
	client    *http.Client
	maxRetries int
	logger    *zap.Logger
}

// NewDownloader 创建下载器
func NewDownloader(logger *zap.Logger) *Downloader {
	return &Downloader{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxRetries: 3,
		logger:     logger,
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
// agentType: "claude_code" 或 "open_code"
// targetDir: 目标目录（如 .claude/ 或 .opencode/）
func (d *Downloader) DownloadSkill(ctx context.Context, skill *model.Skill, agentType, targetDir string) (string, error) {
	// 获取对应智能体类型的下载地址
	url := d.getDownloadURL(skill, agentType)
	if url == "" {
		return "", fmt.Errorf("没有找到 %s 类型的下载地址", agentType)
	}

	// 确定目标文件路径
	fileName := d.getFileName(skill, agentType)
	subDir := "skills"
	if skill.Type == model.SkillTypeRule {
		subDir = "rules"
	}

	targetPath := filepath.Join(targetDir, subDir, fileName)

	// 创建目标目录
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	// 下载文件
	if err := d.downloadWithRetry(ctx, url, targetPath); err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}

	d.logger.Info("Skill 下载完成",
		zap.String("skill", skill.Name),
		zap.String("url", url),
		zap.String("path", targetPath))

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

// getDownloadURL 获取下载地址
func (d *Downloader) getDownloadURL(skill *model.Skill, agentType string) string {
	if skill.InstallSource == nil {
		return ""
	}

	// 尝试从 install_source 中获取对应类型的 URL
	// install_source 格式: {"claude_code": "https://...", "open_code": "https://..."}
	if url, ok := skill.InstallSource[agentType]; ok {
		return url
	}

	// 如果没有特定类型的 URL，尝试使用默认 key
	if url, ok := skill.InstallSource["default"]; ok {
		return url
	}

	// 尝试使用任意可用的 URL
	for _, url := range skill.InstallSource {
		if url != "" {
			return url
		}
	}

	return ""
}

// getFileName 获取保存的文件名
func (d *Downloader) getFileName(skill *model.Skill, agentType string) string {
	// 使用 skill 名称作为文件名
	// OpenCode 可能使用 .ts 扩展名，但这里统一使用 .md
	return skill.Name + ".md"
}

// downloadWithRetry 带重试的下载
func (d *Downloader) downloadWithRetry(ctx context.Context, url, targetPath string) error {
	var lastErr error

	for i := 0; i < d.maxRetries; i++ {
		if err := d.downloadOnce(ctx, url, targetPath); err != nil {
			lastErr = err
			d.logger.Warn("下载重试",
				zap.String("url", url),
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

	return fmt.Errorf("下载失败，已重试 %d 次: %w", d.maxRetries, lastErr)
}

// downloadOnce 执行一次下载
func (d *Downloader) downloadOnce(ctx context.Context, url, targetPath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP 错误: %d %s", resp.StatusCode, resp.Status)
	}

	// 创建临时文件
	tempPath := targetPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	// 写入内容
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("写入文件失败: %w", err)
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