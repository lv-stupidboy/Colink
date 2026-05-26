#!/bin/bash
# Colink 服务状态脚本
# 仅查看当前安装目录下的进程状态（通过 PID 文件识别）
#
# 用法: ./status.sh

set -euo pipefail

INSTALL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PID_FILE="$INSTALL_DIR/.colink-server.pid"
CONFIG_FILE="$INSTALL_DIR/configs/config.yaml"
META_FILE="$INSTALL_DIR/.install-meta"

# 自动选择 python
PYTHON=""
if command -v python3 &>/dev/null; then
    PYTHON=python3
elif command -v python &>/dev/null; then
    PYTHON=python
fi

py_path() {
    if command -v cygpath &>/dev/null; then
        cygpath -m "$1"
    else
        echo "$1"
    fi
}

# 从配置文件读取端口
get_server_port() {
    if [[ -f "$CONFIG_FILE" ]]; then
        grep -A5 '^server:' "$CONFIG_FILE" | grep 'port:' | head -1 | awk '{print $2}' | tr -d '"' | tr -d "'"
    fi
}

# 读取版本
get_version() {
    if [[ -f "$INSTALL_DIR/VERSION" ]]; then
        tr -d '\n\r' < "$INSTALL_DIR/VERSION"
    elif [[ -n "$PYTHON" && -f "$META_FILE" ]]; then
        PY_META=$(py_path "$META_FILE")
        $PYTHON -c "import json; print(json.load(open('$PY_META', encoding='utf-8')).get('version',''))" 2>/dev/null || echo "unknown"
    else
        echo "unknown"
    fi
}

# 检查进程状态
check_process() {
    if [[ ! -f "$PID_FILE" ]]; then
        echo "STOPPED"
        return
    fi

    PID=$(cat "$PID_FILE" 2>/dev/null || echo "")
    if [[ -z "$PID" ]]; then
        echo "STOPPED"
        return
    fi

    if kill -0 "$PID" 2>/dev/null; then
        echo "RUNNING"
    else
        echo "STOPPED"
    fi
}

# 主逻辑
STATUS=$(check_process)
VERSION=$(get_version)
PORT=$(get_server_port)
PORT=${PORT:-26305}

echo "=== Colink Status ==="
echo "  目录:   $INSTALL_DIR"
echo "  版本:   $VERSION"
echo "  端口:   $PORT"

if [[ "$STATUS" == "RUNNING" ]]; then
    PID=$(cat "$PID_FILE")

    # 进程信息
    START_TIME=$(ps -p "$PID" -o lstart= 2>/dev/null || echo "unknown")
    MEM_MB=$(ps -p "$PID" -o rss= 2>/dev/null | awk '{printf "%.1f", $1/1024}' || echo "unknown")
    CPU_PCT=$(ps -p "$PID" -o %cpu= 2>/dev/null | tr -d ' ' || echo "unknown")

    echo -e "  状态:   \033[0;32mRUNNING\033[0m (PID $PID)"
    echo "  启动时间: $START_TIME"
    echo "  内存:   ${MEM_MB} MB"
    echo "  CPU:    ${CPU_PCT}%"

    # 健康检查
    if curl -sf "http://localhost:$PORT/api/v1/system/version" > /dev/null 2>&1; then
        echo -e "  健康:   \033[0;32mOK\033[0m"
    else
        echo -e "  健康:   \033[0;33mUNREACHABLE\033[0m (端口 $PORT 无响应)"
    fi
else
    echo -e "  状态:   \033[0;31mSTOPPED\033[0m"
    # 清理过期 PID 文件
    if [[ -f "$PID_FILE" ]]; then
        rm -f "$PID_FILE"
    fi
fi
