#Requires -Version 5.1
<#
.SYNOPSIS
    清理构建和打包生成的临时文件

.DESCRIPTION
    清理以下目录：
    - bin/                        (Go 后端编译产物)
    - web/dist/                   (主项目前端构建产物)
    - installer-tauri/dist/       (安装器前端构建产物)
    - installer-tauri/src-tauri/target/  (Rust/Tauri 构建产物)
    - installer-tauri/src-tauri/gen/     (Tauri schema 文件)
    - staging/                    (资源同步中间目录)
    - node_modules/               (需显式指定 -NodeModules)

.PARAMETER All
    清理所有临时文件

.PARAMETER Dist
    只清理前端构建产物 (web/dist 和 installer-tauri/dist)

.PARAMETER Bin
    只清理 Go 后端编译产物 (bin/)

.PARAMETER Target
    只清理 Rust 构建产物 (target/ 和 gen/)

.PARAMETER Gen
    只清理 Tauri schema 文件 (gen/)

.PARAMETER NodeModules
    只清理 node_modules 目录

.PARAMETER OldInstaller
    只清理旧 Electron 安装器 (installer/node_modules)

.PARAMETER Staging
    只清理资源同步中间目录

.PARAMETER WhatIf
    预览模式，只显示将要清理的内容，不实际删除

.PARAMETER NoConfirm
    跳过确认提示

.EXAMPLE
    ./clean.ps1 -All
    清理所有临时文件

.EXAMPLE
    ./clean.ps1 -Dist -Target
    清理前端和 Rust 构建产物

.EXAMPLE
    ./clean.ps1 -All -WhatIf
    预览将要清理的内容

.EXAMPLE
    ./clean.ps1 -All -NoConfirm
    清理所有临时文件，不提示确认

.EXAMPLE
    ./clean.ps1 -NodeModules
    清理所有 node_modules（需重新安装依赖）

.EXAMPLE
    ./clean.ps1 -OldInstaller
    只清理废弃的旧 Electron 安装器

.EXAMPLE
    ./clean.ps1 -Bin
    只清理 Go 后端编译产物
#>

param(
    [switch]$All,
    [switch]$Dist,
    [switch]$Bin,
    [switch]$Target,
    [switch]$Gen,
    [switch]$Staging,
    [switch]$NodeModules,
    [switch]$OldInstaller,
    [switch]$WhatIf,
    [switch]$NoConfirm
)

$ErrorActionPreference = "Stop"

# 获取脚本所在目录的父目录（主项目根目录）
$ScriptDir = Split-Path -Parent $PSScriptRoot
$InstallerDir = Join-Path $ScriptDir "installer-tauri"
$SrcTauriDir = Join-Path $InstallerDir "src-tauri"

# 如果没有指定任何参数，默认清理构建产物（不含 node_modules）
if (-not $All -and -not $Dist -and -not $Bin -and -not $Target -and -not $Gen -and -not $Staging -and -not $NodeModules -and -not $OldInstaller) {
    $All = $true
}

# 定义要清理的目录
$DirsToClean = [System.Collections.ArrayList]@()

if ($All -or $Dist) {
    $DirsToClean.Add(@{
        Path = Join-Path $ScriptDir "web/dist"
        Name = "主项目前端构建产物 (web/dist)"
    }) | Out-Null
    $DirsToClean.Add(@{
        Path = Join-Path $InstallerDir "dist"
        Name = "安装器前端构建产物 (installer-tauri/dist)"
    }) | Out-Null
}

if ($All -or $Bin) {
    $DirsToClean.Add(@{
        Path = Join-Path $ScriptDir "bin"
        Name = "Go 后端编译产物 (bin)"
    }) | Out-Null
}

if ($All -or $Staging) {
    # 主项目 staging 目录
    $DirsToClean.Add(@{
        Path = Join-Path $ScriptDir "staging"
        Name = "资源同步中间目录 (staging)"
    }) | Out-Null
}

if ($All -or $Target) {
    # 当清理 target 时，也清理 gen 目录
    $DirsToClean.Add(@{
        Path = Join-Path $SrcTauriDir "target"
        Name = "Rust 构建产物 (src-tauri/target)"
    }) | Out-Null
    $DirsToClean.Add(@{
        Path = Join-Path $SrcTauriDir "gen"
        Name = "Tauri schema 文件 (src-tauri/gen)"
    }) | Out-Null
} elseif ($Gen) {
    # 只清理 gen 目录
    $DirsToClean.Add(@{
        Path = Join-Path $SrcTauriDir "gen"
        Name = "Tauri schema 文件 (src-tauri/gen)"
    }) | Out-Null
} elseif ($Staging) {
    # 只清理 staging 时，单独处理 target 内的 staging 子目录
    $TargetStaging = Join-Path $SrcTauriDir "target/release/staging"
    if (Test-Path $TargetStaging) {
        $DirsToClean.Add(@{
            Path = $TargetStaging
            Name = "target 内 staging 目录"
        }) | Out-Null
    }
}

# node_modules 清理（默认不清理，需显式指定）
if ($NodeModules) {
    $DirsToClean.Add(@{
        Path = Join-Path $ScriptDir "web/node_modules"
        Name = "主项目前端依赖 (web/node_modules)"
    }) | Out-Null
    $DirsToClean.Add(@{
        Path = Join-Path $InstallerDir "node_modules"
        Name = "Tauri 安装器依赖 (installer-tauri/node_modules)"
    }) | Out-Null
}

# 旧 Electron 安装器（废弃项目）
if ($OldInstaller) {
    $DirsToClean.Add(@{
        Path = Join-Path $ScriptDir "installer/node_modules"
        Name = "旧 Electron 安装器依赖 (installer/node_modules)"
    }) | Out-Null
}

# 显示将要清理的目录
Write-Host "`n将要清理的目录:" -ForegroundColor Cyan
$TotalSize = 0
$ExistingDirs = @()

foreach ($Dir in $DirsToClean) {
    if (Test-Path $Dir.Path) {
        $Size = (Get-ChildItem -Path $Dir.Path -Recurse -Force -ErrorAction SilentlyContinue |
                 Measure-Object -Property Length -Sum).Sum
        $SizeMB = [math]::Round($Size / 1MB, 2)
        $TotalSize += $Size
        Write-Host "  - $($Dir.Name): $SizeMB MB" -ForegroundColor Yellow
        $ExistingDirs += $Dir
    } else {
        Write-Host "  - $($Dir.Name): 不存在 (跳过)" -ForegroundColor Gray
    }
}

if ($ExistingDirs.Count -eq 0) {
    Write-Host "`n没有需要清理的目录。" -ForegroundColor Green
    exit 0
}

$TotalSizeMB = [math]::Round($TotalSize / 1MB, 2)
Write-Host "`n总计: $TotalSizeMB MB" -ForegroundColor Cyan

# WhatIf 模式：只显示，不执行
if ($WhatIf) {
    Write-Host "`n[预览模式] 以上目录将被清理，但未实际执行。" -ForegroundColor Magenta
    exit 0
}

# 确认清理
if (-not $NoConfirm) {
    $Confirm = Read-Host "`n是否继续清理? [Y/n]"
    if ($Confirm -ne "" -and $Confirm -ne "Y" -and $Confirm -ne "y") {
        Write-Host "已取消清理。" -ForegroundColor Yellow
        exit 0
    }
}

# 执行清理
Write-Host "`n开始清理..." -ForegroundColor Cyan
$CleanedCount = 0

foreach ($Dir in $ExistingDirs) {
    try {
        Remove-Item -Path $Dir.Path -Recurse -Force
        Write-Host "  ✓ 已清理: $($Dir.Name)" -ForegroundColor Green
        $CleanedCount++
    } catch {
        Write-Host "  ✗ 清理失败: $($Dir.Name)" -ForegroundColor Red
        Write-Host "    错误: $_" -ForegroundColor Red
    }
}

Write-Host "`n清理完成! 已清理 $CleanedCount 个目录，释放 $TotalSizeMB MB 空间。" -ForegroundColor Green