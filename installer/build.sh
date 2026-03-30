#!/bin/bash
# ISDP Complete Build Script (Unix/Linux/macOS)

set -e

echo "===== ISDP Build Started ====="

# 0. Read version and generate full version with timestamp
VERSION="dev"
if [ -f "../isdp/VERSION" ]; then
    VERSION=$(cat "../isdp/VERSION" | tr -d '\n\r')
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
PACKAGE_NAME="ISDP-$FULL_VERSION-$OS-$ARCH"
echo "Version: $FULL_VERSION"
echo "Platform: $OS-$ARCH"

# 1. Clean old artifacts
echo "[1/6] Cleaning old build artifacts..."
rm -rf ../isdp/bin/* 2>/dev/null || true
rm -rf release/*.zip 2>/dev/null || true

# 2. Build backend
echo "[2/6] Building backend..."
cd ../isdp
go build -ldflags "-X main.Version=$FULL_VERSION" -o bin/isdp-server.exe ./cmd/server

# 3. Build frontend
echo "[3/6] Building frontend..."
cd web
npm run build

# 4. Build installer
echo "[4/6] Building installer..."
cd ../../installer
npm install
npm run build

# 5. Package launcher
echo "[5/6] Packaging launcher..."
npm run package:launcher

# 6. Package setup
echo "[6/6] Packaging setup..."
npm run package:setup

# 7. Create ZIP
echo "[7/7] Creating release package..."
export ISDP_FULL_VERSION=$FULL_VERSION
export ISDP_OS=$OS
export ISDP_ARCH=$ARCH
node scripts/create-zip.js

echo "===== Build Complete ====="
echo "Release: release/$PACKAGE_NAME.zip"