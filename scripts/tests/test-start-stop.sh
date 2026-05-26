#!/bin/bash
# 测试 start.sh 和 stop.sh
#
# 注意: 进程启停测试仅在 Linux 上有效，非 Linux 仅验证脚本存在性和语法
# 用法: ./test-start-stop.sh [--dir /path/to/install]

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

echo -e "${YELLOW}=== test-start-stop.sh ===${NC}"

START_SCRIPT="$PROJECT_ROOT/scripts/start.sh"
STOP_SCRIPT="$PROJECT_ROOT/scripts/stop.sh"

# ── Test: 脚本存在性 ──
echo "--- 脚本存在性 ---"
assert_ok "start.sh 存在" test -f "$START_SCRIPT"
assert_ok "stop.sh 存在" test -f "$STOP_SCRIPT"

# ── Test: 脚本语法 ──
echo ""
echo "--- 脚本语法 ---"
if bash -n "$START_SCRIPT" 2>&1; then
    echo -e "  ${GREEN}PASS${NC}: start.sh 语法正确"; PASS=$((PASS+1))
else
    echo -e "  ${RED}FAIL${NC}: start.sh 语法错误"; FAIL=$((FAIL+1))
fi
if bash -n "$STOP_SCRIPT" 2>&1; then
    echo -e "  ${GREEN}PASS${NC}: stop.sh 语法正确"; PASS=$((PASS+1))
else
    echo -e "  ${RED}FAIL${NC}: stop.sh 语法错误"; FAIL=$((FAIL+1))
fi

# ── Test: 关键逻辑存在 ──
echo ""
echo "--- 关键逻辑 ---"
assert_ok "start.sh 包含 PID 文件逻辑" grep -q 'colink-server.pid' "$START_SCRIPT"
assert_ok "start.sh 包含 nohup" grep -q 'nohup' "$START_SCRIPT"
assert_ok "start.sh 包含健康检查" grep -q 'curl' "$START_SCRIPT"
assert_ok "start.sh 包含幂等检查" grep -q 'kill -0' "$START_SCRIPT"
assert_ok "stop.sh 包含 SIGTERM" grep -q 'TERM' "$STOP_SCRIPT"
assert_ok "stop.sh 包含 SIGKILL" grep -q 'KILL' "$STOP_SCRIPT"
assert_ok "stop.sh 包含 pgrep 兜底" grep -q 'pgrep' "$STOP_SCRIPT"
assert_ok "stop.sh 包含超时等待" grep -q '30' "$STOP_SCRIPT"

# ── Test: 进程管理（仅 Linux）──
if [[ "$(uname -s)" != "Linux" ]]; then
    echo ""
    echo -e "${YELLOW}非 Linux 环境，跳过进程管理测试${NC}"
    echo ""
    echo -e "${YELLOW}Summary: PASS=$PASS FAIL=$FAIL${NC}"
    if [[ $FAIL -gt 0 ]]; then exit 1; fi
    exit 0
fi

# 在 Linux 上执行实际进程测试
echo ""
echo "--- 进程管理测试 ---"

INSTALL_DIR="${1:-}"
if [[ -z "$INSTALL_DIR" ]]; then
    OUTPUT_DIR=$(mktemp -d)
    trap "rm -rf '$OUTPUT_DIR'" EXIT

    echo "Building package..."
    bash "$PROJECT_ROOT/scripts/build-linux.sh" --skip-deps --skip-frontend -o "$OUTPUT_DIR" 2>&1
    PACKAGE_PATH=$(find "$OUTPUT_DIR" -name "Colink-Setup-*-linux-*.tar.gz" -type f | head -1)

    INSTALL_BASE=$(mktemp -d)
    INSTALL_DIR="$INSTALL_BASE/colink-test"
    trap "rm -rf '$INSTALL_BASE' '$OUTPUT_DIR'" EXIT

    echo "Deploying..."
    bash "$PROJECT_ROOT/scripts/deploy.sh" --dir "$INSTALL_DIR" --port 28082 --package "$PACKAGE_PATH" 2>&1
fi

PID_FILE="$INSTALL_DIR/.colink-server.pid"

# Test 1: 启动
echo "Starting server..."
if bash "$START_SCRIPT" 2>&1; then
    echo -e "  ${GREEN}PASS${NC}: start.sh 执行成功"; PASS=$((PASS+1))
else
    echo -e "  ${RED}FAIL${NC}: start.sh 执行失败"; FAIL=$((FAIL+1))
fi

# Test 2: PID 文件
sleep 2
if [[ -f "$PID_FILE" ]]; then
    echo -e "  ${GREEN}PASS${NC}: PID 文件已创建"; PASS=$((PASS+1))
    PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
        echo -e "  ${GREEN}PASS${NC}: 进程运行中 (PID $PID)"; PASS=$((PASS+1))
    else
        echo -e "  ${RED}FAIL${NC}: PID $PID 不在运行"; FAIL=$((FAIL+1))
    fi
else
    echo -e "  ${RED}FAIL${NC}: PID 文件未创建"; FAIL=$((FAIL+1))
fi

# Test 3: 幂等性
echo "Starting again (should be idempotent)..."
bash "$START_SCRIPT" 2>&1
PID_COUNT=$(pgrep -c -f "colink-server" 2>/dev/null || echo "1")
if [[ "$PID_COUNT" -le 1 ]]; then
    echo -e "  ${GREEN}PASS${NC}: 启动幂等（仅一个进程）"; PASS=$((PASS+1))
else
    echo -e "  ${RED}FAIL${NC}: 发现 $PID_COUNT 个进程"; FAIL=$((FAIL+1))
fi

# Test 4: 健康检查
PORT=$(grep 'port:' "$INSTALL_DIR/configs/config.yaml" | head -1 | awk '{print $2}')
if curl -sf "http://localhost:${PORT:-28082}/api/v1/system/version" > /dev/null 2>&1; then
    echo -e "  ${GREEN}PASS${NC}: 健康检查通过"; PASS=$((PASS+1))
else
    echo -e "  ${RED}FAIL${NC}: 健康检查未通过"; FAIL=$((FAIL+1))
fi

# Test 5: 停止
echo "Stopping server..."
if bash "$STOP_SCRIPT" 2>&1; then
    echo -e "  ${GREEN}PASS${NC}: stop.sh 执行成功"; PASS=$((PASS+1))
else
    echo -e "  ${RED}FAIL${NC}: stop.sh 执行失败"; FAIL=$((FAIL+1))
fi

# Test 6: PID 文件已清理
if [[ ! -f "$PID_FILE" ]]; then
    echo -e "  ${GREEN}PASS${NC}: PID 文件已删除"; PASS=$((PASS+1))
else
    echo -e "  ${RED}FAIL${NC}: PID 文件未删除"; FAIL=$((FAIL+1))
fi

# Test 7: 进程已退出
sleep 1
if ! pgrep -f "colink-server" >/dev/null 2>&1; then
    echo -e "  ${GREEN}PASS${NC}: 进程已退出"; PASS=$((PASS+1))
else
    echo -e "  ${RED}FAIL${NC}: 进程仍在运行"; FAIL=$((FAIL+1))
fi

# Test 8: 停止未运行服务
echo "Stopping again (should handle gracefully)..."
if bash "$STOP_SCRIPT" 2>&1; then
    echo -e "  ${GREEN}PASS${NC}: 停止未运行服务处理正常"; PASS=$((PASS+1))
else
    echo -e "  ${RED}FAIL${NC}: 停止未运行服务应正常退出"; FAIL=$((FAIL+1))
fi

echo ""
echo -e "${YELLOW}Summary: PASS=$PASS FAIL=$FAIL${NC}"
if [[ $FAIL -gt 0 ]]; then exit 1; fi
