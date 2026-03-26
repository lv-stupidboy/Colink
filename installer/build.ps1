# ISDP 安装器完整构建脚本 (Windows PowerShell)

$ErrorActionPreference = "Stop"

Write-Host "===== ISDP 安装器构建开始 =====" -ForegroundColor Green

# 1. 构建 ISDP 后端
Write-Host "[1/5] 构建 ISDP 后端..." -ForegroundColor Cyan
Push-Location ../isdp
make build
Pop-Location

# 2. 构建 ISDP 前端
Write-Host "[2/5] 构建 ISDP 前端..." -ForegroundColor Cyan
Push-Location ../isdp/web
npm run build
Pop-Location

# 3. 安装依赖并构建安装器代码
Write-Host "[3/5] 构建安装器代码..." -ForegroundColor Cyan
npm install
npm run build

# 4. 打包启动器
Write-Host "[4/5] 打包启动器..." -ForegroundColor Cyan
npm run package:launcher

# 5. 打包安装器（electron-builder 会直接从 ../isdp/ 读取后端和前端）
Write-Host "[5/5] 打包安装器..." -ForegroundColor Cyan
npm run package:setup

# 6. 创建 ZIP 包
Write-Host "[6/6] 创建 ZIP 包..." -ForegroundColor Cyan
node scripts/create-zip.js

Write-Host "===== 构建完成 =====" -ForegroundColor Green
Write-Host "安装器产物: release/ISDP-*.zip" -ForegroundColor Yellow