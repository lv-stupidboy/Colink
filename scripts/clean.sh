#!/bin/bash
# Colink 清理脚本 - 清理构建和打包生成的临时文件
#
# 用法:
#   ./clean.sh [选项]
#
# 选项:
#   -a, --all           清理所有构建产物（默认）
#   -d, --dist          清理前端构建产物 (web/dist, installer-tauri/dist)
#   -b, --bin           清理 Go 后端编译产物 (bin/)
#   -t, --target        清理 Rust 构建产物 (target/, gen/)
#   -g, --gen           清理 Tauri schema 文件 (gen/)
#   -s, --staging       清理资源同步中间目录
#   -n, --node-modules  清理所有 node_modules
#   -w, --whatif        预览模式，只显示不执行
#   -f, --force         跳过确认提示
#   -h, --help          显示帮助信息

set -e

# 获取脚本所在目录的父目录（主项目根目录）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
INSTALLER_DIR="$PROJECT_ROOT/installer-tauri"
SRC_TAURI_DIR="$INSTALLER_DIR/src-tauri"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
GRAY='\033[0;90m'
NC='\033[0m' # No Color

# 默认参数
ALL=false
DIST=false
BIN=false
TARGET=false
GEN=false
STAGING=false
NODE_MODULES=false
WHATIF=false
FORCE=false

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -a|--all)
            ALL=true
            shift
            ;;
        -d|--dist)
            DIST=true
            shift
            ;;
        -b|--bin)
            BIN=true
            shift
            ;;
        -t|--target)
            TARGET=true
            shift
            ;;
        -g|--gen)
            GEN=true
            shift
            ;;
        -s|--staging)
            STAGING=true
            shift
            ;;
        -n|--node-modules)
            NODE_MODULES=true
            shift
            ;;
        -w|--whatif)
            WHATIF=true
            shift
            ;;
        -f|--force)
            FORCE=true
            shift
            ;;
        -h|--help)
            echo "Colink 清理脚本"
            echo ""
            echo "用法: ./clean.sh [选项]"
            echo ""
            echo "选项:"
            echo "  -a, --all           清理所有构建产物（默认）"
            echo "  -d, --dist          清理前端构建产物 (web/dist, installer-tauri/dist)"
            echo "  -b, --bin           清理 Go 后端编译产物 (bin/)"
            echo "  -t, --target        清理 Rust 构建产物 (target/, gen/)"
            echo "  -g, --gen           清理 Tauri schema 文件 (gen/)"
            echo "  -s, --staging       清理资源同步中间目录"
            echo "  -n, --node-modules  清理所有 node_modules"
            echo "  -w, --whatif        预览模式，只显示不执行"
            echo "  -f, --force         跳过确认提示"
            echo "  -h, --help          显示帮助信息"
            exit 0
            ;;
        *)
            echo "未知参数: $1"
            exit 1
            ;;
    esac
done

# 如果没有指定任何参数，默认清理构建产物
if [[ "$ALL" == false && "$DIST" == false && "$BIN" == false && \
      "$TARGET" == false && "$GEN" == false && "$STAGING" == false && \
      "$NODE_MODULES" == false ]]; then
    ALL=true
fi

# 定义要清理的目录数组
DIRS_TO_CLEAN=()

if [[ "$ALL" == true || "$DIST" == true ]]; then
    DIRS_TO_CLEAN+=("$PROJECT_ROOT/web/dist:主项目前端构建产物 (web/dist)")
    DIRS_TO_CLEAN+=("$INSTALLER_DIR/dist:安装器前端构建产物 (installer-tauri/dist)")
fi

if [[ "$ALL" == true || "$BIN" == true ]]; then
    DIRS_TO_CLEAN+=("$PROJECT_ROOT/bin:Go 后端编译产物 (bin)")
fi

if [[ "$ALL" == true || "$STAGING" == true ]]; then
    DIRS_TO_CLEAN+=("$PROJECT_ROOT/staging:资源同步中间目录 (staging)")
fi

if [[ "$ALL" == true || "$TARGET" == true ]]; then
    DIRS_TO_CLEAN+=("$SRC_TAURI_DIR/target:Rust 构建产物 (src-tauri/target)")
    DIRS_TO_CLEAN+=("$SRC_TAURI_DIR/gen:Tauri schema 文件 (src-tauri/gen)")
elif [[ "$GEN" == true ]]; then
    DIRS_TO_CLEAN+=("$SRC_TAURI_DIR/gen:Tauri schema 文件 (src-tauri/gen)")
elif [[ "$STAGING" == true ]]; then
    TARGET_STAGING="$SRC_TAURI_DIR/target/release/staging"
    if [[ -d "$TARGET_STAGING" ]]; then
        DIRS_TO_CLEAN+=("$TARGET_STAGING:target 内 staging 目录")
    fi
fi

if [[ "$NODE_MODULES" == true ]]; then
    DIRS_TO_CLEAN+=("$PROJECT_ROOT/web/node_modules:主项目前端依赖 (web/node_modules)")
    DIRS_TO_CLEAN+=("$INSTALLER_DIR/node_modules:Tauri 安装器依赖 (installer-tauri/node_modules)")
fi

# 计算目录大小函数
get_dir_size() {
    local dir="$1"
    if [[ -d "$dir" ]]; then
        # 使用 du 命令获取大小，兼容不同系统
        local size_kb
        if [[ "$(uname)" == "Darwin" ]]; then
            size_kb=$(du -sk "$dir" 2>/dev/null | cut -f1)
        else
            size_kb=$(du -sb "$dir" 2>/dev/null | cut -f1 | awk '{print int($1/1024)}')
        fi
        # 转换为 MB
        local size_mb=$((size_kb / 1024))
        echo "$size_mb"
    else
        echo "0"
    fi
}

# 显示将要清理的目录
echo ""
echo -e "${CYAN}将要清理的目录:${NC}"
TOTAL_SIZE_MB=0
EXISTING_DIRS=()

for entry in "${DIRS_TO_CLEAN[@]}"; do
    dir="${entry%%:*}"
    name="${entry##*:}"

    if [[ -d "$dir" ]]; then
        size_mb=$(get_dir_size "$dir")
        TOTAL_SIZE_MB=$((TOTAL_SIZE_MB + size_mb))
        echo -e "  - ${name}: ${YELLOW}${size_mb} MB${NC}"
        EXISTING_DIRS+=("$entry")
    else
        echo -e "  - ${name}: ${GRAY}不存在 (跳过)${NC}"
    fi
done

if [[ ${#EXISTING_DIRS[@]} -eq 0 ]]; then
    echo ""
    echo -e "${GREEN}没有需要清理的目录。${NC}"
    exit 0
fi

echo ""
echo -e "${CYAN}总计: ${TOTAL_SIZE_MB} MB${NC}"

# WhatIf 模式：只显示，不执行
if [[ "$WHATIF" == true ]]; then
    echo ""
    echo -e "${CYAN}[预览模式] 以上目录将被清理，但未实际执行。${NC}"
    exit 0
fi

# 确认清理
if [[ "$FORCE" == false ]]; then
    echo ""
    read -p "是否继续清理? [Y/n] " confirm
    if [[ "$confirm" != "" && "$confirm" != "Y" && "$confirm" != "y" ]]; then
        echo -e "${YELLOW}已取消清理。${NC}"
        exit 0
    fi
fi

# 执行清理
echo ""
echo -e "${CYAN}开始清理...${NC}"
CLEANED_COUNT=0

for entry in "${EXISTING_DIRS[@]}"; do
    dir="${entry%%:*}"
    name="${entry##*:}"

    if rm -rf "$dir" 2>/dev/null; then
        echo -e "  ${GREEN}✓${NC} 已清理: ${name}"
        CLEANED_COUNT=$((CLEANED_COUNT + 1))
    else
        echo -e "  ${RED}✗${NC} 清理失败: ${name}"
    fi
done

echo ""
echo -e "${GREEN}清理完成! 已清理 ${CLEANED_COUNT} 个目录，释放 ${TOTAL_SIZE_MB} MB 空间。${NC}"