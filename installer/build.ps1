# Colink Complete Build Script (Windows PowerShell)

$ErrorActionPreference = "Stop"

# Set console encoding to UTF-8
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

# Set mirrors for Chinese users (解决 GitHub 下载慢问题)
$env:ELECTRON_MIRROR = "https://npmmirror.com/mirrors/electron/"
$env:ELECTRON_BUILDER_BINARIES_MIRROR = "https://npmmirror.com/mirrors/electron-builder-binaries/"

Write-Host "===== Colink Build Started =====" -ForegroundColor Green

# 0. Read version and generate full version with timestamp
$VERSION = "dev"
if (Test-Path "..\VERSION") {
    $VERSION = (Get-Content "..\VERSION" -Raw).Trim()
}
$BUILD_TIME = Get-Date -Format "yyyyMMdd-HHmmss"

# Detect platform and architecture
$OS = "windows"
# 检测操作系统架构，而非当前进程架构
if ([Environment]::Is64BitOperatingSystem) {
    $ARCH = "amd64"
} else {
    $ARCH = "386"
}

$FULL_VERSION = "v$VERSION-$BUILD_TIME"
$PACKAGE_NAME = "Colink-$FULL_VERSION-$OS-$ARCH"
Write-Host "Version: $FULL_VERSION" -ForegroundColor Cyan
Write-Host "Platform: $OS-$ARCH" -ForegroundColor Cyan

# 1. Clean old artifacts
Write-Host "[1/8] Cleaning old build artifacts..." -ForegroundColor Cyan
Remove-Item -Path "..\bin\*" -Force -ErrorAction SilentlyContinue
Remove-Item -Path "release\*.zip" -Force -ErrorAction SilentlyContinue

# 2. Generate plugin registry
Write-Host "[2/8] Generating plugin registry..." -ForegroundColor Cyan
Push-Location ..
go run ./tools/genplugins
Pop-Location

# 3. Build backend
Write-Host "[3/8] Building backend..." -ForegroundColor Cyan
Push-Location ..
go build -ldflags "-X main.Version=$FULL_VERSION" -o bin\colink-server.exe .\cmd\server
Pop-Location

# 3.1 Build migrate tool
Write-Host "[3.1/8] Building migrate tool..." -ForegroundColor Cyan
Push-Location ..
go build -o bin\migrate.exe .\cmd\migrate
Pop-Location

# 4. Build frontend (ensure dependencies first)
Write-Host "[4/8] Building frontend..." -ForegroundColor Cyan
Push-Location ..\web
if (-not (Test-Path "node_modules")) {
    Write-Host "  Installing frontend dependencies..." -ForegroundColor Yellow
    npm install
}
npm run build
Pop-Location

# 5. Build installer
Write-Host "[5/8] Building installer..." -ForegroundColor Cyan
npm install
npm run build

# 6. Package launcher
Write-Host "[6/8] Packaging launcher..." -ForegroundColor Cyan
npm run package:launcher

# 7. Package setup
Write-Host "[7/8] Packaging setup..." -ForegroundColor Cyan
npm run package:setup

# 8. Create ZIP
Write-Host "[8/8] Creating release package..." -ForegroundColor Cyan
$env:COLINK_FULL_VERSION = $FULL_VERSION
$env:COLINK_OS = $OS
$env:COLINK_ARCH = $ARCH
node scripts\create-zip.js

Write-Host "===== Build Complete =====" -ForegroundColor Green
Write-Host "Release: release\$PACKAGE_NAME.zip" -ForegroundColor Yellow