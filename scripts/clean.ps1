#Requires -Version 5.1
param(
    [switch]$All,
    [switch]$Dist,
    [switch]$Bin,
    [switch]$Target,
    [switch]$Gen,
    [switch]$Staging,
    [switch]$NodeModules,
    [switch]$WhatIf,
    [switch]$NoConfirm
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $PSScriptRoot
$InstallerDir = Join-Path $ScriptDir 'installer-tauri'
$SrcTauriDir = Join-Path $InstallerDir 'src-tauri'

if (-not $All -and -not $Dist -and -not $Bin -and -not $Target -and -not $Gen -and -not $Staging -and -not $NodeModules) {
    $All = $true
}

$DirsToClean = @()

if ($All -or $Dist) {
    $DirsToClean += [PSCustomObject]@{ Path = Join-Path $ScriptDir 'web/dist'; Name = 'web/dist' }
    $DirsToClean += [PSCustomObject]@{ Path = Join-Path $InstallerDir 'dist'; Name = 'installer-tauri/dist' }
}

if ($All -or $Bin) {
    $DirsToClean += [PSCustomObject]@{ Path = Join-Path $ScriptDir 'bin'; Name = 'bin' }
}

if ($All -or $Staging) {
    $DirsToClean += [PSCustomObject]@{ Path = Join-Path $ScriptDir 'staging'; Name = 'staging' }
}

if ($All -or $Target) {
    $DirsToClean += [PSCustomObject]@{ Path = Join-Path $SrcTauriDir 'target'; Name = 'src-tauri/target' }
    $DirsToClean += [PSCustomObject]@{ Path = Join-Path $SrcTauriDir 'gen'; Name = 'src-tauri/gen' }
} elseif ($Gen) {
    $DirsToClean += [PSCustomObject]@{ Path = Join-Path $SrcTauriDir 'gen'; Name = 'src-tauri/gen' }
}

if ($Staging -and -not $All) {
    $TargetStaging = Join-Path $SrcTauriDir 'target/release/staging'
    if (Test-Path $TargetStaging) {
        $DirsToClean += [PSCustomObject]@{ Path = $TargetStaging; Name = 'target/release/staging' }
    }
}

if ($NodeModules) {
    $DirsToClean += [PSCustomObject]@{ Path = Join-Path $ScriptDir 'web/node_modules'; Name = 'web/node_modules' }
    $DirsToClean += [PSCustomObject]@{ Path = Join-Path $InstallerDir 'node_modules'; Name = 'installer-tauri/node_modules' }
}

Write-Host ''
Write-Host '将要清理的目录:' -ForegroundColor Cyan
$TotalSize = 0
$ExistingDirs = @()

foreach ($Dir in $DirsToClean) {
    if (Test-Path $Dir.Path) {
        $Size = (Get-ChildItem -Path $Dir.Path -Recurse -Force -ErrorAction SilentlyContinue | Measure-Object -Property Length -Sum).Sum
        $SizeMB = [math]::Round($Size / 1MB, 2)
        $TotalSize += $Size
        Write-Host '  -' $Dir.Name': ' $SizeMB 'MB' -ForegroundColor Yellow
        $ExistingDirs += $Dir
    } else {
        Write-Host '  -' $Dir.Name': 不存在' -ForegroundColor Gray
    }
}

if ($ExistingDirs.Count -eq 0) {
    Write-Host ''
    Write-Host '没有需要清理的目录。' -ForegroundColor Green
    exit 0
}

$TotalSizeMB = [math]::Round($TotalSize / 1MB, 2)
Write-Host ''
Write-Host '总计: ' $TotalSizeMB 'MB' -ForegroundColor Cyan

if ($WhatIf) {
    Write-Host ''
    Write-Host '[预览模式] 以上目录将被清理，但未实际执行。' -ForegroundColor Magenta
    exit 0
}

if (-not $NoConfirm) {
    $Confirm = Read-Host '是否继续清理? [Y/n]'
    if ($Confirm -ne '' -and $Confirm -ne 'Y' -and $Confirm -ne 'y') {
        Write-Host '已取消清理。' -ForegroundColor Yellow
        exit 0
    }
}

Write-Host ''
Write-Host '开始清理...' -ForegroundColor Cyan
$CleanedCount = 0

foreach ($Dir in $ExistingDirs) {
    try {
        Remove-Item -Path $Dir.Path -Recurse -Force
        Write-Host '  已清理: ' $Dir.Name -ForegroundColor Green
        $CleanedCount++
    } catch {
        Write-Host '  清理失败: ' $Dir.Name -ForegroundColor Red
        Write-Host '    错误: ' $_.Exception.Message -ForegroundColor Red
    }
}

Write-Host ''
Write-Host '清理完成! 已清理' $CleanedCount '个目录，释放' $TotalSizeMB 'MB空间。' -ForegroundColor Green
