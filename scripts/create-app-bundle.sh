#!/bin/bash
# Create macOS App Bundle for Colink
# 用法: ./create-app-bundle.sh <resources_dir> <app_bundle_path> <version>
# 示例: ./create-app-bundle.sh staging/resources Colink.app 1.0.0

set -e

# Parse arguments
if [ $# -lt 3 ]; then
    echo "Usage: $0 <resources_dir> <app_bundle_path> <version>"
    echo "Example: $0 staging/resources Colink.app 1.0.0"
    exit 1
fi

RESOURCES_DIR="$1"
APP_BUNDLE_PATH="$2"
VERSION="$3"

echo "=== Creating App Bundle ==="
echo "Resources: $RESOURCES_DIR"
echo "Output: $APP_BUNDLE_PATH"
echo "Version: $VERSION"

# Validate resources directory
if [ ! -d "$RESOURCES_DIR" ]; then
    echo "ERROR: Resources directory not found: $RESOURCES_DIR"
    exit 1
fi

# Create App Bundle structure
CONTENTS="$APP_BUNDLE_PATH/Contents"
MACOS="$CONTENTS/MacOS"
RESOURCES="$CONTENTS/Resources"

mkdir -p "$MACOS"
mkdir -p "$RESOURCES"

echo "Directory structure created"

# Copy main executable (from current build)
# Try to find the built binary
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
SRC_TAURI_DIR="$PROJECT_ROOT/installer-tauri/src-tauri"

# Find architecture-specific build
for TARGET_DIR in "$SRC_TAURI_DIR/target/aarch64-apple-darwin/release" \
                  "$SRC_TAURI_DIR/target/x86_64-apple-darwin/release" \
                  "$SRC_TAURI_DIR/target/release"; do
    if [ -f "$TARGET_DIR/colink-installer" ]; then
        MAIN_EXE="$TARGET_DIR/colink-installer"
        break
    fi
done

if [ -z "$MAIN_EXE" ] || [ ! -f "$MAIN_EXE" ]; then
    echo "ERROR: Main executable not found. Build Tauri first with cargo build --release"
    exit 1
fi

cp "$MAIN_EXE" "$MACOS/Colink"
chmod +x "$MACOS/Colink"
echo "Main executable copied: $MAIN_EXE"

# Copy backend binaries (remove .exe extension for Mac)
# Server
if [ -f "$RESOURCES_DIR/colink-server" ]; then
    cp "$RESOURCES_DIR/colink-server" "$MACOS/"
elif [ -f "$RESOURCES_DIR/colink-server.exe" ]; then
    cp "$RESOURCES_DIR/colink-server.exe" "$MACOS/colink-server"
elif [ -f "$PROJECT_ROOT/bin/colink-server" ]; then
    cp "$PROJECT_ROOT/bin/colink-server" "$MACOS/"
elif [ -f "$PROJECT_ROOT/bin/colink-server.exe" ]; then
    cp "$PROJECT_ROOT/bin/colink-server.exe" "$MACOS/colink-server"
else
    echo "Warning: colink-server not found"
fi
chmod +x "$MACOS/colink-server" 2>/dev/null || true

# Migrate
if [ -f "$RESOURCES_DIR/bin/migrate" ]; then
    cp "$RESOURCES_DIR/bin/migrate" "$MACOS/"
elif [ -f "$RESOURCES_DIR/bin/migrate.exe" ]; then
    cp "$RESOURCES_DIR/bin/migrate.exe" "$MACOS/migrate"
elif [ -f "$RESOURCES_DIR/migrate" ]; then
    cp "$RESOURCES_DIR/migrate" "$MACOS/"
elif [ -f "$RESOURCES_DIR/migrate.exe" ]; then
    cp "$RESOURCES_DIR/migrate.exe" "$MACOS/migrate"
elif [ -f "$PROJECT_ROOT/bin/migrate" ]; then
    cp "$PROJECT_ROOT/bin/migrate" "$MACOS/"
elif [ -f "$PROJECT_ROOT/bin/migrate.exe" ]; then
    cp "$PROJECT_ROOT/bin/migrate.exe" "$MACOS/migrate"
else
    echo "Warning: migrate not found"
fi
chmod +x "$MACOS/migrate" 2>/dev/null || true

# Copy Launcher
if [ -f "$RESOURCES_DIR/launcher/Colink" ]; then
    cp "$RESOURCES_DIR/launcher/Colink" "$MACOS/"
elif [ -f "$RESOURCES_DIR/launcher/Colink.exe" ]; then
    cp "$RESOURCES_DIR/launcher/Colink.exe" "$MACOS/Colink"
else
    # Copy from target
    for TARGET_DIR in "$SRC_TAURI_DIR/target/aarch64-apple-darwin/release" \
                      "$SRC_TAURI_DIR/target/x86_64-apple-darwin/release" \
                      "$SRC_TAURI_DIR/target/release"; do
        if [ -f "$TARGET_DIR/Colink" ]; then
            cp "$TARGET_DIR/Colink" "$MACOS/"
            break
        fi
    done
fi
chmod +x "$MACOS/Colink" 2>/dev/null || true

echo "Backend binaries copied"

# Copy Resources directory contents
# Web frontend
if [ -d "$RESOURCES_DIR/web" ]; then
    cp -R "$RESOURCES_DIR/web" "$RESOURCES/"
    echo "Web frontend copied"
fi

# SQL migrations
if [ -d "$RESOURCES_DIR/sql-change" ]; then
    cp -R "$RESOURCES_DIR/sql-change" "$RESOURCES/"
    echo "SQL migrations copied"
fi

# Config example
if [ -f "$RESOURCES_DIR/config.yaml.example" ]; then
    cp "$RESOURCES_DIR/config.yaml.example" "$RESOURCES/"
fi

# VERSION
if [ -f "$RESOURCES_DIR/VERSION" ]; then
    cp "$RESOURCES_DIR/VERSION" "$RESOURCES/"
fi

# Installer config
if [ -f "$RESOURCES_DIR/installer-config.json" ]; then
    cp "$RESOURCES_DIR/installer-config.json" "$RESOURCES/"
fi

# Find and copy icons
ICONS_DIR="$SRC_TAURI_DIR/icons"
ICONS_CACHE="$SRC_TAURI_DIR/target/release/icons-cache"

for ICON_SOURCE in "$ICONS_DIR/AppIcon.icns" "$ICONS_CACHE/AppIcon.icns"; do
    if [ -f "$ICON_SOURCE" ]; then
        cp "$ICON_SOURCE" "$RESOURCES/AppIcon.icns"
        echo "Icon copied: $ICON_SOURCE"
        break
    fi
done

# Copy installer frontend dist (if exists)
INSTALLER_DIR="$PROJECT_ROOT/installer-tauri"
if [ -d "$INSTALLER_DIR/dist" ]; then
    cp -R "$INSTALLER_DIR/dist" "$RESOURCES/dist"
    echo "Installer frontend dist copied"
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

# Verify App Bundle structure
echo ""
echo "=== App Bundle Created ==="
echo "Path: $APP_BUNDLE_PATH"
echo ""
echo "Structure:"
echo "  Contents/"
echo "    MacOS/"
ls -la "$MACOS" | tail -n +2 | while read line; do
    echo "      $line"
done
echo "    Resources/"
ls -la "$RESOURCES" | tail -n +2 | while read line; do
    echo "      $line"
done
echo "    Info.plist ($(wc -c < "$CONTENTS/Info.plist") bytes)"
echo ""
echo "Usage: Drag $APP_BUNDLE_PATH to /Applications to install"