// Package backup 提供数据库导入功能
package backup

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ImportService 数据库导入服务
type ImportService struct {
	db     *sql.DB
	dbType string
}

// NewImportService 创建导入服务
func NewImportService(db *sql.DB, dbType string) *ImportService {
	return &ImportService{db: db, dbType: dbType}
}

// ImportPreviewResult 导入预览结果
type ImportPreviewResult struct {
	SourceVersion   int64            `json:"sourceVersion"`
	CurrentVersion  int64            `json:"currentVersion"`
	Compatible      bool             `json:"compatible"`
	SourceTables    map[string]TableManifest `json:"sourceTables"`
	ExistingTables  []string         `json:"existingTables"`
	NeedsMigration  bool             `json:"needsMigration"`
	Warning         string           `json:"warning,omitempty"`
}

// ImportConfirmRequest 导入确认请求
type ImportConfirmRequest struct {
	ConflictResolution string   `json:"conflictResolution"` // skip, overwrite, merge
	NeedsMigration     bool     `json:"needsMigration"`
	IncludeAssets      bool     `json:"includeAssets"`
	SelectedTables     []string `json:"selectedTables"`
}

// ImportResult 导入结果
type ImportResult struct {
	Success       bool     `json:"success"`
	TablesImported []string `json:"tablesImported"`
	RowCount      int64    `json:"rowCount"`
	AssetsRestored bool    `json:"assetsRestored"`
	Error         string   `json:"error,omitempty"`
}

// ImportPreview 预览导入
func (s *ImportService) ImportPreview(zipData []byte) (*ImportPreviewResult, error) {
	// 解压并读取 manifest
	manifest, err := readManifestFromZip(zipData)
	if err != nil {
		return nil, fmt.Errorf("读取 manifest 失败: %w", err)
	}

	// 获取当前版本
	currentVersion := s.getSchemaVersion(context.Background())

	// 检查兼容性
	compatible := manifest.SchemaVersion <= currentVersion

	// 获取现有表
	existingTables := s.getExistingTables(context.Background())

	return &ImportPreviewResult{
		SourceVersion:   manifest.SchemaVersion,
		CurrentVersion:  currentVersion,
		Compatible:      compatible,
		SourceTables:    manifest.Tables,
		ExistingTables:  existingTables,
		NeedsMigration:  manifest.SchemaVersion > currentVersion,
		Warning:         s.getCompatibilityWarning(manifest.SchemaVersion, currentVersion),
	}, nil
}

// ImportConfirm 执行导入
func (s *ImportService) ImportConfirm(ctx context.Context, zipData []byte, req *ImportConfirmRequest, dataPath string) (*ImportResult, error) {
	result := &ImportResult{
		TablesImported: []string{},
	}

	// 解压 zip
	tempDir, err := os.MkdirTemp("", "isdp-import-")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := unzipData(zipData, tempDir); err != nil {
		return nil, fmt.Errorf("解压失败: %w", err)
	}

	// 按依赖顺序导入表
	tableOrder := s.getTableDependencyOrder()
	var totalRows int

	for _, table := range tableOrder {
		// 检查是否需要导入该表
		if len(req.SelectedTables) > 0 && !contains(req.SelectedTables, table) {
			continue
		}

		dataFile := filepath.Join(tempDir, "data", table+".json")
		if _, err := os.Stat(dataFile); err != nil {
			continue // 文件不存在，跳过
		}

		rowCount, err := s.importTable(ctx, table, dataFile, req.ConflictResolution)
		if err != nil {
			return nil, fmt.Errorf("导入表 %s 失败: %w", table, err)
		}

		result.TablesImported = append(result.TablesImported, table)
		totalRows += rowCount
	}
	result.RowCount = int64(totalRows)

	// 导入资产（可选）
	if req.IncludeAssets {
		assetsZip := filepath.Join(tempDir, "assets.zip")
		if _, err := os.Stat(assetsZip); err == nil {
			assetsPath := filepath.Join(dataPath, "agent-assets")
			os.RemoveAll(assetsPath)
			if err := unzipTo(assetsZip, assetsPath); err != nil {
				return nil, fmt.Errorf("恢复资产目录失败: %w", err)
			}
			result.AssetsRestored = true
		}
	}

	result.Success = true
	return result, nil
}

// getSchemaVersion 获取 Schema 版本
func (s *ImportService) getSchemaVersion(ctx context.Context) int64 {
	if s.dbType == "sqlite" {
		var version int64
		s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version)
		return version
	}
	return 0
}

// getExistingTables 获取现有表列表
func (s *ImportService) getExistingTables(ctx context.Context) []string {
	var tables []string

	if s.dbType == "sqlite" {
		rows, err := s.db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table'")
		if err != nil {
			return tables
		}
		defer rows.Close()

		for rows.Next() {
			var name string
			rows.Scan(&name)
			if !strings.HasPrefix(name, "goose_") {
				tables = append(tables, name)
			}
		}
	}

	return tables
}

// getTableDependencyOrder 获取表依赖顺序
func (s *ImportService) getTableDependencyOrder() []string {
	// 主表在前，绑定表在后
	return []string{
		// 主表
		"agent_configs", "base_agents", "commands", "rules", "settings",
		"skills", "skill_registries", "subagents", "workflow_templates",
		"projects", "threads", "messages", "agent_invocations",
		"invocation_content_blocks", "artifacts", "sandboxes", "knowledge_bases",
		// 绑定表
		"agent_command_bindings", "agent_rule_bindings", "agent_settings_bindings",
		"agent_skill_bindings", "agent_subagent_bindings",
		"command_skill_bindings", "subagent_skill_bindings",
	}
}

// importTable 导入单个表
func (s *ImportService) importTable(ctx context.Context, table, dataFile string, resolution string) (int, error) {
	// 读取 JSON 文件
	data, err := os.ReadFile(dataFile)
	if err != nil {
		return 0, err
	}

	var tableData struct {
		Table   string                   `json:"table"`
		Columns []string                 `json:"columns"`
		Rows    []map[string]interface{} `json:"rows"`
	}

	if err := json.Unmarshal(data, &tableData); err != nil {
		return 0, err
	}

	// 根据冲突处理策略导入
	for _, row := range tableData.Rows {
		switch resolution {
		case "skip":
			// 跳过已存在的记录
			if s.recordExists(ctx, table, row) {
				continue
			}
			s.insertRecord(ctx, table, tableData.Columns, row)
		case "overwrite":
			// 先删除再插入
			s.deleteRecord(ctx, table, row)
			s.insertRecord(ctx, table, tableData.Columns, row)
		case "merge":
			// 如果不存在则插入
			if !s.recordExists(ctx, table, row) {
				s.insertRecord(ctx, table, tableData.Columns, row)
			}
		}
	}

	return len(tableData.Rows), nil
}

// recordExists 检查记录是否存在
func (s *ImportService) recordExists(ctx context.Context, table string, row map[string]interface{}) bool {
	if id, ok := row["id"]; ok {
		var count int
		s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id = ?", table), id).Scan(&count)
		return count > 0
	}
	return false
}

// insertRecord 插入记录
func (s *ImportService) insertRecord(ctx context.Context, table string, columns []string, row map[string]interface{}) error {
	// 构建 INSERT 语句
	placeholders := strings.Repeat("?, ", len(columns))
	placeholders = strings.TrimSuffix(placeholders, ", ")

	values := make([]interface{}, len(columns))
	for i, col := range columns {
		values[i] = row[col]
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(columns, ", "), placeholders)
	_, err := s.db.ExecContext(ctx, query, values...)
	return err
}

// deleteRecord 删除记录
func (s *ImportService) deleteRecord(ctx context.Context, table string, row map[string]interface{}) error {
	if id, ok := row["id"]; ok {
		_, err := s.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = ?", table), id)
		return err
	}
	return nil
}

// getCompatibilityWarning 获取兼容性警告
func (s *ImportService) getCompatibilityWarning(sourceVersion, currentVersion int64) string {
	if sourceVersion > currentVersion {
		return fmt.Sprintf("导出数据的版本 (%d) 高于当前版本 (%d)，建议先升级数据库", sourceVersion, currentVersion)
	}
	return ""
}

// readManifestFromZip 从 zip 读取 manifest
func readManifestFromZip(zipData []byte) (*DatabaseManifest, error) {
	f, err := os.CreateTemp("", "import-*.zip")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	f.Write(zipData)
	f.Close()

	r, err := zip.OpenReader(f.Name())
	if err != nil {
		return nil, err
	}
	defer r.Close()

	for _, file := range r.File {
		if file.Name == "manifest.json" {
			fr, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer fr.Close()

			data, err := io.ReadAll(fr)
			if err != nil {
				return nil, err
			}

			var manifest DatabaseManifest
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, err
			}
			return &manifest, nil
		}
	}

	return nil, fmt.Errorf("manifest.json not found")
}

// unzipData 解压 zip 数据
func unzipData(zipData []byte, destDir string) error {
	f, err := os.CreateTemp("", "import-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	f.Write(zipData)
	f.Close()

	return unzipTo(f.Name(), destDir)
}

// contains 检查字符串是否在数组中
func contains(arr []string, s string) bool {
	for _, a := range arr {
		if a == s {
			return true
		}
	}
	return false
}