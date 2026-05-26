#!/bin/bash
# Colink Linux 升级脚本
# 升级已有安装：检测版本、停止服务、备份、复制新文件、增量迁移、合并配置
#
# 用法: ./upgrade.sh INSTALL_DIR [--force]

set -euo pipefail

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# 自动选择 python 命令（Windows 用 python，Linux 用 python3）
PYTHON=""
if command -v python3 &>/dev/null; then
    PYTHON=python3
elif command -v python &>/dev/null; then
    PYTHON=python
else
    echo -e "${RED}错误: 未找到 python，请安装 python3${NC}"; exit 1
fi

# 路径转换：MSYS/Git Bash 下 Python 需要原生路径
py_path() {
    if command -v cygpath &>/dev/null; then
        cygpath -m "$1"
    else
        echo "$1"
    fi
}

# 默认参数
INSTALL_DIR=""
FORCE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --force) FORCE=true; shift ;;
        -h|--help)
            echo "Colink Linux 升级脚本"
            echo ""
            echo "用法: ./upgrade.sh INSTALL_DIR [--force]"
            echo ""
            echo "参数:"
            echo "  INSTALL_DIR  安装目录（必填）"
            echo ""
            echo "选项:"
            echo "  --force      强制升级（忽略版本检查）"
            echo "  -h, --help   显示帮助"
            echo ""
            echo "安装包自动从项目 output/ 目录查找"
            exit 0
            ;;
        -*) echo "未知选项: $1"; exit 1 ;;
        *) INSTALL_DIR="$1"; shift ;;
    esac
done

# 参数校验
if [[ -z "$INSTALL_DIR" ]]; then
    echo -e "${RED}错误: 请指定安装目录${NC}"; exit 1
fi
if [[ ! -d "$INSTALL_DIR" ]]; then
    echo -e "${RED}错误: 安装目录不存在: $INSTALL_DIR${NC}"
    echo -e "${YELLOW}如需首次安装，请使用 deploy.sh${NC}"
    exit 1
fi

# 自动查找安装包
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PACKAGE_PATH=$(find "$PROJECT_ROOT/output" -name "Colink-Setup-*-linux-*.tar.gz" -type f 2>/dev/null | sort -V | tail -1)

if [[ -z "$PACKAGE_PATH" ]]; then
    echo -e "${RED}错误: 未找到安装包，请先运行 bash scripts/build-linux.sh${NC}"; exit 1
fi

echo -e "${CYAN}=== Colink Linux Upgrade ===${NC}"

# Step 1: 读取当前版本
echo -e "${YELLOW}[1/8] Detecting current version...${NC}"
CURRENT_VERSION=""
META_FILE="$INSTALL_DIR/.install-meta"
if [[ -f "$META_FILE" ]]; then
    PY_META=$(py_path "$META_FILE")
    CURRENT_VERSION=$($PYTHON -c "import json; print(json.load(open('$PY_META', encoding='utf-8')).get('version',''))" 2>/dev/null || "")
fi
if [[ -z "$CURRENT_VERSION" && -f "$INSTALL_DIR/VERSION" ]]; then
    CURRENT_VERSION=$(tr -d '\n\r' < "$INSTALL_DIR/VERSION")
fi
if [[ -z "$CURRENT_VERSION" ]]; then
    echo -e "${RED}错误: 无法检测当前版本。如需首次安装，请使用 deploy.sh${NC}"; exit 1
fi
echo -e "${GREEN}当前版本: $CURRENT_VERSION${NC}"

# Step 2: 读取新版本
echo -e "${YELLOW}[2/8] Reading new version...${NC}"
STAGING="$INSTALL_DIR/.staging-$$"
mkdir -p "$STAGING"
tar -xzf "$PACKAGE_PATH" -C "$STAGING"

NEW_VERSION=""
if [[ -f "$STAGING/VERSION" ]]; then
    NEW_VERSION=$(tr -d '\n\r' < "$STAGING/VERSION")
fi
if [[ -z "$NEW_VERSION" ]]; then
    rm -rf "$STAGING"
    echo -e "${RED}错误: 安装包中未找到 VERSION 文件${NC}"; exit 1
fi
echo -e "${GREEN}目标版本: $NEW_VERSION${NC}"
echo -e "Package: ${GREEN}$PACKAGE_PATH${NC}"

# 版本比较
version_compare() {
    local v1="$1" v2="$2"
    local IFS='.'
    read -ra a <<< "$v1"
    read -ra b <<< "$v2"
    for i in 0 1 2; do
        local x=${a[$i]:-0} y=${b[$i]:-0}
        if (( x < y )); then echo -1; return
        elif (( x > y )); then echo 1; return; fi
    done
    echo 0
}

if [[ "$FORCE" == false ]]; then
    CMP=$(version_compare "$NEW_VERSION" "$CURRENT_VERSION")
    if [[ $CMP -le 0 ]]; then
        rm -rf "$STAGING"
        echo -e "${YELLOW}目标版本 ($NEW_VERSION) 不高于当前版本 ($CURRENT_VERSION)，无需升级${NC}"
        echo -e "${YELLOW}使用 --force 可强制升级${NC}"
        exit 0
    fi
fi

# Step 3: 停止服务
echo -e "${YELLOW}[3/8] Stopping service...${NC}"
PID_FILE="$INSTALL_DIR/.colink-server.pid"
if [[ -f "$PID_FILE" ]]; then
    PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
        echo "  Stopping colink-server (PID $PID)..."
        kill -TERM "$PID" 2>/dev/null || true
        for i in $(seq 1 30); do
            if ! kill -0 "$PID" 2>/dev/null; then break; fi
            sleep 1
        done
        if kill -0 "$PID" 2>/dev/null; then
            echo -e "${YELLOW}  强制终止...${NC}"
            kill -KILL "$PID" 2>/dev/null || true
            sleep 1
        fi
        rm -f "$PID_FILE"
    fi
else
    PID=$(pgrep -f "colink-server" 2>/dev/null || true)
    if [[ -n "$PID" ]]; then
        echo "  Found running process (PID $PID), stopping..."
        kill -TERM $PID 2>/dev/null || true
        sleep 5
    fi
fi
echo -e "${GREEN}服务已停止${NC}"

# Step 4: 备份旧文件
echo -e "${YELLOW}[4/8] Backing up old files...${NC}"
BACKUP_DIR="$INSTALL_DIR/backup"
if [[ -d "$BACKUP_DIR" ]]; then
    rm -rf "$BACKUP_DIR"
fi
mkdir -p "$BACKUP_DIR"

for item in bin configs sql-change web VERSION; do
    if [[ -e "$INSTALL_DIR/$item" ]]; then
        mv "$INSTALL_DIR/$item" "$BACKUP_DIR/"
    fi
done
echo -e "${GREEN}旧文件已备份到 $BACKUP_DIR/${NC}"

# Step 5: 复制新文件
echo -e "${YELLOW}[5/8] Installing new files...${NC}"

for item in bin sql-change web VERSION; do
    if [[ -e "$STAGING/$item" ]]; then
        mv "$STAGING/$item" "$INSTALL_DIR/$item"
    fi
done

if [[ -f "$STAGING/configs/config.yaml.example" ]]; then
    mkdir -p "$INSTALL_DIR/configs"
    cp "$STAGING/configs/config.yaml.example" "$INSTALL_DIR/configs/"
fi

chmod +x "$INSTALL_DIR/bin/"* 2>/dev/null || true

for script in start.sh stop.sh status.sh; do
    if [[ -f "$SCRIPT_DIR/$script" ]]; then
        cp "$SCRIPT_DIR/$script" "$INSTALL_DIR/bin/"
        chmod +x "$INSTALL_DIR/bin/$script"
    fi
done

rm -rf "$STAGING"
echo -e "${GREEN}新文件已安装${NC}"

# Step 6: 合并配置
echo -e "${YELLOW}[6/8] Merging configuration...${NC}"
TEMPLATE="$INSTALL_DIR/configs/config.yaml.example"
USER_CONFIG="$BACKUP_DIR/configs/config.yaml"
TARGET_CONFIG="$INSTALL_DIR/configs/config.yaml"

MERGE_SCRIPT="$SCRIPT_DIR/merge-yaml.py"

if [[ -f "$TEMPLATE" && -f "$USER_CONFIG" && -f "$MERGE_SCRIPT" ]]; then
    $PYTHON "$MERGE_SCRIPT" "$TEMPLATE" "$USER_CONFIG" "$TARGET_CONFIG"
    PY_TARGET=$(py_path "$TARGET_CONFIG")
    $PYTHON -c "
import yaml
with open('$PY_TARGET', 'r', encoding='utf-8') as f:
    c = yaml.safe_load(f)
c.setdefault('deployment', {})['type'] = 'linux'
c.setdefault('logging', {})['dir'] = './logs'
c.setdefault('git_url_conversion', {})['enabled'] = True
with open('$PY_TARGET', 'w', encoding='utf-8') as f:
    yaml.dump(c, f, default_flow_style=False, allow_unicode=True, sort_keys=False)
"
    cp "$TARGET_CONFIG" "$INSTALL_DIR/data/configs/config.yaml"
    echo -e "${GREEN}配置合并完成${NC}"
elif [[ -f "$TEMPLATE" ]]; then
    cp "$TEMPLATE" "$TARGET_CONFIG"
    cp "$TEMPLATE" "$INSTALL_DIR/data/configs/config.yaml"
    echo -e "${GREEN}使用模板配置${NC}"
else
    echo -e "${YELLOW}警告: 未找到配置模板，保留旧配置${NC}"
    if [[ -f "$USER_CONFIG" ]]; then
        mkdir -p "$INSTALL_DIR/configs"
        cp "$USER_CONFIG" "$TARGET_CONFIG"
    fi
fi

# Step 7: 增量数据库迁移
echo -e "${YELLOW}[7/8] Running delta database migrations...${NC}"
MIGRATE="$INSTALL_DIR/bin/migrate"
DB_PATH="$INSTALL_DIR/data/sqlite/colink.db"

if [[ -x "$MIGRATE" && -f "$DB_PATH" ]]; then
    VERSIONS_TO_MIGRATE=()
    for vdir in "$INSTALL_DIR/sql-change"/v*/; do
        [[ -d "$vdir" ]] || continue
        vname=$(basename "$vdir")
        ver="${vname#v}"

        CMP_CUR=$(version_compare "$ver" "$CURRENT_VERSION")
        CMP_NEW=$(version_compare "$ver" "$NEW_VERSION")
        if [[ $CMP_CUR -gt 0 && $CMP_NEW -le 0 ]]; then
            VERSIONS_TO_MIGRATE+=("$ver")
        fi
    done

    if [[ ${#VERSIONS_TO_MIGRATE[@]} -gt 0 ]]; then
        IFS=$'\n' SORTED=($(sort -V <<< "${VERSIONS_TO_MIGRATE[*]}")); unset IFS

        for ver in "${SORTED[@]}"; do
            SQL_DIR="$INSTALL_DIR/sql-change/v${ver}/sqlite"
            if [[ -d "$SQL_DIR" ]]; then
                echo "  Running migration: v$ver"
                "$MIGRATE" up --db "$DB_PATH" --dir "$SQL_DIR" --backup 2>&1 || {
                    echo -e "${YELLOW}  警告: 迁移 v$ver 执行失败${NC}"
                }
            fi
        done
        echo -e "${GREEN}增量迁移完成（${#SORTED[@]} 个版本）${NC}"
    else
        echo -e "${GREEN}无需增量迁移${NC}"
    fi
else
    echo -e "${YELLOW}migrate 工具不存在或数据库不存在，跳过迁移${NC}"
fi

# Step 8: 更新元数据
echo -e "${YELLOW}[8/8] Updating install metadata...${NC}"
INSTALL_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
PY_META=$(py_path "$META_FILE")
ORIG_INSTALL_TIME=$($PYTHON -c "import json; d=json.load(open('$PY_META', encoding='utf-8')); print(d.get('install_time',''))" 2>/dev/null || echo "$INSTALL_TIME")
cat > "$META_FILE" << METAEOF
{
  "install_dir": "$INSTALL_DIR",
  "version": "$NEW_VERSION",
  "install_time": "$ORIG_INSTALL_TIME",
  "upgrade_time": "$INSTALL_TIME",
  "previous_version": "$CURRENT_VERSION",
  "platform": "linux"
}
METAEOF
echo -e "${GREEN}元数据已更新${NC}"

echo ""
echo -e "${CYAN}=== Upgrade Complete ===${NC}"
echo -e "${GREEN}$CURRENT_VERSION → $NEW_VERSION${NC}"
echo -e "备份目录: $BACKUP_DIR"
echo ""
echo "启动服务:"
echo "  cd $INSTALL_DIR && bin/start.sh"
