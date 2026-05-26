#!/bin/bash
# 测试 deploy.sh 首次部署脚本
# 重点关注：目录结构、配置生成、元数据文件、数据库初始化
#
# 用法: ./test-deploy.sh [--package /path/to/tar.gz]

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

echo -e "${YELLOW}=== test-deploy.sh ===${NC}"

# 准备安装包
PACKAGE_PATH="${1:-}"
if [[ -z "$PACKAGE_PATH" ]]; then
    OUTPUT_DIR=$(mktemp -d)
    echo "Building package first..."
    bash "$PROJECT_ROOT/scripts/build-linux.sh" --skip-deps --skip-frontend -o "$OUTPUT_DIR" 2>&1
    PACKAGE_PATH=$(find "$OUTPUT_DIR" -name "Colink-Setup-*-linux-*.tar.gz" -type f | head -1)
    if [[ -z "$PACKAGE_PATH" ]]; then
        echo -e "${RED}构建产物未找到${NC}"; exit 1
    fi
fi

INSTALL_BASE=$(mktemp -d)
INSTALL_DIR="$INSTALL_BASE/colink-test"
trap "rm -rf '$INSTALL_BASE'" EXIT

DEPLOY_SCRIPT="$PROJECT_ROOT/scripts/deploy.sh"
TEST_PORT=28080

# ── Test: 参数校验 ──
echo ""
echo "--- 参数校验 ---"

# 缺少 --dir
if bash "$DEPLOY_SCRIPT" --package "$PACKAGE_PATH" 2>&1; then
    echo -e "  ${RED}FAIL${NC}: 缺少 --dir 应报错"; FAIL=$((FAIL+1))
else
    echo -e "  ${GREEN}PASS${NC}: 缺少 --dir 正确报错"; PASS=$((PASS+1))
fi

# 缺少 --package
if bash "$DEPLOY_SCRIPT" --dir /tmp/nonexist 2>&1; then
    echo -e "  ${RED}FAIL${NC}: 缺少 --package 应报错"; FAIL=$((FAIL+1))
else
    echo -e "  ${GREEN}PASS${NC}: 缺少 --package 正确报错"; PASS=$((PASS+1))
fi

# ── Test: 执行部署 ──
echo ""
echo "--- 执行部署 ---"
if bash "$DEPLOY_SCRIPT" --dir "$INSTALL_DIR" --port "$TEST_PORT" --package "$PACKAGE_PATH" 2>&1; then
    echo -e "  ${GREEN}PASS${NC}: deploy.sh 执行成功"; PASS=$((PASS+1))
else
    echo -e "  ${RED}FAIL${NC}: deploy.sh 执行失败"; FAIL=$((FAIL+1))
fi

# ── Test: 目录结构 ──
echo ""
echo "--- 目录结构 ---"
for d in bin configs data logs sql-change web; do
    assert_ok "目录 $d/ 存在" test -d "$INSTALL_DIR/$d"
done
for d in sqlite agent-assets agent-configs configs repos temp; do
    assert_ok "data/$d/ 存在" test -d "$INSTALL_DIR/data/$d"
done

# ── Test: 二进制文件 ──
echo ""
echo "--- 二进制文件 ---"
assert_ok "colink-server 存在" test -f "$INSTALL_DIR/bin/colink-server"
assert_ok "migrate 存在" test -f "$INSTALL_DIR/bin/migrate"
if [[ "$(uname -s)" == "Linux" ]]; then
    assert_ok "colink-server 可执行" test -x "$INSTALL_DIR/bin/colink-server"
    assert_ok "migrate 可执行" test -x "$INSTALL_DIR/bin/migrate"
fi

# ── Test: 配置文件生成 ──
echo ""
echo "--- 配置文件 ---"
CONFIG_FILE="$INSTALL_DIR/configs/config.yaml"
assert_ok "config.yaml 存在" test -f "$CONFIG_FILE"

if [[ -f "$CONFIG_FILE" ]]; then
    # 端口
    CONFIG_PORT=$(grep '^\s*port:' "$CONFIG_FILE" | head -1 | awk '{print $2}' | tr -d '"' | tr -d "'")
    assert_eq "server.port 为 $TEST_PORT" "$CONFIG_PORT" "$TEST_PORT"

    # deployment.type
    assert_ok "deployment.type 为 linux" grep -q 'type: linux' "$CONFIG_FILE"

    # logging.dir
    assert_ok "logging.dir 为 ./logs" grep -q 'dir: ./logs' "$CONFIG_FILE"

    # logging.level
    assert_ok "logging.level 存在" grep -q 'level:' "$CONFIG_FILE"

    # data.base_path
    assert_ok "data.base_path 存在" grep -q 'base_path:' "$CONFIG_FILE"

    # database.type
    assert_ok "database.type 存在" grep -q 'type: sqlite' "$CONFIG_FILE"
fi

# data/configs/ 也应有配置
assert_ok "data/configs/config.yaml 存在" test -f "$INSTALL_DIR/data/configs/config.yaml"

# ── Test: 元数据 .install-meta ──
echo ""
echo "--- 元数据 .install-meta ---"
META_FILE="$INSTALL_DIR/.install-meta"
assert_ok ".install-meta 存在" test -f "$META_FILE"

if [[ -f "$META_FILE" ]]; then
    PY_META=$(py_path "$META_FILE")
    # 验证 JSON 可解析且包含必需字段
    META_VALID=$(python -c "
import json, sys
with open('$PY_META', encoding='utf-8') as f:
    d = json.load(f)
checks = [
    ('install_dir' in d, 'install_dir'),
    ('version' in d, 'version'),
    ('install_time' in d, 'install_time'),
    ('platform' in d, 'platform'),
    (d.get('platform') == 'linux', 'platform==linux'),
]
for ok, name in checks:
    print(f'{name}={ok}')
" 2>/dev/null || echo "parse_failed")

    if [[ "$META_VALID" != "parse_failed" ]]; then
        assert_ok "元数据包含 install_dir" bash -c "echo '$META_VALID' | grep -q 'install_dir=True'"
        assert_ok "元数据包含 version" bash -c "echo '$META_VALID' | grep -q 'version=True'"
        assert_ok "元数据包含 install_time" bash -c "echo '$META_VALID' | grep -q 'install_time=True'"
        assert_ok "元数据包含 platform" bash -c "echo '$META_VALID' | grep -q 'platform=True'"
        assert_ok "元数据 platform=linux" bash -c "echo '$META_VALID' | grep -q 'platform==linux=True'"

        # 验证 version 与 VERSION 文件一致
        META_VERSION=$(python -c "import json; print(json.load(open('$PY_META', encoding='utf-8'))['version'])" 2>/dev/null || echo "")
        SRC_VERSION=$(tr -d '\n\r' < "$INSTALL_DIR/VERSION" 2>/dev/null || echo "")
        assert_eq "元数据 version 与 VERSION 文件一致" "$META_VERSION" "$SRC_VERSION"

        # 验证 install_dir 指向安装路径
        META_DIR=$(python -c "import json; print(json.load(open('$PY_META', encoding='utf-8'))['install_dir'])" 2>/dev/null || echo "")
        assert_ok "元数据 install_dir 指向安装目录" test "$META_DIR" = "$INSTALL_DIR"

        # 验证 install_time 为 ISO 8601 格式
        META_TIME=$(python -c "import json; print(json.load(open('$PY_META', encoding='utf-8'))['install_time'])" 2>/dev/null || echo "")
        if [[ "$META_TIME" == *"T"* && "$META_TIME" == *"Z"* ]]; then
            echo -e "  ${GREEN}PASS${NC}: install_time 为 ISO 8601 格式"; PASS=$((PASS+1))
        else
            echo -e "  ${RED}FAIL${NC}: install_time 为 ISO 8601 格式 (actual=$META_TIME)"; FAIL=$((FAIL+1))
        fi
    else
        echo -e "  ${RED}FAIL${NC}: .install-meta JSON 解析失败"; FAIL=$((FAIL+1))
    fi
fi

# ── Test: VERSION 文件 ──
echo ""
echo "--- VERSION 文件 ---"
assert_ok "VERSION 文件存在" test -f "$INSTALL_DIR/VERSION"

# ── Test: 数据库初始化 ──
echo ""
echo "--- 数据库 ---"
if [[ "$(uname -s)" == "Linux" ]]; then
    assert_ok "colink.db 存在" test -f "$INSTALL_DIR/data/sqlite/colink.db"
else
    echo -e "  ${YELLOW}SKIP${NC}: colink.db 检查（非 Linux 环境，migrate 为 Linux 二进制）"
fi

# ── Test: 拒绝重复部署 ──
echo ""
echo "--- 重复部署 ---"
if bash "$DEPLOY_SCRIPT" --dir "$INSTALL_DIR" --port "$TEST_PORT" --package "$PACKAGE_PATH" 2>&1; then
    echo -e "  ${RED}FAIL${NC}: 拒绝已存在目录"; FAIL=$((FAIL+1))
else
    echo -e "  ${GREEN}PASS${NC}: 拒绝已存在目录"; PASS=$((PASS+1))
fi

echo ""
echo -e "${YELLOW}Summary: PASS=$PASS FAIL=$FAIL${NC}"
if [[ $FAIL -gt 0 ]]; then exit 1; fi
