package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// 从环境变量或配置获取数据库连接
	dsn := "isdp:dthxzsy@2026@tcp(rm-bp1u503844l66n8g7no.mysql.rds.aliyuncs.com:3306)/dev_lv?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 测试连接
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("✓ 数据库连接成功")

	// 检查 threads 表是否有 type 字段
	var hasTypeColumn bool
	rows, err := db.Query("SHOW COLUMNS FROM threads LIKE 'type'")
	if err != nil {
		log.Printf("Warning: Failed to check columns: %v", err)
	} else {
		hasTypeColumn = rows.Next()
		rows.Close()
	}

	if hasTypeColumn {
		fmt.Println("✓ threads.type 字段已存在")
	} else {
		fmt.Println("→ 添加 threads.type 字段...")
		_, err := db.Exec("ALTER TABLE threads ADD COLUMN type VARCHAR(20) DEFAULT 'workflow' COMMENT '会话类型(workflow-工作流模式, free_discussion-自由协作模式)'")
		if err != nil {
			log.Printf("Error adding type column: %v", err)
		} else {
			fmt.Println("✓ threads.type 字段添加成功")
		}
	}

	// 检查 threads 表是否有 available_agents 字段
	var hasAvailableAgents bool
	rows, err = db.Query("SHOW COLUMNS FROM threads LIKE 'available_agents'")
	if err != nil {
		log.Printf("Warning: Failed to check columns: %v", err)
	} else {
		hasAvailableAgents = rows.Next()
		rows.Close()
	}

	if hasAvailableAgents {
		fmt.Println("✓ threads.available_agents 字段已存在")
	} else {
		fmt.Println("→ 添加 threads.available_agents 字段...")
		_, err := db.Exec("ALTER TABLE threads ADD COLUMN available_agents JSON DEFAULT NULL COMMENT '可用Agent ID列表(JSON数组, 自由协作模式使用)'")
		if err != nil {
			log.Printf("Error adding available_agents column: %v", err)
		} else {
			fmt.Println("✓ threads.available_agents 字段添加成功")
		}
	}

	// 检查 workflow_templates 表是否有 mode 字段
	var hasModeColumn bool
	rows, err = db.Query("SHOW COLUMNS FROM workflow_templates LIKE 'mode'")
	if err != nil {
		log.Printf("Warning: Failed to check columns: %v", err)
	} else {
		hasModeColumn = rows.Next()
		rows.Close()
	}

	if hasModeColumn {
		fmt.Println("✓ workflow_templates.mode 字段已存在")
	} else {
		fmt.Println("→ 添加 workflow_templates.mode 字段...")
		_, err := db.Exec("ALTER TABLE workflow_templates ADD COLUMN mode VARCHAR(20) DEFAULT 'workflow' COMMENT '团队模式(workflow-工作流模式, free-自由模式)'")
		if err != nil {
			log.Printf("Error adding mode column: %v", err)
		} else {
			fmt.Println("✓ workflow_templates.mode 字段添加成功")
		}
	}

	// 检查 multi_mention_requests 表是否存在
	var hasMultiMentionTable bool
	rows, err = db.Query("SHOW TABLES LIKE 'multi_mention_requests'")
	if err != nil {
		log.Printf("Warning: Failed to check tables: %v", err)
	} else {
		hasMultiMentionTable = rows.Next()
		rows.Close()
	}

	if hasMultiMentionTable {
		fmt.Println("✓ multi_mention_requests 表已存在")
	} else {
		fmt.Println("→ 创建 multi_mention_requests 表...")
		_, err := db.Exec(`
			CREATE TABLE multi_mention_requests (
			    id VARCHAR(64) NOT NULL COMMENT '请求唯一标识符',
			    thread_id VARCHAR(64) NOT NULL COMMENT '关联会话ID',
			    initiator VARCHAR(100) NOT NULL COMMENT '发起者Agent ID',
			    callback_to VARCHAR(100) NOT NULL COMMENT '回调Agent ID',
			    targets JSON NOT NULL COMMENT '目标Agent ID列表(JSON数组, 1-3个)',
			    question TEXT NOT NULL COMMENT '问题内容',
			    context TEXT COMMENT '上下文信息',
			    status VARCHAR(20) DEFAULT 'pending' COMMENT '请求状态(pending/running/partial/done/timeout/failed)',
			    timeout_minutes INT DEFAULT 8 COMMENT '超时时间(分钟)',
			    search_evidence JSON COMMENT '搜索证据引用列表(JSON数组)',
			    override_reason TEXT COMMENT '跳过搜索的理由',
			    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
			    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
			    PRIMARY KEY (id),
			    KEY idx_mm_requests_thread_id (thread_id),
			    KEY idx_mm_requests_status (status),
			    KEY idx_mm_requests_initiator (initiator)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多Agent讨论请求表'
		`)
		if err != nil {
			log.Printf("Error creating multi_mention_requests table: %v", err)
		} else {
			fmt.Println("✓ multi_mention_requests 表创建成功")
		}
	}

	// 检查 multi_mention_responses 表是否存在
	var hasMultiMentionResponsesTable bool
	rows, err = db.Query("SHOW TABLES LIKE 'multi_mention_responses'")
	if err != nil {
		log.Printf("Warning: Failed to check tables: %v", err)
	} else {
		hasMultiMentionResponsesTable = rows.Next()
		rows.Close()
	}

	if hasMultiMentionResponsesTable {
		fmt.Println("✓ multi_mention_responses 表已存在")
	} else {
		fmt.Println("→ 创建 multi_mention_responses 表...")
		_, err := db.Exec(`
			CREATE TABLE multi_mention_responses (
			    id VARCHAR(64) NOT NULL COMMENT '响应唯一标识符',
			    request_id VARCHAR(64) NOT NULL COMMENT '关联请求ID',
			    agent_id VARCHAR(100) NOT NULL COMMENT '响应Agent ID',
			    content TEXT NOT NULL COMMENT '响应内容',
			    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
			    PRIMARY KEY (id),
			    KEY idx_mm_responses_request_id (request_id),
			    KEY idx_mm_responses_agent_id (agent_id)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多Agent讨论响应表'
		`)
		if err != nil {
			log.Printf("Error creating multi_mention_responses table: %v", err)
		} else {
			fmt.Println("✓ multi_mention_responses 表创建成功")
		}
	}

	fmt.Println("\n=== 迁移完成 ===")
	os.Exit(0)
}