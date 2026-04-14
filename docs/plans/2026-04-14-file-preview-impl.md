# 文件预览功能实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在 Agent 调用页面实现文件预览功能：点击文件自动打开预览面板，支持专注模式全屏查看。

**Architecture:** 后端新增文件内容 API，前端新增 FilePreviewPanel 和 FocusShell 组件，修改 ThreadView 状态管理实现文件点击触发预览。

**Tech Stack:** Go/Gin (后端), React + Monaco Editor + Ant Design (前端)

---

## Task 1: 后端 - 新增文件内容 API

**Files:**
- Modify: `internal/model/file.go` - 新增响应结构体
- Modify: `internal/service/project/service.go` - 新增 GetFileContent 方法
- Modify: `internal/api/project_handler.go` - 新增 handler 和路由

**Step 1: 新增 FileContentResponse 结构体**

在 `internal/model/file.go` 中添加：

```go
// FileContentResponse 文件内容响应
type FileContentResponse struct {
    Content    string `json:"content"`    // 文件内容
    Size       int64  `json:"size"`       // 文件大小（字节）
    Truncated  bool   `json:"truncated"`  // 是否截断（超过1MB）
    Path       string `json:"path"`       // 文件路径
    IsBinary   bool   `json:"isBinary"`   // 是否二进制文件
}
```

**Step 2: 新增 GetFileContent 方法**

在 `internal/service/project/service.go` 中添加：

```go
import (
    "io"
    "os"
    "path/filepath"
    "strings"
)

const maxFileSize = 1024 * 1024 // 1MB

// GetFileContent 获取文件内容
func (s *Service) GetFileContent(ctx context.Context, basePath, filePath string) (*model.FileContentResponse, error) {
    // 拼接完整路径
    fullPath := filepath.Join(basePath, filePath)
    
    // 检查路径安全性（防止路径穿越）
    if !strings.HasPrefix(fullPath, basePath) {
        return nil, errors.New("invalid path: path traversal detected")
    }
    
    // 检查文件是否存在
    info, err := os.Stat(fullPath)
    if err != nil {
        return nil, err
    }
    
    // 检查是否为目录
    if info.IsDir() {
        return nil, errors.New("path is a directory, not a file")
    }
    
    // 检查是否为二进制文件
    isBinary := s.isBinaryFile(fullPath)
    
    // 读取文件内容
    file, err := os.Open(fullPath)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    // 限制读取大小
    var content string
    var truncated bool
    
    if info.Size() > maxFileSize {
        // 大文件：只读取前 1MB
        buf := make([]byte, maxFileSize)
        n, err := file.Read(buf)
        if err != nil && err != io.EOF {
            return nil, err
        }
        content = string(buf[:n])
        truncated = true
    } else {
        // 小文件：全部读取
        buf, err := io.ReadAll(file)
        if err != nil {
            return nil, err
        }
        content = string(buf)
        truncated = false
    }
    
    return &model.FileContentResponse{
        Content:   content,
        Size:      info.Size(),
        Truncated: truncated,
        Path:      filePath,
        IsBinary:  isBinary,
    }, nil
}

// isBinaryFile 判断是否为二进制文件
func (s *Service) isBinaryFile(path string) bool {
    // 常见二进制文件扩展名
    ext := strings.ToLower(filepath.Ext(path))
    binaryExts := []string{
        ".exe", ".dll", ".so", ".dylib",
        ".zip", ".tar", ".gz", ".rar", ".7z",
        ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
        ".mp3", ".mp4", ".avi", ".mov", ".wav", ".flac",
        ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".ico", ".webp",
        ".woff", ".woff2", ".ttf", ".eot",
        ".sqlite", ".db",
    }
    for _, be := range binaryExts {
        if ext == be {
            return true
        }
    }
    
    // 图片扩展名返回 false（我们会用前端预览）
    imageExts := []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp"}
    for _, ie := range imageExts {
        if ext == ie {
            return false
        }
    }
    
    return false
}
```

**Step 3: 新增 GetFileContent handler**

在 `internal/api/project_handler.go` 中添加：

```go
// GetFileContent 获取文件内容
func (h *ProjectHandler) GetFileContent(c *gin.Context) {
    basePath := c.Query("basePath")
    if basePath == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "basePath is required"})
        return
    }
    
    filePath := c.Query("path")
    if filePath == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
        return
    }
    
    result, err := h.service.GetFileContent(c.Request.Context(), basePath, filePath)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, result)
}

// 在 RegisterRoutes 中添加路由
func (h *ProjectHandler) RegisterRoutes(r *gin.RouterGroup) {
    // ...existing routes...
    files := r.Group("/files")
    {
        files.GET("/browse", h.BrowsePath)
        files.GET("/validate", h.ValidatePath)
        files.GET("/content", h.GetFileContent)  // 新增
        files.POST("/folder", h.CreateFolder)
    }
}
```

**Step 4: 编译验证**

Run: `cd D:/CoLinkProject/Colink-0412/isdp && go build ./cmd/server`
Expected: 编译成功，无错误

**Step 5: Commit**

```bash
git add internal/model/file.go internal/service/project/service.go internal/api/project_handler.go
git commit -m "feat(api): add file content endpoint for preview"
```

---

## Task 2: 前端 API - 新增 files.getContent 方法

**Files:**
- Modify: `web/src/api/client.ts`

**Step 1: 新增 getContent 方法**

在 `web/src/api/client.ts` 的 `files` 对象中添加：

```typescript
files = {
  // 现有方法保持不变...
  browse: (path?: string): Promise<...> => { ... },
  validate: (path: string): Promise<...> => { ... },
  createFolder: (parentPath: string, name: string): Promise<...> => { ... },
  
  // 新增方法
  getContent: (basePath: string, path: string): Promise<{
    content: string;
    size: number;
    truncated: boolean;
    path: string;
    isBinary: boolean;
  }> => {
    const url = `/files/content?basePath=${encodeURIComponent(basePath)}&path=${encodeURIComponent(path)}`;
    return this.request(url, 'GET');
  },
}
```

**Step 2: 验证 TypeScript 无报错**

Run: `cd D:/CoLinkProject/Colink-0412/isdp/web && npm run type-check`（如果没有此命令，用 `npx tsc --noEmit`）
Expected: 无 TypeScript 错误

**Step 3: Commit**

```bash
git add web/src/api/client.ts
git commit -m "feat(api): add files.getContent method for file preview"
```

---

## Task 3: 前端 - 新增 FocusShell 组件

**Files:**
- Create: `web/src/components/thread/FocusShell.tsx`
- Create: `web/src/components/thread/FocusShell.css`

**Step 1: 创建 FocusShell.tsx**

```tsx
import React, { useEffect } from 'react';
import './FocusShell.css';

interface FocusShellProps {
  children: React.ReactNode;
  onExit: () => void;
  title?: string;
}

/**
 * 专注模式容器 - 全屏覆盖，提供退出按钮和 ESC 快捷键
 */
export const FocusShell: React.FC<FocusShellProps> = ({ children, onExit, title }) => {
  // ESC 键退出
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onExit();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onExit]);

  return (
    <div className="focus-shell">
      {/* 退出按钮 */}
      <button
        className="focus-shell-exit"
        onClick={onExit}
        title="退出专注模式 (ESC)"
      >
        <svg width="8" height="8" viewBox="0 0 10 10" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M1 1l8 8M9 1l-8 8" />
        </svg>
        <span>退出专注</span>
      </button>
      
      {/* 标题（可选） */}
      {title && <div className="focus-shell-title">{title}</div>}
      
      {/* 内容区域 */}
      <div className="focus-shell-content">
        {children}
      </div>
    </div>
  );
};

export default FocusShell;
```

**Step 2: 创建 FocusShell.css**

```css
.focus-shell {
  position: fixed;
  inset: 0;
  z-index: 100;
  background: var(--bg-container, #1e1e24);
  display: flex;
  flex-direction: column;
  animation: fade-in 0.2s ease;
}

@keyframes fade-in {
  from { opacity: 0; }
  to { opacity: 1; }
}

.focus-shell-exit {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 101;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
  background: rgba(255, 255, 255, 0.9);
  color: #333;
  border: 1px solid rgba(0, 0, 0, 0.1);
  backdrop-filter: blur(8px);
  cursor: pointer;
  transition: all 0.2s;
}

.focus-shell-exit:hover {
  background: #fff;
  border-color: rgba(0, 0, 0, 0.2);
}

.focus-shell-title {
  padding: 12px 16px;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary, #fff);
  border-bottom: 1px solid var(--border-color, #2a2a32);
}

.focus-shell-content {
  flex: 1;
  min-height: 0;
  overflow: auto;
}

/* 深色模式 */
[data-theme='dark'] .focus-shell {
  background: var(--bg-container);
}

[data-theme='dark'] .focus-shell-exit {
  background: rgba(30, 30, 36, 0.9);
  color: var(--text-primary);
  border-color: var(--border-color);
}

[data-theme='dark'] .focus-shell-exit:hover {
  background: rgba(40, 40, 48, 0.95);
}
```

**Step 3: 验证 TypeScript 无报错**

Run: `cd D:/CoLinkProject/Colink-0412/isdp/web && npx tsc --noEmit`
Expected: 无错误

**Step 4: Commit**

```bash
git add web/src/components/thread/FocusShell.tsx web/src/components/thread/FocusShell.css
git commit -m "feat(ui): add FocusShell component for focus mode"
```

---

## Task 4: 前端 - 新增 FilePreviewPanel 组件

**Files:**
- Create: `web/src/components/thread/FilePreviewPanel.tsx`
- Create: `web/src/components/thread/FilePreviewPanel.css`

**Step 1: 安装 Monaco Editor（如果未安装）**

Run: `cd D:/CoLinkProject/Colink-0412/isdp/web && npm list @monaco-editor/react`
如果未安装，执行：
Run: `cd D:/CoLinkProject/Colink-0412/isdp/web && npm install @monaco-editor/react`

**Step 2: 创建 FilePreviewPanel.tsx**

```tsx
import React, { useState, useEffect, useMemo } from 'react';
import { Button, Space, Spin, message, Tooltip, Empty } from 'antd';
import { CopyOutlined, CheckOutlined, FileOutlined, ExpandOutlined } from '@ant-design/icons';
import Editor from '@monaco-editor/react';
import { FocusShell } from './FocusShell';
import api from '@/api/client';
import './FilePreviewPanel.css';

interface FilePreviewPanelProps {
  basePath: string;
  filePath: string;
  onClose: () => void;
  width?: number;
}

// 根据文件扩展名获取 Monaco 语言 ID
const getLanguage = (filePath: string): string => {
  const ext = filePath.split('.').pop()?.toLowerCase() || '';
  const languageMap: Record<string, string> = {
    // JavaScript/TypeScript
    'js': 'javascript',
    'jsx': 'javascript',
    'ts': 'typescript',
    'tsx': 'typescript',
    'mjs': 'javascript',
    // Web
    'html': 'html',
    'htm': 'html',
    'css': 'css',
    'scss': 'scss',
    'sass': 'scss',
    'less': 'less',
    'vue': 'html',
    'svelte': 'html',
    // Backend
    'go': 'go',
    'py': 'python',
    'java': 'java',
    'kt': 'kotlin',
    'kts': 'kotlin',
    'rs': 'rust',
    'rb': 'ruby',
    'php': 'php',
    'swift': 'swift',
    'c': 'c',
    'cpp': 'cpp',
    'h': 'c',
    'hpp': 'cpp',
    // Shell
    'sh': 'shell',
    'bash': 'shell',
    'zsh': 'shell',
    'ps1': 'powershell',
    // Config
    'json': 'json',
    'yaml': 'yaml',
    'yml': 'yaml',
    'toml': 'ini',
    'xml': 'xml',
    'ini': 'ini',
    'conf': 'ini',
    'cfg': 'ini',
    'env': 'plaintext',
    // Data
    'sql': 'sql',
    'csv': 'plaintext',
    // Docs
    'md': 'markdown',
    'markdown': 'markdown',
    'txt': 'plaintext',
    'log': 'plaintext',
    // Special
    'dockerfile': 'dockerfile',
    'makefile': 'makefile',
  };
  return languageMap[ext] || 'plaintext';
};

// 判断是否为图片文件
const isImageFile = (filePath: string): boolean => {
  const ext = filePath.split('.').pop()?.toLowerCase() || '';
  return ['png', 'jpg', 'jpeg', 'gif', 'svg', 'webp', 'bmp'].includes(ext);
};

export const FilePreviewPanel: React.FC<FilePreviewPanelProps> = ({
  basePath,
  filePath,
  onClose,
  width = 520,
}) => {
  const [loading, setLoading] = useState(true);
  const [content, setContent] = useState<string>('');
  const [size, setSize] = useState<number>(0);
  const [truncated, setTruncated] = useState(false);
  const [isBinary, setIsBinary] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [focusMode, setFocusMode] = useState(false);
  const [copied, setCopied] = useState<'content' | 'path' | null>(null);

  // 获取文件内容
  useEffect(() => {
    const fetchContent = async () => {
      setLoading(true);
      setError(null);
      try {
        const result = await api.files.getContent(basePath, filePath);
        setContent(result.content);
        setSize(result.size);
        setTruncated(result.truncated);
        setIsBinary(result.isBinary);
      } catch (err: any) {
        setError(err.message || '加载文件失败');
      } finally {
        setLoading(false);
      }
    };
    fetchContent();
  }, [basePath, filePath]);

  // 复制内容
  const handleCopyContent = async () => {
    try {
      await navigator.clipboard.writeText(content);
      setCopied('content');
      message.success('已复制内容');
      setTimeout(() => setCopied(null), 2000);
    } catch {
      message.error('复制失败');
    }
  };

  // 复制路径
  const handleCopyPath = async () => {
    try {
      const fullPath = `${basePath}/${filePath}`;
      await navigator.clipboard.writeText(fullPath);
      setCopied('path');
      message.success('已复制路径');
      setTimeout(() => setCopied(null), 2000);
    } catch {
      message.error('复制失败');
    }
  };

  // 文件名显示
  const fileName = filePath.split('/').pop() || filePath;

  // 语言
  const language = useMemo(() => getLanguage(filePath), [filePath]);

  // 是否图片
  const isImage = isImageFile(filePath);

  // 图片 URL
  const imageUrl = `/api/v1/files/image?basePath=${encodeURIComponent(basePath)}&path=${encodeURIComponent(filePath)}`;

  // 渲染内容
  const renderContent = () => {
    if (loading) {
      return (
        <div className="file-preview-loading">
          <Spin />
          <span>加载中...</span>
        </div>
      );
    }

    if (error) {
      return (
        <div className="file-preview-error">
          <Empty description={error} />
        </div>
      );
    }

    if (isBinary) {
      return (
        <div className="file-preview-binary">
          <Empty description="二进制文件无法预览" />
        </div>
      );
    }

    if (isImage) {
      return (
        <div className="file-preview-image">
          <img src={imageUrl} alt={fileName} style={{ maxWidth: '100%', maxHeight: '100%' }} />
        </div>
      );
    }

    return (
      <Editor
        height="100%"
        language={language}
        value={content}
        options={{
          readOnly: true,
          minimap: { enabled: false },
          lineNumbers: 'on',
          fontSize: 13,
          scrollBeyondLastLine: false,
          wordWrap: 'on',
          automaticLayout: true,
          theme: 'vs-dark',
        }}
      />
    );
  };

  // 专注模式内容
  if (focusMode) {
    return (
      <FocusShell onExit={() => setFocusMode(false)} title={fileName}>
        <Editor
          height="100%"
          language={language}
          value={content}
          options={{
            readOnly: true,
            minimap: { enabled: true },
            lineNumbers: 'on',
            fontSize: 14,
            scrollBeyondLastLine: false,
            wordWrap: 'on',
            automaticLayout: true,
            theme: 'vs-dark',
          }}
        />
      </FocusShell>
    );
  }

  return (
    <div className="file-preview-panel" style={{ width }}>
      {/* 工具栏 */}
      <div className="file-preview-toolbar">
        <span className="file-preview-filename" title={filePath}>
          <FileOutlined /> {fileName}
        </span>
        <Space size="small">
          <Tooltip title="复制内容">
            <Button
              size="small"
              icon={copied === 'content' ? <CheckOutlined /> : <CopyOutlined />}
              onClick={handleCopyContent}
              disabled={loading || isBinary || isImage}
            >
              Copy
            </Button>
          </Tooltip>
          <Tooltip title="复制路径">
            <Button
              size="small"
              icon={copied === 'path' ? <CheckOutlined /> : <CopyOutlined />}
              onClick={handleCopyPath}
            >
              Path
            </Button>
          </Tooltip>
          <Tooltip title="专注模式">
            <Button
              size="small"
              icon={<ExpandOutlined />}
              onClick={() => setFocusMode(true)}
              disabled={loading || isBinary || isImage}
            >
              专注
            </Button>
          </Tooltip>
          <Tooltip title="关闭">
            <Button size="small" onClick={onClose}>
              ✕
            </Button>
          </Tooltip>
        </Space>
      </div>

      {/* 文件大小提示 */}
      {size > 0 && (
        <div className="file-preview-meta">
          {size < 1024 ? `${size}B` : size < 1024 * 1024 ? `${Math.round(size / 1024)}KB` : `${Math.round(size / 1024 / 1024)}MB`}
          {truncated && <span className="file-preview-truncated">(已截断)</span>}
        </div>
      )}

      {/* 内容区域 */}
      <div className="file-preview-content">
        {renderContent()}
      </div>
    </div>
  );
};

export default FilePreviewPanel;
```

**Step 3: 创建 FilePreviewPanel.css**

```css
.file-preview-panel {
  height: 100%;
  background: var(--bg-container, #1e1e24);
  border-left: 1px solid var(--border-color, #2a2a32);
  display: flex;
  flex-direction: column;
}

.file-preview-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: var(--bg-elevated, #252530);
  border-bottom: 1px solid var(--border-color, #2a2a32);
}

.file-preview-filename {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary, #fff);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 200px;
}

.file-preview-meta {
  padding: 4px 12px;
  font-size: 11px;
  color: var(--text-secondary, #9ca3af);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  border-bottom: 1px solid var(--border-color, #2a2a32);
}

.file-preview-truncated {
  color: #f59e0b;
  margin-left: 8px;
}

.file-preview-content {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.file-preview-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 100%;
  color: var(--text-secondary);
}

.file-preview-error {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
}

.file-preview-binary {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
}

.file-preview-image {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  background: var(--bg-base, #0a0a0f);
  padding: 16px;
}

/* 深色模式 */
[data-theme='dark'] .file-preview-panel {
  background: var(--bg-container);
  border-left-color: var(--border-color);
}

[data-theme='dark'] .file-preview-toolbar {
  background: var(--bg-elevated);
  border-bottom-color: var(--border-color);
}

[data-theme='dark'] .file-preview-filename {
  color: var(--text-primary);
}

[data-theme='dark'] .file-preview-meta {
  color: var(--text-secondary);
  border-bottom-color: var(--border-color);
}
```

**Step 4: 验证 TypeScript**

Run: `cd D:/CoLinkProject/Colink-0412/isdp/web && npx tsc --noEmit`
Expected: 无错误（可能有 Monaco 类型警告，可忽略）

**Step 5: Commit**

```bash
git add web/src/components/thread/FilePreviewPanel.tsx web/src/components/thread/FilePreviewPanel.css
git commit -m "feat(ui): add FilePreviewPanel component with Monaco Editor"
```

---

## Task 5: 前端 - 修改 FileTree 支持文件点击

**Files:**
- Modify: `web/src/components/FileTree/index.tsx`

**Step 1: 新增 onFileOpen 参数**

修改 `FileTree/index.tsx` 的 Props 接口：

```tsx
interface FileTreeProps {
  projectId: string;
  projectPath?: string;
  onFileSelect?: (path: string, isDir: boolean) => void;
  onFileOpen?: (path: string) => void;  // 新增：点击文件打开预览
  style?: React.CSSProperties;
}
```

**Step 2: 修改 onSelect 处理**

在 `FileTree` 组件中修改 `onSelect` 回调：

```tsx
const FileTree: React.FC<FileTreeProps> = ({ projectId, projectPath, onFileSelect, onFileOpen, style }) => {
  // ...existing code...

  // Handle node selection
  const onSelect: TreeProps['onSelect'] = (selectedKeys, info) => {
    if (selectedKeys.length > 0) {
      const path = selectedKeys[0] as string;
      const node = info.node;
      const isDir = !node.isLeaf;
      
      // 文件：触发打开预览
      if (!isDir && onFileOpen) {
        onFileOpen(path);
      }
      
      // 目录或原有回调
      onFileSelect?.(path, isDir);
    }
  };

  // ...rest of component...
};
```

**Step 3: 验证 TypeScript**

Run: `cd D:/CoLinkProject/Colink-0412/isdp/web && npx tsc --noEmit`
Expected: 无错误

**Step 4: Commit**

```bash
git add web/src/components/FileTree/index.tsx
git commit -m "feat(ui): add onFileOpen callback to FileTree"
```

---

## Task 6: 前端 - 修改 RightPanel Tab 标签

**Files:**
- Modify: `web/src/components/thread/RightPanel.tsx`

**Step 1: 修改 Tab 标签文字**

在 `RightPanel.tsx` 中找到 `items` 数组，修改第一个 Tab 的 label：

```tsx
const items = [
  {
    key: 'code',
    label: (
      <span>
        <CodeOutlined /> 代码变更  {/* 从"代码预览"改为"代码变更" */}
      </span>
    ),
    children: (
      // ...existing children...
    ),
  },
  // ...sandbox tab...
];
```

**Step 2: Commit**

```bash
git add web/src/components/thread/RightPanel.tsx
git commit -m "fix(ui): rename code preview tab to '代码变更'"
```

---

## Task 7: 前端 - 修改 ThreadView 状态管理

**Files:**
- Modify: `web/src/pages/ThreadView.tsx`
- Modify: `web/src/pages/ThreadView.css`

**Step 1: 新增状态变量**

在 `ThreadView.tsx` 状态声明区域添加：

```tsx
// 文件预览状态
const [filePreviewVisible, setFilePreviewVisible] = useState(false);
const [filePreviewPath, setFilePreviewPath] = useState<string | null>(null);
const [focusMode, setFocusMode] = useState(false);

// 文件树默认收起（修改现有状态）
const [fileSidebarVisible, setFileSidebarVisible] = useState(false);  // 改为 false
```

**Step 2: 新增文件打开处理函数**

```tsx
// 处理文件打开（触发预览）
const handleFileOpen = (filePath: string) => {
  setFilePreviewPath(filePath);
  setFilePreviewVisible(true);
  setRightPanelVisible(false);  // 关闭 RightPanel，互斥
};
```

**Step 3: 修改文件树按钮点击处理**

找到文件树按钮的 onClick，确保保持互斥逻辑：

```tsx
<Tooltip title={fileSidebarVisible ? '隐藏文件树' : '显示文件树'}>
  <Button 
    icon={fileSidebarVisible ? <MenuFoldOutlined /> : <MenuUnfoldOutlined />} 
    onClick={() => setFileSidebarVisible(!fileSidebarVisible)} 
    size="small" 
  />
</Tooltip>
```

**Step 4: 修改面板按钮点击处理**

```tsx
<Tooltip title={rightPanelVisible ? '隐藏面板' : '打开代码变更/沙箱面板'}>
  <Button
    icon={<DesktopOutlined />}
    onClick={() => {
      setRightPanelVisible(!rightPanelVisible);
      setFilePreviewVisible(false);  // 关闭文件预览，互斥
      setArtifactsSidebarVisible(false);
      setFileSidebarVisible(false);
    }}
    size="small"
    type={rightPanelVisible || currentSandboxServer ? 'primary' : 'default'}
  >面板</Button>
</Tooltip>
```

**Step 5: 渲染 FilePreviewPanel**

在右侧面板区域添加 FilePreviewPanel 渲染（在 RightPanel 渲染附近）：

```tsx
{/* 文件预览面板 */}
{filePreviewVisible && filePreviewPath && (
  <FilePreviewPanel
    basePath={displayProjectPath}
    filePath={filePreviewPath}
    onClose={() => {
      setFilePreviewVisible(false);
      setFilePreviewPath(null);
    }}
    width={rightPanelWidth}
  />
)}
```

**Step 6: 导入新组件**

在文件顶部添加导入：

```tsx
import { FilePreviewPanel } from '@/components/thread/FilePreviewPanel';
```

**Step 7: 修改 FileTree 的 onFileOpen**

找到 FileTree 组件的渲染，添加 onFileOpen 参数：

```tsx
<FileTree
  projectId={projectId || 'debug'}
  projectPath={displayProjectPath}
  onFileSelect={handleFileSelect}
  onFileOpen={handleFileOpen}  // 新增
/>
```

**Step 8: 验证 TypeScript**

Run: `cd D:/CoLinkProject/Colink-0412/isdp/web && npx tsc --noEmit`
Expected: 无错误

**Step 9: Commit**

```bash
git add web/src/pages/ThreadView.tsx
git commit -m "feat(ui): integrate FilePreviewPanel into ThreadView"
```

---

## Task 8: 后端 - 新增图片预览 API（可选但推荐）

**Files:**
- Modify: `internal/api/project_handler.go`

**Step 1: 新增 GetFileImage handler**

```go
// GetFileImage 获取图片文件（直接返回文件内容）
func (h *ProjectHandler) GetFileImage(c *gin.Context) {
    basePath := c.Query("basePath")
    if basePath == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "basePath is required"})
        return
    }
    
    filePath := c.Query("path")
    if filePath == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
        return
    }
    
    fullPath := filepath.Join(basePath, filePath)
    
    // 安全检查
    if !strings.HasPrefix(fullPath, basePath) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
        return
    }
    
    // 直接返回文件
    c.File(fullPath)
}
```

**Step 2: 注册路由**

```go
files.GET("/image", h.GetFileImage)  // 新增
```

**Step 3: Commit**

```bash
git add internal/api/project_handler.go
git commit -m "feat(api): add file image endpoint for preview"
```

---

## Task 9: 测试验证

**Step 1: 启动后端**

Run: `cd D:/CoLinkProject/Colink-0412/isdp && go run ./cmd/server`
Expected: 服务启动，端口 8080

**Step 2: 启动前端**

Run: `cd D:/CoLinkProject/Colink-0412/isdp/web && npm run dev`
Expected: 前端启动，端口 3000

**Step 3: 手动测试**

1. 进入 Agent 调用页面（ThreadView）
2. 确认文件树默认隐藏
3. 点击"显示文件树"按钮，文件树展开
4. 点击一个代码文件，右侧面板自动打开显示文件内容
5. 点击"专注"按钮，进入全屏模式
6. 按 ESC 或点击"退出专注"，退出全屏
7. 点击 Copy 按钮，确认内容已复制
8. 点击 Path 按钮，确认路径已复制
9. 点击 ✕ 关闭面板
10. 点击"面板"按钮，RightPanel 打开（文件预览关闭）

**Step 4: Commit 验证文档**

```bash
git add docs/plans/2026-04-14-file-preview-design.md docs/plans/2026-04-14-file-preview-impl.md
git commit -m "docs: add file preview feature design and implementation plan"
```

---

## Task 10: 推送代码

**Step 1: 推送到远程**

```bash
git push origin cc
```

Expected: 推送成功

---

## 总结

| Task | 描述 | 文件数 |
|------|------|--------|
| 1 | 后端文件内容 API | 3 |
| 2 | 前端 API 方法 | 1 |
| 3 | FocusShell 组件 | 2 |
| 4 | FilePreviewPanel 组件 | 2 |
| 5 | FileTree 修改 | 1 |
| 6 | RightPanel Tab 改名 | 1 |
| 7 | ThreadView 状态管理 | 1 |
| 8 | 图片预览 API | 1 |
| 9 | 测试验证 | - |
| 10 | 推送代码 | - |