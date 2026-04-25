#!/bin/bash
# Colink 完整发布构建脚本
# 构建所有组件并打包为 ZIP
#
# 用法: ./build-release.sh [--skip-deps]
#
# 选项:
#   --skip-deps    跳过依赖检查/安装（假设依赖已安装）
#   -h, --help     显示帮助信息

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
NC='\033[0m' # No Color

# 解析参数
SKIP_DEPS=false
while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-deps)
            SKIP_DEPS=true
            shift
            ;;
        -h|--help)
            echo "Colink 完整发布构建脚本"
            echo ""
            echo "用法: ./build-release.sh [--skip-deps]"
            echo ""
            echo "选项:"
            echo "  --skip-deps    跳过依赖检查/安装（假设依赖已安装）"
            echo "  -h, --help     显示帮助信息"
            exit 0
            ;;
        *)
            echo "未知参数: $1"
            exit 1
            ;;
    esac
done

echo -e "${CYAN}=== Colink Full Release Build ===${NC}"
echo "Project: $PROJECT_ROOT"

# 读取版本号
VERSION_FILE="$PROJECT_ROOT/VERSION"
if [[ -f "$VERSION_FILE" ]]; then
    VERSION=$(cat "$VERSION_FILE" | tr -d '\n\r')
else
    VERSION="1.0.0"
fi
if [[ -z "$VERSION" ]]; then
    VERSION="1.0.0"
fi
BUILD_TIME=$(date +"%Y%m%d-%H%M%S")
echo -e "${GREEN}Version: $VERSION${NC}"

# Step 0: Install dependencies if needed
echo ""
echo -e "${YELLOW}[0/7] Checking dependencies...${NC}"

WEB_NODE_MODULES="$PROJECT_ROOT/web/node_modules"
INSTALLER_NODE_MODULES="$INSTALLER_DIR/node_modules"

if [[ "$SKIP_DEPS" == false ]]; then
    # Check web dependencies
    if [[ ! -d "$WEB_NODE_MODULES" ]]; then
        echo -e "${CYAN}Installing web dependencies...${NC}"
        cd "$PROJECT_ROOT/web"
        npm install
        if [[ $? -ne 0 ]]; then
            echo -e "${RED}Web dependencies install failed${NC}"
            exit 1
        fi
        echo -e "${GREEN}Web dependencies installed${NC}"
    else
        echo -e "${GREEN}Web dependencies already installed${NC}"
    fi

    # Check installer-tauri dependencies
    if [[ ! -d "$INSTALLER_NODE_MODULES" ]]; then
        echo -e "${CYAN}Installing installer-tauri dependencies...${NC}"
        cd "$INSTALLER_DIR"
        pnpm install
        if [[ $? -ne 0 ]]; then
            echo -e "${RED}Installer dependencies install failed${NC}"
            exit 1
        fi
        echo -e "${GREEN}Installer dependencies installed${NC}"
    else
        echo -e "${GREEN}Installer dependencies already installed${NC}"
    fi
else
    echo -e "${YELLOW}Skipping dependency check (--skip-deps)${NC}"
fi

# Step 1: Build ISDP backend (server + migrate)
echo ""
echo -e "${YELLOW}[1/7] Building ISDP backend...${NC}"
cd "$PROJECT_ROOT"

# Build server
go build -ldflags "-X main.Version=v$VERSION-$BUILD_TIME" -o bin/colink-server ./cmd/server
if [[ $? -ne 0 ]]; then
    echo -e "${RED}Server build failed${NC}"
    exit 1
fi
echo -e "${GREEN}Server built: bin/colink-server${NC}"

# Build migrate
go build -o bin/migrate ./cmd/migrate
if [[ $? -ne 0 ]]; then
    echo -e "${RED}Migrate build failed${NC}"
    exit 1
fi
echo -e "${GREEN}Migrate built: bin/migrate${NC}"

# Step 2: Build ISDP frontend
echo ""
echo -e "${YELLOW}[2/7] Building ISDP frontend...${NC}"
cd "$PROJECT_ROOT/web"
npm run build
if [[ $? -ne 0 ]]; then
    echo -e "${RED}ISDP frontend build failed${NC}"
    exit 1
fi
echo -e "${GREEN}ISDP frontend built: web/dist/${NC}"

# Step 3: Sync resources to staging
echo ""
echo -e "${YELLOW}[3/7] Syncing resources to staging...${NC}"
cd "$PROJECT_ROOT"
STAGING_RESOURCES="$SRC_TAURI_DIR/target/release/staging/resources"
node scripts/sync-resources.js "$STAGING_RESOURCES"
echo -e "${GREEN}Resources synced to staging${NC}"

# Step 4: Copy VERSION file
echo ""
echo -e "${YELLOW}[4/7] Copying VERSION file...${NC}"
cp "$PROJECT_ROOT/VERSION" "$STAGING_RESOURCES/"
echo -e "${GREEN}VERSION copied${NC}"

# Step 5: Build installer-tauri frontend renderer
echo ""
echo -e "${YELLOW}[5/7] Building installer frontend...${NC}"
cd "$INSTALLER_DIR"
pnpm build:renderer
if [[ $? -ne 0 ]]; then
    echo -e "${RED}Installer frontend build failed${NC}"
    exit 1
fi
echo -e "${GREEN}Installer frontend built${NC}"

# Step 5.5: Generate icons from source image
echo ""
echo -e "${YELLOW}[5.5/7] Generating icons...${NC}"
cd "$INSTALLER_DIR"
ICON_SOURCE="$SRC_TAURI_DIR/icons/icon.png"
ICONS_DIR="$SRC_TAURI_DIR/icons"
ICONS_CACHE_DIR="$SRC_TAURI_DIR/target/release/icons-cache"

# Clean icons directory before generation - move generated icons to cache
if [[ -d "$ICONS_DIR" ]]; then
    # Create cache directory
    mkdir -p "$ICONS_CACHE_DIR"
    # Move all generated icons to cache (keep only icon.png)
    for item in "$ICONS_DIR"/*; do
        item_name=$(basename "$item")
        if [[ "$item_name" != "icon.png" ]]; then
            mv "$item" "$ICONS_CACHE_DIR/" 2>/dev/null || true
        fi
    done
fi

if [[ -f "$ICON_SOURCE" ]]; then
    pnpm tauri icon "$ICON_SOURCE" 2>/dev/null || true
    echo -e "${GREEN}Icons generated${NC}"
else
    echo -e "${YELLOW}Icon source not found: $ICON_SOURCE (using existing icons)${NC}"
fi

# Step 6: Build exe
echo ""
echo -e "${YELLOW}[6/7] Building exe...${NC}"
cd "$SRC_TAURI_DIR"

# Build release exe (cross-platform)
if [[ "$(uname)" == "Darwin" ]]; then
    # macOS
    cargo build --release
    BUILT_EXE="$SRC_TAURI_DIR/target/release/colink-installer"
else
    # Linux/Windows (with target suffix for cross-compilation)
    TARGET=""
    if [[ "$(uname -s)" == "MINGW"* || "$(uname -s)" == "CYGWIN"* ]]; then
        # Native Windows
        cargo build --release
        BUILT_EXE="$SRC_TAURI_DIR/target/release/colink-installer.exe"
    else
        # Linux - check for cross-compilation
        cargo build --release
        BUILT_EXE="$SRC_TAURI_DIR/target/release/colink-installer"
    fi
fi

if [[ $? -ne 0 ]]; then
    echo -e "${RED}Build failed${NC}"
    exit 1
fi

# Create Launcher - same binary, different name for mode detection
if [[ "$(uname)" == "Darwin" ]]; then
    COLINK_EXE="$SRC_TAURI_DIR/target/release/Colink"
else
    COLINK_EXE="$SRC_TAURI_DIR/target/release/Colink.exe"
fi
cp "$BUILT_EXE" "$COLINK_EXE"
echo -e "${GREEN}Launcher created: target/release/Colink${NC}"

# Copy launcher to staging resources/launcher
LAUNCHER_DEST_DIR="$STAGING_RESOURCES/launcher"
mkdir -p "$LAUNCHER_DEST_DIR"
cp "$COLINK_EXE" "$LAUNCHER_DEST_DIR/"
echo -e "${GREEN}Launcher copied to staging${NC}"

# Step 7: Create ZIP package
echo ""
echo -e "${YELLOW}[7/7] Creating ZIP package...${NC}"

TARGET_DIR="$SRC_TAURI_DIR/target/release"
STAGING_DIR="$TARGET_DIR/staging"
EXE_DIR="$STAGING_DIR/exe"

# Ensure exe directory exists
mkdir -p "$EXE_DIR"

# Copy Setup exe
if [[ "$(uname)" == "Darwin" ]]; then
    SETUP_SRC="$TARGET_DIR/colink-installer"
    SETUP_DEST="$EXE_DIR/Colink-Setup"
else
    SETUP_SRC="$TARGET_DIR/colink-installer.exe"
    SETUP_DEST="$EXE_DIR/Colink-Setup.exe"
fi
cp "$SETUP_SRC" "$SETUP_DEST"
echo -e "${GREEN}Setup exe copied to staging${NC}"

# Copy installer frontend dist to staging root
DIST_SRC="$INSTALLER_DIR/dist"
DIST_DEST="$STAGING_DIR/dist"
rm -rf "$DIST_DEST"
cp -r "$DIST_SRC" "$DIST_DEST"
echo -e "${GREEN}Installer frontend dist copied${NC}"

# Create Start-Setup script (bash for Unix, bat for Windows)
if [[ "$(uname)" == "Darwin" || "$(uname)" == "Linux" ]]; then
    START_SCRIPT="$STAGING_DIR/Start-Setup.sh"
    cat > "$START_SCRIPT" << 'EOF'
#!/bin/bash
cd exe
./Colink-Setup
EOF
    chmod +x "$START_SCRIPT"
else
    START_SCRIPT="$STAGING_DIR/Start-Setup.bat"
    cat > "$START_SCRIPT" << 'EOF'
@echo off
cd exe
start "" Colink-Setup.exe
EOF
fi

# Create README.txt (UTF-8)
README_PATH="$STAGING_DIR/README.txt"
cat > "$README_PATH" << EOF
Colink Setup v$VERSION 安装说明

=== 使用方式 ===

方式一：双击 Start-Setup 启动（推荐）

方式二：进入 exe 目录，运行 Colink-Setup

=== 目录结构 ===

dist/             前端资源（Tauri 依赖）
exe/              安装程序目录
  Colink-Setup        安装程序
  resources/          后端资源和配置
    launcher/         启动器程序
      Colink

=== 安装后 ===

安装完成后，可通过桌面快捷方式启动 Colink。
已安装用户可通过 Launcher 控制服务启停。
EOF

# Create ZIP
OUTPUT_DIR="$TARGET_DIR/dist"
mkdir -p "$OUTPUT_DIR"

ZIP_NAME="Colink-Setup-$VERSION-$BUILD_TIME.zip"
ZIP_PATH="$OUTPUT_DIR/$ZIP_NAME"

# Remove old zip if exists
rm -f "$ZIP_PATH"

echo -e "${YELLOW}Creating ZIP: $ZIP_PATH${NC}"
cd "$STAGING_DIR"

# Use PowerShell on Windows, zip on Unix
if [[ "$(uname -s)" == "MINGW"* || "$(uname -s)" == "CYGWIN"* || "$(uname -s)" == "MSYS"* ]]; then
    # Windows - use PowerShell Compress-Archive
    # Convert Unix path to Windows path for PowerShell
    WIN_STAGING_DIR=$(cygpath -w "$STAGING_DIR")
    WIN_ZIP_PATH=$(cygpath -w "$ZIP_PATH")
    powershell.exe -NoProfile -Command "Compress-Archive -Path '$WIN_STAGING_DIR/*' -DestinationPath '$WIN_ZIP_PATH' -CompressionLevel Optimal -Force"
else
    # Unix - use zip command
    zip -r "$ZIP_PATH" . -x "*.DS_Store"
fi

# Get zip size (cross-platform)
if [[ "$(uname -s)" == "MINGW"* || "$(uname -s)" == "CYGWIN"* || "$(uname -s)" == "MSYS"* ]]; then
    # Windows - use ls or wc
    ZIP_SIZE=$(wc -c < "$ZIP_PATH" | tr -d ' ')
else
    # Unix - use stat
    ZIP_SIZE=$(stat -f%z "$ZIP_PATH" 2>/dev/null || stat --printf=%s "$ZIP_PATH" 2>/dev/null || wc -c < "$ZIP_PATH")
fi
ZIP_SIZE_MB=$((ZIP_SIZE / 1024 / 1024))

echo ""
echo -e "${CYAN}=== Build Complete ===${NC}"
echo -e "${GREEN}Output: $ZIP_PATH${NC}"
echo -e "${GREEN}Size: ${ZIP_SIZE_MB} MB${NC}"
echo ""
echo -e "${YELLOW}ZIP Contents:${NC}"
echo "  - Start-Setup script"
echo "  - README.txt"
echo "  - dist/            (installer frontend)"
echo "  - exe/"
echo "    - Colink-Setup"
echo "    - resources/"
echo "      - colink-server"
echo "      - web/          (isdp frontend)"
echo "      - launcher/"
echo "        - Colink"
echo ""
echo -e "${YELLOW}Usage: Unzip and run exe/Colink-Setup or Start-Setup${NC}"

# Clean icons directory after build - move generated icons to cache
if [[ -d "$ICONS_DIR" ]]; then
    mkdir -p "$ICONS_CACHE_DIR"
    for item in "$ICONS_DIR"/*; do
        item_name=$(basename "$item")
        if [[ "$item_name" != "icon.png" ]]; then
            mv "$item" "$ICONS_CACHE_DIR/" 2>/dev/null || true
        fi
    done
    echo -e "${GREEN}Icons cleaned (moved to target/release/icons-cache)${NC}"
fi