#!/bin/bash
# ISDP 安装器完整构建脚本 (Unix/Linux/macOS 开发环境)

set -e

echo "===== ISDP 安装器构建开始 ====="

# 1. 构建 ISDP 后端
echo "[1/6] 构建 ISDP 后端..."
cd ../isdp
make build
mkdir -p ../installer/resources/app
cp bin/isdp.exe ../installer/resources/app/isdp-server.exe 2>/dev/null || cp bin/isdp ../installer/resources/app/isdp-server

# 2. 构建 ISDP 前端
echo "[2/6] 构建 ISDP 前端..."
cd web
npm run build
mkdir -p ../installer/resources/app/web
cp -r dist/* ../installer/resources/app/web/

# 3. 安装安装器依赖
echo "[3/6] 安装安装器依赖..."
cd ../../installer
npm install

# 4. 构建安装器代码
echo "[4/6] 构建安装器代码..."
npm run build

# 5. 打包启动器
echo "[5/6] 打包启动器..."
npm run package:launcher

mkdir -p resources/launcher
cp release/*/ISDP-Launcher*.exe resources/launcher/ISDP-Launcher.exe 2>/dev/null || true

# 6. 打包安装器
echo "[6/6] 打包安装器..."
npm run package

echo "===== 构建完成 ====="
echo "安装器产物: release/*/ISDP-Setup-*.exe"