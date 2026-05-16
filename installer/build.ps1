# Colink Complete Build Script (Windows PowerShell)
# Optimized with parallel execution

$ErrorActionPreference = "Stop"

# 确定项目根目录（build.ps1在installer子目录）
$scriptPath = $MyInvocation.MyCommand.Path
$installerDir = Split-Path -Parent $scriptPath
$projectRoot = Split-Path -Parent $installerDir

Write-Host "Project root: $projectRoot" -ForegroundColor Gray
Write-Host "Installer dir: $installerDir" -ForegroundColor Gray

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

# Set mirrors for Chinese users
$env:ELECTRON_MIRROR = "https://npmmirror.com/mirrors/electron/"
$env:ELECTRON_BUILDER_BINARIES_MIRROR = "https://npmmirror.com/mirrors/electron-builder-binaries/"

# Fix winCodeSign cache (avoid symlink errors on Windows)
$winCodeSignCache = "$env:LOCALAPPDATA\electron-builder\Cache\winCodeSign"
$winCodeSignDir = "$winCodeSignCache\winCodeSign-2.6.0"
$rceditPath = "$winCodeSignDir\rcedit-x64.exe"
if (-not (Test-Path $rceditPath)) {
    Write-Host "[Pre-cache] Setting up winCodeSign cache..." -ForegroundColor Yellow
    if (Test-Path $winCodeSignCache) { Remove-Item $winCodeSignCache -Force -Recurse }
    New-Item -ItemType Directory -Path $winCodeSignDir -Force | Out-Null
    $winCodeSignUrl = "https://npmmirror.com/mirrors/electron-builder-binaries/winCodeSign-2.6.0/winCodeSign-2.6.0.7z"
    $winCodeSign7z = "$winCodeSignCache\winCodeSign.7z"
    # Use curl.exe for download
    curl.exe -L -o $winCodeSign7z $winCodeSignUrl
    if (-not (Test-Path $winCodeSign7z)) {
        Write-Host "[Pre-cache] Download failed, skipping pre-cache setup" -ForegroundColor Red
    } else {
        # Extract with 7z - use cmd.exe to avoid PowerShell stderr handling issues
        $sevenZipPath = Join-Path $projectRoot "apps\desktop\node_modules\7zip-bin\win\x64\7za.exe"
        cmd.exe /c "`"$sevenZipPath`" x `"$winCodeSign7z`" -o`"$winCodeSignDir`" -y 2>nul"
        # Remove problematic darwin and linux directories (not needed for Windows builds)
        if (Test-Path "$winCodeSignDir\darwin") { Remove-Item "$winCodeSignDir\darwin" -Force -Recurse -ErrorAction SilentlyContinue }
        if (Test-Path "$winCodeSignDir\linux") { Remove-Item "$winCodeSignDir\linux" -Force -Recurse -ErrorAction SilentlyContinue }
        Remove-Item $winCodeSign7z -Force -ErrorAction SilentlyContinue
        if (Test-Path $rceditPath) {
            Write-Host "[Pre-cache] winCodeSign cache ready" -ForegroundColor Green
        } else {
            Write-Host "[Pre-cache] winCodeSign extraction incomplete, will retry during build" -ForegroundColor Yellow
        }
    }
}

Write-Host "===== Colink Build Started (Parallel) =====" -ForegroundColor Green

# 0. Read version
$VERSION = "dev"
if (Test-Path "$projectRoot\VERSION") {
    $VERSION = (Get-Content "$projectRoot\VERSION" -Raw).Trim()
}
$BUILD_TIME = Get-Date -Format "yyyyMMdd-HHmmss"
$FULL_VERSION = "v$VERSION-$BUILD_TIME"
$PACKAGE_NAME = "Colink-$FULL_VERSION-windows-amd64"
Write-Host "Version: $FULL_VERSION" -ForegroundColor Cyan

# 1. Clean old artifacts (fail loudly, not silently)
Write-Host "[1/8] Cleaning old build artifacts..." -ForegroundColor Cyan
$cleanupFailed = 0

if (Test-Path "$projectRoot\bin") {
    try { Remove-Item "$projectRoot\bin" -Force -Recurse -ErrorAction Stop }
    catch { Write-Host "  WARNING: bin directory locked, files may be stale" -ForegroundColor Yellow; $cleanupFailed++ }
}
if (Test-Path "$projectRoot\apps\desktop\resources\bin") {
    try { Remove-Item "$projectRoot\apps\desktop\resources\bin" -Force -Recurse -ErrorAction Stop }
    catch { Write-Host "  WARNING: apps/desktop/resources/bin locked, may contain OLD binaries!" -ForegroundColor Red; $cleanupFailed++ }
}
if (Test-Path "$projectRoot\apps\desktop\resources\web") {
    try { Remove-Item "$projectRoot\apps\desktop\resources\web" -Force -Recurse -ErrorAction Stop }
    catch { Write-Host "  WARNING: apps/desktop/resources/web locked" -ForegroundColor Yellow; $cleanupFailed++ }
}
if (Test-Path "$projectRoot\apps\desktop\release") {
    try { Remove-Item "$projectRoot\apps\desktop\release" -Force -Recurse -ErrorAction Stop }
    catch { Write-Host "  WARNING: apps/desktop/release locked (electron-builder output)" -ForegroundColor Yellow; $cleanupFailed++ }
}
try { Remove-Item "$installerDir\release\*.zip" -Force -ErrorAction Stop } catch {}
try { Remove-Item "$installerDir\release\win-unpacked" -Force -Recurse -ErrorAction Stop } catch {}
try { Remove-Item "$installerDir\packages" -Force -Recurse -ErrorAction Stop } catch {}

if ($cleanupFailed -gt 0) {
    Write-Host "  Cleanup failed for $cleanupFailed critical directories!" -ForegroundColor Red
    Write-Host "  Please ensure no Colink processes are running and retry" -ForegroundColor Red
    exit 1
}
Write-Host "  Cleanup complete" -ForegroundColor Green

# 2. Generate plugin registry
Write-Host "[2/8] Generating plugin registry..." -ForegroundColor Cyan
Push-Location $projectRoot
go run ./tools/genplugins
Pop-Location

# Create directories
New-Item -ItemType Directory -Path "$projectRoot\bin" -Force | Out-Null
New-Item -ItemType Directory -Path "$projectRoot\bin\windows-amd64" -Force | Out-Null
New-Item -ItemType Directory -Path "$installerDir\packages\runtime\tools" -Force | Out-Null
New-Item -ItemType Directory -Path "$installerDir\packages\runtime\data\configs" -Force | Out-Null

# 3. Backend build (parallel with frontend prep)
Write-Host "[3/8] Building backend..." -ForegroundColor Cyan

$backendJob = Start-Job -ScriptBlock {
    param($projectRoot, $FULL_VERSION)
    Set-Location $projectRoot
    $env:PATH = $env:PATH + ";C:\Program Files\Go\bin"
    go build -ldflags "-X main.Version=$FULL_VERSION" -o "bin\colink-server.exe" "./cmd/server"
    # Copy to platform dir, handle locked files
    try {
        Copy-Item "bin\colink-server.exe" "bin\windows-amd64\colink-server.exe" -Force -ErrorAction Stop
    } catch {
        Write-Host "  windows-amd64 copy skipped (file locked)" -ForegroundColor Yellow
    }
    go build -o "bin\migrate.exe" "./cmd/migrate"
    go build -o "bin\mcp-server.exe" "./cmd/mcp-server"
} -ArgumentList $projectRoot, $FULL_VERSION

# Frontend build in main process (avoid job environment issues)
Write-Host "[3.1/8] Building frontend..." -ForegroundColor Cyan
Push-Location "$projectRoot\web"
if (-not (Test-Path "node_modules")) { npm install }
npm run build
if ($LASTEXITCODE -ne 0) {
    Write-Host "Frontend build failed with exit code $LASTEXITCODE" -ForegroundColor Red
    Pop-Location
    exit 1
}
Pop-Location
Write-Host "  Frontend complete" -ForegroundColor Green

# Wait for backend
Write-Host "  Waiting for backend..." -ForegroundColor Gray
$backendResult = Wait-Job $backendJob | Receive-Job
Remove-Job $backendJob

# Wait for file handles to release (PowerShell job process may still hold handles)
Write-Host "  Waiting for file handles to release..." -ForegroundColor Gray
$serverExe = "$projectRoot\bin\colink-server.exe"
$maxWaitSeconds = 10  # seconds
$waitedSeconds = 0
while ($waitedSeconds -lt $maxWaitSeconds) {
    try {
        # Try to open file for writing - will fail if locked
        $fs = [System.IO.File]::OpenWrite($serverExe)
        $fs.Close()
        $fs.Dispose()
        break  # File is accessible
    } catch {
        Start-Sleep -Milliseconds 500
        $waitedSeconds += 0.5
    }
}
if ($waitedSeconds -ge $maxWaitSeconds) {
    Write-Host "  WARNING: $serverExe still locked after $maxWaitSeconds seconds" -ForegroundColor Yellow
}
Write-Host "  Backend complete" -ForegroundColor Green

# Copy migrate tool (bundle-server.mjs handles colink-server copy to desktop)
Write-Host "[3.2/8] Copying migrate tool..." -ForegroundColor Cyan
Copy-Item "$projectRoot\bin\migrate.exe" "$installerDir\packages\runtime\tools\migrate.exe" -Force

# Copy runtime packages (can run while frontend builds)
Write-Host "[3.3/8] Copying runtime packages..." -ForegroundColor Cyan
$sqlChangeSrc = "$projectRoot\sql-change"
$sqlChangeDest = "$installerDir\packages\runtime\data\sql-change"
if (Test-Path $sqlChangeSrc) {
    New-Item -ItemType Directory -Path $sqlChangeDest -Force | Out-Null
    Get-ChildItem $sqlChangeSrc -Directory | Where-Object { $_.Name -match '^v\d+\.\d+' } | ForEach-Object {
        Copy-Item $_.FullName "$sqlChangeDest\$($_.Name)" -Force -Recurse
    }
}
Copy-Item "$projectRoot\VERSION" "$installerDir\packages\VERSION" -Force
Copy-Item "$projectRoot\configs\config.yaml.example" "$installerDir\packages\runtime\data\configs\config.yaml.example" -Force

Copy-Item "$projectRoot\VERSION" "$installerDir\packages\VERSION" -Force
Copy-Item "$projectRoot\configs\config.yaml.example" "$installerDir\packages\runtime\data\configs\config.yaml.example" -Force

# 4. Build desktop application
Write-Host "[4/8] Building desktop application..." -ForegroundColor Cyan
Push-Location "$projectRoot\apps\desktop"
if (-not (Test-Path "node_modules")) { npm install }
npm run build
if ($LASTEXITCODE -ne 0) {
    Write-Host "Desktop build failed with exit code $LASTEXITCODE" -ForegroundColor Red
    Pop-Location
    exit 1
}
npx electron-builder --win --config electron-builder.portable.yml
Pop-Location

# Copy desktop to packages
Write-Host "[4.1/8] Copying desktop app..." -ForegroundColor Cyan
$desktopReleaseDir = "$projectRoot\apps\desktop\release\portable\win-unpacked"
$packagesDesktopDir = "$installerDir\packages\desktop"
if (Test-Path $packagesDesktopDir) { Remove-Item $packagesDesktopDir -Force -Recurse }
New-Item -ItemType Directory -Path $packagesDesktopDir -Force | Out-Null
Copy-Item "$desktopReleaseDir\*" $packagesDesktopDir -Force -Recurse

# 5-6. Build installer
Write-Host "[5/8] Building installer..." -ForegroundColor Cyan
Push-Location $installerDir
npm install
npm run build
if ($LASTEXITCODE -ne 0) {
    Write-Host "Installer build failed with exit code $LASTEXITCODE" -ForegroundColor Red
    Pop-Location
    exit 1
}
npm run package:setup
Pop-Location

# 7. Create ZIP using PowerShell (handles timezone correctly)
Write-Host "[7/8] Creating release package..." -ForegroundColor Cyan
$env:COLINK_FULL_VERSION = $FULL_VERSION
$env:COLINK_OS = "windows"
$env:COLINK_ARCH = "amd64"

$distDir = "$installerDir\release\win-unpacked"
$outputZip = "$installerDir\release\$PACKAGE_NAME.zip"

# Use PowerShell Compress-Archive for correct timezone handling
if (Test-Path $outputZip) { Remove-Item $outputZip -Force }
Compress-Archive -Path "$distDir\*" -DestinationPath $outputZip -CompressionLevel Optimal

Write-Host "Created: $outputZip" -ForegroundColor Green

Write-Host "===== Build Complete =====" -ForegroundColor Green
Write-Host "Release: $installerDir\release\$PACKAGE_NAME.zip" -ForegroundColor Yellow