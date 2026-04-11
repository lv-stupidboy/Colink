package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// Result 操作结果（JSON 输出）
type Result struct {
	Success        bool   `json:"success"`
	CurrentVersion int64  `json:"currentVersion,omitempty"`
	TargetVersion  int64  `json:"targetVersion,omitempty"`
	Init           bool   `json:"init,omitempty"`
	Migrations     int    `json:"migrations,omitempty"`
	BackupPath     string `json:"backupPath,omitempty"`
	Error          string `json:"error,omitempty"`
	Message        string `json:"message,omitempty"`
}

// CLI 参数
type CLIArgs struct {
	Command        string // init, up, down, status, reset, version
	DBPath         string // SQLite 数据库路径
	Version        string // 版本号，如 "1.0.1"
	InitSQLPath    string // 初始化 SQL 路径
	MigrationsDir  string // 迁移脚本目录
	Target         int64  // 目标 goose 版本（可选）
	DryRun         bool   // 预览模式
	Backup         bool   // 迁移前备份
	Verbose        bool   // 详细输出
	JSONOutput     bool   // JSON 格式输出
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
	case "init":
		runInit(args)
	case "status":
		runWithDB(args, runStatus)
	case "version":
		runWithDB(args, runVersion)
	case "up":
		runWithDB(args, runUp)
	case "down":
		runWithDB(args, runDown)
	case "reset":
		runWithDB(args, runReset)
	default:
		outputError(args.JSONOutput, fmt.Sprintf("未知命令: %s\n可用命令: init, up, down, status, reset, version", args.Command))
		os.Exit(1)
	}
}

func parseArgs() CLIArgs {
	args := CLIArgs{
		Command:     "status",
		InitSQLPath: "sql-change/init/init-sqlite.sql",
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
		case "--init-sql":
			if i+1 < len(os.Args) {
				args.InitSQLPath = os.Args[i+1]
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
		case "init", "up", "down", "status", "reset", "version":
			args.Command = arg
		}
	}

	// 构建迁移目录路径：sql-change/v{version}/sqlite/
	if args.MigrationsDir == "" && args.Version != "" {
		args.MigrationsDir = fmt.Sprintf("sql-change/v%s/sqlite", args.Version)
	}

	return args
}

// runInit 执行初始化（首次安装）
func runInit(args CLIArgs) {
	result := Result{}

	// 1. 检查 init SQL 文件
	if _, err := os.Stat(args.InitSQLPath); os.IsNotExist(err) {
		result.Error = fmt.Sprintf("init SQL not found: %s", args.InitSQLPath)
		outputResult(args.JSONOutput, result)
		os.Exit(1)
	}

	// 2. 检查数据库是否已存在
	if _, err := os.Stat(args.DBPath); err == nil {
		db, err := openDB(args.DBPath)
		if err != nil {
			result.Error = fmt.Sprintf("打开数据库失败: %v", err)
			outputResult(args.JSONOutput, result)
			os.Exit(1)
		}
		defer db.Close()

		// 检查表数量
		var tableCount int
		ctx := context.Background()
		db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tableCount)

		result.Success = true
		result.Message = fmt.Sprintf("database already exists (%d tables)", tableCount)
		outputResult(args.JSONOutput, result)
		return
	}

	// 3. 创建数据库目录
	os.MkdirAll(filepath.Dir(args.DBPath), 0755)

	// 4. 读取 init SQL
	sqlContent, err := os.ReadFile(args.InitSQLPath)
	if err != nil {
		result.Error = fmt.Sprintf("读取 init SQL 失败: %v", err)
		outputResult(args.JSONOutput, result)
		os.Exit(1)
	}

	// 5. 创建数据库并执行 SQL
	db, err := openDB(args.DBPath)
	if err != nil {
		result.Error = fmt.Sprintf("创建数据库失败: %v", err)
		outputResult(args.JSONOutput, result)
		os.Exit(1)
	}
	defer db.Close()

	ctx := context.Background()
	db.ExecContext(ctx, "PRAGMA foreign_keys = ON")

	// 执行 SQL（事务）
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		result.Error = fmt.Sprintf("开始事务失败: %v", err)
		outputResult(args.JSONOutput, result)
		os.Exit(1)
	}

	statements := splitSQL(string(sqlContent))
	for i, stmt := range statements {
		if stmt == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			tx.Rollback()
			result.Error = fmt.Sprintf("执行语句 %d 失败: %v", i+1, err)
			outputResult(args.JSONOutput, result)
			os.Exit(1)
		}
	}

	if err := tx.Commit(); err != nil {
		result.Error = fmt.Sprintf("提交事务失败: %v", err)
		outputResult(args.JSONOutput, result)
		os.Exit(1)
	}

	// 验证表数量
	var tableCount int
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tableCount)

	result.Success = true
	result.Init = true
	result.Message = fmt.Sprintf("initialized %d tables from %s", tableCount, args.InitSQLPath)
	outputResult(args.JSONOutput, result)
}

// runWithDB 打开数据库后执行操作
func runWithDB(args CLIArgs, fn func(context.Context, *sql.DB, CLIArgs)) {
	db, err := openDB(args.DBPath)
	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("打开数据库失败: %v", err))
		os.Exit(1)
	}
	defer db.Close()

	ctx := context.Background()
	fn(ctx, db, args)
}

func runStatus(ctx context.Context, db *sql.DB, args CLIArgs) {
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

func runVersion(ctx context.Context, db *sql.DB, args CLIArgs) {
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

func runUp(ctx context.Context, db *sql.DB, args CLIArgs) {
	if args.MigrationsDir == "" {
		outputError(args.JSONOutput, "需要指定 --version 或 --dir")
		os.Exit(1)
	}

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

	// 备份
	var backupPath string
	if args.Backup {
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

	newVersion, _ := goose.GetDBVersionContext(ctx, db)

	if args.JSONOutput {
		outputResult(args.JSONOutput, Result{
			Success:        true,
			CurrentVersion: currentVersion,
			TargetVersion:  newVersion,
			Migrations:     int(newVersion - currentVersion),
			BackupPath:     backupPath,
		})
	} else {
		fmt.Printf("✓ Migrated: %d -> %d\n", currentVersion, newVersion)
		if backupPath != "" {
			fmt.Printf("  Backup: %s\n", backupPath)
		}
	}
}

func runDown(ctx context.Context, db *sql.DB, args CLIArgs) {
	if args.MigrationsDir == "" {
		outputError(args.JSONOutput, "需要指定 --version 或 --dir")
		os.Exit(1)
	}

	if args.DryRun {
		fmt.Println("Dry-run: would rollback one migration")
		return
	}

	var err error
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
	outputResult(args.JSONOutput, Result{Success: true, TargetVersion: newVersion})
}

func runReset(ctx context.Context, db *sql.DB, args CLIArgs) {
	if args.DryRun {
		fmt.Println("Dry-run: would reset all migrations to version 0")
		return
	}

	err := goose.ResetContext(ctx, db, args.MigrationsDir)
	if err != nil {
		outputError(args.JSONOutput, fmt.Sprintf("重置失败: %v", err))
		os.Exit(1)
	}

	outputResult(args.JSONOutput, Result{Success: true, TargetVersion: 0})
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

func splitSQL(sql string) []string {
	var statements []string
	var current strings.Builder

	for _, line := range strings.Split(sql, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}

		current.WriteString(line)
		current.WriteString("\n")

		if strings.HasSuffix(trimmed, ";") {
			statements = append(statements, current.String())
			current.Reset()
		}
	}

	if current.Len() > 0 {
		statements = append(statements, current.String())
	}

	return statements
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