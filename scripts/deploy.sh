#!/bin/bash
# Colink Linux 部署脚本
# 首次安装：创建目录结构、复制文件、初始化数据库、写入元数据
#
# 用法: ./deploy.sh INSTALL_DIR

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

# 解析参数
INSTALL_DIR=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            echo "Colink Linux 部署脚本"
            echo ""
            echo "用法: ./deploy.sh INSTALL_DIR"
            echo ""
            echo "参数:"
            echo "  INSTALL_DIR  安装目录（必填）"
            echo ""
            echo "端口由配置文件模板 config.yaml.example 中的 server.port 指定"
            echo "安装包自动从项目 output/ 目录查找"
            echo "  -h, --help   显示帮助"
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
if [[ -d "$INSTALL_DIR" ]]; then
    echo -e "${RED}错误: 安装目录已存在: $INSTALL_DIR${NC}"
    echo -e "${YELLOW}如需升级，请使用 upgrade.sh${NC}"
    exit 1
fi

# 自动查找安装包
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PACKAGE_PATH=$(find "$PROJECT_ROOT/output" -name "Colink-Setup-*-linux-*.tar.gz" -type f 2>/dev/null | sort -V | tail -1)

if [[ -z "$PACKAGE_PATH" ]]; then
    echo -e "${RED}错误: 未找到安装包，请先运行 bash scripts/build-linux.sh${NC}"; exit 1
fi

echo -e "${CYAN}=== Colink Linux Deploy ===${NC}"
echo -e "Install dir: ${GREEN}$INSTALL_DIR${NC}"
echo -e "Package:     ${GREEN}$PACKAGE_PATH${NC}"

# Step 1: 磁盘空间检查
AVAILABLE_KB=$(df -k "$(dirname "$INSTALL_DIR")" 2>/dev/null | awk 'NR==2{print $4}' || echo "999999")
AVAILABLE_MB=$((AVAILABLE_KB / 1024))
if [[ $AVAILABLE_MB -lt 500 ]]; then
    echo -e "${RED}错误: 磁盘空间不足（需要 >=500MB，可用 ${AVAILABLE_MB}MB）${NC}"; exit 1
fi
echo -e "${GREEN}磁盘空间: ${AVAILABLE_MB}MB 可用${NC}"

# Step 2: 创建目录结构
echo -e "${YELLOW}[1/6] Creating directory structure...${NC}"
mkdir -p "$INSTALL_DIR"/{bin,configs,logs,sql-change,web}
mkdir -p "$INSTALL_DIR"/data/{sqlite,agent-assets,agent-configs,configs,repos,temp}
echo -e "${GREEN}目录结构创建完成${NC}"

# Step 3: 解压并复制文件
echo -e "${YELLOW}[2/6] Extracting package...${NC}"
STAGING="$INSTALL_DIR/.staging-$$"
mkdir -p "$STAGING"
tar -xzf "$PACKAGE_PATH" -C "$STAGING"

# 复制文件
if [[ -f "$STAGING/bin/colink-server" ]]; then
    cp "$STAGING/bin/colink-server" "$INSTALL_DIR/bin/"
    chmod +x "$INSTALL_DIR/bin/colink-server"
fi
if [[ -f "$STAGING/bin/migrate" ]]; then
    cp "$STAGING/bin/migrate" "$INSTALL_DIR/bin/"
    chmod +x "$INSTALL_DIR/bin/migrate"
fi
if [[ -d "$STAGING/web" ]]; then
    cp -r "$STAGING/web/"* "$INSTALL_DIR/web/"
fi
if [[ -d "$STAGING/sql-change" ]]; then
    cp -r "$STAGING/sql-change/"* "$INSTALL_DIR/sql-change/"
fi
if [[ -f "$STAGING/configs/config.yaml.example" ]]; then
    cp "$STAGING/configs/config.yaml.example" "$INSTALL_DIR/configs/"
fi
if [[ -f "$STAGING/VERSION" ]]; then
    cp "$STAGING/VERSION" "$INSTALL_DIR/"
fi

# 复制管理脚本
for script in start.sh stop.sh status.sh; do
    if [[ -f "$SCRIPT_DIR/$script" ]]; then
        cp "$SCRIPT_DIR/$script" "$INSTALL_DIR/bin/"
        chmod +x "$INSTALL_DIR/bin/$script"
    fi
done

rm -rf "$STAGING"
echo -e "${GREEN}文件复制完成${NC}"

# Step 4: 生成配置文件（端口从模板读取，仅覆盖 deployment.type 和 logging）
echo -e "${YELLOW}[3/6] Generating config.yaml...${NC}"
TEMPLATE="$INSTALL_DIR/configs/config.yaml.example"
CONFIG_FILE="$INSTALL_DIR/configs/config.yaml"

if [[ -f "$TEMPLATE" ]]; then
    PY_TEMPLATE=$(py_path "$TEMPLATE")
    PY_CONFIG=$(py_path "$CONFIG_FILE")
    $PYTHON -c "
import yaml
with open('$PY_TEMPLATE', 'r', encoding='utf-8') as f:
    t = yaml.safe_load(f) or {}
t.setdefault('deployment', {})['type'] = 'linux'
t.setdefault('logging', {})['dir'] = './logs'
t.setdefault('logging', {})['level'] = 'info'
t.setdefault('logging', {})['format'] = 'console'
t.setdefault('git_url_conversion', {})['enabled'] = True
with open('$PY_CONFIG', 'w', encoding='utf-8') as f:
    yaml.dump(t, f, default_flow_style=False, allow_unicode=True, sort_keys=False)
"
else
    # 无模板，生成最小配置
    cat > "$CONFIG_FILE" << EOF
server:
  port: 26305
  mode: release

data:
  base_path: ./data

database:
  type: sqlite
  path: ./data/sqlite/colink.db

deployment:
  type: linux

logging:
  level: info
  format: console
  dir: ./logs
EOF
fi
echo -e "${GREEN}配置文件生成完成${NC}"

# Step 5: 运行数据库迁移
echo -e "${YELLOW}[4/6] Running database migrations...${NC}"
MIGRATE="$INSTALL_DIR/bin/migrate"
DB_PATH="$INSTALL_DIR/data/sqlite/colink.db"

if [[ -x "$MIGRATE" ]]; then
    # 收集所有版本目录，按版本号排序
    VERSIONS=()
    for vdir in "$INSTALL_DIR/sql-change"/v*/; do
        [[ -d "$vdir" ]] || continue
        vname=$(basename "$vdir")
        ver="${vname#v}"
        VERSIONS+=("$ver")
    done

    # 版本排序
    IFS=$'\n' SORTED=($(sort -V <<< "${VERSIONS[*]}")); unset IFS

    for ver in "${SORTED[@]}"; do
        SQL_DIR="$INSTALL_DIR/sql-change/v${ver}/sqlite"
        if [[ -d "$SQL_DIR" ]]; then
            echo "  Running migration: v$ver"
            "$MIGRATE" up --db "$DB_PATH" --dir "$SQL_DIR" 2>&1 || {
                echo -e "${YELLOW}  警告: 迁移 v$ver 执行失败（可能已存在）${NC}"
            }
        fi
    done
    echo -e "${GREEN}数据库迁移完成${NC}"
else
    echo -e "${YELLOW}migrate 工具不存在，跳过数据库迁移${NC}"
fi

# Step 6: 写入元数据
echo -e "${YELLOW}[5/6] Writing install metadata...${NC}"
VERSION=$(tr -d '\n\r' < "$INSTALL_DIR/VERSION" 2>/dev/null || echo "unknown")
INSTALL_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

cat > "$INSTALL_DIR/.install-meta" << METAEOF
{
  "install_dir": "$INSTALL_DIR",
  "version": "$VERSION",
  "install_time": "$INSTALL_TIME",
  "platform": "linux"
}
METAEOF
echo -e "${GREEN}元数据写入完成${NC}"

# Step 7: 复制日志配置到 data/configs/
cp "$CONFIG_FILE" "$INSTALL_DIR/data/configs/config.yaml"

echo -e "${YELLOW}[6/6] Complete!${NC}"
echo ""
echo -e "${CYAN}=== Deploy Complete ===${NC}"
echo -e "${GREEN}安装目录: $INSTALL_DIR${NC}"
echo -e "${GREEN}版本: $VERSION${NC}"
echo ""
echo "启动服务:"
echo "  cd $INSTALL_DIR && bin/start.sh"
echo ""
echo "停止服务:"
echo "  cd $INSTALL_DIR && bin/stop.sh"
