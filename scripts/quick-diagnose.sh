#!/bin/bash
# 快速诊断 - 集成所有检查
# 用法: ./scripts/quick-diagnose.sh

echo "=== Quick Diagnosis for OpenCode Loop ==="

# 检查日志中的重复模式
echo "1. Checking duplicate notifications..."
if [ -d "data/logs" ]; then
    DUPLICATE_COUNT=$(tail -100 data/logs/*.log 2>/dev/null | grep -c "duplicate session/update" || echo "0")
    if [ "$DUPLICATE_COUNT" -gt 5 ]; then
        echo "⚠️  WARNING: Found $DUPLICATE_COUNT duplicate notifications"
        echo "Last duplicates:"
        tail -100 data/logs/*.log 2>/dev/null | grep "duplicate session/update" | tail -5
    else
        echo "✅ No significant duplicates found (count: $DUPLICATE_COUNT)"
    fi
else
    echo "⚠️  data/logs directory not found"
fi

# 检查进程是否卡住
echo ""
echo "2. Checking process state..."
OPENCODE_PID=$(pgrep -f "opencode" | head -1)
if [ -n "$OPENCODE_PID" ]; then
    # Unix: 检查进程状态
    if [ "$(uname)" != "Windows_NT" ] && [ -f "/proc/$OPENCODE_PID/status" ]; then
        PROCESS_STATE=$(cat /proc/$OPENCODE_PID/status 2>/dev/null | grep "State" | awk '{print $2}' || echo "unknown")
        echo "PID: $OPENCODE_PID, State: $PROCESS_STATE"
    else
        echo "PID: $OPENCODE_PID (State check not available on Windows)"
        PROCESS_STATE="unknown"
    fi
    
    # 检查通知频率
    if [ -d "data/logs" ]; then
        NOTIFICATION_COUNT=$(tail -200 data/logs/*.log 2>/dev/null | grep -c "ACP: notification count" || echo "0")
        echo "Notification count (last 200 lines): $NOTIFICATION_COUNT"
        
        if [ "$PROCESS_STATE" = "R" ] && [ "$NOTIFICATION_COUNT" -gt 100 ]; then
            echo "⚠️  WARNING: Process running with high notification rate"
            echo "Recommendation: kill -9 $OPENCODE_PID (or taskkill /F /PID $OPENCODE_PID on Windows)"
        else
            echo "✅ Process state normal"
        fi
    fi
else
    echo "✅ No OpenCode process running"
fi

# 检查 cleanup 是否调用
echo ""
echo "3. Checking cleanup status..."
if [ -d "data/logs" ]; then
    CLEANUP_COUNT=$(tail -50 data/logs/*.log 2>/dev/null | grep -c "cleanup called" || echo "0")
    if [ "$CLEANUP_COUNT" -eq 0 ]; then
        echo "⚠️  WARNING: No cleanup called in recent logs"
        echo "Process may not have terminated properly"
    else
        echo "✅ Cleanup called $CLEANUP_COUNT times"
    fi
else
    echo "⚠️  Cannot check cleanup (no logs)"
fi

# 检查 stderr 错误
echo ""
echo "4. Checking stderr errors..."
if [ -d "data/logs" ]; then
    STDERR_ERRORS=$(tail -100 data/logs/*.log 2>/dev/null | grep -c "ACP: stderr output" || echo "0")
    if [ "$STDERR_ERRORS" -gt 10 ]; then
        echo "⚠️  WARNING: High stderr output count: $STDERR_ERRORS"
        echo "Last stderr lines:"
        tail -100 data/logs/*.log 2>/dev/null | grep "ACP: stderr output" | tail -5
    else
        echo "✅ Stderr output normal (count: $STDERR_ERRORS)"
    fi
fi

# 检查 WebSocket 连接状态
echo ""
echo "5. Checking WebSocket connections..."
if [ -d "data/logs" ]; then
    WS_CONNECTED=$(tail -50 data/logs/*.log 2>/dev/null | grep -c "WebSocket connected" || echo "0")
    WS_CLOSED=$(tail -50 data/logs/*.log 2>/dev/null | grep -c "WebSocket.*closed" || echo "0")
    echo "WebSocket connected: $WS_CONNECTED, closed: $WS_CLOSED"
    
    if [ "$WS_CONNECTED" -gt "$WS_CLOSED" ]; then
        echo "⚠️  WARNING: More connections than closures (potential leak)"
    else
        echo "✅ WebSocket balance normal"
    fi
fi

echo ""
echo "=== Diagnosis Complete ==="
echo ""
echo "Next steps:"
echo "- If duplicates found: check OpenCode CLI for bugs"
echo "- If process stuck: kill process and check cleanup logic"
echo "- If cleanup missing: check transport.Close() implementation"
echo "- For detailed logs: change logging.level to debug in config.yaml"