#!/bin/bash
# Colink Complete Build Script (Unix/Linux/macOS)

set -e

# Set mirrors for Chinese users (解决 GitHub 下载慢问题)
export ELECTRON_MIRROR="https://npmmirror.com/mirrors/electron/"
export ELECTRON_BUILDER_BINARIES_MIRROR="https://npmmirror.com/mirrors/electron-builder-binaries/"

echo "===== Colink Build Started ====="

# 0. Read version and generate full version with timestamp
VERSION="dev"
if [ -f "../VERSION" ]; then
    VERSION=$(cat "../VERSION" | tr -d '\n\r')
fi
BUILD_TIME=$(date +%Y%m%d-%H%M%S)

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    darwin) OS="darwin" ;;
    linux)  OS="linux" ;;
    mingw*|msys*|cygwin*) OS="windows" ;;
    *) OS="unknown" ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    i386|i686) ARCH="386" ;;
    armv7l) ARCH="arm" ;;
    *) ARCH="unknown" ;;
esac

FULL_VERSION="v$VERSION-$BUILD_TIME"
PACKAGE_NAME="Colink-$FULL_VERSION-$OS-$ARCH"
echo "Version: $FULL_VERSION"
echo "Platform: $OS-$ARCH"

# 1. Clean old artifacts
echo "[1/8] Cleaning old build artifacts..."
rm -rf ../bin/* 2>/dev/null || true
rm -rf release/*.zip 2>/dev/null || true

# 2. Generate plugin registry
echo "[2/8] Generating plugin registry..."
cd ..
go run ./tools/genplugins

# 3. Build backend
echo "[3/8] Building backend..."
go build -ldflags "-X main.Version=$FULL_VERSION" -o bin/colink-server.exe ./cmd/server

# 4. Build frontend (ensure dependencies first)
echo "[4/8] Building frontend..."
cd web
if [ ! -d "node_modules" ]; then
    echo "  Installing frontend dependencies..."
    npm install
fi
npm run build

# 4.1 Build desktop application
echo "[4.1/8] Building desktop application..."
cd ../apps/desktop
if [ ! -d "node_modules" ]; then
    echo "  Installing desktop dependencies..."
    npm install
fi
npm run build
# Package as portable directory
echo "  Packaging desktop app as portable..."
npx electron-builder --win --config electron-builder.portable.yml

# Copy packaged desktop app to installer packages
echo "  Copying desktop app to packages..."
cd ../installer
desktopReleaseDir="../apps/desktop/release/portable/win-unpacked"
packagesDesktopDir="packages/desktop"
rm -rf "$packagesDesktopDir" 2>/dev/null || true
if [ -d "$desktopReleaseDir" ]; then
    cp -r "$desktopReleaseDir" "$packagesDesktopDir"
    echo "  Desktop app copied to packages/desktop"
else
    echo "  WARNING: Portable desktop app not found at $desktopReleaseDir"
fi

# 5. Build installer
echo "[5/8] Building installer..."
npm install
npm run build

# 6. Package setup
echo "[6/8] Packaging setup..."
npm run package:setup

# 7. Create ZIP
echo "[7/8] Creating release package..."
export COLINK_FULL_VERSION=$FULL_VERSION
export COLINK_OS=$OS
export COLINK_ARCH=$ARCH
node scripts/create-zip.js

echo "===== Build Complete ====="
echo "Release: release/$PACKAGE_NAME.zip"