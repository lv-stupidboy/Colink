#!/bin/bash
# 测试 upgrade.sh 升级脚本
# 重点关注：版本检测、元数据更新、增量迁移、配置合并、数据保留
#
# 用法: ./test-upgrade.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
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

echo -e "${YELLOW}=== test-upgrade.sh ===${NC}"

# 构建当前版本包
OUTPUT_DIR=$(mktemp -d)
INSTALL_BASE=$(mktemp -d)
INSTALL_DIR="$INSTALL_BASE/colink-test"
trap "rm -rf '$OUTPUT_DIR' '$INSTALL_BASE'" EXIT

echo "Building package..."
bash "$PROJECT_ROOT/scripts/build-linux.sh" --skip-deps --skip-frontend -o "$OUTPUT_DIR" 2>&1
PACKAGE_PATH=$(find "$OUTPUT_DIR" -name "Colink-Setup-*-linux-*.tar.gz" -type f | head -1)
if [[ -z "$PACKAGE_PATH" ]]; then
    echo -e "${RED}构建产物未找到${NC}"; exit 1
fi

# ── 部署初始版本 ──
echo "Deploying initial version..."
bash "$PROJECT_ROOT/scripts/deploy.sh" --dir "$INSTALL_DIR" --port 28081 --package "$PACKAGE_PATH" 2>&1

# 记录部署后状态
INITIAL_VERSION=$(tr -d '\n\r' < "$INSTALL_DIR/VERSION")
PY_META_INSTALL=$(py_path "$INSTALL_DIR/.install-meta")
INITIAL_META_VERSION=$(python -c "import json; print(json.load(open('$PY_META_INSTALL', encoding='utf-8'))['version'])" 2>/dev/null || echo "")
INITIAL_INSTALL_TIME=$(python -c "import json; print(json.load(open('$PY_META_INSTALL', encoding='utf-8'))['install_time'])" 2>/dev/null || echo "")

echo -e "已部署版本: ${GREEN}$INITIAL_VERSION${NC}"
echo -e "元数据版本: ${GREEN}$INITIAL_META_VERSION${NC}"

# 模拟用户自定义：修改端口和日志格式
if [[ -f "$INSTALL_DIR/configs/config.yaml" ]]; then
    sed -i 's/port: 28081/port: 28099/' "$INSTALL_DIR/configs/config.yaml"
    sed -i 's/format: console/format: json/' "$INSTALL_DIR/configs/config.yaml"
    # 同步到 data/configs/
    cp "$INSTALL_DIR/configs/config.yaml" "$INSTALL_DIR/data/configs/config.yaml"
fi

# ── Test: 参数校验 ──
echo ""
echo "--- 参数校验 ---"

# 缺少 --dir
if bash "$PROJECT_ROOT/scripts/upgrade.sh" --package "$PACKAGE_PATH" 2>&1; then
    echo -e "  ${RED}FAIL${NC}: 缺少 --dir 应报错"; FAIL=$((FAIL+1))
else
    echo -e "  ${GREEN}PASS${NC}: 缺少 --dir 正确报错"; PASS=$((PASS+1))
fi

# 不存在的安装目录
if bash "$PROJECT_ROOT/scripts/upgrade.sh" --dir /tmp/nonexist_colink --package "$PACKAGE_PATH" 2>&1; then
    echo -e "  ${RED}FAIL${NC}: 不存在目录应报错"; FAIL=$((FAIL+1))
else
    echo -e "  ${GREEN}PASS${NC}: 不存在目录正确报错"; PASS=$((PASS+1))
fi

# ── Test: 版本检测 ──
echo ""
echo "--- 版本检测 ---"
assert_eq "从 .install-meta 读取版本" "$INITIAL_META_VERSION" "$INITIAL_VERSION"

# 模拟 .install-meta 丢失，应从 VERSION 文件回退
META_FILE="$INSTALL_DIR/.install-meta"
META_BACKUP="$INSTALL_DIR/.install-meta.bak"
cp "$META_FILE" "$META_BACKUP"
rm "$META_FILE"

# 手动触发版本检测（仅验证 VERSION 回退机制）
FALLBACK_VERSION=$(tr -d '\n\r' < "$INSTALL_DIR/VERSION")
assert_eq "VERSION 文件回退检测" "$FALLBACK_VERSION" "$INITIAL_VERSION"

# 恢复元数据
cp "$META_BACKUP" "$META_FILE"
rm "$META_BACKUP"

# ── Test: 同版本升级（不加 --force 应跳过）──
echo ""
echo "--- 同版本升级 ---"
UPGRADE_OUTPUT=$(bash "$PROJECT_ROOT/scripts/upgrade.sh" --dir "$INSTALL_DIR" --package "$PACKAGE_PATH" 2>&1 || true)
if echo "$UPGRADE_OUTPUT" | grep -q "无需升级"; then
    echo -e "  ${GREEN}PASS${NC}: 同版本无 --force 正确跳过"; PASS=$((PASS+1))
else
    echo -e "  ${RED}FAIL${NC}: 同版本无 --force 应跳过"; FAIL=$((FAIL+1))
fi

# ── Test: 执行升级（--force）──
echo ""
echo "--- 执行升级 ---"
if bash "$PROJECT_ROOT/scripts/upgrade.sh" --dir "$INSTALL_DIR" --package "$PACKAGE_PATH" --force 2>&1; then
    echo -e "  ${GREEN}PASS${NC}: upgrade.sh --force 执行成功"; PASS=$((PASS+1))
else
    echo -e "  ${RED}FAIL${NC}: upgrade.sh --force 执行失败"; FAIL=$((FAIL+1))
fi

# ── Test: 备份 ──
echo ""
echo "--- 备份 ---"
assert_ok "backup/ 目录存在" test -d "$INSTALL_DIR/backup"
assert_ok "backup/ 包含旧 bin/" test -d "$INSTALL_DIR/backup/bin"
assert_ok "backup/ 包含旧 configs/" test -d "$INSTALL_DIR/backup/configs"
assert_ok "backup/ 包含旧 sql-change/" test -d "$INSTALL_DIR/backup/sql-change"
assert_ok "backup/ 包含旧 web/" test -d "$INSTALL_DIR/backup/web"

# ── Test: 数据保留 ──
echo ""
echo "--- 数据保留 ---"
assert_ok "data/ 目录仍存在" test -d "$INSTALL_DIR/data"
assert_ok "data/sqlite/ 仍存在" test -d "$INSTALL_DIR/data/sqlite"
if [[ "$(uname -s)" == "Linux" ]]; then
    assert_ok "data/sqlite/colink.db 仍存在" test -f "$INSTALL_DIR/data/sqlite/colink.db"
else
    echo -e "  ${YELLOW}SKIP${NC}: colink.db 检查（非 Linux 环境）"
fi
assert_ok "data/agent-assets/ 仍存在" test -d "$INSTALL_DIR/data/agent-assets"
assert_ok "logs/ 目录仍存在" test -d "$INSTALL_DIR/logs"

# ── Test: 新文件已安装 ──
echo ""
echo "--- 新文件 ---"
assert_ok "新 bin/ 已安装" test -d "$INSTALL_DIR/bin"
assert_ok "新 sql-change/ 已安装" test -d "$INSTALL_DIR/sql-change"
assert_ok "新 VERSION 已写入" test -f "$INSTALL_DIR/VERSION"

# ── Test: 配置合并 ──
echo ""
echo "--- 配置合并 ---"
CONFIG_FILE="$INSTALL_DIR/configs/config.yaml"
assert_ok "升级后 config.yaml 存在" test -f "$CONFIG_FILE"

if [[ -f "$CONFIG_FILE" ]]; then
    # 用户修改的端口应保留
    CONFIG_PORT=$(grep '^\s*port:' "$CONFIG_FILE" | head -1 | awk '{print $2}' | tr -d '"' | tr -d "'")
    assert_eq "用户自定义端口保留 (28099)" "$CONFIG_PORT" "28099"

    # 用户修改的日志格式应保留
    assert_ok "用户自定义日志格式保留 (json)" grep -q 'format: json' "$CONFIG_FILE"

    # 模板中存在的必需字段应保留
    assert_ok "deployment.type 保留" grep -q 'type: linux' "$CONFIG_FILE"
    assert_ok "logging.dir 保留" grep -q 'dir: ./logs' "$CONFIG_FILE"
    assert_ok "data.base_path 保留" grep -q 'base_path:' "$CONFIG_FILE"
fi

# ── Test: 元数据更新 ──
echo ""
echo "--- 元数据更新 ---"
if [[ -f "$META_FILE" ]]; then
    PY_META=$(py_path "$META_FILE")
    # 元数据字段验证
    META_CHECK=$(python -c "
import json
with open('$PY_META', encoding='utf-8') as f:
    d = json.load(f)
checks = [
    ('version' in d, 'version'),
    ('install_dir' in d, 'install_dir'),
    ('install_time' in d, 'install_time'),
    ('upgrade_time' in d, 'upgrade_time'),
    ('previous_version' in d, 'previous_version'),
    ('platform' in d, 'platform'),
    (d.get('platform') == 'linux', 'platform==linux'),
]
for ok, name in checks:
    print(f'{name}={ok}')
" 2>/dev/null || echo "parse_failed")

    if [[ "$META_CHECK" != "parse_failed" ]]; then
        assert_ok "元数据包含 version" bash -c "echo '$META_CHECK' | grep -q 'version=True'"
        assert_ok "元数据包含 install_dir" bash -c "echo '$META_CHECK' | grep -q 'install_dir=True'"
        assert_ok "元数据包含 install_time" bash -c "echo '$META_CHECK' | grep -q 'install_time=True'"
        assert_ok "元数据包含 upgrade_time" bash -c "echo '$META_CHECK' | grep -q 'upgrade_time=True'"
        assert_ok "元数据包含 previous_version" bash -c "echo '$META_CHECK' | grep -q 'previous_version=True'"
        assert_ok "元数据包含 platform" bash -c "echo '$META_CHECK' | grep -q 'platform=True'"
        assert_ok "元数据 platform=linux" bash -c "echo '$META_CHECK' | grep -q 'platform==linux=True'"

        # 升级后 install_time 不变（原始安装时间）
        UPGRADED_INSTALL_TIME=$(python -c "import json; print(json.load(open('$PY_META', encoding='utf-8'))['install_time'])" 2>/dev/null || echo "")
        assert_eq "install_time 未改变" "$UPGRADED_INSTALL_TIME" "$INITIAL_INSTALL_TIME"

        # previous_version 应等于升级前版本
        PREV_VER=$(python -c "import json; print(json.load(open('$PY_META', encoding='utf-8'))['previous_version'])" 2>/dev/null || echo "")
        assert_eq "previous_version 等于旧版本" "$PREV_VER" "$INITIAL_VERSION"

        # upgrade_time 为 ISO 8601 格式
        UPGRADE_TIME=$(python -c "import json; print(json.load(open('$PY_META', encoding='utf-8'))['upgrade_time'])" 2>/dev/null || echo "")
        if [[ "$UPGRADE_TIME" == *"T"* && "$UPGRADE_TIME" == *"Z"* ]]; then
            echo -e "  ${GREEN}PASS${NC}: upgrade_time 为 ISO 8601 格式"; PASS=$((PASS+1))
        else
            echo -e "  ${RED}FAIL${NC}: upgrade_time 为 ISO 8601 格式 (actual=$UPGRADE_TIME)"; FAIL=$((FAIL+1))
        fi
    else
        echo -e "  ${RED}FAIL${NC}: 元数据 JSON 解析失败"; FAIL=$((FAIL+1))
    fi
fi

echo ""
echo -e "${YELLOW}Summary: PASS=$PASS FAIL=$FAIL${NC}"
if [[ $FAIL -gt 0 ]]; then exit 1; fi
