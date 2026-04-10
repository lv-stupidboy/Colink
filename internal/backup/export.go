// Package backup 提供数据库导出功能
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
	"time"
)

// ExportService 数据库导出服务
type ExportService struct {
	db   *sql.DB
	dbType string
}

// NewExportService 创建导出服务
func NewExportService(db *sql.DB, dbType string) *ExportService {
	return &ExportService{db: db, dbType: dbType}
}

// ExportRequest 导出请求
type ExportRequest struct {
	Type         ExportType // full, data-only, schema-only
	Tables       []string   // 指定表（可选，默认全部）
	IncludeAssets bool       // 是否包含文件资产
	DataPath     string      // data 目录路径
}

// ExportType 导出类型
type ExportType string

const (
	ExportFull       ExportType = "full"        // 完整导出：Schema + 数据 + 文件
	ExportDataOnly   ExportType = "data-only"   // 仅数据
	ExportSchemaOnly ExportType = "schema-only" // 仅 Schema
)

// ExportResult 导出结果
type ExportResult struct {
	Success    bool      `json:"success"`
	Filename   string    `json:"filename"`
	Size       int64     `json:"size"`
	Version    int64     `json:"version"`
	ExportedAt time.Time `json:"exportedAt"`
	TableCount int       `json:"tableCount"`
	RowCount   int64     `json:"rowCount"`
	Error      string    `json:"error,omitempty"`
}

// Export 执行导出
func (s *ExportService) Export(ctx context.Context, req *ExportRequest, outputPath string) (*ExportResult, error) {
	result := &ExportResult{
		ExportedAt: time.Now(),
	}

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "isdp-export-")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 导出 manifest
	manifest := &DatabaseManifest{
		SchemaVersion: s.getSchemaVersion(ctx),
		ExportedAt:    time.Now(),
		DatabaseType:  s.dbType,
		Tables:        make(map[string]TableManifest),
	}

	// 导出数据
	tables := s.getTableList(req.Tables)
	result.TableCount = len(tables)
	var totalRows int

	for _, table := range tables {
		rowCount, err := s.exportTable(ctx, table, tempDir)
		if err != nil {
			return nil, fmt.Errorf("导出表 %s 失败: %w", table, err)
		}
		manifest.Tables[table] = TableManifest{RowCount: rowCount}
		totalRows += rowCount
	}
	result.RowCount = int64(totalRows)

	// 写入 manifest.json
	manifestPath := filepath.Join(tempDir, "manifest.json")
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(manifestPath, manifestData, 0644)

	// 导出文件资产（可选）
	if req.IncludeAssets && req.DataPath != "" {
		assetsPath := filepath.Join(req.DataPath, "agent-assets")
		if _, err := os.Stat(assetsPath); err == nil {
			assetsZip := filepath.Join(tempDir, "assets.zip")
			if err := zipDir(assetsPath, assetsZip); err != nil {
				return nil, fmt.Errorf("导出资产目录失败: %w", err)
			}
		}
	}

	// 打包为 zip
	if outputPath == "" {
		outputPath = filepath.Join(os.TempDir(), fmt.Sprintf("isdp-export-%s.zip", time.Now().Format("20060102-150405")))
	}

	zipData, err := createZipFromDir(tempDir)
	if err != nil {
		return nil, fmt.Errorf("打包失败: %w", err)
	}

	os.WriteFile(outputPath, zipData, 0644)
	result.Filename = filepath.Base(outputPath)
	result.Size = int64(len(zipData))
	result.Success = true

	return result, nil
}

// DatabaseManifest 数据库导出清单
type DatabaseManifest struct {
	SchemaVersion int64                 `json:"schemaVersion"`
	ExportedAt    time.Time             `json:"exportedAt"`
	DatabaseType  string                `json:"databaseType"`
	AppVersion    string                `json:"appVersion"`
	Tables        map[string]TableManifest `json:"tables"`
}

// TableManifest 表清单
type TableManifest struct {
	RowCount int `json:"rowCount"`
}

// getSchemaVersion 获取 Schema 版本
func (s *ExportService) getSchemaVersion(ctx context.Context) int64 {
	// SQLite 使用 PRAGMA user_version
	if s.dbType == "sqlite" {
		var version int64
		s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version)
		return version
	}
	return 0
}

// getTableList 获取表列表
func (s *ExportService) getTableList(specifiedTables []string) []string {
	if len(specifiedTables) > 0 {
		return specifiedTables
	}

	// 默认导出所有主表
	return []string{
		"agent_configs", "agent_invocations", "artifacts", "base_agents",
		"commands", "invocation_content_blocks", "knowledge_bases",
		"messages", "projects", "rules", "sandboxes", "settings",
		"skill_registries", "skills", "subagents", "threads", "workflow_templates",
	}
}

// exportTable 导出单个表
func (s *ExportService) exportTable(ctx context.Context, table, tempDir string) (int, error) {
	// 查询表数据
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", table))
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	// 获取列名
	columns, err := rows.Columns()
	if err != nil {
		return 0, err
	}

	// 收集数据
	var tableData []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return 0, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		tableData = append(tableData, row)
	}

	// 写入 JSON 文件
	dataFile := filepath.Join(tempDir, "data", table+".json")
	os.MkdirAll(filepath.Dir(dataFile), 0755)

	fileData := map[string]interface{}{
		"table":   table,
		"columns": columns,
		"rows":    tableData,
	}

	jsonData, _ := json.MarshalIndent(fileData, "", "  ")
	os.WriteFile(dataFile, jsonData, 0644)

	return len(tableData), nil
}

// createZipFromDir 从目录创建 zip
func createZipFromDir(dir string) ([]byte, error) {
	// 创建内存 zip
	f, err := os.CreateTemp("", "export-*.zip")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath := strings.TrimPrefix(path, dir)
		if relPath == "" {
			return nil
		}
		relPath = strings.TrimPrefix(relPath, "/")

		if info.IsDir() {
			w.Create(relPath + "/")
			return nil
		}

		fw, err := w.Create(relPath)
		if err != nil {
			return err
		}

		fr, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fr.Close()

		io.Copy(fw, fr)
		return nil
	})

	w.Close()
	f.Close()

	return os.ReadFile(f.Name())
}