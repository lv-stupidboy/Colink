#!/bin/bash
# ISDP Database Migration Tool
# 用于管理数据库版本和团队协作

set -e

DB_PATH="data/isdp.db"
MIGRATIONS_DIR="scripts"

usage() {
    echo "Usage: $0 [command]"
    echo "Commands:"
    echo "  status    - Show current database migration status"
    echo "  up        - Apply pending migrations"
    echo "  down      - Rollback last migration (not supported in SQLite)"
    echo "  new <name> - Create a new migration file"
    echo ""
}

status() {
    echo "Checking database status..."

    if [ ! -f "$DB_PATH" ]; then
        echo "Database not found at $DB_PATH"
        exit 1
    fi

    # 使用sqlite3命令行工具查看数据库信息
    if command -v sqlite3 &> /dev/null; then
        echo "Tables in database:"
        sqlite3 "$DB_PATH" ".tables"

        echo ""
        echo "Schema overview:"
        sqlite3 "$DB_PATH" ".schema"
    else
        echo "sqlite3 command not available, using Go schema extractor..."
        go run check_current_schema.go
    fi
}

apply_migrations() {
    echo "Applying migrations..."

    # 检查数据库是否存在，如果不存在则创建
    if [ ! -f "$DB_PATH" ]; then
        echo "Creating new database at $DB_PATH"
        mkdir -p data
        # 这里可以使用Go程序初始化数据库
        go run cmd/initdb/main.go || echo "Initialize database with initial schema..."
    fi

    # 查找所有迁移文件并按名称排序
    for migration_file in $(ls $MIGRATIONS_DIR/[0-9]*.sql 2>/dev/null | sort); do
        if [ -f "$migration_file" ]; then
            echo "Applying migration: $migration_file"

            if command -v sqlite3 &> /dev/null; then
                sqlite3 "$DB_PATH" < "$migration_file"
            else
                echo "Error: sqlite3 command not available. Please install SQLite3."
                exit 1
            fi
        fi
    done

    echo "All migrations applied successfully!"
}

create_migration() {
    if [ -z "$2" ]; then
        echo "Error: Migration name is required"
        usage
        exit 1
    fi

    TIMESTAMP=$(date +"%Y%m%d%H%M%S")
    NAME="$TIMESTAMP""_$2"
    FILE_PATH="$MIGRATIONS_DIR/$NAME.sql"

    echo "Creating new migration: $FILE_PATH"

    cat > "$FILE_PATH" << EOF
-- Migration: $NAME
-- Description:
-- Author:

-- UP (Apply this migration)
/*
ALTER TABLE table_name ADD COLUMN new_column TEXT;
*/

-- DOWN (Rollback this migration)
/*
ALTER TABLE table_name DROP COLUMN new_column;
*/
EOF

    echo "Migration file created: $FILE_PATH"
    echo "Please edit this file to add your migration SQL."
}

case "${1:-status}" in
    "status")
        status
        ;;
    "up")
        apply_migrations
        ;;
    "down")
        echo "Rollback not supported in SQLite. Use with caution."
        ;;
    "new")
        create_migration $1 $2
        ;;
    *)
        usage
        ;;
esac