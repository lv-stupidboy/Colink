// Package backup 提供数据库备份恢复功能
package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupConfig 备份配置
type BackupConfig struct {
	Enabled    bool          // 是否启用自动备份
	Interval   string        // 备份间隔：daily, weekly, manual
	Retention  int           // 保留备份数量
	Path       string        // 备份目录路径
	DBPath     string        // SQLite 数据库路径
	DataPath   string        // data 目录路径
}

// BackupService 备份服务
type BackupService struct {
	config BackupConfig
}

// NewBackupService 创建备份服务
func NewBackupService(config BackupConfig) *BackupService {
	return &BackupService{config: config}
}

// BackupResult 备份结果
type BackupResult struct {
	Success    bool      `json:"success"`
	Timestamp  string    `json:"timestamp"`
	DBFile     string    `json:"dbFile"`
	AssetsFile string    `json:"assetsFile"`
	Error      string    `json:"error,omitempty"`
}

// Backup 执行备份
func (s *BackupService) Backup(ctx context.Context) (*BackupResult, error) {
	if s.config.DBPath == "" {
		return nil, fmt.Errorf("数据库路径未配置")
	}

	timestamp := time.Now().Format("20060102-150405")

	// 创建备份目录
	if err := os.MkdirAll(s.config.Path, 0755); err != nil {
		return nil, fmt.Errorf("创建备份目录失败: %w", err)
	}

	result := &BackupResult{
		Timestamp: timestamp,
	}

	// SQLite 备份：复制 db 文件
	if _, err := os.Stat(s.config.DBPath); err == nil {
		dbBackup := filepath.Join(s.config.Path, fmt.Sprintf("isdp-%s.db", timestamp))
		if err := copyFile(s.config.DBPath, dbBackup); err != nil {
			return nil, fmt.Errorf("备份数据库失败: %w", err)
		}
		result.DBFile = dbBackup
	}

	// 备份资产目录
	assetsPath := filepath.Join(s.config.DataPath, "agent-assets")
	if _, err := os.Stat(assetsPath); err == nil {
		assetsZip := filepath.Join(s.config.Path, fmt.Sprintf("assets-%s.zip", timestamp))
		if err := zipDir(assetsPath, assetsZip); err != nil {
			return nil, fmt.Errorf("备份资产目录失败: %w", err)
		}
		result.AssetsFile = assetsZip
	}

	// 清理过期备份
	s.cleanupOldBackups()

	result.Success = true
	return result, nil
}

// Restore 恢复备份
func (s *BackupService) Restore(ctx context.Context, timestamp string) error {
	if s.config.DBPath == "" {
		return fmt.Errorf("数据库路径未配置")
	}

	// 先备份当前状态
	s.Backup(ctx)

	// 恢复数据库
	dbBackup := filepath.Join(s.config.Path, fmt.Sprintf("isdp-%s.db", timestamp))
	if _, err := os.Stat(dbBackup); err == nil {
		if err := copyFile(dbBackup, s.config.DBPath); err != nil {
			return fmt.Errorf("恢复数据库失败: %w", err)
		}
	}

	// 恢复资产目录
	assetsZip := filepath.Join(s.config.Path, fmt.Sprintf("assets-%s.zip", timestamp))
	if _, err := os.Stat(assetsZip); err == nil {
		assetsPath := filepath.Join(s.config.DataPath, "agent-assets")
		// 先清空目录
		os.RemoveAll(assetsPath)
		if err := unzipTo(assetsZip, assetsPath); err != nil {
			return fmt.Errorf("恢复资产目录失败: %w", err)
		}
	}

	return nil
}

// ListBackups 列出所有备份
func (s *BackupService) ListBackups(ctx context.Context) ([]BackupInfo, error) {
	entries, err := os.ReadDir(s.config.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupInfo{}, nil
		}
		return nil, err
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 解析时间戳
		var timestamp string
		if strings.HasPrefix(name, "isdp-") && strings.HasSuffix(name, ".db") {
			timestamp = strings.TrimSuffix(strings.TrimPrefix(name, "isdp-"), ".db")
		} else if strings.HasPrefix(name, "assets-") && strings.HasSuffix(name, ".zip") {
			timestamp = strings.TrimSuffix(strings.TrimPrefix(name, "assets-"), ".zip")
		}

		if timestamp != "" {
			backups = append(backups, BackupInfo{
				Timestamp: timestamp,
				Size:      info.Size(),
				CreatedAt: info.ModTime(),
			})
		}
	}

	// 按时间排序（最新的在前）
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// BackupInfo 备份信息
type BackupInfo struct {
	Timestamp string    `json:"timestamp"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

// cleanupOldBackups 清理过期备份
func (s *BackupService) cleanupOldBackups() {
	if s.config.Retention <= 0 {
		return
	}

	entries, err := os.ReadDir(s.config.Path)
	if err != nil {
		return
	}

	// 收集所有备份文件
	var backupFiles []string
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			if strings.HasPrefix(name, "isdp-") || strings.HasPrefix(name, "assets-") {
				backupFiles = append(backupFiles, filepath.Join(s.config.Path, name))
			}
		}
	}

	// 按时间排序
	sort.Slice(backupFiles, func(i, j int) bool {
		infoI, _ := os.Stat(backupFiles[i])
		infoJ, _ := os.Stat(backupFiles[j])
		return infoI.ModTime().After(infoJ.ModTime())
	})

	// 删除超出保留数量的备份
	if len(backupFiles) > s.config.Retention*2 { // 每次备份有两个文件（db + assets）
		for i := s.config.Retention * 2; i < len(backupFiles); i++ {
			os.Remove(backupFiles[i])
		}
	}
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0644)
}