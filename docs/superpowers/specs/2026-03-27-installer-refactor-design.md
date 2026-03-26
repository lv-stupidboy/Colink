---
name: Windows安装器设计 v2
description: ISDP Windows平台安装器设计方案修订版，修复嵌套打包问题，优化ZIP包结构和启动器功能
type: project
---

# ISDP Windows 安装器设计文档 v2

## 修订说明

本版本是对 v1 设计的重大修订，主要解决以下问题：

| 问题 | 原设计 | 修订后 |
|------|--------|--------|
| 嵌套打包 | launcher 包含完整 Electron 应用，导致 1.6GB app.asar | 精确控制打包内容，预期 ~20MB |
| ZIP 结构 | 单一 exe 安装器 | 包含安装器 + 运行时 + 启动器的完整包 |
| 启动器功能 | 系统托盘 + 后台运行 | 面板窗口，无托盘，无开机自启 |
| 升级支持 | 未明确 | 支持重复安装和版本升级 |

---

## 一、交付物结构

### 1.1 用户下载的 ZIP 包

```
ISDP-1.0.0.zip                    # 压缩包 ~300MB
└── ISDP/                         # 解压后的根目录
    │
    ├── ISDP-Setup.exe            # 安装向导程序 (~50MB)
    │
    ├── runtime/                  # 运行时依赖
    │   ├── isdp-server.exe       # Go 后端服务 (27MB)
    │   └── web/                  # 前端静态文件 (24MB)
    │       ├── index.html
    │       └── assets/
    │
    └── launcher/                 # 启动器组件 (~170MB)
        ├── ISDP.exe              # 启动器主程序
        ├── resources/
        │   ├── app.asar          # 启动器代码 (~20MB)
        │   └── icon.ico
        ├── locales/              # 国际化文件
        ├── *.dll                 # Electron 运行时
        ├── *.pak
        └── icudtl.dat
```

### 1.2 安装后的目标目录

```
C:\Users\<用户>\ISDP\              # 用户自选目录
│
├── ISDP.exe                      # 启动器（从 launcher/ 复制）
├── resources/
│   ├── app.asar
│   └── icon.ico
├── locales/
├── *.dll, *.pak
│
├── isdp-server.exe               # 后端服务
├── web/                          # 前端静态文件
│
└── data/                         # 用户数据（安装时创建）
    ├── configs/
    │   └── config.yaml          # 数据库配置等
    ├── logs/
    ├── agent-assets/
    ├── agent-configs/
    └── repos/
```

---

## 二、安装流程

### 2.1 流程图

```
[欢迎] → [目录选择] → [依赖检测] → [模式选择] → [系统配置] → [安装] → [完成]
```

> **注意**：[模式选择] 仅在有缺失依赖时显示，让用户选择自动安装或手动安装。

### 2.2 各步骤详情

#### Step 1: 欢迎

- 展示 ISDP 品牌 Logo 和版本信息
- 说明安装向导将完成的任务
- 提供"开始安装"按钮

#### Step 2: 目录选择

- 用户自选安装位置
- 显示所需空间 (~500MB) 和可用空间
- **升级检测**：若目录已存在，提示"升级"或"重新安装"

#### Step 3: 依赖检测

| 依赖 | 必需 | 检测命令 |
|------|------|----------|
| Node.js | 否 | `node --version` |
| Git | 否 | `git --version` |
| Claude CLI | 否 | `claude --version` |

显示检测结果，对于缺失的依赖：
- 提供"自动安装"按钮（执行 `npm install -g`）
- 提供"手动安装"提示（显示安装命令）
- 允许跳过

#### Step 4: 数据库配置

- MySQL 连接信息
- 支持从已有配置读取（升级场景）
- 测试连接按钮

#### Step 5: 安装进度

```
┌─────────────────────────────────────────────┐
│ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓░░░░░░░ 65%                │
└─────────────────────────────────────────────┘
正在复制启动器文件...
```

进度步骤：
1. 停止已有服务（升级场景）
2. 复制启动器文件
3. 复制运行时文件
4. 生成配置文件
5. 创建快捷方式
6. 写入注册表

#### Step 6: 完成

- 显示安装成功
- 显示安装位置
- 可选项：
  - [✓] 创建桌面快捷方式
  - [✓] 创建开始菜单快捷方式
  - [ ] 立即启动 ISDP

---

## 三、启动器设计

### 3.1 功能定位

启动器是一个独立的控制面板应用，**不是系统托盘程序**。

### 3.2 界面设计

```
┌─────────────────────────────────────────────────────────┐
│  ┌──────┐ ISDP 控制面板 v1.0.0             [_][□][×]   │
│  │ ISDP │                                               │
│  └──────┘                                               │
├─────────────────────────────────────────────────────────┤
│                                                         │
│   服务状态                                              │
│   ┌─────────────────────────────────────────────┐       │
│   │  后端服务    ● 运行中    [停止]             │       │
│   │  访问地址: http://localhost:8080            │       │
│   └─────────────────────────────────────────────┘       │
│                                                         │
│   快速操作                                              │
│   [打开 Web 界面]    [查看日志]    [打开配置目录]        │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 3.3 功能清单

| 功能 | 说明 |
|------|------|
| 服务状态 | 显示后端服务运行状态 |
| 启动/停止 | 控制后端服务 |
| 打开 Web 界面 | 在浏览器打开 http://localhost:8080 |
| 查看日志 | 打开日志目录 |
| 打开配置目录 | 打开 data/configs/ 目录 |

**移除的功能**：
- ~~系统托盘图标~~
- ~~开机自启~~
- ~~后台静默运行~~

### 3.4 启动行为

```
双击 ISDP.exe
    ↓
显示控制面板窗口
    ↓
自动启动后端服务
    ↓
用户使用平台
```

关闭窗口时：
- 服务继续运行（后台）
- 或询问是否停止服务

---

## 四、升级与容错

### 4.1 重复安装

当检测到目标目录已存在时：

```
┌─────────────────────────────────────────┐
│ 检测到已安装 ISDP 1.0.0                  │
│                                         │
│ ○ 升级到 1.1.0（保留数据）              │
│ ○ 重新安装（保留数据）                  │
│ ○ 全新安装（清除所有数据）              │
│                                         │
│ [取消]              [继续]              │
└─────────────────────────────────────────┘
```

### 4.2 数据保护

| 目录 | 升级时 | 全新安装时 |
|------|--------|-----------|
| `data/configs/` | 保留 | 创建默认 |
| `data/logs/` | 保留 | 清空 |
| `data/agent-assets/` | 保留 | 保留（若存在） |
| `data/repos/` | 保留 | 保留（若存在） |

### 4.3 错误处理

| 错误 | 处理方式 |
|------|----------|
| 文件被占用 | 提示关闭 ISDP 后重试，或提供强制关闭选项 |
| 权限不足 | 提示以管理员身份运行 |
| 磁盘空间不足 | 提前检测并提示，阻止继续 |
| 复制失败 | 显示具体错误，提供重试按钮 |

---

## 五、构建流程修复

### 5.1 问题根因

原构建流程存在嵌套打包问题：

```
resources/launcher/win-unpacked/
└── resources/app/
    ├── node_modules/     ← 不应包含
    ├── release/          ← 递归嵌套
    └── src/              ← 源码不应包含
```

### 5.2 修复后的构建流程

```bash
# Step 1: 构建后端
cd isdp && make build
mkdir -p ../installer/resources/app
cp bin/isdp.exe ../installer/resources/app/isdp-server.exe

# Step 2: 构建前端
cd isdp/web && npm run build
mkdir -p ../../installer/resources/app/web
cp -r dist/* ../../installer/resources/app/web/

# Step 3: 安装依赖并构建安装器代码
cd installer
npm install
npm run build

# Step 4: 打包启动器（输出到 release/launcher/）
npm run package:launcher

# Step 5: 打包安装器（会从 release/launcher/ 读取启动器）
npm run package

# Step 6: 创建 ZIP 包
npm run package:zip
```

> **清理时机**：`npm run package:launcher` 会先清理 `release/launcher/` 目录再打包。不需要手动清理。

### 5.3 electron-builder.launcher.yml 修改

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

asar: true

# 精确控制打包内容
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

### 5.4 electron-builder.yml 修改

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

asar: true

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
  # 启动器
  - from: "release/launcher"
    to: "launcher"
```

> **注意**：启动器先构建到 `release/launcher/`，然后安装器打包时从这里复制。

### 5.5 预期包大小

| 组件 | 大小 |
|------|------|
| ISDP-Setup.exe（安装器） | ~50MB |
| launcher/（启动器） | ~170MB |
| runtime/（后端+前端） | ~51MB |
| **总计** | **~270MB** |

---

## 六、重构任务清单

> **重要说明**：这是一次重构，需要删除现有代码并重新组织。

### 6.1 需要删除的文件

| 文件 | 原因 |
|------|------|
| `src/main/tray.ts` | 移除系统托盘功能 |
| `src/main/launcher-entry.ts` | 原托盘启动器入口，需要重写 |

### 6.2 需要修改的文件

| 文件 | 修改内容 |
|------|----------|
| `src/main/index.ts` | 删除 `createTray()` 调用，修改窗口关闭逻辑为直接退出 |
| `src/renderer/src/pages/Dashboard.tsx` | 删除托盘相关提示文字 |
| `electron-builder.yml` | 按本文档更新配置 |
| `electron-builder.launcher.yml` | 按本文档更新配置 |
| `package.json` | 添加 `package:launcher` 脚本 |
| `build.ps1` / `build.sh` | 按本文档更新构建流程 |

### 6.3 窗口关闭行为（明确）

关闭窗口时：
1. 弹出确认对话框："关闭 ISDP 控制面板？后端服务将继续运行。"
2. 用户确认后，窗口关闭，服务继续后台运行
3. 用户可通过桌面快捷方式或开始菜单重新打开控制面板

**避免用户困惑**：
- 窗口显示服务状态，用户知道服务在运行
- 关闭时明确告知服务继续运行
- 任务管理器可见 `isdp-server.exe` 进程

---

## 七、文件清单

```
installer/
├── package.json                      # 添加 package:launcher 脚本
├── tsconfig.json
├── electron-builder.yml              # 修改配置
├── electron-builder.launcher.yml     # 修改配置
├── electron.vite.config.ts
│
├── scripts/
│   └── create-zip.js
│
├── src/
│   ├── main/
│   │   ├── index.ts                  # 修改：删除托盘调用
│   │   ├── installer.ts
│   │   ├── service-manager.ts
│   │   └── tray.ts                   # 删除
│   │
│   ├── preload/
│   │   └── index.ts
│   │
│   └── renderer/
│       ├── index.html
│       └── src/
│           ├── App.tsx
│           ├── types/
│           │   └── index.ts
│           ├── pages/
│           │   ├── Welcome.tsx
│           │   ├── DirectorySelect.tsx
│           │   ├── DependencyCheck.tsx
│           │   ├── ModeSelect.tsx     # 保留：让用户选择手动/自动安装
│           │   ├── SystemConfig.tsx
│           │   ├── Installing.tsx
│           │   ├── Complete.tsx
│           │   └── Dashboard.tsx      # 修改：移除托盘提示
│           └── components/
│               └── Layout.tsx
│
├── resources/
│   ├── isdp-server.exe               # 构建时复制
│   ├── web/                          # 构建时复制
│   └── launcher/                     # 构建时生成
│
└── build/
    └── icon.ico
```

---

## 八、验证方法

1. **构建验证**
   - 检查 `app.asar` 大小应 < 50MB
   - 检查最终 ZIP 包大小应 < 350MB

2. **安装验证**
   - 全新安装：安装成功，服务可启动
   - 重复安装：数据保留，升级成功
   - 权限测试：普通用户安装到用户目录

3. **启动器验证**
   - 控制面板正常显示
   - 服务启停功能正常
   - 快捷方式可用

4. **卸载验证**
   - 通过控制面板卸载
   - 用户数据保留（询问后）

---

## 九、后续扩展

1. 版本自动检测和更新提示
2. 静默安装模式（命令行参数）
3. 多语言支持
4. 配置导入/导出