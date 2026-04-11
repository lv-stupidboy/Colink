package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// Result 操作结果（JSON 输出）
type Result struct {
	Success        bool   `json:"success"`
	CurrentVersion int64  `json:"currentVersion,omitempty"`
	TargetVersion  int64  `json:"targetVersion,omitempty"`
	Migrations     int    `json:"migrations,omitempty"`
	BackupPath     string `json:"backupPath,omitempty"`
	Error          string `json:"error,omitempty"`
	Message        string `json:"message,omitempty"`
}

// CLI 参数
type CLIArgs struct {
	Command       string // up, down, status, version
	DBPath        string // SQLite 数据库路径
	Version       string // 版本号，如 "1.1.0"
	MigrationsDir string // 迁移脚本目录
	Target        int64  // 目标 goose 版本（可选）
	DryRun        bool   // 预览模式
	Backup        bool   // 迁移前备份
	Verbose       bool   // 详细输出
	JSONOutput    bool   // JSON 格式输出
}

func main() {
	args := parseArgs()

	// 设置 goose 方言
	if err := goose.SetDialect("sqlite3"); err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("设置方言失败: %v", err))
		os.Exit(1)
	}

	// 执行命令
	switch args.Command {
	case "status":
		runStatus(args)
	case "version":
		runVersion(args)
	case "up":
		runUp(args)
	case "down":
		runDown(args)
	default:
		outputError(args.JSONOutput, fmt.Sprintf("未知命令: %s\n可用命令: up, down, status, version", args.Command))
		os.Exit(1)
	}
}

func parseArgs() CLIArgs {
	args := CLIArgs{
		Command: "status",
	}

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "--db":
			if i+1 < len(os.Args) {
				args.DBPath = os.Args[i+1]
				i++
			}
		case "--version":
			if i+1 < len(os.Args) {
				args.Version = os.Args[i+1]
				i++
			}
		case "--dir":
			if i+1 < len(os.Args) {
				args.MigrationsDir = os.Args[i+1]
				i++
			}
		case "--target":
			if i+1 < len(os.Args) {
				fmt.Sscanf(os.Args[i+1], "%d", &args.Target)
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
			goose.SetVerbose(true)
		case "up", "down", "status", "version":
			args.Command = arg
		}
	}

	// 构建迁移目录路径：sql-change/v{version}/sqlite/
	if args.MigrationsDir == "" && args.Version != "" {
		args.MigrationsDir = fmt.Sprintf("sql-change/v%s/sqlite", args.Version)
	}

	return args
}

// runStatus 显示当前数据库版本和迁移状态
func runStatus(args CLIArgs) {
	// 数据库不存在时返回版本 0
	if _, err := os.Stat(args.DBPath); os.IsNotExist(err) {
		if args.JSONOutput {
			outputResult(args.JSONOutput, Result{Success: true, CurrentVersion: 0, Message: "database not found"})
		} else {
			fmt.Println("Current goose version: 0 (database not found)")
		}
		return
	}

	db, err := openDB(args.DBPath)
	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("打开数据库失败: %v", err))
		os.Exit(1)
	}
	defer db.Close()

	ctx := context.Background()
	version, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("获取版本失败: %v", err))
		os.Exit(1)
	}

	if args.JSONOutput {
		outputResult(args.JSONOutput, Result{Success: true, CurrentVersion: version})
		return
	}

	fmt.Printf("Current goose version: %d\n", version)

	if args.MigrationsDir != "" {
		fmt.Println("\nMigration status:")
		goose.StatusContext(ctx, db, args.MigrationsDir)
	} else {
		fmt.Println("\n(Use --version to check pending migrations)")
	}
}

// runVersion 显示当前数据库版本
func runVersion(args CLIArgs) {
	// 数据库不存在时返回版本 0
	if _, err := os.Stat(args.DBPath); os.IsNotExist(err) {
		if args.JSONOutput {
			outputResult(args.JSONOutput, Result{Success: true, CurrentVersion: 0, Message: "database not found"})
		} else {
			fmt.Println("Current goose version: 0 (database not found)")
		}
		return
	}

	db, err := openDB(args.DBPath)
	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("打开数据库失败: %v", err))
		os.Exit(1)
	}
	defer db.Close()

	ctx := context.Background()
	version, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("获取版本失败: %v", err))
		os.Exit(1)
	}

	if args.JSONOutput {
		outputResult(args.JSONOutput, Result{Success: true, CurrentVersion: version})
	} else {
		fmt.Printf("Current goose version: %d\n", version)
	}
}

// runUp 执行迁移（包括首次初始化）
// 如果数据库不存在，会自动创建并执行迁移
func runUp(args CLIArgs) {
	if args.MigrationsDir == "" {
		outputError(args.JSONOutput, "需要指定 --version 或 --dir")
		os.Exit(1)
	}

	// 检查迁移目录是否存在
	if _, err := os.Stat(args.MigrationsDir); os.IsNotExist(err) {
		outputError(args.JSONOutput, fmt.Sprintf("迁移目录不存在: %s", args.MigrationsDir))
		os.Exit(1)
	}

	// 检查数据库是否已存在
	dbExists := true
	if _, err := os.Stat(args.DBPath); os.IsNotExist(err) {
		dbExists = false
		// 创建数据库目录
		os.MkdirAll(filepath.Dir(args.DBPath), 0755)
	}

	// 打开数据库（不存在时会创建）
	db, err := openDB(args.DBPath)
	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("打开数据库失败: %v", err))
		os.Exit(1)
	}
	defer db.Close()

	ctx := context.Background()

	// 获取当前版本
	currentVersion, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("获取版本失败: %v", err))
		os.Exit(1)
	}

	if args.DryRun {
		fmt.Printf("Dry-run: would migrate from version %d\n", currentVersion)
		goose.StatusContext(ctx, db, args.MigrationsDir)
		return
	}

	// 备份（仅数据库已存在时）
	var backupPath string
	if args.Backup && dbExists && currentVersion > 0 {
		backupPath = createBackup(args.DBPath, args.Verbose)
	}

	// 执行迁移
	if args.Target > 0 {
		err = goose.UpToContext(ctx, db, args.MigrationsDir, args.Target)
	} else {
		err = goose.UpContext(ctx, db, args.MigrationsDir)
	}

	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("迁移失败: %v", err))
		os.Exit(1)
	}

	// 获取新版本
	newVersion, _ := goose.GetDBVersionContext(ctx, db)

	// 验证表数量（用于确认初始化成功）
	var tableCount int
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tableCount)

	migrations := int(newVersion - currentVersion)
	var message string
	if !dbExists {
		message = fmt.Sprintf("initialized database with %d tables (version %d)", tableCount, newVersion)
	} else if migrations > 0 {
		message = fmt.Sprintf("migrated %d versions (%d -> %d)", migrations, currentVersion, newVersion)
	} else {
		message = fmt.Sprintf("no migrations needed (version %d)", currentVersion)
	}

	if args.JSONOutput {
		outputResult(args.JSONOutput, Result{
			Success:        true,
			CurrentVersion: currentVersion,
			TargetVersion:  newVersion,
			Migrations:     migrations,
			BackupPath:     backupPath,
			Message:        message,
		})
	} else {
		fmt.Printf("✓ %s\n", message)
		if backupPath != "" {
			fmt.Printf("  Backup: %s\n", backupPath)
		}
	}
}

// runDown 回滚迁移
func runDown(args CLIArgs) {
	if args.MigrationsDir == "" {
		outputError(args.JSONOutput, "需要指定 --version 或 --dir")
		os.Exit(1)
	}

	if _, err := os.Stat(args.DBPath); os.IsNotExist(err) {
		outputError(args.JSONOutput, "数据库不存在")
		os.Exit(1)
	}

	if args.DryRun {
		fmt.Println("Dry-run: would rollback one migration")
		return
	}

	db, err := openDB(args.DBPath)
	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("打开数据库失败: %v", err))
		os.Exit(1)
	}
	defer db.Close()

	ctx := context.Background()

	// 备份
	var backupPath string
	if args.Backup {
		backupPath = createBackup(args.DBPath, args.Verbose)
	}

	// 执行回滚
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
	outputResult(args.JSONOutput, Result{Success: true, TargetVersion: newVersion, BackupPath: backupPath})
}

// openDB 打开 SQLite 数据库
func openDB(dbPath string) (*sql.DB, error) {
	dsn := dbPath + "?_loc=auto"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	return db, nil
}

func createBackup(dbPath string, verbose bool) string {
	timestamp := time.Now().Format("20060102-150405")
	backupPath := dbPath + ".backup-" + timestamp

	input, err := os.ReadFile(dbPath)
	if err != nil {
		if verbose {
			log.Printf("备份失败: %v", err)
		}
		return ""
	}
	os.WriteFile(backupPath, input, 0644)

	if verbose {
		log.Printf("备份已创建: %s", backupPath)
	}
	return backupPath
}

func outputResult(jsonOutput bool, result Result) {
	if jsonOutput {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	} else {
		if result.Success {
			fmt.Printf("✓ %s\n", result.Message)
		} else {
			fmt.Printf("✗ %s\n", result.Error)
		}
	}
}

func outputError(jsonOutput bool, errMsg string) {
	if jsonOutput {
		data, _ := json.MarshalIndent(Result{Success: false, Error: errMsg}, "", "  ")
		fmt.Println(string(data))
	} else {
		log.Fatalf("Error: %s", errMsg)
	}
}