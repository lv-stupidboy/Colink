#!/bin/bash
# Colink Linux 编译打包脚本
# 构建 Go 后端 + 前端，打包为 tar.gz 用于 Linux 部署
#
# 用法: ./build-linux.sh [--skip-deps] [--skip-frontend] [-o OUTPUT_DIR] [-h]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# 默认参数
SKIP_DEPS=false
SKIP_FRONTEND=false
OUTPUT_DIR=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-deps) SKIP_DEPS=true; shift ;;
        --skip-frontend) SKIP_FRONTEND=true; shift ;;
        -o) OUTPUT_DIR="$2"; shift 2 ;;
        -h|--help)
            echo "Colink Linux 编译打包脚本"
            echo ""
            echo "用法: ./build-linux.sh [选项]"
            echo ""
            echo "选项:"
            echo "  --skip-deps       跳过依赖检查"
            echo "  --skip-frontend   跳过前端构建"
            echo "  -o DIR            输出目录（默认: 项目根目录/output）"
            echo "  -h, --help        显示帮助"
            exit 0
            ;;
        *) echo "未知参数: $1"; exit 1 ;;
    esac
done

echo -e "${CYAN}=== Colink Linux Build ===${NC}"

# 读取版本号
VERSION_FILE="$PROJECT_ROOT/VERSION"
if [[ -f "$VERSION_FILE" ]]; then
    VERSION=$(tr -d '\n\r' < "$VERSION_FILE")
else
    VERSION="1.0.0"
fi
[[ -z "$VERSION" ]] && VERSION="1.0.0"
BUILD_TIME=$(date +"%Y%m%d-%H%M%S")
ARCH=$(uname -m)
[[ "$ARCH" == "x86_64" ]] && ARCH="amd64"
[[ "$ARCH" == "aarch64" ]] && ARCH="arm64"

FULL_VERSION="v${VERSION}-${BUILD_TIME}"
echo -e "${GREEN}Version: $FULL_VERSION  Arch: $ARCH${NC}"

# 临时目录
STAGING="$PROJECT_ROOT/staging-linux"
rm -rf "$STAGING"
mkdir -p "$STAGING"/{bin,configs,sql-change,web}

# Step 0: 依赖检查
if [[ "$SKIP_DEPS" == false ]]; then
    echo -e "${YELLOW}[0/5] Checking dependencies...${NC}"
    if ! command -v go &>/dev/null; then
        echo -e "${RED}Go not found. Please install Go 1.21+${NC}"; exit 1
    fi
    if [[ "$SKIP_FRONTEND" == false ]] && ! command -v npm &>/dev/null; then
        echo -e "${RED}npm not found. Please install Node.js 18+${NC}"; exit 1
    fi
    echo -e "${GREEN}Dependencies OK${NC}"
else
    echo -e "${YELLOW}[0/5] Skipping dependency check${NC}"
fi

# Step 1: 生成插件注册表
echo -e "${YELLOW}[1/5] Generating plugin registry...${NC}"
cd "$PROJECT_ROOT"
if [[ -f "./tools/genplugins/main.go" ]]; then
    go run ./tools/genplugins
fi
echo -e "${GREEN}Plugin registry generated${NC}"

# Step 2: 编译 Go 后端
echo -e "${YELLOW}[2/5] Building Go backend...${NC}"
cd "$PROJECT_ROOT"

GOOS=linux go build -ldflags "-X main.Version=${FULL_VERSION}" \
    -o "$STAGING/bin/colink-server" ./cmd/server
echo -e "${GREEN}colink-server built${NC}"

GOOS=linux go build -o "$STAGING/bin/migrate" ./cmd/migrate
echo -e "${GREEN}migrate built${NC}"

# Step 3: 构建前端
if [[ "$SKIP_FRONTEND" == false ]]; then
    echo -e "${YELLOW}[3/5] Building frontend...${NC}"
    cd "$PROJECT_ROOT/web"
    npm run build
    cp -r dist/* "$STAGING/web/"
    echo -e "${GREEN}Frontend built${NC}"
else
    echo -e "${YELLOW}[3/5] Skipping frontend build${NC}"
    if [[ -d "$PROJECT_ROOT/web/dist" ]]; then
        cp -r "$PROJECT_ROOT/web/dist/"* "$STAGING/web/"
        echo -e "${GREEN}Using existing frontend dist${NC}"
    fi
fi

# Step 4: 复制资源和配置
echo -e "${YELLOW}[4/5] Copying resources...${NC}"
cp "$PROJECT_ROOT/configs/config.yaml.example" "$STAGING/configs/"
cp -r "$PROJECT_ROOT/sql-change/"* "$STAGING/sql-change/"
cp "$PROJECT_ROOT/VERSION" "$STAGING/"
echo -e "${GREEN}Resources copied${NC}"

# Step 5: 打包 tar.gz
echo -e "${YELLOW}[5/5] Creating tar.gz package...${NC}"
[[ -z "$OUTPUT_DIR" ]] && OUTPUT_DIR="$PROJECT_ROOT/output"
mkdir -p "$OUTPUT_DIR"

PACKAGE_NAME="Colink-Setup-${VERSION}-${BUILD_TIME}-linux-${ARCH}.tar.gz"
PACKAGE_PATH="$OUTPUT_DIR/$PACKAGE_NAME"

cd "$STAGING"
tar -czf "$PACKAGE_PATH" -C "$STAGING" .

PACKAGE_SIZE=$(wc -c < "$PACKAGE_PATH")
PACKAGE_SIZE_MB=$((PACKAGE_SIZE / 1024 / 1024))

# 清理
rm -rf "$STAGING"

echo ""
echo -e "${CYAN}=== Build Complete ===${NC}"
echo -e "${GREEN}Output: $PACKAGE_PATH${NC}"
echo -e "${GREEN}Size: ${PACKAGE_SIZE_MB} MB${NC}"
echo ""
echo -e "${YELLOW}Package contents:${NC}"
echo "  bin/colink-server   (server binary)"
echo "  bin/migrate         (migration tool)"
echo "  web/                (frontend)"
echo "  configs/            (config template)"
echo "  sql-change/         (database migrations)"
echo "  VERSION             (version file)"
