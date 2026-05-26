#!/bin/bash
# Colink 服务启动脚本
#
# 用法: ./start.sh [-h]

set -euo pipefail

INSTALL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PID_FILE="$INSTALL_DIR/.colink-server.pid"
LOG_DIR="$INSTALL_DIR/logs"
CONFIG_FILE="$INSTALL_DIR/configs/config.yaml"

# 从配置文件读取端口
get_server_port() {
    if [[ -f "$CONFIG_FILE" ]]; then
        grep -A5 '^server:' "$CONFIG_FILE" | grep 'port:' | head -1 | awk '{print $2}' | tr -d '"' | tr -d "'"
    fi
}

# Step 1: 检查是否已在运行
if [[ -f "$PID_FILE" ]]; then
    OLD_PID=$(cat "$PID_FILE")
    if kill -0 "$OLD_PID" 2>/dev/null; then
        echo "colink-server 已在运行 (PID $OLD_PID)"
        exit 0
    fi
    # 过期 PID 文件，清理
    rm -f "$PID_FILE"
fi

# Step 2: 确保日志目录存在
mkdir -p "$LOG_DIR"

# Step 3: 启动服务
cd "$INSTALL_DIR"
SERVER_BIN="$INSTALL_DIR/bin/colink-server"

if [[ ! -x "$SERVER_BIN" ]]; then
    echo "错误: 未找到 colink-server 可执行文件: $SERVER_BIN"
    exit 1
fi

nohup "$SERVER_BIN" >> "$LOG_DIR/server-stdout.log" 2>&1 &
SERVER_PID=$!

# Step 4: 写入 PID 文件
echo "$SERVER_PID" > "$PID_FILE"

# Step 5: 验证启动
PORT=$(get_server_port)
PORT=${PORT:-26305}

for i in $(seq 1 10); do
    if ! kill -0 "$SERVER_PID" 2>/dev/null; then
        echo "错误: 服务进程在启动期间退出"
        cat "$LOG_DIR/server-stdout.log" | tail -20
        rm -f "$PID_FILE"
        exit 1
    fi
    if curl -sf "http://localhost:$PORT/api/v1/system/version" > /dev/null 2>&1; then
        echo "colink-server 启动成功 (PID $SERVER_PID, port $PORT)"
        exit 0
    fi
    sleep 1
done

echo "colink-server 进程运行中 (PID $SERVER_PID)，但健康检查尚未通过"
echo "请检查日志: $LOG_DIR/server.log"
