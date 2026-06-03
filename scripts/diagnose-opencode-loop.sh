#!/bin/bash
# OpenCode 无限循环诊断脚本
# 用法: ./scripts/diagnose-opencode-loop.sh

echo "=== OpenCode Process Monitor ==="
echo "Checking for running OpenCode processes..."

# 1. 检查进程状态
echo ""
echo "=== Process Status ==="
if pgrep -f "opencode" > /dev/null; then
    ps aux | grep -E "opencode|bun" | grep -v grep
else
    echo "No OpenCode process found"
fi

# 2. 检查进程树（Windows 不支持 pgrep -P，跨平台兼容）
echo ""
echo "=== Process Tree ==="
if [ "$(uname)" = "Windows_NT" ] || [ "$(expr substr $(uname -s) 1 5)" = "MINGW" ]; then
    # Windows: 使用 tasklist
    if pgrep -f "opencode" > /dev/null; then
        PARENT_PID=$(pgrep -f "opencode")
        echo "Parent PID: $PARENT_PID"
        tasklist /FI "PID eq $PARENT_PID" /V 2>/dev/null || echo "tasklist not available"
    fi
else
    # Unix/Linux: 使用 pgrep -P
    if pgrep -f "opencode" > /dev/null; then
        PARENT_PID=$(pgrep -f "opencode")
        echo "Parent PID: $PARENT_PID"
        pgrep -P $PARENT_PID | while read pid; do
            echo "Child process: PID $pid"
            ps -p $pid -o pid,ppid,cmd
        done
    fi
fi

# 3. 检查 stdout/stderr 是否仍在写入（仅 Unix）
echo ""
echo "=== File Descriptors ==="
if [ "$(uname)" != "Windows_NT" ]; then
    if pgrep -f "opencode" > /dev/null; then
        PID=$(pgrep -f "opencode")
        if [ -d "/proc/$PID/fd" ]; then
            ls -l /proc/$PID/fd/ 2>/dev/null | grep -E "pipe|socket" || echo "No pipe/socket fds"
        else
            echo "/proc not available on this platform"
        fi
    fi
else
    echo "File descriptor check not available on Windows"
fi

# 4. 检查最近的日志
echo ""
echo "=== Recent ACP Logs (last 50 lines) ==="
if [ -d "data/logs" ]; then
    tail -50 data/logs/*.log 2>/dev/null | grep -E "ACP:|notification|duplicate" | tail -20 || echo "No ACP logs found"
else
    echo "data/logs directory not found"
fi

# 5. 检查进程状态（僵尸/运行）- 仅 Unix
echo ""
echo "=== Process State ==="
if [ "$(uname)" != "Windows_NT" ]; then
    if pgrep -f "opencode" > /dev/null; then
        PID=$(pgrep -f "opencode")
        if [ -f "/proc/$PID/status" ]; then
            cat /proc/$PID/status 2>/dev/null | grep -E "State|Threads" || echo "Status file not readable"
        else
            echo "/proc not available"
        fi
    fi
else
    echo "Process state check not available on Windows"
fi

echo ""
echo "=== Diagnosis Complete ==="