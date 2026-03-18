#!/bin/bash
# ISDP MySQL Schema Management Tool
# 用于管理华为云RDS MySQL上的多Schema隔离环境

set -e

# 从环境变量读取MySQL连接信息
MYSQL_HOST="${MYSQL_HOST:-localhost}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASS="${MYSQL_PASS:-}"
MYSQL_DB="${MYSQL_DB:-isdp_dev}"

# Use MYSQL_PWD for password (more secure than -p flag)
export MYSQL_PWD="$MYSQL_PASS"
MYSQL_CMD="mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER $MYSQL_DB"

# 脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
    echo "ISDP MySQL Schema Management Tool"
    echo ""
    echo "Usage: $0 <command> [schema_name]"
    echo ""
    echo "Commands:"
    echo "  create <name>   Create a new schema and initialize tables"
    echo "  drop <name>     Drop an existing schema"
    echo "  list            List all ISDP schemas (dev_* and shared)"
    echo ""
    echo "Environment Variables:"
    echo "  MYSQL_HOST      MySQL host (default: localhost)"
    echo "  MYSQL_PORT      MySQL port (default: 3306)"
    echo "  MYSQL_USER      MySQL username (default: root)"
    echo "  MYSQL_PASS      MySQL password"
    echo "  MYSQL_DB        MySQL database (default: isdp_dev)"
    echo ""
    echo "Examples:"
    echo "  $0 create dev_zhangsan"
    echo "  $0 create shared"
    echo "  $0 drop dev_zhangsan"
    echo "  $0 list"
}

create_schema() {
    local schema_name=$1

    if [ -z "$schema_name" ]; then
        echo "Error: schema name is required"
        usage
        exit 1
    fi

    echo "Creating schema '$schema_name'..."

    # 创建Schema
    $MYSQL_CMD -e "CREATE SCHEMA IF NOT EXISTS \`$schema_name\` DEFAULT CHARACTER SET utf8mb4;"

    # 初始化表结构
    echo "Initializing tables..."
    $MYSQL_CMD -e "USE \`$schema_name\`; SOURCE $SCRIPT_DIR/init_db_mysql.sql;"

    echo "Schema '$schema_name' created and initialized successfully!"
}

drop_schema() {
    local schema_name=$1

    if [ -z "$schema_name" ]; then
        echo "Error: schema name is required"
        usage
        exit 1
    fi

    # 安全检查：不允许删除 shared schema
    if [ "$schema_name" = "shared" ]; then
        echo "Error: cannot drop 'shared' schema (protected)"
        exit 1
    fi

    echo "Dropping schema '$schema_name'..."
    $MYSQL_CMD -e "DROP SCHEMA IF EXISTS \`$schema_name\`;"

    echo "Schema '$schema_name' dropped successfully!"
}

list_schemas() {
    echo "ISDP Schemas:"
    $MYSQL_CMD -e "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME LIKE 'dev_%' OR SCHEMA_NAME = 'shared';" 2>/dev/null || echo "  (no schemas found or connection failed)"
}

# 主逻辑
case "$1" in
    create)
        create_schema "$2"
        ;;
    drop)
        drop_schema "$2"
        ;;
    list)
        list_schemas
        ;;
    -h|--help|help)
        usage
        ;;
    *)
        usage
        exit 1
        ;;
esac