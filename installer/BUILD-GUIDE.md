# Colink Build Scripts Guide

## 构建脚本对比

| 脚本 | 用途 | 构建时间 | 适用场景 |
|------|------|---------|---------|
| `build.ps1` | 完整构建 | ~115秒 | 正式发布、首次构建 |
| `build-fast.ps1` | 增量构建 | **~20-65秒** | 开发测试、重复构建 |

## 使用方法

### 1. 完整构建（首次构建或发布）
```powershell
cd installer
.\build.ps1
```

**特点**：
- 清理所有旧构建产物
- 重新构建所有组件
- 生成完整发布包
- 包含签名和优化步骤

---

### 2. 快速增量构建（推荐）
```powershell
cd installer
.\build-fast.ps1
```

**特点**：
- ✅ **增量检测**：只构建变化的部分
- ✅ **智能缓存**：保存 hash 避免重复构建
- ✅ **npm 缓存**：package-lock 变化才安装
- ✅ **预缓存依赖**：提前下载 electron-builder 依赖

**构建时间对比**：

| 场景 | build.ps1 | build-fast.ps1 | 提升 |
|------|-----------|----------------|------|
| 无修改重新构建 | 115秒 | **~20秒** | 83% ↓ |
| 前端修改 | 115秒 | **~55秒** | 52% ↓ |
| 后端修改 | 115秒 | **~35秒** | 70% ↓ |
| 完整修改 | 115秒 | **~65秒** | 43% ↓ |

---

### 3. 开发模式快速构建（最快）
```powershell
cd installer
$env:COLINK_DEV_BUILD = "true"
.\build-fast.ps1
```

**特点**：
- ⚡ 跳过 TypeScript 检查
- ⚡ 跳过 electron-builder 签名
- ⚡ 使用 development 模式打包
- ⚡ 关闭 sourcemap 生成

**构建时间**：**~30秒**

---

## 缓存机制

build-fast.ps1 使用 `.build-cache/` 目录保存构建 hash：

```
.build-cache/
├── backend.hash       # 后端源码 hash
├── frontend.hash      # 前端源码 hash
├── desktop.hash       # 桌面应用源码 hash
├── installer.hash     # 安装器源码 hash
└── npm-*.hash         # npm package-lock hash
```

**清理缓存**（强制重新构建）：
```powershell
Remove-Item .build-cache -Force -Recurse
```

---

## 构建优化技术

### 1. 增量构建检测
- 计算源文件 MD5 hash
- 对比缓存 hash 判断是否需要构建
- 无修改时跳过构建步骤

### 2. 智能 npm 安装
- 只在 `package-lock.json` 变化时安装
- 使用 `npm ci --prefer-offline`（缓存优先）
- 跳过审计和进度显示

### 3. 预缓存依赖
- 提前下载 electron-builder 必需文件：
  - Electron 二进制（19.1.9）
  - winCodeSign（签名工具）
  - NSIS（打包工具）
- 使用国内镜像加速下载

### 4. 并行构建
- 后端构建在后台 Job 执行
- 前端构建在主进程执行（避免环境问题）

### 5. Vite 构建优化
- Chunk 分割（减少首屏加载）
- esbuild 替代 terser（更快的压缩）
- 关闭 sourcemap（开发构建）

---

## 常见问题

### Q1: 为什么还是很慢？
**可能原因**：
- 首次构建（无缓存）
- 修改了大量源文件
- npm 包更新导致重新安装

**解决方案**：
```powershell
# 检查哪个步骤慢
$env:COLINK_DEV_BUILD = "true"
.\build-fast.ps1

# 清理缓存重新测试
Remove-Item .build-cache -Force -Recurse
.\build-fast.ps1
```

---

### Q2: 构建产物在哪里？
```
installer/
├── release/
│   ├── Colink-v1.0.0-20260505-143525-windows-amd64.zip  # 最终发布包
│   └── win-unpacked/                                    # 未打包文件
└── packages/
    ├── desktop/    # 桌面应用
    └── runtime/    # 服务端
```

---

### Q3: 如何只构建某个部分？
```powershell
# 只构建后端
go build -ldflags "-X main.Version=test" -o bin/colink-server.exe ./cmd/server

# 只构建前端
cd web && npm run build

# 只构建桌面应用
cd apps/desktop && npm run build
```

---

## 性能对比测试

**测试环境**：
- Windows 11 Pro
- Go 1.22
- Node.js 20.x
- 16GB RAM

**测试结果**（2026-05-05）：

| 构建步骤 | 老版本 | 新版本 | 优化后 |
|---------|--------|--------|--------|
| Backend | 5.6秒 | 5.6秒 | 无变化（已最快） |
| Frontend | 53秒 | 35秒 | **-18秒** |
| Desktop | 12秒 | 12秒 | 无变化 |
| electron-builder | 43秒 | 20秒 | **-23秒** |
| npm install | 8秒 | 3秒 | **-5秒** |
| **总计（无修改）** | 115秒 | **20秒** | **-95秒** |

---

## 最佳实践

1. **日常开发**：使用 `build-fast.ps1`
2. **正式发布**：使用 `build.ps1`（包含完整优化）
3. **快速测试**：设置 `$env:COLINK_DEV_BUILD = "true"`
4. **清理缓存**：出现问题时删除 `.build-cache`

---

## 反馈与改进

如果遇到问题或有优化建议，请：
1. 检查 `.build-cache` 是否正常
2. 测试构建时间：`Measure-Command { .\build-fast.ps1 }`
3. 提交反馈到团队