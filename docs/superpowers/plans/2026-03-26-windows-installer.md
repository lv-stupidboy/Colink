# Windows 安装器实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 创建ISDP Windows平台的图形化安装器，包含安装向导、依赖管理、系统配置和启动器功能。

**Architecture:** 基于Electron + React的单体应用，安装器主进程负责安装逻辑和文件操作，渲染进程提供7步安装向导UI。安装完成后生成独立的启动器程序。

**Tech Stack:** Electron 30+, React 18, TypeScript, Ant Design 5.x, electron-vite, electron-builder, mysql2

**Spec:** `docs/superpowers/specs/2026-03-26-windows-installer-design.md`

---

## 文件结构

```
installer/
├── package.json                    # 项目配置
├── tsconfig.json                   # TypeScript配置
├── tsconfig.node.json              # Node.js相关TS配置
├── electron.vite.config.ts         # Vite配置
├── electron-builder.yml            # 打包配置
│
├── src/
│   ├── main/                       # 主进程
│   │   ├── index.ts                # Electron入口
│   │   ├── installer.ts            # 安装核心逻辑
│   │   ├── service-manager.ts      # 服务管理
│   │   ├── tray.ts                 # 托盘管理
│   │   └── utils.ts                # 工具函数
│   │
│   ├── preload/                    # 预加载脚本
│   │   └── index.ts
│   │
│   └── renderer/                   # 渲染进程
│       ├── index.html
│       └── src/
│           ├── main.tsx            # React入口
│           ├── App.tsx             # 主组件
│           ├── pages/              # 安装向导页面
│           │   ├── Welcome.tsx
│           │   ├── DirectorySelect.tsx
│           │   ├── DependencyCheck.tsx
│           │   ├── ModeSelect.tsx
│           │   ├── SystemConfig.tsx
│           │   ├── Installing.tsx
│           │   └── Complete.tsx
│           ├── components/         # 通用组件
│           │   ├── Layout.tsx
│           │   ├── StepNav.tsx
│           │   └── ConfigSection.tsx
│           ├── services/           # 服务层
│           │   ├── dependency-checker.ts
│           │   ├── npm-installer.ts
│           │   ├── config-generator.ts
│           │   └── database-connector.ts
│           ├── types/              # 类型定义
│           │   └── index.ts
│           └── styles/
│               └── global.css
│
├── resources/                      # 资源文件
│   └── icon.ico
│
└── build/                          # 构建产物
    └── icon.ico
```

---

## Task 1: 项目初始化

**Files:**
- Create: `installer/package.json`
- Create: `installer/tsconfig.json`
- Create: `installer/tsconfig.node.json`
- Create: `installer/.gitignore`

- [ ] **Step 1: 创建 installer 目录**

```bash
mkdir -p D:/00-codes/isdp/isdp/installer
```

- [ ] **Step 2: 创建 package.json**

创建文件 `installer/package.json`:

```json
{
  "name": "isdp-installer",
  "version": "1.0.0",
  "description": "ISDP Windows Installer",
  "main": "./out/main/index.js",
  "scripts": {
    "dev": "electron-vite dev",
    "build": "electron-vite build",
    "postinstall": "electron-builder install-app-deps",
    "package": "electron-builder --win",
    "package:dir": "electron-builder --win --dir"
  },
  "dependencies": {
    "antd": "^5.15.0",
    "mysql2": "^3.9.0",
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "yaml": "^2.3.0"
  },
  "devDependencies": {
    "@types/node": "^20.11.0",
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0",
    "@vitejs/plugin-react": "^4.2.0",
    "electron": "^30.0.0",
    "electron-builder": "^24.13.0",
    "electron-vite": "^2.0.0",
    "typescript": "^5.3.0",
    "vite": "^5.1.0"
  }
}
```

- [ ] **Step 3: 创建 tsconfig.json**

创建文件 `installer/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/renderer/src/*"]
    }
  },
  "include": ["src/renderer/src"]
}
```

- [ ] **Step 4: 创建 tsconfig.node.json**

创建文件 `installer/tsconfig.node.json`:

```json
{
  "compilerOptions": {
    "composite": true,
    "skipLibCheck": true,
    "module": "ESNext",
    "moduleResolution": "bundler",
    "allowSyntheticDefaultImports": true,
    "strict": true
  },
  "include": ["electron.vite.config.ts", "src/main/**/*", "src/preload/**/*"]
}
```

- [ ] **Step 5: 创建 .gitignore**

创建文件 `installer/.gitignore`:

```
node_modules/
out/
dist/
*.log
.DS_Store
```

- [ ] **Step 6: 安装依赖**

```bash
cd D:/00-codes/isdp/isdp/installer && npm install
```

- [ ] **Step 7: Commit**

```bash
git add installer/
git commit -m "feat(installer): initialize project structure"
```

---

## Task 2: Vite 和 Electron Builder 配置

**Files:**
- Create: `installer/electron.vite.config.ts`
- Create: `installer/electron-builder.yml`

- [ ] **Step 1: 创建 electron.vite.config.ts**

创建文件 `installer/electron.vite.config.ts`:

```typescript
import { resolve } from 'path'
import { defineConfig, externalizeDepsPlugin } from 'electron-vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  main: {
    plugins: [externalizeDepsPlugin()]
  },
  preload: {
    plugins: [externalizeDepsPlugin()]
  },
  renderer: {
    resolve: {
      alias: {
        '@': resolve('src/renderer/src')
      }
    },
    plugins: [react()]
  }
})
```

- [ ] **Step 2: 创建 electron-builder.yml**

创建文件 `installer/electron-builder.yml`:

```yaml
appId: com.isdp.installer
productName: ISDP-Setup
directories:
  output: release/${version}

win:
  target:
    - target: nsis
      arch: [x64]
  icon: build/icon.ico

nsis:
  oneClick: false
  perMachine: true
  allowElevation: true
  allowToChangeInstallationDirectory: false
  installerIcon: build/icon.ico
  uninstallerIcon: build/icon.ico
  createDesktopShortcut: false
  createStartMenuShortcut: false
  shortcutName: ISDP

extraResources:
  - from: "resources/app"
    to: "app"
    filter:
      - "**/*"
```

- [ ] **Step 3: Commit**

```bash
git add installer/
git commit -m "feat(installer): add vite and electron-builder config"
```

---

## Task 3: 主进程入口

**Files:**
- Create: `installer/src/main/index.ts`

- [ ] **Step 1: 创建主进程入口**

创建文件 `installer/src/main/index.ts`:

```typescript
import { app, BrowserWindow, ipcMain } from 'electron'
import { join } from 'path'

// 判断是否为开发模式
const isDev = !app.isPackaged

let mainWindow: BrowserWindow | null = null

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 900,
    height: 600,
    minWidth: 800,
    minHeight: 500,
    frame: false,  // 无边框窗口
    resizable: true,
    webPreferences: {
      preload: join(__dirname, '../preload/index.js'),
      contextIsolation: true,
      nodeIntegration: false
    }
  })

  // 开发模式加载本地服务器，生产模式加载打包文件
  if (isDev) {
    mainWindow.loadURL('http://localhost:5173')
    mainWindow.webContents.openDevTools()
  } else {
    mainWindow.loadFile(join(__dirname, '../renderer/index.html'))
  }
}

app.whenReady().then(() => {
  createWindow()

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow()
    }
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit()
  }
})

// IPC 处理：窗口控制
ipcMain.on('window-minimize', () => mainWindow?.minimize())
ipcMain.on('window-close', () => mainWindow?.close())

// IPC 处理：获取应用路径
ipcMain.handle('get-app-path', () => app.getAppPath())

// IPC 处理：获取资源路径
ipcMain.handle('get-resource-path', () => {
  return isDev
    ? join(__dirname, '../../resources')
    : process.resourcesPath
})
```

- [ ] **Step 2: Commit**

```bash
git add installer/
git commit -m "feat(installer): add main process entry"
```

---

## Task 4: 预加载脚本

**Files:**
- Create: `installer/src/preload/index.ts`

- [ ] **Step 1: 创建预加载脚本**

创建文件 `installer/src/preload/index.ts`:

```typescript
import { contextBridge, ipcRenderer } from 'electron'

// 暴露安全的 API 给渲染进程
contextBridge.exposeInMainWorld('electronAPI', {
  // 窗口控制
  minimizeWindow: () => ipcRenderer.send('window-minimize'),
  closeWindow: () => ipcRenderer.send('window-close'),

  // 路径获取
  getAppPath: () => ipcRenderer.invoke('get-app-path'),
  getResourcePath: () => ipcRenderer.invoke('get-resource-path'),

  // 安装相关
  checkDependency: (dep: string) => ipcRenderer.invoke('check-dependency', dep),
  installDependency: (dep: string) => ipcRenderer.invoke('install-dependency', dep),
  copyFiles: (src: string, dest: string) => ipcRenderer.invoke('copy-files', src, dest),
  generateConfig: (config: object) => ipcRenderer.invoke('generate-config', config),
  testDatabaseConnection: (config: object) => ipcRenderer.invoke('test-database-connection', config),
  createShortcut: (path: string) => ipcRenderer.invoke('create-shortcut', path),

  // 进度回调
  onInstallProgress: (callback: (progress: any) => void) => {
    ipcRenderer.on('install-progress', (_event, progress) => callback(progress))
  }
})
```

- [ ] **Step 2: Commit**

```bash
git add installer/
git commit -m "feat(installer): add preload script"
```

---

## Task 5: 类型定义

**Files:**
- Create: `installer/src/renderer/src/types/index.ts`

- [ ] **Step 1: 创建类型定义**

创建文件 `installer/src/renderer/src/types/index.ts`:

```typescript
// 安装步骤
export type StepId = 1 | 2 | 3 | 4 | 5 | 6 | 7

// 依赖项
export interface Dependency {
  name: string           // 显示名称
  key: string            // 标识符: 'nodejs' | 'git' | 'claude' | 'opencode'
  required: boolean      // 是否必需
  version?: string       // 检测到的版本
  installed: boolean     // 是否已安装
}

// 安装模式
export type InstallMode = 'auto' | 'manual' | 'skip'

// 数据库配置
export interface DatabaseConfig {
  host: string
  port: number
  database: string
  username: string
  password: string
}

// 安装配置
export interface InstallConfig {
  installDir: string
  installMode: InstallMode
  dependencies: Dependency[]
  database: DatabaseConfig
  createShortcut: boolean
  launchNow: boolean
}

// 安装进度
export interface InstallProgress {
  step: string
  status: 'pending' | 'running' | 'success' | 'failed'
  progress?: number  // 0-100
  message?: string
}

// Electron API 类型声明
declare global {
  interface Window {
    electronAPI: {
      minimizeWindow: () => void
      closeWindow: () => void
      getAppPath: () => Promise<string>
      getResourcePath: () => Promise<string>
      checkDependency: (dep: string) => Promise<{ installed: boolean; version?: string }>
      installDependency: (dep: string) => Promise<{ success: boolean; error?: string }>
      copyFiles: (src: string, dest: string) => Promise<{ success: boolean; error?: string }>
      generateConfig: (config: object) => Promise<{ success: boolean; error?: string }>
      testDatabaseConnection: (config: object) => Promise<{ success: boolean; error?: string }>
      createShortcut: (path: string) => Promise<{ success: boolean }>
      onInstallProgress: (callback: (progress: InstallProgress) => void) => void
    }
  }
}

export {}
```

- [ ] **Step 2: Commit**

```bash
git add installer/
git commit -m "feat(installer): add type definitions"
```

---

## Task 6: 全局样式

**Files:**
- Create: `installer/src/renderer/src/styles/global.css`

- [ ] **Step 1: 创建全局样式**

创建文件 `installer/src/renderer/src/styles/global.css`:

```css
:root {
  /* ISDP 翡翠绿主题 */
  --primary-color: #10b981;
  --primary-hover: #34d399;
  --primary-active: #059669;
  --primary-bg: #d1fae5;
  --primary-light: #ecfdf5;

  /* 功能色 */
  --success-color: #52c41a;
  --warning-color: #faad14;
  --error-color: #ff4d4f;

  /* 中性色 */
  --text-primary: #333333;
  --text-secondary: #666666;
  --border-color: #e8e8e8;
  --bg-base: #ffffff;
  --bg-secondary: #f5f7fa;
}

* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

html, body, #root {
  height: 100%;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
}

/* 标题栏样式 */
.title-bar {
  height: 48px;
  background: linear-gradient(90deg, var(--primary-color) 0%, var(--primary-active) 100%);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  -webkit-app-region: drag;
  user-select: none;
}

.title-bar h1 {
  color: #fff;
  font-size: 16px;
  font-weight: 500;
}

.title-bar-controls {
  display: flex;
  gap: 8px;
  -webkit-app-region: no-drag;
}

.title-bar-btn {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: none;
  cursor: pointer;
}

.title-bar-btn.minimize { background: #ffbd2e; }
.title-bar-btn.close { background: #ff5f57; }

/* 步骤导航 */
.step-nav {
  width: 180px;
  background: var(--bg-secondary);
  padding: 24px 16px;
  border-right: 1px solid var(--border-color);
}

.step-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  margin-bottom: 8px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.3s;
}

.step-item:hover {
  background: rgba(16, 185, 129, 0.1);
}

.step-item.active {
  background: var(--primary-bg);
  color: var(--primary-color);
}

.step-item.completed {
  color: var(--success-color);
}

.step-number {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 500;
  border: 2px solid var(--border-color);
  background: var(--bg-base);
}

.step-item.active .step-number {
  border-color: var(--primary-color);
  color: var(--primary-color);
}

.step-item.completed .step-number {
  border-color: var(--success-color);
  background: var(--success-color);
  color: #fff;
}

/* 内容区域 */
.content-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 30px 40px;
}

/* 底部按钮 */
.footer-buttons {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding-top: 20px;
  border-top: 1px solid var(--border-color);
  margin-top: auto;
}
```

- [ ] **Step 2: Commit**

```bash
git add installer/
git commit -m "feat(installer): add global styles"
```

---

## Task 7: React 入口和主组件

**Files:**
- Create: `installer/src/renderer/index.html`
- Create: `installer/src/renderer/src/main.tsx`
- Create: `installer/src/renderer/src/App.tsx`

- [ ] **Step 1: 创建 index.html**

创建文件 `installer/src/renderer/index.html`:

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="Content-Security-Policy" content="default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'">
  <title>ISDP 安装向导</title>
</head>
<body>
  <div id="root"></div>
  <script type="module" src="./src/main.tsx"></script>
</body>
</html>
```

- [ ] **Step 2: 创建 main.tsx**

创建文件 `installer/src/renderer/src/main.tsx`:

```typescript
import React from 'react'
import ReactDOM from 'react-dom/client'
import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import App from './App'
import './styles/global.css'

const theme = {
  token: {
    colorPrimary: '#10b981',
    colorSuccess: '#52c41a',
    colorWarning: '#faad14',
    colorError: '#ff4d4f',
    borderRadius: 6,
  },
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ConfigProvider theme={theme} locale={zhCN}>
      <App />
    </ConfigProvider>
  </React.StrictMode>
)
```

- [ ] **Step 3: 创建 App.tsx**

创建文件 `installer/src/renderer/src/App.tsx`:

```typescript
import { useState } from 'react'
import { Button, Space } from 'antd'
import { StepId, InstallConfig, Dependency } from './types'
import Layout from './components/Layout'
import Welcome from './pages/Welcome'
import DirectorySelect from './pages/DirectorySelect'
import DependencyCheck from './pages/DependencyCheck'
import ModeSelect from './pages/ModeSelect'
import SystemConfig from './pages/SystemConfig'
import Installing from './pages/Installing'
import Complete from './pages/Complete'

const PAGES = {
  1: Welcome,
  2: DirectorySelect,
  3: DependencyCheck,
  4: ModeSelect,
  5: SystemConfig,
  6: Installing,
  7: Complete,
}

const STEP_LABELS = ['欢迎', '目录选择', '依赖检测', '模式选择', '系统配置', '安装', '完成']

export default function App() {
  const [currentStep, setCurrentStep] = useState<StepId>(1)
  const [config, setConfig] = useState<InstallConfig>({
    installDir: 'C:\\Program Files\\ISDP',
    installMode: 'auto',
    dependencies: [],
    database: {
      host: '',
      port: 3306,
      database: 'isdp',
      username: 'root',
      password: '',
    },
    createShortcut: true,
    launchNow: true,
  })

  const [hasMissingDeps, setHasMissingDeps] = useState(false)

  const PageComponent = PAGES[currentStep]

  const handleNext = () => {
    // 如果在依赖检测步骤没有缺失依赖，跳过模式选择
    if (currentStep === 3 && !hasMissingDeps) {
      setCurrentStep(5 as StepId)
    } else if (currentStep < 7) {
      setCurrentStep((currentStep + 1) as StepId)
    }
  }

  const handlePrev = () => {
    // 如果从系统配置返回且没有缺失依赖，跳回依赖检测
    if (currentStep === 5 && !hasMissingDeps) {
      setCurrentStep(3 as StepId)
    } else if (currentStep > 1) {
      setCurrentStep((currentStep - 1) as StepId)
    }
  }

  const handleConfigUpdate = (updates: Partial<InstallConfig>) => {
    setConfig(prev => ({ ...prev, ...updates }))
  }

  const handleDependenciesUpdate = (deps: Dependency[]) => {
    handleConfigUpdate({ dependencies: deps })
    setHasMissingDeps(deps.some(d => !d.installed))
  }

  return (
    <Layout
      currentStep={currentStep}
      stepLabels={STEP_LABELS}
    >
      <PageComponent
        config={config}
        onConfigUpdate={handleConfigUpdate}
        onDependenciesUpdate={handleDependenciesUpdate}
        onComplete={() => setCurrentStep(7)}
      />

      {currentStep !== 6 && currentStep !== 7 && (
        <div className="footer-buttons">
          <Space>
            {currentStep > 1 && (
              <Button onClick={handlePrev}>上一步</Button>
            )}
            {currentStep === 1 && (
              <Button type="primary" onClick={handleNext}>开始安装</Button>
            )}
            {currentStep > 1 && currentStep < 6 && (
              <Button type="primary" onClick={handleNext}>
                {currentStep === 5 ? '安装' : '下一步'}
              </Button>
            )}
          </Space>
        </div>
      )}
    </Layout>
  )
}
```

- [ ] **Step 4: Commit**

```bash
git add installer/
git commit -m "feat(installer): add React entry and App component"
```

---

## Task 8: Layout 和 StepNav 组件

**Files:**
- Create: `installer/src/renderer/src/components/Layout.tsx`

- [ ] **Step 1: 创建 Layout 组件**

创建文件 `installer/src/renderer/src/components/Layout.tsx`:

```typescript
import { ReactNode } from 'react'

interface LayoutProps {
  currentStep: number
  stepLabels: string[]
  children: ReactNode
}

export default function Layout({ currentStep, stepLabels, children }: LayoutProps) {
  const handleMinimize = () => {
    window.electronAPI?.minimizeWindow()
  }

  const handleClose = () => {
    window.electronAPI?.closeWindow()
  }

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {/* 标题栏 */}
      <div className="title-bar">
        <h1>ISDP 安装向导</h1>
        <div className="title-bar-controls">
          <button className="title-bar-btn minimize" onClick={handleMinimize} />
          <button className="title-bar-btn close" onClick={handleClose} />
        </div>
      </div>

      {/* 主内容区 */}
      <div style={{ flex: 1, display: 'flex', overflow: 'hidden' }}>
        {/* 步骤导航 */}
        <nav className="step-nav">
          {stepLabels.map((label, index) => {
            const stepNum = index + 1
            const isActive = stepNum === currentStep
            const isCompleted = stepNum < currentStep

            return (
              <div
                key={stepNum}
                className={`step-item ${isActive ? 'active' : ''} ${isCompleted ? 'completed' : ''}`}
              >
                <div className="step-number">
                  {isCompleted ? '✓' : stepNum}
                </div>
                <span className="step-label">{label}</span>
              </div>
            )
          })}
        </nav>

        {/* 内容区域 */}
        <main className="content-area">
          {children}
        </main>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add installer/
git commit -m "feat(installer): add Layout component"
```

---

## Task 9: Welcome 页面

**Files:**
- Create: `installer/src/renderer/src/pages/Welcome.tsx`

- [ ] **Step 1: 创建 Welcome 页面**

创建文件 `installer/src/renderer/src/pages/Welcome.tsx`:

```typescript
import { InstallConfig } from '../types'

interface WelcomeProps {
  config: InstallConfig
}

export default function Welcome({}: WelcomeProps) {
  return (
    <div style={{
      flex: 1,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      textAlign: 'center'
    }}>
      <div style={{ fontSize: 80, marginBottom: 20 }}>🚀</div>
      <h2 style={{ fontSize: 24, marginBottom: 12, color: '#333' }}>
        欢迎使用 ISDP 智能开发平台
      </h2>
      <p style={{ color: '#666', lineHeight: 2, marginBottom: 40 }}>
        本向导将帮助您完成：<br />
        · 安装 ISDP 核心程序<br />
        · 检测并安装运行依赖<br />
        · 配置数据库连接
      </p>
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add installer/
git commit -m "feat(installer): add Welcome page"
```

---

## Task 10: DirectorySelect 页面

**Files:**
- Create: `installer/src/renderer/src/pages/DirectorySelect.tsx`

- [ ] **Step 1: 创建 DirectorySelect 页面**

创建文件 `installer/src/renderer/src/renderer/src/pages/DirectorySelect.tsx`:

```typescript
import { Button, Input } from 'antd'
import { FolderOpenOutlined } from '@ant-design/icons'
import { InstallConfig } from '../types'

interface DirectorySelectProps {
  config: InstallConfig
  onConfigUpdate: (updates: Partial<InstallConfig>) => void
}

export default function DirectorySelect({ config, onConfigUpdate }: DirectorySelectProps) {
  const handleBrowse = async () => {
    // TODO: 调用主进程打开目录选择对话框
    // const result = await window.electronAPI.selectDirectory()
    // if (result) onConfigUpdate({ installDir: result })
  }

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>选择安装位置</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>请选择 ISDP 的安装目录</p>

      <div style={{ marginBottom: 20 }}>
        <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 8 }}>
          安装目录
        </label>
        <div style={{ display: 'flex', gap: 12 }}>
          <Input
            value={config.installDir}
            onChange={(e) => onConfigUpdate({ installDir: e.target.value })}
            style={{ flex: 1 }}
          />
          <Button icon={<FolderOpenOutlined />} onClick={handleBrowse}>
            浏览...
          </Button>
        </div>
      </div>

      <div style={{ display: 'flex', gap: 40, color: '#666', fontSize: 14 }}>
        <span>所需空间：约 500 MB</span>
        <span>可用空间：120 GB</span>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add installer/
git commit -m "feat(installer): add DirectorySelect page"
```

---

## Task 11: DependencyCheck 页面

**Files:**
- Create: `installer/src/renderer/src/pages/DependencyCheck.tsx`
- Create: `installer/src/renderer/src/services/dependency-checker.ts`

- [ ] **Step 1: 创建 dependency-checker 服务**

创建文件 `installer/src/renderer/src/services/dependency-checker.ts`:

```typescript
import { Dependency } from '../types'

const DEPENDENCIES_CONFIG = [
  { name: 'Node.js', key: 'nodejs', required: true, command: 'node --version' },
  { name: 'Git', key: 'git', required: true, command: 'git --version' },
  { name: 'Claude CLI', key: 'claude', required: false, command: 'claude --version' },
  { name: 'OpenCode', key: 'opencode', required: false, command: 'opencode --version' },
]

export async function checkAllDependencies(): Promise<Dependency[]> {
  const results: Dependency[] = []

  for (const dep of DEPENDENCIES_CONFIG) {
    try {
      const result = await window.electronAPI.checkDependency(dep.key)
      results.push({
        name: dep.name,
        key: dep.key,
        required: dep.required,
        installed: result.installed,
        version: result.version,
      })
    } catch {
      results.push({
        name: dep.name,
        key: dep.key,
        required: dep.required,
        installed: false,
      })
    }
  }

  return results
}
```

- [ ] **Step 2: 创建 DependencyCheck 页面**

创建文件 `installer/src/renderer/src/pages/DependencyCheck.tsx`:

```typescript
import { useEffect, useState } from 'react'
import { Spin, Tag } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons'
import { Dependency } from '../types'
import { checkAllDependencies } from '../services/dependency-checker'

interface DependencyCheckProps {
  config: InstallConfig
  onDependenciesUpdate: (deps: Dependency[]) => void
}

export default function DependencyCheck({ onDependenciesUpdate }: DependencyCheckProps) {
  const [loading, setLoading] = useState(true)
  const [dependencies, setDependencies] = useState<Dependency[]>([])

  useEffect(() => {
    checkAllDependencies().then(deps => {
      setDependencies(deps)
      onDependenciesUpdate(deps)
      setLoading(false)
    })
  }, [onDependenciesUpdate])

  const missingCount = dependencies.filter(d => !d.installed).length

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>依赖检测</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>
        {loading ? '正在检测系统运行环境...' : '检测结果如下'}
      </p>

      {loading ? (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <Spin size="large" />
        </div>
      ) : (
        <>
          <div style={{
            background: '#fafafa',
            borderRadius: 8,
            padding: 20,
            marginBottom: 20
          }}>
            {dependencies.map(dep => (
              <div
                key={dep.key}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '12px 16px',
                  borderBottom: '1px solid #e8e8e8',
                }}
              >
                <span style={{ fontWeight: 500 }}>{dep.name}</span>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <Tag color={dep.installed ? 'success' : 'warning'}>
                    {dep.installed ? `已安装 ${dep.version}` : '未安装'}
                  </Tag>
                </div>
              </div>
            ))}
          </div>

          {missingCount > 0 && (
            <p style={{ color: '#fa8c16', fontWeight: 500 }}>
              检测到 <strong>{missingCount}</strong> 个依赖项缺失
            </p>
          )}
        </>
      )}
    </div>
  )
}
```

- [ ] **Step 3: Commit**

```bash
git add installer/
git commit -m "feat(installer): add DependencyCheck page and service"
```

---

## Task 12: ModeSelect 页面

**Files:**
- Create: `installer/src/renderer/src/pages/ModeSelect.tsx`

- [ ] **Step 1: 创建 ModeSelect 页面**

创建文件 `installer/src/renderer/src/pages/ModeSelect.tsx`:

```typescript
import { Radio, Space } from 'antd'
import { InstallConfig, Dependency, InstallMode } from '../types'

interface ModeSelectProps {
  config: InstallConfig
  onConfigUpdate: (updates: Partial<InstallConfig>) => void
}

export default function ModeSelect({ config, onConfigUpdate }: ModeSelectProps) {
  const missingDeps = config.dependencies.filter(d => !d.installed)

  const handleModeChange = (mode: InstallMode) => {
    onConfigUpdate({ installMode: mode })
  }

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>选择安装方式</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>
        检测到以下依赖未安装：{missingDeps.map(d => d.name).join('、')}
      </p>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div
          onClick={() => handleModeChange('auto')}
          style={{
            padding: 20,
            border: `2px solid ${config.installMode === 'auto' ? '#10b981' : '#e8e8e8'}`,
            borderRadius: 8,
            cursor: 'pointer',
            background: config.installMode === 'auto' ? '#d1fae5' : '#fff',
          }}
        >
          <div style={{ fontWeight: 600, marginBottom: 8 }}>
            <Radio checked={config.installMode === 'auto'} />
            <span style={{ marginLeft: 8 }}>自动安装（推荐）</span>
          </div>
          <p style={{ color: '#666', fontSize: 13, marginLeft: 30 }}>
            安装器将自动下载并安装缺失的依赖项
          </p>
        </div>

        <div
          onClick={() => handleModeChange('manual')}
          style={{
            padding: 20,
            border: `2px solid ${config.installMode === 'manual' ? '#10b981' : '#e8e8e8'}`,
            borderRadius: 8,
            cursor: 'pointer',
            background: config.installMode === 'manual' ? '#d1fae5' : '#fff',
          }}
        >
          <div style={{ fontWeight: 600, marginBottom: 8 }}>
            <Radio checked={config.installMode === 'manual'} />
            <span style={{ marginLeft: 8 }}>手动安装</span>
          </div>
          <p style={{ color: '#666', fontSize: 13, marginLeft: 30 }}>
            我将自行安装依赖，完成后继续
          </p>
        </div>

        <div
          onClick={() => handleModeChange('skip')}
          style={{
            padding: 20,
            border: `2px solid ${config.installMode === 'skip' ? '#10b981' : '#e8e8e8'}`,
            borderRadius: 8,
            cursor: 'pointer',
            background: config.installMode === 'skip' ? '#d1fae5' : '#fff',
          }}
        >
          <div style={{ fontWeight: 600, marginBottom: 8 }}>
            <Radio checked={config.installMode === 'skip'} />
            <span style={{ marginLeft: 8 }}>跳过安装</span>
          </div>
          <p style={{ color: '#666', fontSize: 13, marginLeft: 30 }}>
            暂不安装这些依赖，后续在平台中配置其他 Agent 类型
          </p>
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add installer/
git commit -m "feat(installer): add ModeSelect page"
```

---

## Task 13: ConfigSection 组件

**Files:**
- Create: `installer/src/renderer/src/components/ConfigSection.tsx`

- [ ] **Step 1: 创建 ConfigSection 组件**

创建文件 `installer/src/renderer/src/components/ConfigSection.tsx`:

```typescript
import { ReactNode, useState } from 'react'

interface ConfigSectionProps {
  title: string
  defaultCollapsed?: boolean
  children: ReactNode
}

export default function ConfigSection({
  title,
  defaultCollapsed = false,
  children
}: ConfigSectionProps) {
  const [collapsed, setCollapsed] = useState(defaultCollapsed)

  return (
    <div style={{
      border: '1px solid #e8e8e8',
      borderRadius: 8,
      marginBottom: 16,
      overflow: 'hidden'
    }}>
      <div
        onClick={() => setCollapsed(!collapsed)}
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '14px 16px',
          background: '#fafafa',
          cursor: 'pointer',
          fontWeight: 500,
        }}
      >
        <span>{title}</span>
        <span style={{ transform: collapsed ? 'rotate(-90deg)' : 'none', transition: 'transform 0.3s' }}>
          ▼
        </span>
      </div>
      {!collapsed && (
        <div style={{ padding: 20, borderTop: '1px solid #e8e8e8' }}>
          {children}
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add installer/
git commit -m "feat(installer): add ConfigSection component"
```

---

## Task 14: SystemConfig 页面

**Files:**
- Create: `installer/src/renderer/src/pages/SystemConfig.tsx`
- Create: `installer/src/renderer/src/services/database-connector.ts`

- [ ] **Step 1: 创建 database-connector 服务**

创建文件 `installer/src/renderer/src/services/database-connector.ts`:

```typescript
import { DatabaseConfig } from '../types'

export interface TestResult {
  success: boolean
  error?: string
}

export async function testConnection(config: DatabaseConfig): Promise<TestResult> {
  try {
    const result = await window.electronAPI.testDatabaseConnection(config)
    return result
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '未知错误',
    }
  }
}
```

- [ ] **Step 2: 创建 SystemConfig 页面**

创建文件 `installer/src/renderer/src/pages/SystemConfig.tsx`:

```typescript
import { useState } from 'react'
import { Button, Input, Row, Col, message } from 'antd'
import { CheckCircleOutlined } from '@ant-design/icons'
import ConfigSection from '../components/ConfigSection'
import { InstallConfig } from '../types'
import { testConnection } from '../services/database-connector'

interface SystemConfigProps {
  config: InstallConfig
  onConfigUpdate: (updates: Partial<InstallConfig>) => void
}

export default function SystemConfig({ config, onConfigUpdate }: SystemConfigProps) {
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null)

  const handleDbChange = (field: string, value: string | number) => {
    onConfigUpdate({
      database: { ...config.database, [field]: value }
    })
  }

  const handleTestConnection = async () => {
    setTesting(true)
    setTestResult(null)

    const result = await testConnection(config.database)
    setTesting(false)

    if (result.success) {
      setTestResult({ success: true, message: '连接成功' })
      message.success('数据库连接成功')
    } else {
      setTestResult({ success: false, message: result.error || '连接失败' })
      message.error(result.error || '数据库连接失败')
    }
  }

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>系统配置</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>请配置 ISDP 运行所需的参数</p>

      <ConfigSection title="数据库配置">
        <Row gutter={20}>
          <Col span={16}>
            <div style={{ marginBottom: 16 }}>
              <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 6 }}>
                数据库地址
              </label>
              <Input
                value={config.database.host}
                onChange={(e) => handleDbChange('host', e.target.value)}
                placeholder="rm-xxx.mysql.rds.aliyuncs.com"
              />
            </div>
          </Col>
          <Col span={8}>
            <div style={{ marginBottom: 16 }}>
              <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 6 }}>
                端口
              </label>
              <Input
                type="number"
                value={config.database.port}
                onChange={(e) => handleDbChange('port', parseInt(e.target.value) || 3306)}
              />
            </div>
          </Col>
        </Row>

        <div style={{ marginBottom: 16 }}>
          <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 6 }}>
            数据库名
          </label>
          <Input
            value={config.database.database}
            onChange={(e) => handleDbChange('database', e.target.value)}
          />
        </div>

        <Row gutter={20}>
          <Col span={12}>
            <div style={{ marginBottom: 16 }}>
              <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 6 }}>
                用户名
              </label>
              <Input
                value={config.database.username}
                onChange={(e) => handleDbChange('username', e.target.value)}
              />
            </div>
          </Col>
          <Col span={12}>
            <div style={{ marginBottom: 16 }}>
              <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 6 }}>
                密码
              </label>
              <Input.Password
                value={config.database.password}
                onChange={(e) => handleDbChange('password', e.target.value)}
              />
            </div>
          </Col>
        </Row>

        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Button onClick={handleTestConnection} loading={testing}>
            测试连接
          </Button>
          {testResult && (
            <span style={{ color: testResult.success ? '#52c41a' : '#ff4d4f' }}>
              {testResult.success && <CheckCircleOutlined style={{ marginRight: 4 }} />}
              {testResult.message}
            </span>
          )}
        </div>
      </ConfigSection>

      <ConfigSection title="高级设置（可选）" defaultCollapsed>
        <div style={{ color: '#999', textAlign: 'center', padding: 20 }}>
          预留扩展空间
        </div>
      </ConfigSection>
    </div>
  )
}
```

- [ ] **Step 3: Commit**

```bash
git add installer/
git commit -m "feat(installer): add SystemConfig page and database connector"
```

---

## Task 15: Installing 页面

**Files:**
- Create: `installer/src/renderer/src/pages/Installing.tsx`
- Create: `installer/src/renderer/src/services/config-generator.ts`

- [ ] **Step 1: 创建 config-generator 服务**

创建文件 `installer/src/renderer/src/services/config-generator.ts`:

```typescript
import YAML from 'yaml'
import { InstallConfig } from '../types'

export async function generateConfigFile(config: InstallConfig, targetPath: string): Promise<{ success: boolean; error?: string }> {
  try {
    const yamlContent = YAML.stringify({
      server: {
        port: 8080,
        mode: 'release',
      },
      database: {
        type: 'mysql',
        mysql: {
          host: config.database.host,
          port: config.database.port,
          database: config.database.database,
          username: config.database.username,
          password: config.database.password,
          charset: 'utf8mb4',
        },
      },
      claude: {
        path: 'claude',
        default_model: 'claude-sonnet-4-6',
        timeout: '30m',
      },
      logging: {
        level: 'info',
        format: 'json',
      },
    })

    const result = await window.electronAPI.generateConfig({ path: targetPath, content: yamlContent })
    return result
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '生成配置失败',
    }
  }
}
```

- [ ] **Step 2: 创建 Installing 页面**

创建文件 `installer/src/renderer/src/pages/Installing.tsx`:

```typescript
import { useEffect, useState } from 'react'
import { Progress } from 'antd'
import { CheckCircleOutlined, LoadingOutlined } from '@ant-design/icons'
import { InstallConfig, InstallProgress } from '../types'

interface InstallingProps {
  config: InstallConfig
  onComplete: () => void
}

interface StepProgress {
  step: string
  status: 'pending' | 'running' | 'success' | 'failed'
  progress: number
}

const INSTALL_STEPS = [
  { key: 'copy', label: '复制文件' },
  { key: 'claude', label: '安装 Claude CLI' },
  { key: 'opencode', label: '安装 OpenCode' },
  { key: 'config', label: '生成配置文件' },
]

export default function Installing({ config, onComplete }: InstallingProps) {
  const [steps, setSteps] = useState<StepProgress[]>(
    INSTALL_STEPS.map(s => ({ step: s.key, status: 'pending', progress: 0 }))
  )

  useEffect(() => {
    // 监听安装进度
    window.electronAPI.onInstallProgress((progress: InstallProgress) => {
      setSteps(prev => prev.map(s =>
        s.step === progress.step
          ? { ...s, status: progress.status, progress: progress.progress || 0 }
          : s
      ))

      if (progress.status === 'success' && progress.step === 'config') {
        setTimeout(onComplete, 500)
      }
    })

    // TODO: 触发安装流程
    // 实际实现中需要调用主进程开始安装
  }, [onComplete])

  const getStepIcon = (status: string) => {
    switch (status) {
      case 'success': return <CheckCircleOutlined style={{ color: '#52c41a' }} />
      case 'running': return <LoadingOutlined style={{ color: '#10b981' }} />
      default: return <span style={{ color: '#d9d9d9' }}>○</span>
    }
  }

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>正在安装...</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>请稍候，安装程序正在配置您的系统</p>

      <div>
        {steps.map((step, index) => (
          <div
            key={step.step}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 16,
              padding: '16px 0',
              borderBottom: index < steps.length - 1 ? '1px solid #f0f0f0' : 'none',
            }}
          >
            <div style={{ width: 24 }}>{getStepIcon(step.status)}</div>
            <div style={{ flex: 1 }}>{INSTALL_STEPS.find(s => s.key === step.step)?.label}</div>
            {step.status === 'running' && (
              <div style={{ width: 150 }}>
                <Progress percent={step.progress} size="small" />
              </div>
            )}
            {step.status === 'success' && (
              <span style={{ color: '#52c41a' }}>完成</span>
            )}
            {step.status === 'pending' && (
              <span style={{ color: '#999' }}>等待中</span>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
```

- [ ] **Step 3: Commit**

```bash
git add installer/
git commit -m "feat(installer): add Installing page and config generator"
```

---

## Task 16: Complete 页面

**Files:**
- Create: `installer/src/renderer/src/pages/Complete.tsx`

- [ ] **Step 1: 创建 Complete 页面**

创建文件 `installer/src/renderer/src/pages/Complete.tsx`:

```typescript
import { Checkbox, Button, message } from 'antd'
import { InstallConfig } from '../types'

interface CompleteProps {
  config: InstallConfig
  onConfigUpdate: (updates: Partial<InstallConfig>) => void
}

export default function Complete({ config, onConfigUpdate }: CompleteProps) {
  const handleFinish = () => {
    // 创建快捷方式
    if (config.createShortcut) {
      window.electronAPI.createShortcut(config.installDir)
    }

    message.success('安装完成！')

    // 如果选择立即启动，启动服务
    if (config.launchNow) {
      // TODO: 启动服务
    }

    // 关闭安装器
    window.electronAPI.closeWindow()
  }

  return (
    <div style={{
      flex: 1,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      textAlign: 'center'
    }}>
      <div style={{
        width: 80,
        height: 80,
        background: '#52c41a',
        borderRadius: '50%',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontSize: 40,
        color: '#fff',
        marginBottom: 24
      }}>
        ✓
      </div>

      <h2 style={{ fontSize: 22, marginBottom: 16, color: '#333' }}>安装完成！</h2>

      <div style={{
        background: '#f5f5f5',
        padding: '12px 20px',
        borderRadius: 6,
        marginBottom: 24,
        fontFamily: 'monospace',
        color: '#666'
      }}>
        安装位置：{config.installDir}
      </div>

      <div style={{ marginBottom: 30 }}>
        <div style={{ marginBottom: 10 }}>
          <Checkbox
            checked={config.createShortcut}
            onChange={(e) => onConfigUpdate({ createShortcut: e.target.checked })}
          >
            创建桌面快捷方式
          </Checkbox>
        </div>
        <div>
          <Checkbox
            checked={config.launchNow}
            onChange={(e) => onConfigUpdate({ launchNow: e.target.checked })}
          >
            立即启动 ISDP
          </Checkbox>
        </div>
      </div>

      <Button type="primary" size="large" onClick={handleFinish}>
        完成
      </Button>
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add installer/
git commit -m "feat(installer): add Complete page"
```

---

## Task 17: 主进程安装逻辑

**Files:**
- Modify: `installer/src/main/index.ts` - 添加 IPC handlers
- Create: `installer/src/main/installer.ts`

- [ ] **Step 1: 创建 installer.ts**

创建文件 `installer/src/main/installer.ts`:

```typescript
import { exec } from 'child_process'
import { promisify } from 'util'
import { copyFile, mkdir, writeFile } from 'fs/promises'
import { join, dirname } from 'path'
import { app, BrowserWindow, shell } from 'electron'

const execAsync = promisify(exec)

export interface DependencyCheckResult {
  installed: boolean
  version?: string
}

// 检测依赖
export async function checkDependency(key: string): Promise<DependencyCheckResult> {
  const commands: Record<string, string> = {
    nodejs: 'node --version',
    git: 'git --version',
    claude: 'claude --version',
    opencode: 'opencode --version',
  }

  const cmd = commands[key]
  if (!cmd) return { installed: false }

  try {
    const { stdout } = await execAsync(cmd)
    const versionMatch = stdout.match(/(\d+\.\d+\.\d+)/)
    return {
      installed: true,
      version: versionMatch ? versionMatch[1] : stdout.trim(),
    }
  } catch {
    return { installed: false }
  }
}

// 安装 npm 包
export async function installNpmPackage(packageName: string): Promise<{ success: boolean; error?: string }> {
  try {
    await execAsync(`npm install -g ${packageName}`, { timeout: 120000 })
    return { success: true }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '安装失败',
    }
  }
}

// 复制应用文件
export async function copyApplicationFiles(
  srcDir: string,
  destDir: string,
  onProgress?: (progress: number) => void
): Promise<{ success: boolean; error?: string }> {
  try {
    // 创建目标目录
    await mkdir(destDir, { recursive: true })

    // TODO: 实现文件复制逻辑
    // 这里需要根据实际文件结构实现

    onProgress?.(100)
    return { success: true }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '复制失败',
    }
  }
}

// 生成配置文件
export async function generateConfigFile(
  destPath: string,
  content: string
): Promise<{ success: boolean; error?: string }> {
  try {
    await mkdir(dirname(destPath), { recursive: true })
    await writeFile(destPath, content, 'utf-8')
    return { success: true }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '生成配置失败',
    }
  }
}

// 创建桌面快捷方式
export async function createDesktopShortcut(targetPath: string): Promise<boolean> {
  // Windows 创建快捷方式需要使用特殊方法
  // 可以使用 electron-shell 或者外部工具
  return true
}
```

- [ ] **Step 2: 更新主进程入口**

在 `installer/src/main/index.ts` 中添加 IPC handlers:

```typescript
// 在文件末尾添加：

import {
  checkDependency,
  installNpmPackage,
  copyApplicationFiles,
  generateConfigFile,
  createDesktopShortcut
} from './installer'
import mysql from 'mysql2/promise'

// IPC: 依赖检测
ipcMain.handle('check-dependency', async (_event, key: string) => {
  return checkDependency(key)
})

// IPC: 安装依赖
ipcMain.handle('install-dependency', async (_event, key: string) => {
  const packages: Record<string, string> = {
    claude: '@anthropic-ai/claude-cli',
    opencode: '@anthropic-ai/opencode',
  }
  if (packages[key]) {
    return installNpmPackage(packages[key])
  }
  return { success: false, error: '未知的依赖' }
})

// IPC: 复制文件
ipcMain.handle('copy-files', async (_event, src: string, dest: string) => {
  return copyApplicationFiles(src, dest)
})

// IPC: 生成配置
ipcMain.handle('generate-config', async (_event, data: { path: string; content: string }) => {
  return generateConfigFile(data.path, data.content)
})

// IPC: 数据库连接测试
ipcMain.handle('test-database-connection', async (_event, config: any) => {
  try {
    const connection = await mysql.createConnection({
      host: config.host,
      port: config.port,
      user: config.username,
      password: config.password,
      database: config.database,
      connectTimeout: 5000,
    })
    await connection.ping()
    await connection.end()
    return { success: true }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '连接失败',
    }
  }
})

// IPC: 创建快捷方式
ipcMain.handle('create-shortcut', async (_event, path: string) => {
  return { success: await createDesktopShortcut(path) }
})
```

- [ ] **Step 3: Commit**

```bash
git add installer/
git commit -m "feat(installer): add main process installer logic and IPC handlers"
```

---

## Task 18: 资源文件和图标

**Files:**
- Create: `installer/resources/icon.ico`
- Create: `installer/build/icon.ico`

- [ ] **Step 1: 准备图标文件**

说明：需要准备 ISDP 的图标文件（.ico 格式），放置在以下位置：
- `installer/resources/icon.ico` - 应用内图标
- `installer/build/icon.ico` - 安装器图标

可以从现有前端项目中提取或设计新的图标。

- [ ] **Step 2: 创建 resources 目录结构**

```bash
mkdir -p D:/00-codes/isdp/isdp/installer/resources/app
mkdir -p D:/00-codes/isdp/isdp/installer/build
```

- [ ] **Step 3: Commit**

```bash
git add installer/
git commit -m "feat(installer): add resources directory structure"
```

---

## Task 19: 构建和测试

- [ ] **Step 1: 本地开发测试**

```bash
cd D:/00-codes/isdp/isdp/installer
npm run dev
```

验证：
- 窗口正确显示
- 各页面可正常切换
- UI 样式符合设计

- [ ] **Step 2: 构建测试**

```bash
npm run build
```

验证：
- 构建无错误
- 输出文件在 `out/` 目录

- [ ] **Step 3: 打包测试**

```bash
npm run package:dir
```

验证：
- 打包无错误
- 产物在 `release/` 目录

- [ ] **Step 4: Commit**

```bash
git add installer/
git commit -m "feat(installer): complete build and test"
```

---

## 验证清单

- [ ] 在全新 Windows 机器上测试安装流程
- [ ] 验证依赖检测功能（Node.js、Git、Claude CLI、OpenCode）
- [ ] 验证数据库连接测试功能
- [ ] 验证文件复制和配置生成
- [ ] 验证安装完成后的启动功能
- [ ] 验证卸载功能

---

## 后续任务

1. 实现启动器（ISDP-Launcher.exe）独立程序
2. 添加 Node.js 和 Git 的自动安装逻辑
3. 完善错误处理和回滚机制
4. 添加更新检测功能
5. 支持静默安装模式