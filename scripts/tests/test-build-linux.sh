#!/bin/bash
# 测试 build-linux.sh 编译打包脚本
#
# 用法: ./test-build-linux.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

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

assert_contains() {
    local desc="$1" haystack="$2" needle="$3"
    if [[ "$haystack" == *"$needle"* ]]; then
        echo -e "  ${GREEN}PASS${NC}: $desc"
        PASS=$((PASS+1))
    else
        echo -e "  ${RED}FAIL${NC}: $desc (expected to contain '$needle')"
        FAIL=$((FAIL+1))
    fi
}

echo -e "${YELLOW}=== test-build-linux.sh ===${NC}"

BUILD_SCRIPT="$PROJECT_ROOT/scripts/build-linux.sh"
if [[ ! -f "$BUILD_SCRIPT" ]]; then
    echo -e "${RED}build-linux.sh 不存在${NC}"; exit 1
fi

OUTPUT_DIR=$(mktemp -d)
trap "rm -rf '$OUTPUT_DIR'" EXIT

# ── Test 1: 执行构建 ──
echo "Running build-linux.sh --skip-deps --skip-frontend..."
if bash "$BUILD_SCRIPT" --skip-deps --skip-frontend -o "$OUTPUT_DIR" 2>&1; then
    assert_ok "build-linux.sh 执行成功" true
else
    assert_ok "build-linux.sh 执行成功" false
    echo -e "${RED}构建失败，终止测试${NC}"
    echo -e "${YELLOW}Summary: PASS=$PASS FAIL=$FAIL${NC}"
    exit 1
fi

# ── Test 2: 产物存在 ──
TARBALL=$(find "$OUTPUT_DIR" -name "Colink-Setup-*-linux-*.tar.gz" -type f 2>/dev/null | head -1)
if [[ -n "$TARBALL" && -f "$TARBALL" ]]; then
    echo -e "  ${GREEN}PASS${NC}: tar.gz 产物存在"
    PASS=$((PASS+1))
else
    echo -e "  ${RED}FAIL${NC}: tar.gz 产物不存在"
    FAIL=$((FAIL+1))
    echo -e "${RED}终止测试${NC}"
    echo -e "${YELLOW}Summary: PASS=$PASS FAIL=$FAIL${NC}"
    exit 1
fi

# ── Test 3: 文件名格式 ──
TARBALL_NAME=$(basename "$TARBALL")
assert_contains "文件名包含版本号" "$TARBALL_NAME" "Colink-Setup-"
assert_contains "文件名包含 linux" "$TARBALL_NAME" "-linux-"
assert_contains "文件名包含架构" "$TARBALL_NAME" "-amd64" || \
assert_contains "文件名包含架构(arm64)" "$TARBALL_NAME" "-arm64"

# ── Test 4: 解压后包含必要文件 ──
CHECK_DIR=$(mktemp -d)
tar -xzf "$TARBALL" -C "$CHECK_DIR"

assert_ok "包含 bin/colink-server" test -f "$CHECK_DIR/bin/colink-server"
assert_ok "包含 bin/migrate" test -f "$CHECK_DIR/bin/migrate"
assert_ok "包含 web/ 目录" test -d "$CHECK_DIR/web"
assert_ok "包含 sql-change/ 目录" test -d "$CHECK_DIR/sql-change"
assert_ok "包含 configs/config.yaml.example" test -f "$CHECK_DIR/configs/config.yaml.example"
assert_ok "包含 VERSION" test -f "$CHECK_DIR/VERSION"

# ── Test 5: VERSION 内容匹配 ──
SRC_VERSION=$(tr -d '\n\r' < "$PROJECT_ROOT/VERSION" 2>/dev/null || echo "")
PKG_VERSION=$(tr -d '\n\r' < "$CHECK_DIR/VERSION" 2>/dev/null || echo "")
assert_eq "VERSION 内容与源码一致" "$PKG_VERSION" "$SRC_VERSION"

# ── Test 6: 二进制文件格式（仅 Linux）──
if [[ "$(uname -s)" == "Linux" ]]; then
    FILE_TYPE=$(file "$CHECK_DIR/bin/colink-server" 2>/dev/null || echo "unknown")
    assert_contains "colink-server 为 ELF 二进制" "$FILE_TYPE" "ELF"
    FILE_TYPE2=$(file "$CHECK_DIR/bin/migrate" 2>/dev/null || echo "unknown")
    assert_contains "migrate 为 ELF 二进制" "$FILE_TYPE2" "ELF"
else
    echo -e "  ${YELLOW}SKIP${NC}: ELF 格式检测（非 Linux 环境）"
fi

# ── Test 7: sql-change 包含版本子目录 ──
SQL_DIRS=$(find "$CHECK_DIR/sql-change" -maxdepth 1 -type d -name 'v*' 2>/dev/null | wc -l)
if [[ "$SQL_DIRS" -gt 0 ]]; then
    echo -e "  ${GREEN}PASS${NC}: sql-change 包含 $SQL_DIRS 个版本子目录"
    PASS=$((PASS+1))
else
    echo -e "  ${RED}FAIL${NC}: sql-change 无版本子目录"
    FAIL=$((FAIL+1))
fi

# ── Test 8: config.yaml.example 包含关键字段 ──
EXAMPLE_CONFIG="$CHECK_DIR/configs/config.yaml.example"
if [[ -f "$EXAMPLE_CONFIG" ]]; then
    assert_ok "模板包含 server 节" grep -q '^server:' "$EXAMPLE_CONFIG"
    assert_ok "模板包含 data 节" grep -q '^data:' "$EXAMPLE_CONFIG"
    assert_ok "模板包含 logging 节" grep -q '^logging:' "$EXAMPLE_CONFIG"
    assert_ok "模板包含 logging.dir" grep -q 'dir:' "$EXAMPLE_CONFIG"
    assert_ok "模板包含 deployment 节" grep -q '^deployment:' "$EXAMPLE_CONFIG"
fi

rm -rf "$CHECK_DIR"

echo ""
echo -e "${YELLOW}Summary: PASS=$PASS FAIL=$FAIL${NC}"
if [[ $FAIL -gt 0 ]]; then exit 1; fi
