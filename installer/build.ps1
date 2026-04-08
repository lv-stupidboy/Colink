# Colink Complete Build Script (Windows PowerShell)

$ErrorActionPreference = "Stop"

# Set console encoding to UTF-8
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

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
Write-Host "[1/6] Cleaning old build artifacts..." -ForegroundColor Cyan
Remove-Item -Path "..\bin\*" -Force -ErrorAction SilentlyContinue
Remove-Item -Path "release\*.zip" -Force -ErrorAction SilentlyContinue

# 2. Build backend
Write-Host "[2/6] Building backend..." -ForegroundColor Cyan
Push-Location ..
go build -ldflags "-X main.Version=$FULL_VERSION" -o bin\colink-server.exe .\cmd\server
Pop-Location

# 3. Build frontend (ensure dependencies first)
Write-Host "[3/6] Building frontend..." -ForegroundColor Cyan
Push-Location ..\web
if (-not (Test-Path "node_modules")) {
    Write-Host "  Installing frontend dependencies..." -ForegroundColor Yellow
    npm install
}
npm run build
Pop-Location

# 4. Build installer
Write-Host "[4/6] Building installer..." -ForegroundColor Cyan
npm install
npm run build

# 5. Package launcher
Write-Host "[5/6] Packaging launcher..." -ForegroundColor Cyan
npm run package:launcher

# 6. Package setup
Write-Host "[6/6] Packaging setup..." -ForegroundColor Cyan
npm run package:setup

# 7. Create ZIP
Write-Host "[7/7] Creating release package..." -ForegroundColor Cyan
$env:COLINK_FULL_VERSION = $FULL_VERSION
$env:COLINK_OS = $OS
$env:COLINK_ARCH = $ARCH
node scripts\create-zip.js

Write-Host "===== Build Complete =====" -ForegroundColor Green
Write-Host "Release: release\$PACKAGE_NAME.zip" -ForegroundColor Yellow