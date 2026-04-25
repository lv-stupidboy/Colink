#!/usr/bin/env pwsh
# 同步资源到 installer-tauri
# 用法: ./sync-resources.ps1

$ProjectRoot = Split-Path -Parent $PSScriptRoot

Write-Host "=== Sync Resources ===" -ForegroundColor Cyan
Set-Location $ProjectRoot
& node scripts/sync-resources.js