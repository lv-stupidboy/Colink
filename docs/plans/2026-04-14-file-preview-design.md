# Agent 调用页面优化 - 文件预览功能设计

**日期:** 2026-04-14  
**状态:** 设计完成，待实现

---

## 需求概述

优化 Agent 调用页面（ThreadView），实现：

1. **左侧文件树默认收起** - 初始状态隐藏，用户可通过按钮展开
2. **点击文件自动打开文件预览面板** - 点击文件后右侧面板自动打开并显示文件内容
3. **代码预览改为文件预览** - 支持查看任意文件（不只是代码变更）
4. **专注模式** - 全屏查看文件，有退出按钮 + ESC 快捷键

---

## 设计细节

### 1. 文件树默认收起

**改动位置:** `ThreadView.tsx`

- `fileSidebarVisible` 状态默认值改为 `false`
- 用户点击顶部"显示文件树"按钮展开

### 2. 文件预览面板（FilePreviewPanel）

**新组件:** `web/src/components/thread/FilePreviewPanel.tsx`

```
┌──────────────────────────────────────┐
│ [文件名]  Copy  Path  专注  ✕       │  ← 工具栏
├──────────────────────────────────────┤
│                                      │
│   Monaco Editor (只读模式)           │  ← 文件内容
│   - 语法高亮                         │
│   - 行号显示                         │
│                                      │
└──────────────────────────────────────┘
```

**功能:**

| 按钮 | 功能 |
|------|------|
| Copy | 复制文件全部内容到剪贴板 |
| Path | 复制文件绝对路径 |
| 专注 | 进入全屏专注模式 |
| ✕ | 关闭面板 |

**文件类型处理:**

| 类型 | 处理方式 |
|------|----------|
| 文本文件（代码、配置、文档等） | Monaco Editor 展示内容，语法高亮 |
| 图片（png, jpg, gif, svg） | 显示图片预览 |
| 二进制/超大文件 | 提示"无法预览" |

**支持的文本文件类型:**

- 代码: `.js`, `.jsx`, `.ts`, `.tsx`, `.go`, `.py`, `.java`, `.c`, `.cpp`, `.rs`, `.rb`, `.php`, `.swift`, `.sh`, `.html`, `.css`, `.scss`, `.vue`, `.svelte`, `.sql`
- 配置: `.json`, `.yaml`, `.yml`, `.toml`, `.xml`, `.ini`, `.conf`, `.env`
- 文档: `.md`, `.txt`, `.log`, `.csv`
- 特殊: `LICENSE`, `README`, `Makefile`, `Dockerfile`, `.gitignore`

### 3. 专注模式（FocusShell）

**新组件:** `web/src/components/thread/FocusShell.tsx`

```
┌──────────────────────────────────────────────────────────┐
│                                          [退出专注] ×    │  ← 退出按钮
├──────────────────────────────────────────────────────────┤
│                                                          │
│                    Monaco Editor 全屏                     │
│                                                          │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

**交互:**

- 点击"专注"按钮 → 进入全屏模式，覆盖整个页面
- 右上角"退出专注"按钮 或 ESC 键 → 退出，恢复原布局

**样式:**

- 全屏覆盖：`position: fixed; inset: 0; z-index: 100`
- 背景：深色主题背景色
- 退出按钮：类似 clowder-ai 的设计，右上角浮动

### 4. 右侧面板区域管理

**状态管理:** `ThreadView.tsx`

```typescript
// 现有状态
rightPanelVisible: boolean        // RightPanel 显示状态
rightPanelActiveTab: 'code' | 'sandbox'

// 新增状态
filePreviewVisible: boolean       // FilePreviewPanel 显示状态
filePreviewPath: string | null    // 当前预览文件路径
filePreviewContent: string | null // 文件内容
focusMode: boolean                // 专注模式状态
```

**互斥逻辑:**

- 点击文件 → 打开 FilePreviewPanel，关闭 RightPanel
- 点击顶部"面板"按钮 → 打开 RightPanel，关闭 FilePreviewPanel

### 5. API 依赖

**需要新增后端接口:**

```typescript
// 获取文件内容
GET /files/content?basePath={basePath}&path={filePath}
Response: { content: string, size: number, truncated: boolean }
```

**前端 API 新增:**

```typescript
// web/src/api/client.ts
files = {
  // 现有方法...
  getContent: (basePath: string, path: string): Promise<{
    content: string;
    size: number;
    truncated: boolean;
  }> => {
    const url = `/files/content?basePath=${encodeURIComponent(basePath)}&path=${encodeURIComponent(path)}`;
    return this.request(url, 'GET');
  },
}
```

---

## 文件修改清单

### 前端

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `ThreadView.tsx` | 修改 | fileSidebarVisible 默认 false；新增 filePreview 相关状态；处理文件点击 |
| `ThreadView.css` | 修改 | 添加 FocusShell 全屏样式 |
| `FileTree/index.tsx` | 修改 | 新增 onFileOpen 回调参数 |
| `RightPanel.tsx` | 修改 | Tab 标签从"代码预览"改为"代码变更" |
| `FilePreviewPanel.tsx` | **新增** | 文件预览面板组件 |
| `FocusShell.tsx` | **新增** | 专注模式容器组件 |
| `api/client.ts` | 修改 | 新增 files.getContent 方法 |

### 后端

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `api/project_handler.go` | 修改 | 新增 GetFileContent handler |
| `service/project/service.go` | 修改 | 新增 GetFileContent 方法 |
| `model/file.go` | 修改 | 新增 FileContentResponse 结构体 |

---

## 组件结构

```
ThreadView.tsx
├── 左侧文件树（默认收起）
├── 消息区（ChatMessageList）
├── 左侧状态栏（StatusPanel）
└── 右侧面板区域
    ├── RightPanel（代码变更/沙箱）- 手动打开
    │   ├── Tab: 代码变更 (CodePanel)
    │   └── Tab: 沙箱 (SandboxPanel)
    │
    └── FilePreviewPanel（文件预览）- 点击文件自动打开
        ├── 工具栏（Copy/Path/专注/关闭）
        ├── Monaco Editor（文件内容）
        └── FocusShell（专注模式容器）
            ├── 退出按钮
            └── Monaco Editor 全屏
```

---

## 实现优先级

1. **P0 - 核心功能**
   - 后端：新增文件内容 API
   - 前端：FilePreviewPanel + FocusShell 组件
   - ThreadView：状态管理 + 文件点击处理

2. **P1 - 体验优化**
   - 文件树默认收起
   - RightPanel Tab 标签改名
   - 加载状态、错误处理

3. **P2 - 细节完善**
   - 图片预览
   - 大文件截断提示
   - 深色模式适配

---

## 参考

- clowder-ai 专注模式实现：`WorkspaceFocusShell.tsx`, `WorkspaceFileViewer.tsx`
- Monaco Editor：https://microsoft.github.io/monaco-editor/