#!/bin/bash
# Colink 服务停止脚本
# 仅停止当前安装目录下的进程（通过 PID 文件识别）
#
# 用法: ./stop.sh

set -euo pipefail

INSTALL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PID_FILE="$INSTALL_DIR/.colink-server.pid"

# Step 1: 从 PID 文件获取进程
PID=""
if [[ -f "$PID_FILE" ]]; then
    PID=$(cat "$PID_FILE")
fi

# PID 文件不存在或进程已退出
if [[ -z "$PID" ]] || ! kill -0 "$PID" 2>/dev/null; then
    echo "colink-server 未在运行"
    rm -f "$PID_FILE"
    exit 0
fi

# Step 2: 发送 SIGTERM
echo "正在停止 colink-server (PID $PID)..."
kill -TERM "$PID" 2>/dev/null || true

# Step 3: 等待退出
for i in $(seq 1 30); do
    if ! kill -0 "$PID" 2>/dev/null; then
        echo "colink-server 已停止"
        rm -f "$PID_FILE"
        exit 0
    fi
    sleep 1
done

# Step 4: 强制终止
echo "colink-server 未能优雅停止，发送 SIGKILL..."
kill -KILL "$PID" 2>/dev/null || true
sleep 1
rm -f "$PID_FILE"
echo "colink-server 已强制终止"
