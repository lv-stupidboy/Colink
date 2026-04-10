package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"  // 纯 Go SQLite 驱动，无需 CGO
	"github.com/pressly/goose/v3"
)

// MigrationResult 迁移结果（JSON 输出）
type MigrationResult struct {
	Success        bool   `json:"success"`
	CurrentVersion int64  `json:"currentVersion,omitempty"`
	TargetVersion  int64  `json:"targetVersion,omitempty"`
	BackupPath     string `json:"backupPath,omitempty"`
	Error          string `json:"error,omitempty"`
	Message        string `json:"message,omitempty"`
}

// CLI 参数
type CLIArgs struct {
	Command        string // up, down, status, reset, version
	DBPath         string // SQLite 数据库路径或 MySQL DSN
	DBType         string // sqlite3 或 mysql
	Target         int64  // 目标版本（可选）
	MigrationsDir  string // 迁移脚本目录
	DryRun         bool   // 预览模式
	Backup         bool   // 迁移前备份
	Verbose        bool   // 详细输出
	JSONOutput     bool   // JSON 格式输出
}

func main() {
	args := parseArgs()

	// 打开数据库连接
	db, err := openDatabase(args.DBPath, args.DBType)
	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("打开数据库失败: %v", err))
		os.Exit(1)
	}
	defer db.Close()

	// 设置 goose 方言
	if err := goose.SetDialect(args.DBType); err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("设置方言失败: %v", err))
		os.Exit(1)
	}

	// 设置详细模式
	if args.Verbose {
		goose.SetVerbose(true)
	}

	ctx := context.Background()

	// 执行命令
	switch args.Command {
	case "status":
		runStatus(ctx, db, args)
	case "version":
		runVersion(ctx, db, args)
	case "up":
		runUp(ctx, db, args)
	case "down":
		runDown(ctx, db, args)
	case "reset":
		runReset(ctx, db, args)
	default:
		outputError(args.JSONOutput, fmt.Sprintf("未知命令: %s", args.Command))
		os.Exit(1)
	}
}

func parseArgs() CLIArgs {
	args := CLIArgs{
		Command:       "status",
		DBType:        "sqlite",  // modernc.org/sqlite 使用 "sqlite" 作为驱动名
		MigrationsDir: "sql-change/migrations",
	}

	// 解析命令行参数
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "--db":
			if i+1 < len(os.Args) {
				args.DBPath = os.Args[i+1]
				i++
			}
		case "--type":
			if i+1 < len(os.Args) {
				args.DBType = os.Args[i+1]
				i++
			}
		case "--dir":
			if i+1 < len(os.Args) {
				args.MigrationsDir = os.Args[i+1]
				i++
			}
		case "--target":
			if i+1 < len(os.Args) {
				var v int64
				fmt.Sscanf(os.Args[i+1], "%d", &v)
				args.Target = v
				i++
			}
		case "--dry-run":
			args.DryRun = true
		case "--backup":
			args.Backup = true
		case "--json":
			args.JSONOutput = true
		case "-v", "--verbose":
			args.Verbose = true
		case "up", "down", "status", "reset", "version":
			args.Command = arg
		}
	}

	return args
}

func openDatabase(dbPath string, dbType string) (*sql.DB, error) {
	var driver string

	switch dbType {
	case "sqlite", "sqlite3":
		driver = "sqlite"  // modernc.org/sqlite 驱动名
	case "mysql":
		driver = "mysql"
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", dbType)
	}

	db, err := sql.Open(driver, dbPath)
	if err != nil {
		return nil, err
	}

	// SQLite 特殊配置
	if driver == "sqlite" {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %v", err)
	}

	return db, nil
}

func runStatus(ctx context.Context, db *sql.DB, args CLIArgs) {
	if args.JSONOutput {
		// JSON 输出需要获取版本信息
		version, err := goose.GetDBVersionContext(ctx, db)
		if err != nil {
			outputError(args.JSONOutput, fmt.Sprintf("获取版本失败: %v", err))
			os.Exit(1)
		}
		result := MigrationResult{
			Success:        true,
			CurrentVersion: version,
			Message:        "Use goose.Status for detailed migration list",
		}
		jsonData, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonData))
	} else {
		// 使用 goose.Status 打印详细状态
		if err := goose.StatusContext(ctx, db, args.MigrationsDir); err != nil {
			log.Fatalf("获取状态失败: %v", err)
		}
	}
}

func runVersion(ctx context.Context, db *sql.DB, args CLIArgs) {
	version, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("获取版本失败: %v", err))
		os.Exit(1)
	}

	if args.JSONOutput {
		result := MigrationResult{
			Success:        true,
			CurrentVersion: version,
		}
		jsonData, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonData))
	} else {
		fmt.Printf("Current version: %d\n", version)
	}
}

func runUp(ctx context.Context, db *sql.DB, args CLIArgs) {
	// 获取当前版本
	currentVersion, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("获取当前版本失败: %v", err))
		os.Exit(1)
	}

	// DryRun 模式只预览
	if args.DryRun {
		if args.JSONOutput {
			result := MigrationResult{
				Success:        true,
				CurrentVersion: currentVersion,
				Message:        "Dry-run mode - no migrations executed",
			}
			jsonData, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(jsonData))
		} else {
			fmt.Printf("Dry-run: would migrate from version %d\n", currentVersion)
			goose.StatusContext(ctx, db, args.MigrationsDir)
		}
		return
	}

	// 执行迁移前备份（SQLite）
	if args.Backup && (args.DBType == "sqlite" || args.DBType == "sqlite3") {
		backupPath := createBackup(args.DBPath)
		if args.Verbose {
			log.Printf("Backup created: %s", backupPath)
		}
	}

	// 执行迁移
	if args.Target > 0 {
		err = goose.UpToContext(ctx, db, args.MigrationsDir, args.Target)
	} else {
		err = goose.UpContext(ctx, db, args.MigrationsDir)
	}

	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("执行迁移失败: %v", err))
		os.Exit(1)
	}

	// 获取新版本
	newVersion, _ := goose.GetDBVersionContext(ctx, db)

	// 输出结果
	result := MigrationResult{
		Success:        true,
		CurrentVersion: currentVersion,
		TargetVersion:  newVersion,
	}

	if args.Backup && (args.DBType == "sqlite" || args.DBType == "sqlite3") {
		result.BackupPath = args.DBPath + ".backup"
	}

	if args.JSONOutput {
		jsonData, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonData))
	} else {
		fmt.Printf("Migration completed: %d -> %d\n", currentVersion, newVersion)
	}
}

func runDown(ctx context.Context, db *sql.DB, args CLIArgs) {
	currentVersion, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("获取当前版本失败: %v", err))
		os.Exit(1)
	}

	if args.DryRun {
		if args.JSONOutput {
			result := MigrationResult{
				Success:        true,
				CurrentVersion: currentVersion,
				Message:        "Dry-run mode - would rollback one migration",
			}
			jsonData, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(jsonData))
		} else {
			fmt.Println("Dry-run: would rollback one migration")
		}
		return
	}

	// 回滚
	if args.Target > 0 {
		err = goose.DownToContext(ctx, db, args.MigrationsDir, args.Target)
	} else {
		err = goose.DownContext(ctx, db, args.MigrationsDir)
	}

	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("回滚失败: %v", err))
		os.Exit(1)
	}

	newVersion, _ := goose.GetDBVersionContext(ctx, db)

	result := MigrationResult{
		Success:        true,
		CurrentVersion: currentVersion,
		TargetVersion:  newVersion,
	}

	if args.JSONOutput {
		jsonData, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonData))
	} else {
		fmt.Printf("Rollback completed: %d -> %d\n", currentVersion, newVersion)
	}
}

func runReset(ctx context.Context, db *sql.DB, args CLIArgs) {
	if args.DryRun {
		if args.JSONOutput {
			result := MigrationResult{
				Success: true,
				Message: "Dry-run mode - would reset all migrations",
			}
			jsonData, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(jsonData))
		} else {
			fmt.Println("Dry-run: would reset all migrations to version 0")
		}
		return
	}

	err := goose.ResetContext(ctx, db, args.MigrationsDir)
	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("重置失败: %v", err))
		os.Exit(1)
	}

	result := MigrationResult{
		Success:        true,
		CurrentVersion: 0,
		TargetVersion:  0,
	}

	if args.JSONOutput {
		jsonData, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonData))
	} else {
		fmt.Println("All migrations reset to version 0")
	}
}

func outputError(jsonOutput bool, errMsg string) {
	if jsonOutput {
		result := MigrationResult{
			Success: false,
			Error:   errMsg,
		}
		jsonData, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonData))
	} else {
		log.Fatalf("Error: %s", errMsg)
	}
}

func createBackup(dbPath string) string {
	timestamp := time.Now().Format("20060102-150405")
	backupPath := dbPath + ".backup-" + timestamp

	input, err := os.ReadFile(dbPath)
	if err != nil {
		log.Printf("Backup failed: %v", err)
		return ""
	}
	os.WriteFile(backupPath, input, 0644)

	return backupPath
}