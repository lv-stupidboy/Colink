# Colink Fast Build Script (Windows PowerShell)
# Optimized with incremental build detection and caching

$ErrorActionPreference = "Stop"

# 确定项目根目录（兼容直接运行和调用运行）
if ($MyInvocation.MyCommand.Path) {
    $scriptPath = $MyInvocation.MyCommand.Path
} elseif ($PSScriptRoot) {
    $scriptPath = Join-Path $PSScriptRoot "build-fast.ps1"
} else {
    $scriptPath = Join-Path $PWD.Path "build-fast.ps1"
}

$installerDir = Split-Path -Parent $scriptPath
$projectRoot = Split-Path -Parent $installerDir
$cacheDir = Join-Path $projectRoot ".build-cache"

Write-Host "Project root: $projectRoot" -ForegroundColor Gray
Write-Host "Cache dir: $cacheDir" -ForegroundColor Gray

# 创建缓存目录
if (-not (Test-Path $cacheDir)) {
    New-Item -ItemType Directory -Path $cacheDir -Force | Out-Null
}

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

# 开发模式标志（跳过部分优化步骤）
$devBuild = $env:COLINK_DEV_BUILD -eq "true"
if ($devBuild) {
    Write-Host "===== DEV BUILD MODE (Fast) =====" -ForegroundColor Yellow
}

Write-Host "===== Colink Fast Build Started =====" -ForegroundColor Green

# 0. Read version
$VERSION = "dev"
if (Test-Path "$projectRoot\VERSION") {
    $VERSION = (Get-Content "$projectRoot\VERSION" -Raw).Trim()
}
$BUILD_TIME = Get-Date -Format "yyyyMMdd-HHmmss"
$FULL_VERSION = "v$VERSION-$BUILD_TIME"
$PACKAGE_NAME = "Colink-$FULL_VERSION-windows-amd64"
Write-Host "Version: $FULL_VERSION" -ForegroundColor Cyan

# ===== 辅助函数 =====

function NeedsRebuild($targetDir, $sourceDir, $cacheFile) {
    if (-not (Test-Path $targetDir)) {
        Write-Host "  Target not found: $targetDir" -ForegroundColor Gray
        return true
    }

    # 计算源文件 hash（排除 lock 文件）
    $sourceFiles = Get-ChildItem $sourceDir -Recurse -File -ErrorAction SilentlyContinue |
                   Where-Object { $_.Name -notmatch "package-lock\.json|\.lock|node_modules" }

    if (-not $sourceFiles) {
        Write-Host "  No source files found" -ForegroundColor Gray
        return false
    }

    $currentHash = ($sourceFiles | ForEach-Object {
        Get-FileHash $_.FullName -Algorithm MD5 -ErrorAction SilentlyContinue
    } | Select-Object -ExpandProperty Hash | Sort-Object) -join '|'

    # 对比缓存 hash
    if (Test-Path $cacheFile) {
        $cachedHash = Get-Content $cacheFile -ErrorAction SilentlyContinue
        if ($currentHash -eq $cachedHash) {
            Write-Host "  Source unchanged (cached)" -ForegroundColor Green
            return false
        }
    }

    # 保存 hash
    $currentHash | Out-File $cacheFile -Encoding UTF8 -Force
    Write-Host "  Source changed, rebuild needed" -ForegroundColor Yellow
    return true
}

function NeedsNpmInstall($dir) {
    $lockFile = Join-Path $dir "package-lock.json"
    $modulesDir = Join-Path $dir "node_modules"
    $hashFile = Join-Path $cacheDir "npm-$($dir.GetHashCode()).hash"

    if (-not (Test-Path $lockFile)) { return false }
    if (-not (Test-Path $modulesDir)) {
        Write-Host "  node_modules not found: $dir" -ForegroundColor Yellow
        return true
    }

    $currentHash = Get-FileHash $lockFile -Algorithm MD5 | Select-Object -ExpandProperty Hash
    if (Test-Path $hashFile) {
        $cachedHash = Get-Content $hashFile -ErrorAction SilentlyContinue
        if ($currentHash -eq $cachedHash) {
            Write-Host "  npm packages unchanged: $dir" -ForegroundColor Green
            return false
        }
    }

    $currentHash | Out-File $hashFile -Encoding UTF8 -Force
    Write-Host "  package-lock changed, reinstall needed: $dir" -ForegroundColor Yellow
    return true
}

function PreCacheElectronBuilder() {
    Write-Host "[Pre-cache] Checking electron-builder dependencies..." -ForegroundColor Cyan

    $electronCache = "$env:LOCALAPPDATA\electron-builder\Cache"
    $cacheItems = @{
        "electron\19.1.9" = "https://npmmirror.com/mirrors/electron/19.1.9/electron-v19.1.9-win32-x64.zip"
        "winCodeSign\winCodeSign-2.6.0" = "https://npmmirror.com/mirrors/electron-builder-binaries/winCodeSign-2.6.0/winCodeSign-2.6.0.7z"
        "nsis\nsis-3.0.4.1" = "https://npmmirror.com/mirrors/electron-builder-binaries/nsis-3.0.4.1/nsis-3.0.4.1.7z"
    }

    foreach ($item in $cacheItems.GetEnumerator()) {
        $cachePath = Join-Path $electronCache $item.Key
        if (-not (Test-Path $cachePath)) {
            Write-Host "  Downloading: $($item.Key)" -ForegroundColor Yellow
            try {
                $url = $item.Value
                $zipFile = "$cachePath.zip"
                New-Item -ItemType Directory -Path (Split-Path $cachePath -Parent) -Force | Out-Null

                # 使用 curl.exe 下载（更快）
                curl.exe -L -o $zipFile $url --silent --show-error

                if (Test-Path $zipFile) {
                    # 解压（使用 7zip）
                    $sevenZipPath = Join-Path $projectRoot "apps\desktop\node_modules\7zip-bin\win\x64\7za.exe"
                    if (Test-Path $sevenZipPath) {
                        cmd.exe /c "`"$sevenZipPath`" x `"$zipFile`" -o`"$cachePath`" -y >nul 2>&1"
                    }
                    Remove-Item $zipFile -Force -ErrorAction SilentlyContinue
                    Write-Host "  Cached: $($item.Key)" -ForegroundColor Green
                }
            } catch {
                Write-Host "  Pre-cache failed: $($item.Key)" -ForegroundColor Yellow
            }
        } else {
            Write-Host "  Already cached: $($item.Key)" -ForegroundColor Gray
        }
    }
}

# ===== 构建步骤 =====

# 1. Clean old artifacts (只在完整构建时)
if (-not $devBuild) {
    Write-Host "[1/8] Cleaning old build artifacts..." -ForegroundColor Cyan
    try { if (Test-Path "$projectRoot\bin") { Remove-Item "$projectRoot\bin" -Force -Recurse } } catch {}
    try { Remove-Item "$installerDir\release\*.zip" -Force } catch {}
    try { Remove-Item "$installerDir\release\win-unpacked" -Force -Recurse } catch {}
    try { Remove-Item "$installerDir\packages" -Force -Recurse } catch {}
}

# 2. Generate plugin registry（快速，始终执行）
Write-Host "[2/8] Generating plugin registry..." -ForegroundColor Cyan
Push-Location $projectRoot
go run ./tools/genplugins
Pop-Location

# Create directories
New-Item -ItemType Directory -Path "$projectRoot\bin" -Force | Out-Null
New-Item -ItemType Directory -Path "$projectRoot\bin\windows-amd64" -Force | Out-Null
New-Item -ItemType Directory -Path "$installerDir\packages\runtime\tools" -Force | Out-Null
New-Item -ItemType Directory -Path "$installerDir\packages\runtime\data\configs" -Force | Out-Null

# Pre-cache electron-builder（提前下载依赖）
if (-not $devBuild) {
    PreCacheElectronBuilder
}

# 3. Backend build（检测源码变化）
$backendHashFile = Join-Path $cacheDir "backend.hash"
$backendNeedsBuild = NeedsRebuild "$projectRoot\bin\colink-server.exe" "$projectRoot\internal" $backendHashFile

if ($backendNeedsBuild -or $devBuild) {
    Write-Host "[3/8] Building backend..." -ForegroundColor Cyan

    $backendJob = Start-Job -ScriptBlock {
        param($projectRoot, $FULL_VERSION)
        Set-Location $projectRoot
        $env:PATH = $env:PATH + ";C:\Program Files\Go\bin"
        go build -ldflags "-X main.Version=$FULL_VERSION" -o "bin\colink-server.exe" "./cmd/server"
        Copy-Item "bin\colink-server.exe" "bin\windows-amd64\colink-server.exe" -Force
        go build -o "bin\migrate.exe" "./cmd/migrate"
    } -ArgumentList $projectRoot, $FULL_VERSION

    Write-Host "  Backend building in background..." -ForegroundColor Gray
} else {
    Write-Host "[3/8] Backend unchanged, skipping build" -ForegroundColor Green
}

# 3.1. Frontend build（检测源码变化）
$frontendHashFile = Join-Path $cacheDir "frontend.hash"
$frontendNeedsBuild = NeedsRebuild "$projectRoot\web\dist" "$projectRoot\web\src" $frontendHashFile

if ($frontendNeedsBuild -or $devBuild) {
    Write-Host "[3.1/8] Building frontend..." -ForegroundColor Cyan
    Push-Location "$projectRoot\web"

    if (NeedsNpmInstall "$projectRoot\web") {
        Write-Host "  Installing npm packages..." -ForegroundColor Yellow
        npm ci --prefer-offline --no-audit --progress=false
    }

    if ($devBuild) {
        # 开发模式：跳过 TypeScript 检查
        $env:SKIP_TYPE_CHECK = "true"
        npx vite build --mode development
    } else {
        npm run build
    }

    Pop-Location
    Write-Host "  Frontend complete" -ForegroundColor Green
} else {
    Write-Host "[3.1/8] Frontend unchanged, skipping build" -ForegroundColor Green
}

# Wait for backend job
if ($backendNeedsBuild -or $devBuild) {
    Write-Host "  Waiting for backend..." -ForegroundColor Gray
    $backendResult = Wait-Job $backendJob | Receive-Job
    Remove-Job $backendJob
    Write-Host "  Backend complete" -ForegroundColor Green
}

# Copy backend files
Write-Host "[3.2/8] Copying backend files..." -ForegroundColor Cyan
$desktopBinDir = "$projectRoot\apps\desktop\resources\bin"
New-Item -ItemType Directory -Path $desktopBinDir -Force | Out-Null
Copy-Item "$projectRoot\bin\colink-server.exe" "$desktopBinDir\colink-server.exe" -Force
Copy-Item "$projectRoot\bin\migrate.exe" "$installerDir\packages\runtime\tools\migrate.exe" -Force

# Copy runtime packages
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

# 4. Build desktop application（检测源码变化）
$desktopHashFile = Join-Path $cacheDir "desktop.hash"
$desktopNeedsBuild = NeedsRebuild "$projectRoot\apps\desktop\out" "$projectRoot\apps\desktop\src" $desktopHashFile

if ($desktopNeedsBuild -or $devBuild) {
    Write-Host "[4/8] Building desktop application..." -ForegroundColor Cyan
    Push-Location "$projectRoot\apps\desktop"

    if (NeedsNpmInstall "$projectRoot\apps\desktop") {
        Write-Host "  Installing npm packages..." -ForegroundColor Yellow
        npm ci --prefer-offline --no-audit --progress=false
    }

    npm run build

    if ($devBuild) {
        # 开发模式：跳过签名，使用 dir 打包（更快）
        npx electron-builder --win --config electron-builder.portable.yml --dir
    } else {
        npx electron-builder --win --config electron-builder.portable.yml
    }

    Pop-Location
    Write-Host "  Desktop complete" -ForegroundColor Green
} else {
    Write-Host "[4/8] Desktop unchanged, skipping build" -ForegroundColor Green
}

# Copy desktop to packages
Write-Host "[4.1/8] Copying desktop app..." -ForegroundColor Cyan
$desktopReleaseDir = "$projectRoot\apps\desktop\release\portable\win-unpacked"
$packagesDesktopDir = "$installerDir\packages\desktop"
if (Test-Path $packagesDesktopDir) { Remove-Item $packagesDesktopDir -Force -Recurse }
New-Item -ItemType Directory -Path $packagesDesktopDir -Force | Out-Null
Copy-Item "$desktopReleaseDir\*" $packagesDesktopDir -Force -Recurse

# 5-6. Build installer（检测源码变化）
$installerHashFile = Join-Path $cacheDir "installer.hash"
$installerNeedsBuild = NeedsRebuild "$installerDir\out" "$installerDir\src" $installerHashFile

if ($installerNeedsBuild -or $devBuild) {
    Write-Host "[5/8] Building installer..." -ForegroundColor Cyan
    Push-Location $installerDir

    if (NeedsNpmInstall $installerDir) {
        Write-Host "  Installing npm packages..." -ForegroundColor Yellow
        npm ci --prefer-offline --no-audit --progress=false
    }

    npm run build
    npm run package:setup
    Pop-Location
    Write-Host "  Installer complete" -ForegroundColor Green
} else {
    Write-Host "[5/8] Installer unchanged, skipping build" -ForegroundColor Green
}

# 7. Create ZIP
Write-Host "[7/8] Creating release package..." -ForegroundColor Cyan
$env:COLINK_FULL_VERSION = $FULL_VERSION
$env:COLINK_OS = "windows"
$env:COLINK_ARCH = "amd64"

$distDir = "$installerDir\release\win-unpacked"
$outputZip = "$installerDir\release\$PACKAGE_NAME.zip"

if (Test-Path $outputZip) { Remove-Item $outputZip -Force }

# 使用 PowerShell Compress-Archive（处理时区正确）
Compress-Archive -Path "$distDir\*" -DestinationPath $outputZip -CompressionLevel Optimal

Write-Host "Created: $outputZip" -ForegroundColor Green

# 构建统计
Write-Host "===== Build Complete =====" -ForegroundColor Green
Write-Host "Release: $installerDir\release\$PACKAGE_NAME.zip" -ForegroundColor Yellow

# 显示缓存统计
$cacheFiles = Get-ChildItem $cacheDir -File -ErrorAction SilentlyContinue
if ($cacheFiles) {
    Write-Host "Build cache files: $($cacheFiles.Count)" -ForegroundColor Gray
    Write-Host "Cache saves rebuild time when files unchanged!" -ForegroundColor Cyan
}