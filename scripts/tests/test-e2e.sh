#!/bin/bash
# 端到端测试：部署 → 启动 → 验证 → 停止 → 升级 → 启动 → 验证
# 重点关注完整的元数据生命周期：install → upgrade → verify
#
# 注意: 进程启停测试仅在 Linux 上有效
# 用法: ./test-e2e.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# 路径转换：MSYS/Git Bash 下 Python 需要原生路径
py_path() {
    if command -v cygpath &>/dev/null; then
        cygpath -m "$1"
    else
        echo "$1"
    fi
}

PASS=0
FAIL=0

assert_ok() {
    local desc="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        echo -e "  ${GREEN}PASS${NC}: $desc"
        PASS=$((PASS+1))
    else
        echo -e "  ${RED}FAIL${NC}: $desc"
        FAIL=$((FAIL+1))
    fi
}

assert_eq() {
    local desc="$1" actual="$2" expected="$3"
    if [[ "$actual" == "$expected" ]]; then
        echo -e "  ${GREEN}PASS${NC}: $desc"
        PASS=$((PASS+1))
    else
        echo -e "  ${RED}FAIL${NC}: $desc (expected='$expected', actual='$actual')"
        FAIL=$((FAIL+1))
    fi
}

echo -e "${CYAN}=== test-e2e.sh (End-to-End) ===${NC}"

OUTPUT_DIR=$(mktemp -d)
INSTALL_BASE=$(mktemp -d)
INSTALL_DIR="$INSTALL_BASE/colink-test"
trap "rm -rf '$OUTPUT_DIR' '$INSTALL_BASE'" EXIT

# ── Step 1: 构建 ──
echo -e "${YELLOW}[1/7] Building package...${NC}"
bash "$PROJECT_ROOT/scripts/build-linux.sh" --skip-deps --skip-frontend -o "$OUTPUT_DIR" 2>&1
PACKAGE_PATH=$(find "$OUTPUT_DIR" -name "Colink-Setup-*-linux-*.tar.gz" -type f | head -1)
if [[ -z "$PACKAGE_PATH" ]]; then
    echo -e "${RED}构建产物未找到${NC}"; exit 1
fi
echo -e "${GREEN}Package: $(basename $PACKAGE_PATH)${NC}"

# ── Step 2: 部署 ──
echo -e "${YELLOW}[2/7] Deploying...${NC}"
bash "$PROJECT_ROOT/scripts/deploy.sh" --dir "$INSTALL_DIR" --port 28083 --package "$PACKAGE_PATH" 2>&1

# ── Step 3: 验证部署 ──
echo -e "${YELLOW}[3/7] Verifying deployment...${NC}"
assert_ok "config.yaml 存在" test -f "$INSTALL_DIR/configs/config.yaml"
assert_ok ".install-meta 存在" test -f "$INSTALL_DIR/.install-meta"
assert_ok "VERSION 文件存在" test -f "$INSTALL_DIR/VERSION"
if [[ "$(uname -s)" == "Linux" ]]; then
    assert_ok "data/sqlite/colink.db 存在" test -f "$INSTALL_DIR/data/sqlite/colink.db"
else
    echo -e "  ${YELLOW}SKIP${NC}: colink.db 检查（非 Linux 环境）"
fi
assert_ok "logs/ 目录存在" test -d "$INSTALL_DIR/logs"

DEPLOYED_VERSION=$(tr -d '\n\r' < "$INSTALL_DIR/VERSION")
echo -e "  已部署版本: ${GREEN}$DEPLOYED_VERSION${NC}"

# 验证元数据初始状态
META_FILE="$INSTALL_DIR/.install-meta"
PY_META=$(py_path "$META_FILE")
INITIAL_INSTALL_TIME=$(python -c "import json; print(json.load(open('$PY_META', encoding='utf-8'))['install_time'])" 2>/dev/null || echo "")
assert_ok "元数据有 install_time" test -n "$INITIAL_INSTALL_TIME"

# 不应有 upgrade_time（首次安装）
HAS_UPGRADE_TIME=$(python -c "import json; print('upgrade_time' in json.load(open('$PY_META', encoding='utf-8')))" 2>/dev/null || echo "False")
assert_eq "首次安装无 upgrade_time" "$HAS_UPGRADE_TIME" "False"

# ── Step 4-5: 启动和停止（仅 Linux）──
IS_LINUX=false
[[ "$(uname -s)" == "Linux" ]] && IS_LINUX=true

if $IS_LINUX; then
    echo -e "${YELLOW}[4/7] Starting server...${NC}"
    bash "$INSTALL_DIR/bin/start.sh" 2>&1 || bash "$PROJECT_ROOT/scripts/start.sh" 2>&1
    sleep 3

    PORT=$(grep 'port:' "$INSTALL_DIR/configs/config.yaml" | head -1 | awk '{print $2}')
    if curl -sf "http://localhost:${PORT:-28083}/api/v1/system/version" > /dev/null 2>&1; then
        echo -e "  ${GREEN}PASS${NC}: 服务启动后健康检查通过"; PASS=$((PASS+1))
    else
        echo -e "  ${RED}FAIL${NC}: 服务启动后健康检查未通过"; FAIL=$((FAIL+1))
    fi

    echo -e "${YELLOW}[5/7] Stopping server...${NC}"
    bash "$INSTALL_DIR/bin/stop.sh" 2>&1 || bash "$PROJECT_ROOT/scripts/stop.sh" 2>&1
    sleep 2
    assert_ok "服务已停止" bash -c '! pgrep -f colink-server >/dev/null 2>&1'
else
    echo -e "${YELLOW}[4-5/7] 跳过进程启停测试（非 Linux 环境）${NC}"
fi

# ── Step 6: 升级 ──
echo -e "${YELLOW}[6/7] Upgrading...${NC}"

# 模拟用户自定义配置
if [[ -f "$INSTALL_DIR/configs/config.yaml" ]]; then
    sed -i 's/format: console/format: json/' "$INSTALL_DIR/configs/config.yaml"
    cp "$INSTALL_DIR/configs/config.yaml" "$INSTALL_DIR/data/configs/config.yaml"
fi

bash "$PROJECT_ROOT/scripts/upgrade.sh" --dir "$INSTALL_DIR" --package "$PACKAGE_PATH" --force 2>&1

# ── Step 7: 验证升级 ──
echo -e "${YELLOW}[7/7] Verifying upgrade...${NC}"

# 基础验证
assert_ok "升级后 config.yaml 存在" test -f "$INSTALL_DIR/configs/config.yaml"
assert_ok "升级后 .install-meta 存在" test -f "$INSTALL_DIR/.install-meta"
assert_ok "升级后 backup/ 存在" test -d "$INSTALL_DIR/backup"
assert_ok "data/ 目录完整" test -d "$INSTALL_DIR/data/sqlite"

# 配置合并验证
assert_ok "配置合并: 用户自定义值保留 (format: json)" grep -q 'format: json' "$INSTALL_DIR/configs/config.yaml"
assert_ok "配置合并: logging.dir 保留" grep -q 'dir: ./logs' "$INSTALL_DIR/configs/config.yaml"
assert_ok "配置合并: deployment.type 保留" grep -q 'type: linux' "$INSTALL_DIR/configs/config.yaml"

# 元数据生命周期验证
if [[ -f "$META_FILE" ]]; then
    PY_META=$(py_path "$META_FILE")
    # install_time 不应改变
    UPGRADED_INSTALL_TIME=$(python -c "import json; print(json.load(open('$PY_META', encoding='utf-8'))['install_time'])" 2>/dev/null || echo "")
    assert_eq "install_time 未改变（原始安装时间保留）" "$UPGRADED_INSTALL_TIME" "$INITIAL_INSTALL_TIME"

    # upgrade_time 应存在
    UPGRADE_TIME=$(python -c "import json; print(json.load(open('$PY_META', encoding='utf-8')).get('upgrade_time',''))" 2>/dev/null || echo "")
    assert_ok "upgrade_time 已写入" test -n "$UPGRADE_TIME"

    # previous_version 应等于升级前版本
    PREV_VER=$(python -c "import json; print(json.load(open('$PY_META', encoding='utf-8')).get('previous_version',''))" 2>/dev/null || echo "")
    assert_eq "previous_version 等于旧版本" "$PREV_VER" "$DEPLOYED_VERSION"

    # version 应等于当前 VERSION
    UPGRADED_VERSION=$(python -c "import json; print(json.load(open('$PY_META', encoding='utf-8'))['version'])" 2>/dev/null || echo "")
    CURRENT_VERSION=$(tr -d '\n\r' < "$INSTALL_DIR/VERSION" 2>/dev/null || echo "")
    assert_eq "version 与 VERSION 文件一致" "$UPGRADED_VERSION" "$CURRENT_VERSION"
fi

# 升级后再启动验证（仅 Linux）
if $IS_LINUX; then
    echo "Starting server after upgrade..."
    bash "$INSTALL_DIR/bin/start.sh" 2>&1 || bash "$PROJECT_ROOT/scripts/start.sh" 2>&1
    sleep 3

    PORT=$(grep 'port:' "$INSTALL_DIR/configs/config.yaml" | head -1 | awk '{print $2}')
    if curl -sf "http://localhost:${PORT:-28083}/api/v1/system/version" > /dev/null 2>&1; then
        echo -e "  ${GREEN}PASS${NC}: 升级后服务启动正常"; PASS=$((PASS+1))
    else
        echo -e "  ${RED}FAIL${NC}: 升级后服务启动失败"; FAIL=$((FAIL+1))
    fi

    bash "$INSTALL_DIR/bin/stop.sh" 2>&1 || bash "$PROJECT_ROOT/scripts/stop.sh" 2>&1
fi

echo ""
echo -e "${CYAN}=== E2E Test Complete ===${NC}"
echo -e "${YELLOW}Summary: PASS=$PASS FAIL=$FAIL${NC}"
if [[ $FAIL -gt 0 ]]; then exit 1; fi
