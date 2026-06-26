#!/bin/bash
# Colink Mac Release Build
# 构建 macOS 版本的 Colink 安装程序（DMG）
# 用法: ./build-mac.sh [--target aarch64|x86_64]

set -e

# Detect project root
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
INSTALLER_DIR="$PROJECT_ROOT/installer-tauri"
SRC_TAURI_DIR="$INSTALLER_DIR/src-tauri"

# Parse arguments
TARGET_ARCH=""
while [[ $# -gt 0 ]]; do
    case $1 in
        --target)
            TARGET_ARCH="$2"
            shift 2
            ;;
        *)
            echo "Unknown argument: $1"
            exit 1
            ;;
    esac
done

# Detect architecture if not specified
if [ -z "$TARGET_ARCH" ]; then
    ARCH=$(uname -m)
    if [ "$ARCH" == "arm64" ]; then
        TARGET_ARCH="aarch64"
    else
        TARGET_ARCH="x86_64"
    fi
fi

RUST_TARGET="${TARGET_ARCH}-apple-darwin"

# Read version
VERSION=$(cat "$PROJECT_ROOT/VERSION" 2>/dev/null || echo "1.0.0")
VERSION=$(echo "$VERSION" | tr -d '\n')
BUILD_TIME=$(date +"%Y%m%d-%H%M%S")

echo "=== Colink Mac Release Build ==="
echo "Project: $PROJECT_ROOT"
echo "Version: $VERSION"
echo "Build Time: $BUILD_TIME"
echo "Target: $RUST_TARGET"

# Step 0: Check dependencies
echo ""
echo "[0/7] Checking dependencies..."

# Check if we're on macOS
if [[ "$(uname)" != "Darwin" ]]; then
    echo "ERROR: This script must be run on macOS"
    exit 1
fi

# Check for required tools
command -v go >/dev/null 2>&1 || { echo "ERROR: Go not installed"; exit 1; }
command -v node >/dev/null 2>&1 || { echo "ERROR: Node.js not installed"; exit 1; }
command -v pnpm >/dev/null 2>&1 || { echo "ERROR: pnpm not installed"; exit 1; }
command -v cargo >/dev/null 2>&1 || { echo "ERROR: Rust/Cargo not installed"; exit 1; }

# Check web dependencies
if [ ! -d "$PROJECT_ROOT/web/node_modules" ]; then
    echo "Installing web dependencies..."
    cd "$PROJECT_ROOT/web"
    npm install
fi
echo "Web dependencies ready"

# Check installer dependencies
if [ ! -d "$INSTALLER_DIR/node_modules" ]; then
    echo "Installing installer-tauri dependencies..."
    cd "$INSTALLER_DIR"
    pnpm install
fi
echo "Installer dependencies ready"

# Step 1: Build backend (Go)
echo ""
echo "[1/7] Building backend..."
cd "$PROJECT_ROOT"

# Build server (no .exe extension on Mac)
go build -ldflags "-X main.Version=v$VERSION-$BUILD_TIME" -o bin/colink-server ./cmd/server
echo "Server built: bin/colink-server"

# Build migrate
go build -o bin/migrate ./cmd/migrate
echo "Migrate built: bin/migrate"

# Build mcp-server (provides post_message tool for A2A)
go build -o bin/mcp-server ./cmd/mcp-server
echo "MCP server built: bin/mcp-server"

# Step 2: Build frontend
echo ""
echo "[2/7] Building frontend..."
cd "$PROJECT_ROOT/web"
npm run build
echo "Frontend built: web/dist/"

# Step 3: Sync resources to staging
echo ""
echo "[3/7] Syncing resources to staging..."
cd "$PROJECT_ROOT"
STAGING_RESOURCES="$SRC_TAURI_DIR/target/release/staging/resources"
node scripts/sync-resources.js "$STAGING_RESOURCES"
echo "Resources synced to staging"

# Copy VERSION file
cp "$PROJECT_ROOT/VERSION" "$STAGING_RESOURCES/"
echo "VERSION copied"

# Step 4: Build installer frontend
echo ""
echo "[4/7] Building installer frontend..."
cd "$INSTALLER_DIR"
pnpm build:renderer
echo "Installer frontend built"

# Step 5: Generate icons
echo ""
echo "[5/7] Generating icons..."
ICON_SOURCE="$SRC_TAURI_DIR/icons/icon.png"
ICONS_DIR="$SRC_TAURI_DIR/icons"
ICONS_CACHE="$SRC_TAURI_DIR/target/release/icons-cache"

# Cache generated icons (keep only source)
if [ -d "$ICONS_DIR" ]; then
    mkdir -p "$ICONS_CACHE"
    find "$ICONS_DIR" -mindepth 1 ! -name "icon.png" -exec mv {} "$ICONS_CACHE/" 2>/dev/null || true
fi

if [ -f "$ICON_SOURCE" ]; then
    cd "$INSTALLER_DIR"
    pnpm tauri icon "$ICON_SOURCE" 2>/dev/null || echo "Icon generation skipped (using existing)"
    echo "Icons generated"
else
    echo "Icon source not found: $ICON_SOURCE"
fi

# Step 6: Build Tauri (Rust)
echo ""
echo "[6/7] Building Tauri for $RUST_TARGET..."
cd "$SRC_TAURI_DIR"

# Check if target is installed
if ! rustup target list --installed | grep -q "$RUST_TARGET"; then
    echo "Installing Rust target $RUST_TARGET..."
    rustup target add "$RUST_TARGET"
fi

cargo build --release --target "$RUST_TARGET"
echo "Tauri built"

# Create Launcher binary (same binary, different name for mode detection)
BUILT_BIN="$SRC_TAURI_DIR/target/$RUST_TARGET/release/colink-installer"
LAUNCHER_BIN="$SRC_TAURI_DIR/target/$RUST_TARGET/release/Colink"

if [ -f "$BUILT_BIN" ]; then
    cp "$BUILT_BIN" "$LAUNCHER_BIN"
    chmod +x "$LAUNCHER_BIN"
    echo "Launcher created: $LAUNCHER_BIN"
else
    echo "ERROR: Built binary not found: $BUILT_BIN"
    exit 1
fi

# Copy launcher to staging
LAUNCHER_DEST="$STAGING_RESOURCES/launcher"
mkdir -p "$LAUNCHER_DEST"
cp "$LAUNCHER_BIN" "$LAUNCHER_DEST/Colink"
echo "Launcher copied to staging"

# Step 7: Create App Bundle and DMG
echo ""
echo "[7/7] Creating App Bundle and DMG..."

# Build Setup binary (same as installer binary)
SETUP_BIN="$SRC_TAURI_DIR/target/$RUST_TARGET/release/colink-installer"

# Create App Bundle structure
APP_BUNDLE="$SRC_TAURI_DIR/target/$RUST_TARGET/release/Colink.app"
CONTENTS="$APP_BUNDLE/Contents"
MACOS="$CONTENTS/MacOS"
RESOURCES="$CONTENTS/Resources"

mkdir -p "$MACOS"
mkdir -p "$RESOURCES"

# Copy main executable (Setup mode - will be renamed by user to Colink.app for Launcher)
cp "$SETUP_BIN" "$MACOS/Colink"
chmod +x "$MACOS/Colink"
echo "Main executable copied"

# Copy backend binaries
cp "$STAGING_RESOURCES/colink-server" "$MACOS/" 2>/dev/null || \
    cp "$PROJECT_ROOT/bin/colink-server" "$MACOS/" || \
    echo "Warning: colink-server not found"
chmod +x "$MACOS/colink-server" 2>/dev/null || true

cp "$STAGING_RESOURCES/bin/migrate" "$MACOS/" 2>/dev/null || \
    cp "$PROJECT_ROOT/bin/migrate" "$MACOS/" || \
    echo "Warning: migrate not found"
chmod +x "$MACOS/migrate" 2>/dev/null || true

cp "$STAGING_RESOURCES/packages/runtime/tools/mcp-server" "$MACOS/" 2>/dev/null || \
    cp "$PROJECT_ROOT/bin/mcp-server" "$MACOS/" || \
    echo "Warning: mcp-server not found"
chmod +x "$MACOS/mcp-server" 2>/dev/null || true

# Copy launcher
cp "$LAUNCHER_DEST/Colink" "$MACOS/" 2>/dev/null || true

# Copy Resources
if [ -d "$STAGING_RESOURCES/web" ]; then
    cp -R "$STAGING_RESOURCES/web" "$RESOURCES/"
fi
if [ -d "$STAGING_RESOURCES/sql-change" ]; then
    cp -R "$STAGING_RESOURCES/sql-change" "$RESOURCES/"
fi
if [ -f "$STAGING_RESOURCES/config.yaml.example" ]; then
    cp "$STAGING_RESOURCES/config.yaml.example" "$RESOURCES/"
fi
if [ -f "$STAGING_RESOURCES/VERSION" ]; then
    cp "$STAGING_RESOURCES/VERSION" "$RESOURCES/"
fi
if [ -f "$STAGING_RESOURCES/installer-config.json" ]; then
    cp "$STAGING_RESOURCES/installer-config.json" "$RESOURCES/"
fi

# Copy icons
ICNS_ICON="$SRC_TAURI_DIR/icons/AppIcon.icns"
if [ -f "$ICNS_ICON" ]; then
    cp "$ICNS_ICON" "$RESOURCES/"
else
    # Try from cache
    if [ -f "$ICONS_CACHE/AppIcon.icns" ]; then
        cp "$ICONS_CACHE/AppIcon.icns" "$RESOURCES/"
    fi
fi

# Copy installer frontend dist
DIST_SRC="$INSTALLER_DIR/dist"
if [ -d "$DIST_SRC" ]; then
    cp -R "$DIST_SRC" "$RESOURCES/dist"
fi

echo "Resources copied"

# Generate Info.plist
PLIST_CONTENT="<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">
<plist version=\"1.0\">
<dict>
    <key>CFBundleName</key>
    <string>Colink</string>
    <key>CFBundleDisplayName</key>
    <string>Colink</string>
    <key>CFBundleIdentifier</key>
    <string>com.colink.installer</string>
    <key>CFBundleVersion</key>
    <string>$VERSION</string>
    <key>CFBundleShortVersionString</key>
    <string>$VERSION</string>
    <key>CFBundleExecutable</key>
    <string>Colink</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon.icns</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.15</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>"

echo "$PLIST_CONTENT" > "$CONTENTS/Info.plist"
echo "Info.plist generated"

# Create DMG
DMG_DIR="$SRC_TAURI_DIR/target/$RUST_TARGET/release/dist"
mkdir -p "$DMG_DIR"

DMG_NAME="Colink-Setup-$VERSION-$BUILD_TIME-$TARGET_ARCH.dmg"
DMG_PATH="$DMG_DIR/$DMG_NAME"

# Remove old DMG if exists
rm -f "$DMG_PATH"

echo "Creating DMG: $DMG_PATH"
hdiutil create -volname "Colink" -srcfolder "$APP_BUNDLE" -ov -format UDZO "$DMG_PATH"

# Get DMG size
DMG_SIZE=$(du -h "$DMG_PATH" | cut -f1)

# Cleanup: move generated icons to cache
if [ -d "$ICONS_DIR" ]; then
    mkdir -p "$ICONS_CACHE"
    find "$ICONS_DIR" -mindepth 1 ! -name "icon.png" -exec mv {} "$ICONS_CACHE/" 2>/dev/null || true
fi

echo ""
echo "=== Build Complete ==="
echo "Output: $DMG_PATH"
echo "Size: $DMG_SIZE"
echo ""
echo "App Bundle: $APP_BUNDLE"
echo ""
echo "DMG Contents:"
echo "  - Colink.app/"
echo "    - Contents/MacOS/Colink (main executable)"
echo "    - Contents/MacOS/colink-server"
echo "    - Contents/MacOS/migrate"
echo "    - Contents/Resources/..."
echo ""
echo "Usage: Mount DMG and drag Colink.app to /Applications"

# Return to project root
cd "$PROJECT_ROOT"
echo ""
echo "Returned to: $PROJECT_ROOT"