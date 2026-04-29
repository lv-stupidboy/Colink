# Colink Complete Build Script (Windows PowerShell)

$ErrorActionPreference = "Stop"

# Set console encoding to UTF-8
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

# Set Go path if not in PATH
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    $goPath = "C:\Program Files\Go\bin"
    if (Test-Path "$goPath\go.exe") {
        $env:PATH = "$goPath;$env:PATH"
        Write-Host "Added Go to PATH: $goPath" -ForegroundColor Yellow
    }
}

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
try {
    if (Test-Path "..\bin") {
        Get-ChildItem -Path "..\bin" -Force | ForEach-Object { Remove-Item $_.FullName -Force -Recurse -ErrorAction SilentlyContinue }
    }
} catch {}
try { Remove-Item -Path "release\*.zip" -Force -ErrorAction SilentlyContinue } catch {}
try { Remove-Item -Path "release\Colink-*" -Force -Recurse -ErrorAction SilentlyContinue } catch {}
try { Remove-Item -Path "release\launcher" -Force -Recurse -ErrorAction SilentlyContinue } catch {}
try { Remove-Item -Path "release\win-unpacked" -Force -Recurse -ErrorAction SilentlyContinue } catch {}
try { Remove-Item -Path "packages\desktop" -Force -Recurse -ErrorAction SilentlyContinue } catch {}

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

# 4.1 Build desktop application
Write-Host "[4.1/8] Building desktop application..." -ForegroundColor Cyan
Push-Location ..\apps\desktop
if (-not (Test-Path "node_modules")) {
    Write-Host "  Installing desktop dependencies..." -ForegroundColor Yellow
    npm install
}
npm run build
# Package as portable directory
Write-Host "  Packaging desktop app as portable..." -ForegroundColor Yellow
npx electron-builder --win --config electron-builder.portable.yml
Pop-Location

# Copy packaged desktop app to installer packages
Write-Host "  Copying desktop app to packages..." -ForegroundColor Yellow
$desktopReleaseDir = Join-Path $PWD "..\apps\desktop\release\portable\win-unpacked"
$packagesDesktopDir = Join-Path $PWD "packages\desktop"
if (Test-Path $packagesDesktopDir) {
    Remove-Item -Path $packagesDesktopDir -Force -Recurse -ErrorAction SilentlyContinue
}
if (Test-Path $desktopReleaseDir) {
    Write-Host "  Source: $desktopReleaseDir" -ForegroundColor Gray
    Write-Host "  Dest: $packagesDesktopDir" -ForegroundColor Gray
    # Create destination directory first
    New-Item -ItemType Directory -Path $packagesDesktopDir -Force | Out-Null
    # Copy all files and directories
    Get-ChildItem -Path $desktopReleaseDir | Copy-Item -Destination $packagesDesktopDir -Force -Recurse
    Write-Host "  Desktop app copied to packages/desktop" -ForegroundColor Green
} else {
    Write-Host "  WARNING: Portable desktop app not found at $desktopReleaseDir" -ForegroundColor Yellow
}

# 5. Build installer
Write-Host "[5/8] Building installer..." -ForegroundColor Cyan
npm install
npm run build

# 6. Package setup
Write-Host "[6/8] Packaging setup..." -ForegroundColor Cyan
npm run package:setup

# 7. Create ZIP
Write-Host "[7/8] Creating release package..." -ForegroundColor Cyan
$env:COLINK_FULL_VERSION = $FULL_VERSION
$env:COLINK_OS = $OS
$env:COLINK_ARCH = $ARCH
node scripts\create-zip.js

Write-Host "===== Build Complete =====" -ForegroundColor Green
Write-Host "Release: release\$PACKAGE_NAME.zip" -ForegroundColor Yellow