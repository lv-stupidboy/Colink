package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// MySQL 连接配置 - 从 config.yaml 读取
	dsn := "isdp:dthxzsy@2026@tcp(rm-bp1u503844l66n8g7no.mysql.rds.aliyuncs.com:3306)/dev_lv?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}

	fmt.Println("Connected to MySQL database")

	// 迁移 1: 为 threads 表添加 name 字段
	migrateThreadName(db)

	// 迁移 2: 为 workflow_templates 表添加 transitions 字段
	migrateWorkflowTransitions(db)

	// 迁移 3: 为 agent_configs 表添加能力声明字段
	migrateAgentCapabilities(db)

	fmt.Println("\nAll migrations completed!")
}

func migrateThreadName(db *sql.DB) {
	fmt.Println("\n[Migration 1] Adding 'name' column to threads table...")

	// 检查列是否存在
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'threads' AND COLUMN_NAME = 'name'
	`).Scan(&count)
	if err != nil {
		log.Fatalf("Failed to check column: %v", err)
	}

	if count > 0 {
		fmt.Println("  Column 'name' already exists, skipping")
		return
	}

	// 添加列
	_, err = db.Exec("ALTER TABLE threads ADD COLUMN name VARCHAR(255) NOT NULL DEFAULT '' COMMENT '会话名称'")
	if err != nil {
		log.Fatalf("Failed to add column: %v", err)
	}
	fmt.Println("  Successfully added 'name' column")
}

func migrateWorkflowTransitions(db *sql.DB) {
	fmt.Println("\n[Migration 2] Adding 'transitions' column to workflow_templates table...")

	// 检查列是否存在
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'workflow_templates' AND COLUMN_NAME = 'transitions'
	`).Scan(&count)
	if err != nil {
		log.Fatalf("Failed to check column: %v", err)
	}

	if count > 0 {
		fmt.Println("  Column 'transitions' already exists, skipping")
		return
	}

	// 添加列
	_, err = db.Exec("ALTER TABLE workflow_templates ADD COLUMN transitions JSON DEFAULT NULL COMMENT 'A2A路由转换规则'")
	if err != nil {
		log.Fatalf("Failed to add column: %v", err)
	}
	fmt.Println("  Successfully added 'transitions' column")
}

func migrateAgentCapabilities(db *sql.DB) {
	fmt.Println("\n[Migration 3] Adding capability columns to agent_configs table...")

	columns := []struct {
		name string
		sql  string
	}{
		{"capabilities", "ALTER TABLE agent_configs ADD COLUMN capabilities JSON DEFAULT NULL COMMENT 'Agent能力声明'"},
		{"dependencies", "ALTER TABLE agent_configs ADD COLUMN dependencies JSON DEFAULT NULL COMMENT 'Agent依赖配置'"},
		{"outputs", "ALTER TABLE agent_configs ADD COLUMN outputs JSON DEFAULT NULL COMMENT 'Agent输出配置'"},
	}

	for _, col := range columns {
		// 检查列是否存在
		var count int
		err := db.QueryRow(`
			SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'agent_configs' AND COLUMN_NAME = ?
		`, col.name).Scan(&count)
		if err != nil {
			log.Fatalf("Failed to check column %s: %v", col.name, err)
		}

		if count > 0 {
			fmt.Printf("  Column '%s' already exists, skipping\n", col.name)
			continue
		}

		// 添加列
		_, err = db.Exec(col.sql)
		if err != nil {
			log.Fatalf("Failed to add column %s: %v", col.name, err)
		}
		fmt.Printf("  Successfully added '%s' column\n", col.name)
	}
}