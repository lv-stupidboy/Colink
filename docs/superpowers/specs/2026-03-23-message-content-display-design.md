# 消息内容展示优化设计

## 概述

优化 AI 对话框中的内容展示方式，将视觉内容（架构图、图表、图片）与代码内容分离展示，提升用户体验。

**核心原则：**
- 视觉内容直接在气泡内展示，一目了然
- 代码内容在右侧面板展示，支持 Diff 对比和深入操作
- 右侧面板可收起，不影响对话流畅性

---

## 内容展示策略

### 展示位置分配

| 内容类型 | 展示位置 | 原因 |
|----------|----------|------|
| 🖼️ 架构图/流程图 | **气泡内卡片** | 视觉内容，一眼可理解，无需深入阅读 |
| 📊 数据图表 | **气泡内卡片** | 交互式展示即可，点击可放大 |
| 📷 截图/图片 | **气泡内卡片** | 图片预览，支持点击放大 |
| 💻 代码 | **右侧面板** | 需要 Diff 对比、复制、编辑等深入操作 |
| 📋 表格数据 | **右侧面板** | 需要排序、筛选、导出等操作 |
| 📝 长文档（>100行） | **右侧面板** | 需要滚动阅读、编辑、保存 |
| 📦 JSON/YAML 数据 | **右侧面板** | 支持折叠展开、语法高亮、复制 |
| ❌ 错误日志/堆栈 | **气泡内折叠卡片** | 模拟终端样式，支持复制 |

---

## 界面设计

### 1. 对话气泡 - 视觉内容卡片

视觉内容（架构图、图表、图片）直接在气泡内以卡片形式展示：

```
┌─────────────────────────────────────────────┐
│ AI 助手                              10:30  │
├─────────────────────────────────────────────┤
│ 好的，这是用户登录流程的架构图：              │
│                                             │
│ ┌─────────────────────────────────────────┐ │
│ │ 🖼️ 用户登录流程图        [放大] [下载]  │ │
│ ├─────────────────────────────────────────┤ │
│ │                                         │ │
│ │    ┌─────────┐                          │ │
│ │    │ 用户输入 │                          │ │
│ │    └────┬────┘                          │ │
│ │         ↓                               │ │
│ │    ┌─────────┐                          │ │
│ │    │ 前端验证 │                          │ │
│ │    └────┬────┘                          │ │
│ │         ↓                               │ │
│ │    ┌─────────┐                          │ │
│ │    │ API认证  │                          │ │
│ │    └─────────┘                          │ │
│ │                                         │ │
│ └─────────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
```

**卡片功能：**
- 标题显示内容类型图标和名称
- 右上角提供"放大"、"下载"按钮
- Mermaid/PlantUML 自动渲染
- 支持缩放预览

### 2. 对话气泡 - 代码入口

代码内容在气泡内只显示摘要和入口，点击后右侧面板展开详情：

```
┌─────────────────────────────────────────────┐
│ AI 助手                              10:32  │
├─────────────────────────────────────────────┤
│ 好的，我已生成登录功能的完整代码：            │
│                                             │
│ • LoginPage.tsx - 登录组件                   │
│ • useAuth.ts - 认证 Hook                    │
│ • auth.api.ts - API 接口                    │
│                                             │
│ ┌─────────────────────────────────────────┐ │
│ │ 📄 LoginPage.tsx            +32 行      │ │
│ │ const LoginPage = () => { const [email  │ │
│ │                         点击查看详情 →   │ │
│ └─────────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
```

### 3. 右侧面板 - 多文件列表

面板采用 Git Changes 风格，文件纵向排列：

```
┌───────────────────────────────────────────┐
│ 📄 代码变更        [+86] [-12]    [−] [×] │
├───────────────────────────────────────────┤
│ ▼ 📄 LoginPage.tsx            +42  -5     │
├───────────────────────────────────────────┤
│ ▶ 📄 useAuth.ts               +28  -3     │
├───────────────────────────────────────────┤
│ ▶ 📄 auth.api.ts              +16  -4     │
├───────────────────────────────────────────┤
│ ▶ 📄 types.ts        [新增]   +12         │
├───────────────────────────────────────────┤
│                                           │
│        [✅ 全部应用]    [📋 复制全部]      │
└───────────────────────────────────────────┘
```

**文件列表功能：**
- 点击展开/收起代码详情
- 显示每个文件的变更行数（+X -Y）
- 新增文件有绿色"新增"标签
- 面板顶部显示总变更统计

### 4. 右侧面板 - Split Diff 视图

展开文件后，显示左右对比的 Diff 视图：

```
┌─────────────────────────────────────────────────────────┐
│ ▼ 📄 LoginPage.tsx                        +42  -5      │
├──────────────────────────┬──────────────────────────────┤
│       原始代码            │         变更后               │
├──────────────────────────┼──────────────────────────────┤
│ 1  import React from...  │ 1  import React, { useState}│
│ 2                        │ 2  import { Form } from...   │
│ 3  const LoginPage = ()  │ 3  import { useAuth } from..│
│ 4    // TODO: implement  │ 4                            │
│ 5    return <div>Login   │ 5  const LoginPage = () => { │
│ 6  };                    │ 6    const [email, setEmail] │
│ 7                        │ 7    const { login } = use.. │
│ 8  export default...     │ 8    ...展开 15 行...        │
│                          │ 28 export default LoginPage; │
└──────────────────────────┴──────────────────────────────┘
```

**Diff 视图特点：**
- 左侧：原始代码（黄色背景）
- 右侧：变更后代码（白色/绿色背景）
- 红色删除线：被删除的行
- 绿色背景：新增的行
- 支持折叠未修改的代码
- 两侧同步滚动

### 5. 右侧面板 - 收起状态

面板可收起为侧边栏，不影响对话：

```
┌─────────────────────────────────┬────┐
│                                 │ 📄 │
│        对话区域                  │ 代 │
│                                 │ 码 │
│                                 │ 变 │
│                                 │ 更 │
│                                 │    │
│                                 │+86 │
│                                 │-12 │
│                                 │ ◀  │
└─────────────────────────────────┴────┘
```

**收起状态功能：**
- 显示文件数量图标
- 显示总变更统计
- 点击可重新展开

---

## 交互设计

### 面板操作

| 操作 | 效果 |
|------|------|
| 点击 `−` | 面板收起为侧边栏 |
| 点击 `×` | 完全关闭面板 |
| 点击侧边栏 `◀` | 重新展开面板 |

### 文件操作

| 操作 | 效果 |
|------|------|
| 点击文件名 | 展开/收起代码详情 |
| 点击"全部应用" | 将所有变更写入文件 |
| 点击"复制全部" | 复制所有代码到剪贴板 |

### 代码操作

| 操作 | 效果 |
|------|------|
| 滚动左侧 | 右侧同步滚动 |
| 点击"展开 X 行" | 展开折叠的代码 |
| 鼠标悬停代码行 | 高亮对应行 |

---

## 技术实现要点

### 1. 内容类型识别

基于 Markdown 代码块的语言标识符自动判断内容类型：

```typescript
type ContentType =
  | 'text'           // 纯文本
  | 'code'           // 代码块
  | 'diagram'        // 架构图（Mermaid/PlantUML）
  | 'chart'          // 数据图表
  | 'image'          // 图片
  | 'table'          // 表格
  | 'document'       // 长文档
  | 'json'           // JSON/YAML 数据
  | 'error-log';     // 错误日志/堆栈

interface ContentBlock {
  type: ContentType;
  content: string;
  language?: string;  // 代码语言
  metadata?: {
    filename?: string;
    additions?: number;
    deletions?: number;
    isNew?: boolean;
  };
}

// 内容类型检测函数
function detectContentType(codeBlock: string, language: string): ContentType {
  // 架构图语言
  if (['mermaid', 'plantuml', 'graphviz'].includes(language)) return 'diagram';

  // JSON/YAML 数据
  if (['json', 'yaml', 'yml'].includes(language)) return 'json';

  // 错误日志（包含 Error/Exception/Stack 等关键词）
  if (language === 'log' || /Error:|Exception|Stack trace/i.test(codeBlock)) {
    return 'error-log';
  }

  // 代码
  if (language) return 'code';

  return 'text';
}
```

### 2. 原始代码获取

Diff 对比需要获取文件的原始内容，有以下几种方式：

```typescript
interface OriginalCodeSource {
  // 方式1：从本地文件系统读取（调试模式）
  fromFileSystem: (filePath: string) => Promise<string>;

  // 方式2：从后端 API 获取（工作流模式）
  fromAPI: (projectId: string, filePath: string) => Promise<string>;

  // 方式3：从消息历史中提取（AI 之前的输出）
  fromMessageHistory: (threadId: string, filename: string) => string | null;
}

// 获取原始代码的优先级
async function getOriginalCode(
  projectPath: string,
  filename: string
): Promise<string | null> {
  // 1. 尝试从本地文件读取
  if (projectPath) {
    try {
      return await readFile(`${projectPath}/${filename}`);
    } catch { /* 文件不存在 */ }
  }

  // 2. 返回 null 表示新文件
  return null;
}
```

### 2. Diff 算法

使用 `diff-match-patch` 或类似库计算代码差异：

```typescript
import DiffMatchPatch from 'diff-match-patch';

function computeDiff(original: string, modified: string): DiffResult {
  const dmp = new DiffMatchPatch();
  const diffs = dmp.diff_main(original, modified);
  dmp.diff_cleanupSemantic(diffs);
  return diffs;
}
```

### 3. 架构图渲染

使用 Mermaid 渲染架构图：

```typescript
import mermaid from 'mermaid';

async function renderDiagram(code: string): Promise<string> {
  const { svg } = await mermaid.render('diagram-id', code);
  return svg;
}
```

### 4. 状态管理

```typescript
interface CodePanelState {
  isOpen: boolean;
  isCollapsed: boolean;
  activeFileId: string | null;
  expandedFiles: Set<string>;
  files: FileChange[];
}

interface FileChange {
  id: string;
  filename: string;
  originalContent: string;
  modifiedContent: string;
  additions: number;
  deletions: number;
  isNew: boolean;
}
```

---

## 新增组件

### 1. ContentCard 组件

视觉内容卡片，用于在气泡内展示架构图、图表、图片：

```
components/thread/ContentCard.tsx
```

### 2. CodePreviewButton 组件

代码预览入口按钮，显示摘要和点击入口：

```
components/thread/CodePreviewButton.tsx
```

### 3. CodePanel 组件

右侧代码预览面板，包含文件列表和 Diff 视图：

```
components/thread/CodePanel/
├── index.tsx           # 面板容器
├── FileList.tsx        # 文件列表
├── FileItem.tsx        # 单个文件项
├── SplitDiff.tsx       # 左右对比视图
└── CodePanel.css       # 样式
```

### 4. DiffViewer 组件

Split Diff 视图组件，显示左右对比：

```
components/thread/DiffViewer.tsx
```

---

## 样式规范

### 颜色

```css
/* 新增代码 */
--diff-addition-bg: #e6ffed;
--diff-addition-text: #28a745;

/* 删除代码 */
--diff-deletion-bg: #ffeef0;
--diff-deletion-text: #cb2431;

/* 原始代码区域 */
--diff-original-bg: #fffbe6;

/* 新文件标签 */
--new-file-bg: #e6ffed;
--new-file-text: #28a745;
```

### 尺寸

```css
/* 右侧面板 */
--code-panel-width: 520px;
--code-panel-collapsed-width: 40px;

/* 文件列表项高度 */
--file-item-height: 42px;

/* 代码行高 */
--code-line-height: 24px;
```

---

## 验收标准

1. **视觉内容展示**
   - 架构图在气泡内正确渲染
   - 支持放大和下载
   - 图片支持点击预览

2. **代码预览入口**
   - 显示代码摘要和文件名
   - 点击后右侧面板展开

3. **右侧面板**
   - 文件列表纵向排列
   - 支持展开/收起
   - 显示变更统计

4. **Split Diff 视图**
   - 左右对比正确显示
   - 新增/删除行高亮
   - 支持同步滚动

5. **面板收起**
   - 点击收起为侧边栏
   - 显示统计信息
   - 可重新展开

---

## 文件变更清单

### 新增文件

| 文件路径 | 说明 |
|----------|------|
| `components/thread/ContentCard.tsx` | 视觉内容卡片组件 |
| `components/thread/ContentCard.css` | 卡片样式 |
| `components/thread/CodePreviewButton.tsx` | 代码预览入口 |
| `components/thread/CodePanel/index.tsx` | 代码面板容器 |
| `components/thread/CodePanel/FileList.tsx` | 文件列表 |
| `components/thread/CodePanel/FileItem.tsx` | 文件项 |
| `components/thread/CodePanel/SplitDiff.tsx` | Diff 视图 |
| `components/thread/CodePanel/CodePanel.css` | 面板样式 |

### 修改文件

| 文件路径 | 说明 |
|----------|------|
| `pages/ThreadView.tsx` | 集成新组件，调整布局 |
| `pages/ThreadView.css` | 布局样式调整 |
| `store/debugThread.ts` | 新增代码面板状态 |
| `types/index.ts` | 新增内容类型定义 |

### 新增依赖

| 依赖包 | 用途 |
|--------|------|
| `diff-match-patch` | Diff 算法 |
| `mermaid` | 架构图渲染 |

---

## 后续优化

1. **代码语法高亮** - 使用 highlight.js 或 prism.js
2. **增量应用** - 支持选择性应用部分文件变更
3. **代码编辑** - 支持在 Diff 视图中编辑代码
4. **快捷键** - 添加键盘快捷操作
5. **全屏模式** - Diff 视图支持全屏对比

---

## 边界情况处理

### 空状态

面板无代码时显示引导文案：

```
┌───────────────────────────────────────┐
│                                       │
│         📄 暂无代码变更               │
│                                       │
│   AI 生成的代码将在这里展示对比       │
│                                       │
└───────────────────────────────────────┘
```

### 大文件处理

超过 1000 行的文件需要懒加载：

```typescript
const LARGE_FILE_THRESHOLD = 1000;

interface FileLoadState {
  isLoading: boolean;
  loadedLines: number;
  totalLines: number;
}

// 大文件提示
function LargeFileNotice({ totalLines }: { totalLines: number }) {
  return (
    <div className="large-file-notice">
      文件较大（{totalLines} 行），点击加载完整内容
    </div>
  );
}
```

### 多消息代码切换

当面板中存在多条消息的代码时，顶部增加消息切换：

```
┌───────────────────────────────────────────┐
│ [消息1] [消息2] [消息3]        [+86] [-12] │
├───────────────────────────────────────────┤
│ ...                                       │
└───────────────────────────────────────────┘
```

### 架构图渲染失败

Mermaid 渲染失败时显示源码：

```typescript
async function renderDiagram(code: string): Promise<{ svg: string } | { error: string }> {
  try {
    const { svg } = await mermaid.render('diagram-id', code);
    return { svg };
  } catch (error) {
    return { error: error.message };
  }
}

// 失败时降级展示
function DiagramCard({ code }: { code: string }) {
  const result = await renderDiagram(code);

  if ('error' in result) {
    return (
      <div className="diagram-error">
        <Alert type="warning" message="架构图渲染失败" />
        <pre className="diagram-source">{code}</pre>
      </div>
    );
  }

  return <div dangerouslySetInnerHTML={{ __html: result.svg }} />;
}
```

---

## 响应式设计

### 桌面端（>= 1024px）

- 右侧面板固定宽度 520px
- 可收起为侧边栏 40px
- Split Diff 左右并排显示

### 平板端（768px - 1023px）

- 右侧面板宽度调整为 100%
- 覆盖对话区域，支持滑动手势关闭
- Split Diff 改为上下堆叠显示

### 移动端（< 768px）

- 右侧面板改为底部抽屉
- 默认半屏高度，可拖拽调整
- 文件列表横向滑动
- Diff 视图全屏展示

```css
/* 响应式断点 */
@media (max-width: 1023px) {
  .code-panel {
    position: fixed;
    width: 100%;
    height: 60vh;
    bottom: 0;
    border-radius: 16px 16px 0 0;
  }

  .split-diff {
    flex-direction: column;
  }
}

@media (max-width: 767px) {
  .code-panel {
    height: 70vh;
  }
}
```

---

## 错误处理与加载状态

### 加载状态

```typescript
// 架构图渲染中
<Spin tip="渲染中...">
  <div className="diagram-placeholder" />
</Spin>

// 代码 Diff 计算中
<Skeleton active paragraph={{ rows: 10 }} />
```

### 错误处理

| 场景 | 处理方式 |
|------|----------|
| Mermaid 语法错误 | 显示源码 + 错误提示 |
| 文件读取失败 | 显示"无法获取原始代码"，仅显示变更后内容 |
| Diff 计算超时 | 显示完整内容，不做对比 |
| 网络请求失败 | 显示重试按钮 |