#!/usr/bin/env pwsh
# Colink 完整发布构建脚本
# 构建所有组件并打包为 ZIP
# 用法: ./build-release.ps1

$ErrorActionPreference = "Stop"

# Handle both native PowerShell and Git Bash invocation
if ($PSScriptRoot) {
    $ProjectRoot = Split-Path -Parent $PSScriptRoot
} else {
    # Fallback: find project root by VERSION file
    $currentDir = $PWD.Path
    $ProjectRoot = $null

    # Check current directory and its parents for VERSION file
    while ($currentDir -and -not (Test-Path "$currentDir\VERSION")) {
        $parentDir = Split-Path -Parent $currentDir
        if ($parentDir -eq $currentDir) { break }  # Reached root
        $currentDir = $parentDir
    }

    if (Test-Path "$currentDir\VERSION") {
        $ProjectRoot = $currentDir
    } else {
        Write-Host "ERROR: Cannot find project root (VERSION file not found)" -ForegroundColor Red
        Write-Host "Please run this script from the project directory or specify path explicitly" -ForegroundColor Red
        exit 1
    }
}

$InstallerDir = Join-Path $ProjectRoot "installer-tauri"
$SrcTauriDir = Join-Path $InstallerDir "src-tauri"

Write-Host "=== Colink Full Release Build ===" -ForegroundColor Cyan
Write-Host "Project: $ProjectRoot"

# 读取版本号 (使用 ReadAllText 避免 Git Bash 环境下 Get-Content 编码问题)
$VersionPath = Join-Path $ProjectRoot "VERSION"
$VERSION = [System.IO.File]::ReadAllText($VersionPath).Trim()
if (-not $VERSION) { $VERSION = "1.0.0" }
$BUILD_TIME = Get-Date -Format "yyyyMMdd-HHmmss"
Write-Host "Version: $VERSION" -ForegroundColor Green

# Step 0: Install dependencies if needed
Write-Host "`n[0/7] Checking dependencies..." -ForegroundColor Yellow

# Check web dependencies
$WebNodeModules = Join-Path $ProjectRoot "web/node_modules"
if (-not (Test-Path $WebNodeModules)) {
    Write-Host "Installing web dependencies..." -ForegroundColor Cyan
    Set-Location "$ProjectRoot/web"
    & npm install
    if (-not $?) {
        Write-Host "Web dependencies install failed" -ForegroundColor Red
        exit 1
    }
    Write-Host "Web dependencies installed" -ForegroundColor Green
} else {
    Write-Host "Web dependencies already installed" -ForegroundColor Green
}

# Check installer-tauri dependencies
$InstallerNodeModules = Join-Path $InstallerDir "node_modules"
if (-not (Test-Path $InstallerNodeModules)) {
    Write-Host "Installing installer-tauri dependencies..." -ForegroundColor Cyan
    Set-Location $InstallerDir
    & pnpm install
    if (-not $?) {
        Write-Host "Installer dependencies install failed" -ForegroundColor Red
        exit 1
    }
    Write-Host "Installer dependencies installed" -ForegroundColor Green
} else {
    Write-Host "Installer dependencies already installed" -ForegroundColor Green
}

# Step 1: Build ISDP backend (server + migrate)
Write-Host "`n[1/7] Building ISDP backend..." -ForegroundColor Yellow
Set-Location $ProjectRoot

# Build server
& go build -ldflags "-X main.Version=v$VERSION-$BUILD_TIME" -o bin/colink-server.exe ./cmd/server
if (-not $?) {
    Write-Host "Server build failed" -ForegroundColor Red
    exit 1
}
Write-Host "Server built: bin/colink-server.exe" -ForegroundColor Green

# Build migrate
& go build -o bin/migrate.exe ./cmd/migrate
if (-not $?) {
    Write-Host "Migrate build failed" -ForegroundColor Red
    exit 1
}
Write-Host "Migrate built: bin/migrate.exe" -ForegroundColor Green

# Step 2: Build ISDP frontend
Write-Host "`n[2/7] Building ISDP frontend..." -ForegroundColor Yellow
Set-Location "$ProjectRoot/web"
& npm run build
if (-not $?) {
    Write-Host "ISDP frontend build failed" -ForegroundColor Red
    exit 1
}
Write-Host "ISDP frontend built: web/dist/" -ForegroundColor Green

# Step 3: Sync resources to staging
Write-Host "`n[3/7] Syncing resources to staging..." -ForegroundColor Yellow
Set-Location $ProjectRoot
$StagingResources = Join-Path $SrcTauriDir "target/release/staging/resources"
& node scripts/sync-resources.js $StagingResources
Write-Host "Resources synced to staging" -ForegroundColor Green

# Step 4: Copy VERSION file
Write-Host "`n[4/7] Copying VERSION file..." -ForegroundColor Yellow
Copy-Item "$ProjectRoot/VERSION" $StagingResources -Force
Write-Host "VERSION copied" -ForegroundColor Green

# Step 5: Build installer-tauri frontend renderer
Write-Host "`n[5/7] Building installer frontend..." -ForegroundColor Yellow
Set-Location $InstallerDir
& pnpm build:renderer
if (-not $?) {
    Write-Host "Installer frontend build failed" -ForegroundColor Red
    exit 1
}
Write-Host "Installer frontend built" -ForegroundColor Green

# Step 5.5: Generate icons from source image
Write-Host "`n[5.5/7] Generating icons..." -ForegroundColor Yellow
Set-Location $InstallerDir
$IconSource = Join-Path $SrcTauriDir "icons/icon.png"
$IconsDir = Join-Path $SrcTauriDir "icons"
$IconsCacheDir = Join-Path $SrcTauriDir "target/release/icons-cache"

# Clean icons directory before generation - move generated icons to cache
if (Test-Path $IconsDir) {
    # Create cache directory
    if (-not (Test-Path $IconsCacheDir)) {
        New-Item -ItemType Directory -Path $IconsCacheDir -Force | Out-Null
    }
    # Move all generated icons and directories to cache (keep only icon.png)
    Get-ChildItem $IconsDir | Where-Object { $_.Name -ne "icon.png" } | ForEach-Object {
        $destPath = Join-Path $IconsCacheDir $_.Name
        if ($_ -is [System.IO.DirectoryInfo]) {
            # For directories, remove existing in cache first, then move
            if (Test-Path $destPath) { Remove-Item $destPath -Recurse -Force }
        }
        Move-Item $_.FullName $IconsCacheDir -Force -ErrorAction SilentlyContinue
    }
}

if (Test-Path $IconSource) {
    # tauri icon outputs progress to stderr, temporarily disable error stop
    $PrevErrorAction = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $output = & pnpm tauri icon $IconSource 2>&1 | Out-Null
    $ErrorActionPreference = $PrevErrorAction
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Icons generated" -ForegroundColor Green
    } else {
        Write-Host "Icon generation failed (continuing with existing icons)" -ForegroundColor Yellow
    }
} else {
    Write-Host "Icon source not found: $IconSource (using existing icons)" -ForegroundColor Yellow
}

# Step 6: Build exe (both Setup and Launcher use same binary)
Write-Host "`n[6/7] Building exe..." -ForegroundColor Yellow
Set-Location $SrcTauriDir

# Build release exe
& cargo build --release
if (-not $?) {
    Write-Host "Build failed" -ForegroundColor Red
    exit 1
}

$BuiltExe = Join-Path $SrcTauriDir "target/release/colink-installer.exe"

# Create Launcher (Colink.exe) - same binary, different name for mode detection
$ColinkExe = Join-Path $SrcTauriDir "target/release/Colink.exe"
Copy-Item $BuiltExe $ColinkExe -Force
Write-Host "Launcher created: target/release/Colink.exe" -ForegroundColor Green

# Copy launcher to staging resources/launcher
$LauncherDestDir = Join-Path $StagingResources "launcher"
if (-not (Test-Path $LauncherDestDir)) {
    New-Item -ItemType Directory -Path $LauncherDestDir -Force | Out-Null
}
Copy-Item $ColinkExe (Join-Path $LauncherDestDir "Colink.exe") -Force
Write-Host "Launcher copied to staging" -ForegroundColor Green

# Step 7: Create ZIP package
Write-Host "`n[7/7] Creating ZIP package..." -ForegroundColor Yellow

# Create staging directory structure for ZIP
$TargetDir = Join-Path $SrcTauriDir "target/release"
$StagingDir = Join-Path $TargetDir "staging"
$ExeDir = Join-Path $StagingDir "exe"

# Ensure exe directory exists
if (-not (Test-Path $ExeDir)) {
    New-Item -ItemType Directory -Path $ExeDir -Force | Out-Null
}

# Copy Setup exe (colink-installer.exe renamed to Colink-Setup.exe)
$SetupSrc = Join-Path $TargetDir "colink-installer.exe"
$SetupDest = Join-Path $ExeDir "Colink-Setup.exe"
Copy-Item $SetupSrc $SetupDest -Force
Write-Host "Setup exe copied to staging" -ForegroundColor Green

# Copy installer frontend dist to staging root (for ../dist resolution)
$DistSrc = Join-Path $InstallerDir "dist"
$DistDest = Join-Path $StagingDir "dist"
if (Test-Path $DistDest) {
    Remove-Item $DistDest -Recurse -Force
}
Copy-Item -Path $DistSrc -Destination $DistDest -Recurse -Force
Write-Host "Installer frontend dist copied" -ForegroundColor Green

# Create Start-Setup.bat
$StartBat = Join-Path $StagingDir "Start-Setup.bat"
$BatContent = @"
@echo off
cd exe
start "" Colink-Setup.exe
"@
Set-Content -Path $StartBat -Value $BatContent -Encoding ASCII

# Create README.txt (UTF-8 without BOM for proper Chinese display)
$ReadmePath = Join-Path $StagingDir "README.txt"
$utf8NoBom = New-Object System.Text.UTF8Encoding $False
$writer = [System.IO.StreamWriter]::new($ReadmePath, $false, $utf8NoBom)
$writer.WriteLine("Colink Setup v$VERSION 安装说明")
$writer.WriteLine()
$writer.WriteLine("=== 使用方式 ===")
$writer.WriteLine()
$writer.WriteLine("方式一：双击 Start-Setup.bat 启动（推荐）")
$writer.WriteLine()
$writer.WriteLine("方式二：进入 exe 目录，双击 Colink-Setup.exe")
$writer.WriteLine()
$writer.WriteLine("=== 目录结构 ===")
$writer.WriteLine()
$writer.WriteLine("dist/             前端资源（Tauri 依赖）")
$writer.WriteLine("exe/              安装程序目录")
$writer.WriteLine("  Colink-Setup.exe    安装程序")
$writer.WriteLine("  resources/          后端资源和配置")
$writer.WriteLine("    launcher/         启动器程序")
$writer.WriteLine("      Colink.exe")
$writer.WriteLine()
$writer.WriteLine("=== 安装后 ===")
$writer.WriteLine()
$writer.WriteLine("安装完成后，可通过桌面快捷方式启动 Colink。")
$writer.WriteLine("已安装用户可通过 Launcher 控制服务启停。")
$writer.Close()

# Create ZIP
$OutputDir = Join-Path $TargetDir "dist"
if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
}

$ZipName = "Colink-Setup-$VERSION-$BUILD_TIME.zip"
$ZipPath = Join-Path $OutputDir $ZipName

# Remove old zip if exists
if (Test-Path $ZipPath) {
    Remove-Item $ZipPath -Force
}

Write-Host "Creating ZIP: $ZipPath" -ForegroundColor Yellow
Compress-Archive -Path "$StagingDir/*" -DestinationPath $ZipPath -CompressionLevel Optimal

# Get zip size
$ZipSize = (Get-Item $ZipPath).Length / 1MB
$ZipSizeStr = "{0:N2} MB" -f $ZipSize

Write-Host "`n=== Build Complete ===" -ForegroundColor Cyan
Write-Host "Output: $ZipPath" -ForegroundColor Green
Write-Host "Size: $ZipSizeStr" -ForegroundColor Green
Write-Host "`nZIP Contents:" -ForegroundColor Yellow
Write-Host "  - Start-Setup.bat" -ForegroundColor White
Write-Host "  - README.txt" -ForegroundColor White
Write-Host "  - dist/            (installer frontend)" -ForegroundColor White
Write-Host "  - exe/" -ForegroundColor White
Write-Host "    - Colink-Setup.exe" -ForegroundColor Gray
Write-Host "    - resources/" -ForegroundColor Gray
Write-Host "      - colink-server.exe" -ForegroundColor DarkGray
Write-Host "      - web/          (isdp frontend)" -ForegroundColor DarkGray
Write-Host "      - launcher/" -ForegroundColor DarkGray
Write-Host "        - Colink.exe" -ForegroundColor DarkGray
Write-Host "`nUsage: Unzip and run exe/Colink-Setup.exe or Start-Setup.bat" -ForegroundColor Yellow

# Clean icons directory after build - move generated icons to cache to avoid polluting source
$IconsDir = Join-Path $SrcTauriDir "icons"
$IconsCacheDir = Join-Path $SrcTauriDir "target/release/icons-cache"
if (Test-Path $IconsDir) {
    # Ensure cache directory exists
    if (-not (Test-Path $IconsCacheDir)) {
        New-Item -ItemType Directory -Path $IconsCacheDir -Force | Out-Null
    }
    # Move all generated icons and directories to cache (keep only icon.png in source)
    Get-ChildItem $IconsDir | Where-Object { $_.Name -ne "icon.png" } | ForEach-Object {
        $destPath = Join-Path $IconsCacheDir $_.Name
        if ($_ -is [System.IO.DirectoryInfo]) {
            # For directories, remove existing in cache first, then move
            if (Test-Path $destPath) { Remove-Item $destPath -Recurse -Force }
        }
        Move-Item $_.FullName $IconsCacheDir -Force -ErrorAction SilentlyContinue
    }
    Write-Host "Icons cleaned (moved to target/release/icons-cache)" -ForegroundColor Green
}