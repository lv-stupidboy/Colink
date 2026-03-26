# ISDP 安装器重构实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复安装器嵌套打包问题，移除系统托盘功能，优化ZIP包结构，使最终包大小从1.9GB降至~300MB。

**Architecture:** 安装器（ISDP-Setup.exe）+ 运行时（runtime/）+ 启动器（launcher/）分离，启动器使用面板窗口而非系统托盘。

**Tech Stack:** Electron 30, electron-builder, React, Ant Design

---

## 文件结构总览

### 需要删除的文件

| 文件 | 原因 |
|------|------|
| `installer/src/main/tray.ts` | 移除系统托盘功能（被 launcher-entry.ts 使用） |
| `installer/src/main/launcher-entry.ts` | 原托盘启动器入口，不再需要 |

### 需要修改的文件

| 文件 | 修改内容 |
|------|----------|
| `installer/src/main/index.ts` | 删除托盘相关代码，修改窗口关闭行为 |
| `installer/src/renderer/src/pages/Dashboard.tsx` | 删除托盘提示文字 |
| `installer/electron-builder.yml` | 更新打包配置 |
| `installer/electron-builder.launcher.yml` | 更新启动器打包配置 |
| `installer/package.json` | 添加 package:launcher 脚本 |
| `installer/build.ps1` | 更新构建流程 |
| `installer/build.sh` | 更新构建流程 |
| `installer/scripts/create-zip.js` | 更新ZIP包结构 |

---

## Task 1: 删除托盘相关文件

**Files:**
- Delete: `installer/src/main/tray.ts`
- Delete: `installer/src/main/launcher-entry.ts`

> **说明**：`tray.ts` 被 `launcher-entry.ts` 导入，`launcher-entry.ts` 不再需要。`index.ts` 有自己内联的托盘函数，将在 Task 2 中处理。

- [ ] **Step 1: 删除 tray.ts**

Windows PowerShell:
```powershell
Remove-Item installer\src\main\tray.ts
```

Unix Bash:
```bash
rm installer/src/main/tray.ts
```

- [ ] **Step 2: 删除 launcher-entry.ts**

Windows PowerShell:
```powershell
Remove-Item installer\src\main\launcher-entry.ts
```

Unix Bash:
```bash
rm installer/src/main/launcher-entry.ts
```

- [ ] **Step 3: 提交删除**

```bash
git add -A && git commit -m "refactor(installer): remove tray and launcher-entry files"
```

---

## Task 2: 修改主入口文件 - 删除托盘相关代码

**Files:**
- Modify: `installer/src/main/index.ts`

> **背景**：`index.ts` 中有内联定义的 `createTray()` 和 `updateTrayMenu()` 函数，以及多处对这些函数的调用。需要全部删除。

- [ ] **Step 1: 删除 electron 导入中的托盘相关类型**

将第1行的导入：
```typescript
import { app, BrowserWindow, ipcMain, dialog, shell, Tray, Menu, nativeImage } from 'electron'
```

修改为：
```typescript
import { app, BrowserWindow, ipcMain, dialog, shell } from 'electron'
```

- [ ] **Step 2: 删除 tray 变量声明**

删除第25行：
```typescript
let tray: Tray | null = null
```

- [ ] **Step 3: 删除 createTray() 和 updateTrayMenu() 函数**

删除第101-200行（两个函数的完整定义）：
```typescript
// 创建托盘
function createTray() {
  // ... 整个函数体
}

// 更新托盘菜单
function updateTrayMenu() {
  // ... 整个函数体
}
```

- [ ] **Step 4: 删除 updateTrayMenu() 调用**

找到并删除以下位置的 `updateTrayMenu()` 调用：
- 第323行：`updateTrayMenu()`（在 start-service handler 中）
- 第329行：`updateTrayMenu()`（在 stop-service handler 中）
- 第411行：`updateTrayMenu()`（在 uninstall handler 中）

- [ ] **Step 5: 删除 createTray() 调用**

删除第468行的 `createTray()` 调用：
```typescript
// 删除这行
createTray()
```

- [ ] **Step 6: 提交修改**

```bash
git add installer/src/main/index.ts && git commit -m "refactor(installer): remove tray functions from index.ts"
```

---

## Task 3: 修改主入口文件 - 更新窗口关闭行为

**Files:**
- Modify: `installer/src/main/index.ts`

- [ ] **Step 1: 修改窗口关闭逻辑**

将第92-98行的窗口关闭逻辑：
```typescript
  // 关闭窗口时最小化到托盘而不是退出
  mainWindow.on('close', (event) => {
    if (!app.isQuitting) {
      event.preventDefault()
      mainWindow?.hide()
    }
  })
```

修改为：
```typescript
  // 关闭窗口时弹出确认对话框
  mainWindow.on('close', (event) => {
    const choice = dialog.showMessageBoxSync(mainWindow!, {
      type: 'question',
      buttons: ['取消', '确认关闭'],
      defaultId: 1,
      cancelId: 0,
      title: '关闭 ISDP',
      message: '关闭 ISDP 控制面板？',
      detail: '后端服务将继续在后台运行。您可以通过桌面快捷方式重新打开控制面板。'
    })

    if (choice === 0) {
      event.preventDefault()
    }
  })
```

- [ ] **Step 2: 修改 window-close IPC 处理**

将第230行：
```typescript
ipcMain.on('window-close', () => mainWindow?.hide())
```

修改为：
```typescript
ipcMain.on('window-close', () => mainWindow?.close())
```

- [ ] **Step 3: 提交修改**

```bash
git add installer/src/main/index.ts && git commit -m "refactor(installer): change window close behavior with confirmation dialog"
```

---

## Task 4: 修改 Dashboard 页面

**Files:**
- Modify: `installer/src/renderer/src/pages/Dashboard.tsx`

- [ ] **Step 1: 修改托盘提示文字**

将第148-152行：
```tsx
      <div style={{ marginTop: 24, textAlign: 'center' }}>
        <Text type="secondary">
          关闭窗口后程序将继续在系统托盘运行
        </Text>
      </div>
```

修改为：
```tsx
      <div style={{ marginTop: 24, textAlign: 'center' }}>
        <Text type="secondary">
          关闭窗口后服务将继续在后台运行
        </Text>
      </div>
```

- [ ] **Step 2: 提交修改**

```bash
git add installer/src/renderer/src/pages/Dashboard.tsx && git commit -m "refactor(installer): update dashboard close hint text"
```

---

## Task 5: 更新 electron-builder.launcher.yml

**Files:**
- Modify: `installer/electron-builder.launcher.yml`

> **说明**：将输出目录从 `resources/launcher` 改为 `release/launcher`，避免与源码目录混淆。同时添加 files 排除规则，防止打包不需要的文件。

- [ ] **Step 1: 替换整个配置文件**

```yaml
appId: com.isdp.launcher
productName: ISDP
directories:
  output: release/launcher

win:
  target:
    - target: dir
      arch: [x64]
  icon: build/icon.ico
  sign: null

asar: true

# 精确控制打包内容，排除不需要的文件
files:
  - "out/**/*"
  - "package.json"
  - "!**/node_modules/**"
  - "!**/src/**"
  - "!**/release/**"
  - "!**/resources/**"
  - "!**/*.ts"

extraResources:
  - from: "build/icon.ico"
    to: "icon.ico"
```

- [ ] **Step 2: 提交修改**

```bash
git add installer/electron-builder.launcher.yml && git commit -m "refactor(installer): update launcher build config with file exclusions"
```

---

## Task 6: 更新 electron-builder.yml

**Files:**
- Modify: `installer/electron-builder.yml`

- [ ] **Step 1: 替换整个配置文件**

```yaml
appId: com.isdp.installer
productName: ISDP Setup
directories:
  output: release

win:
  target:
    - target: dir
      arch: [x64]
  icon: build/icon.ico
  sign: null

asar: true

# 精确控制打包内容
files:
  - "out/**/*"
  - "package.json"
  - "!**/node_modules/**"
  - "!**/src/**"
  - "!**/resources/**"

extraResources:
  # 后端服务
  - from: "resources/app/isdp-server.exe"
    to: "runtime/isdp-server.exe"
  # 前端静态文件
  - from: "resources/app/web"
    to: "runtime/web"
  # 启动器（从 release/launcher/ 复制）
  - from: "release/launcher"
    to: "launcher"
```

- [ ] **Step 2: 提交修改**

```bash
git add installer/electron-builder.yml && git commit -m "refactor(installer): update installer build config with runtime and launcher"
```

---

## Task 7: 更新 package.json

**Files:**
- Modify: `installer/package.json`

- [ ] **Step 1: 在 scripts 中添加 package:launcher 脚本**

在现有的 `scripts` 块中添加 `package:launcher` 脚本：

```json
{
  "scripts": {
    "dev": "electron-vite dev",
    "build": "electron-vite build",
    "postinstall": "electron-builder install-app-deps",
    "package:launcher": "electron-builder --win --config electron-builder.launcher.yml",
    "package": "npm run build && electron-builder --win --config electron-builder.yml",
    "package:zip": "npm run package && node scripts/create-zip.js"
  }
}
```

- [ ] **Step 2: 提交修改**

```bash
git add installer/package.json && git commit -m "refactor(installer): add package:launcher script"
```

---

## Task 8: 更新构建脚本 (build.ps1)

**Files:**
- Modify: `installer/build.ps1`

- [ ] **Step 1: 替换整个构建脚本**

```powershell
# ISDP 安装器完整构建脚本 (Windows PowerShell)

$ErrorActionPreference = "Stop"

Write-Host "===== ISDP 安装器构建开始 =====" -ForegroundColor Green

# 1. 构建 ISDP 后端
Write-Host "[1/6] 构建 ISDP 后端..." -ForegroundColor Cyan
Push-Location ../isdp
make build
New-Item -ItemType Directory -Force -Path ../installer/resources/app | Out-Null
Copy-Item bin/isdp.exe ../installer/resources/app/isdp-server.exe
Pop-Location

# 2. 构建 ISDP 前端
Write-Host "[2/6] 构建 ISDP 前端..." -ForegroundColor Cyan
Push-Location ../isdp/web
npm run build
New-Item -ItemType Directory -Force -Path ../../installer/resources/app/web | Out-Null
Copy-Item -Recurse -Force dist/* ../../installer/resources/app/web/
Pop-Location

# 3. 安装依赖并构建安装器代码
Write-Host "[3/6] 构建安装器代码..." -ForegroundColor Cyan
npm install
npm run build

# 4. 打包启动器
Write-Host "[4/6] 打包启动器..." -ForegroundColor Cyan
npm run package:launcher

# 5. 打包安装器
Write-Host "[5/6] 打包安装器..." -ForegroundColor Cyan
npm run package

# 6. 创建 ZIP 包
Write-Host "[6/6] 创建 ZIP 包..." -ForegroundColor Cyan
node scripts/create-zip.js

Write-Host "===== 构建完成 =====" -ForegroundColor Green
Write-Host "安装器产物: release/ISDP-*.zip" -ForegroundColor Yellow
```

- [ ] **Step 2: 提交修改**

```bash
git add installer/build.ps1 && git commit -m "refactor(installer): update build script for new workflow"
```

---

## Task 9: 更新构建脚本 (build.sh)

**Files:**
- Modify: `installer/build.sh`

- [ ] **Step 1: 替换整个构建脚本**

```bash
#!/bin/bash
# ISDP 安装器完整构建脚本 (Unix/Linux/macOS 开发环境)

set -e

echo "===== ISDP 安装器构建开始 ====="

# 1. 构建 ISDP 后端
echo "[1/6] 构建 ISDP 后端..."
cd ../isdp
make build
mkdir -p ../installer/resources/app
cp bin/isdp ../installer/resources/app/isdp-server.exe 2>/dev/null || cp bin/isdp.exe ../installer/resources/app/isdp-server.exe 2>/dev/null || true

# 2. 构建 ISDP 前端
echo "[2/6] 构建 ISDP 前端..."
cd web
npm run build
mkdir -p ../../installer/resources/app/web
cp -r dist/* ../../installer/resources/app/web/

# 3. 安装依赖并构建安装器代码
echo "[3/6] 构建安装器代码..."
cd ../../installer
npm install
npm run build

# 4. 打包启动器
echo "[4/6] 打包启动器..."
npm run package:launcher

# 5. 打包安装器
echo "[5/6] 打包安装器..."
npm run package

# 6. 创建 ZIP 包
echo "[6/6] 创建 ZIP 包..."
node scripts/create-zip.js

echo "===== 构建完成 ====="
echo "安装器产物: release/ISDP-*.zip"
```

- [ ] **Step 2: 提交修改**

```bash
git add installer/build.sh && git commit -m "refactor(installer): update build script for new workflow"
```

---

## Task 10: 更新 create-zip.js

**Files:**
- Modify: `installer/scripts/create-zip.js`

- [ ] **Step 1: 更新 ZIP 打包脚本**

```javascript
const { createWriteStream, existsSync, readdirSync, statSync } = require('fs')
const { join } = require('path')
const archiver = require('archiver')

// 获取版本号
const packageJson = require('../package.json')
const version = packageJson.version

// 源目录和输出文件
const releaseDir = join(__dirname, '../release')
const distDir = join(releaseDir, 'win-unpacked')
const outputFile = join(releaseDir, `ISDP-${version}.zip`)

console.log('Source:', distDir)
console.log('Output:', outputFile)

// 检查源目录
if (!existsSync(distDir)) {
  console.error('Error: Source directory not found:', distDir)
  process.exit(1)
}

// 创建 zip 文件
const output = createWriteStream(outputFile)
const archive = archiver('zip', { zlib: { level: 9 } })

output.on('close', () => {
  const size = (archive.pointer() / 1024 / 1024).toFixed(2)
  console.log(`✓ Created: ${outputFile}`)
  console.log(`  Size: ${size} MB`)
})

archive.on('error', (err) => {
  console.error('Archive error:', err)
  throw err
})

archive.on('progress', (progress) => {
  if (progress.entries.total > 0) {
    const percent = Math.round((progress.entries.processed / progress.entries.total) * 100)
    process.stdout.write(`\rPackaging: ${percent}% (${progress.entries.processed}/${progress.entries.total} files)`)
  }
})

archive.pipe(output)

// 添加文件到 ISDP 目录
console.log('Packaging files...')

const files = readdirSync(distDir)
for (const file of files) {
  const filePath = join(distDir, file)
  const stat = statSync(filePath)
  if (stat.isDirectory()) {
    archive.directory(filePath, `ISDP/${file}`)
  } else {
    archive.file(filePath, { name: `ISDP/${file}` })
  }
}

archive.finalize()
```

- [ ] **Step 2: 提交修改**

```bash
git add installer/scripts/create-zip.js && git commit -m "refactor(installer): update zip script for new structure"
```

---

## Task 11: 清理旧资源目录

**Files:**
- Delete: `installer/resources/launcher/` (整个目录)

- [ ] **Step 1: 删除旧的 launcher 目录**

Windows PowerShell:
```powershell
Remove-Item -Recurse -Force installer\resources\launcher
```

Unix Bash:
```bash
rm -rf installer/resources/launcher
```

- [ ] **Step 2: 确保 app 目录占位符存在**

Windows PowerShell:
```powershell
New-Item -ItemType Directory -Force -Path installer\resources\app | Out-Null
New-Item -ItemType file -Force -Path installer\resources\app\.gitkeep | Out-Null
```

Unix Bash:
```bash
mkdir -p installer/resources/app
touch installer/resources/app/.gitkeep
```

- [ ] **Step 3: 提交修改**

```bash
git add -A && git commit -m "refactor(installer): clean up old launcher resources"
```

---

## Task 12: 验证构建 - TypeScript 编译

**Files:**
- Test: TypeScript 编译验证

- [ ] **Step 1: 运行 TypeScript 编译检查**

```bash
cd installer && npm run build
```

预期：编译成功，无错误。

- [ ] **Step 2: 如果编译失败，修复错误**

根据错误信息修复代码问题。

---

## Task 13: 验证构建 - 完整构建

**Files:**
- Test: 完整构建验证

- [ ] **Step 1: 执行完整构建**

Windows PowerShell:
```powershell
cd installer
.\build.ps1
```

或 Unix Bash:
```bash
cd installer
./build.sh
```

- [ ] **Step 2: 检查构建产物大小**

Windows PowerShell:
```powershell
# 检查 app.asar 大小
Get-ChildItem installer\release\win-unpacked\resources\

# 检查最终 ZIP 大小
Get-ChildItem installer\release\ISDP-*.zip
```

或 Unix Bash:
```bash
# 检查 app.asar 大小
ls -la installer/release/win-unpacked/resources/

# 检查最终 ZIP 大小
ls -la installer/release/ISDP-*.zip
```

预期：
- `app.asar` < 50MB
- ZIP 包 < 350MB

- [ ] **Step 3: 检查 ZIP 包结构**

解压 ZIP 包，验证结构：
```
ISDP/
├── ISDP.exe (安装器)
├── resources/
├── runtime/
│   ├── isdp-server.exe
│   └── web/
└── launcher/
    ├── ISDP.exe
    └── resources/
```

---

## Task 14: 最终提交 - 更新 CHANGELOG

**Files:**
- Modify: `docs/CHANGELOG.md`

- [ ] **Step 1: 在 CHANGELOG 开头添加变更记录**

```markdown
## 2026-03-27 安装器重构

### 背景
原安装器存在嵌套打包问题，导致 app.asar 达到 1.6GB，安装过程复制运行时文件失败。

### 目标
1. 修复嵌套打包问题，将包大小降至 ~300MB
2. 移除系统托盘功能，改用面板窗口
3. 支持 ZIP 包分发结构

### 核心变更
#### 安装器改动
- 删除 `src/main/tray.ts` - 移除系统托盘
- 删除 `src/main/launcher-entry.ts` - 不再需要独立启动器入口
- 修改 `src/main/index.ts` - 移除托盘函数，更新窗口关闭行为
- 修改 `src/renderer/src/pages/Dashboard.tsx` - 更新提示文字
- 更新 `electron-builder.yml` - 新的打包结构
- 更新 `electron-builder.launcher.yml` - 精确控制打包内容
- 更新构建脚本 - 新的构建流程

### 新增/修改文件列表
| 文件 | 改动类型 | 说明 |
|------|----------|------|
| installer/src/main/tray.ts | 删除 | 移除托盘功能 |
| installer/src/main/launcher-entry.ts | 删除 | 不再需要 |
| installer/src/main/index.ts | 修改 | 窗口关闭行为 |
| installer/electron-builder.yml | 修改 | 打包配置 |
| installer/electron-builder.launcher.yml | 修改 | 启动器配置 |

### 验证方法
1. 执行 `.\build.ps1` 完成构建
2. 检查 ZIP 包大小应 < 350MB
3. 测试安装流程

### 影响范围
installer 模块
```

- [ ] **Step 2: 提交 CHANGELOG**

```bash
git add docs/CHANGELOG.md && git commit -m "docs: update changelog for installer refactor"
```

---

## 预期结果

| 指标 | 修复前 | 修复后 |
|------|--------|--------|
| app.asar 大小 | 1.6GB | < 50MB |
| ZIP 包大小 | 1.9GB | ~300MB |
| 构建时间 | 长 | 显著缩短 |
| 启动器功能 | 系统托盘 | 面板窗口 |