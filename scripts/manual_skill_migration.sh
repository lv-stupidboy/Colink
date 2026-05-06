#!/bin/bash
# Skill 存储路径手动割接脚本
# 使用方法: ./scripts/manual_skill_migration.sh <db_path> <skills_dir>

DB_PATH="${1:-./data/sqlite/colink.db}"
SKILLS_DIR="${2:-./data/agent-assets/skills}"

if [ ! -f "$DB_PATH" ]; then
    echo "数据库不存在: $DB_PATH"
    exit 1
fi

if [ ! -d "$SKILLS_DIR" ]; then
    echo "Skills 目录不存在: $SKILLS_DIR"
    exit 1
fi

echo "开始割接..."
echo "数据库: $DB_PATH"
echo "Skills 目录: $SKILLS_DIR"

# 查询所有 skill
SKILLS=$(sqlite3 "$DB_PATH" "SELECT id, name FROM skills;")

MIGRATED=0
SKIPPED=0

while IFS='|' read -r ID NAME; do
    SRC_DIR="$SKILLS_DIR/$NAME"
    DST_DIR="$SKILLS_DIR/$ID"
    
    if [ -d "$SRC_DIR" ]; then
        if [ -d "$DST_DIR" ]; then
            rm -rf "$DST_DIR"
        fi
        mv "$SRC_DIR" "$DST_DIR"
        echo "迁移: $NAME -> $ID"
        MIGRATED=$((MIGRATED + 1))
    else
        echo "跳过: $NAME (目录不存在)"
        SKIPPED=$((SKIPPED + 1))
    fi
done <<< "$SKILLS"

echo "割接完成: $MIGRATED 个迁移, $SKIPPED 个跳过"
