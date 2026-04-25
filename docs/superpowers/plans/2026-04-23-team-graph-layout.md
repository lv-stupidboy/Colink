# TeamGraphEditor 有环图自动布局实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 TeamGraphEditor 画布添加 dagre 层级自动布局，解决边交叉、环路不清晰、缺乏自动布局的问题。

**Architecture:** 新增 useAutoLayout Hook 封装 dagre 布局算法，修改 graphUtils 添加布局配置，在 useGraphStore 添加 relayout 方法，GraphCanvas 处理边样式区分回边，Toolbar 新增"自动布局"按钮。

**Tech Stack:** React + ReactFlow (@xyflow/react) + dagre (@dagrejs/dagre) + Zustand

---

## 文件结构

| 文件 | 操作 | 说明 |
|------|------|------|
| `web/package.json` | 修改 | 新增 @dagrejs/dagre 依赖 |
| `web/src/pages/Workflow/TeamGraphEditor/useAutoLayout.ts` | 新增 | dagre 布局算法 Hook |
| `web/src/pages/Workflow/TeamGraphEditor/graphUtils.ts` | 修改 | 新增布局配置和布局函数 |
| `web/src/pages/Workflow/TeamGraphEditor/useGraphStore.ts` | 修改 | 新增 relayout 方法，修改 loadData 调用布局 |
| `web/src/pages/Workflow/TeamGraphEditor/GraphCanvas.tsx` | 修改 | 处理边样式，区分正向边和回边 |
| `web/src/pages/Workflow/TeamGraphEditor/Toolbar.tsx` | 修改 | 新增"自动布局"按钮 |

---

### Task 1: 安装 dagre 依赖

**Files:**
- Modify: `web/package.json`

- [ ] **Step 1: 安装 @dagrejs/dagre**

```bash
cd web && npm install @dagrejs/dagre
```

- [ ] **Step 2: 验证安装成功**

检查 `web/package.json` 中 `dependencies` 应包含：
```json
"@dagrejs/dagre": "^0.x.x"
```

- [ ] **Step 3: 提交依赖变更**

```bash
git add web/package.json web/package-lock.json
git commit -m "chore: add @dagrejs/dagre dependency for graph layout"
```

---

### Task 2: 创建 useAutoLayout Hook

**Files:**
- Create: `web/src/pages/Workflow/TeamGraphEditor/useAutoLayout.ts`

- [ ] **Step 1: 创建 useAutoLayout.ts 文件**

```typescript
// web/src/pages/Workflow/TeamGraphEditor/useAutoLayout.ts
import dagre from '@dagrejs/dagre';
import { MarkerType } from '@xyflow/react';
import type { Node, Edge } from '@xyflow/react';

export interface LayoutResult {
  nodes: Node[];
  nodeLevels: Map<string, number>;
}

export interface LayoutConfig {
  direction: 'LR' | 'TB';
  nodeSep: number;
  rankSep: number;
}

const DEFAULT_CONFIG: LayoutConfig = {
  direction: 'LR',
  nodeSep: 50,
  rankSep: 100,
};

/**
 * 使用 dagre 算法计算层级布局
 * @param nodes 节点列表
 * @param edges 边列表
 * @param config 布局配置
 * @returns 布局后的节点位置和层级映射
 */
export function calculateLayout(
  nodes: Node[],
  edges: Edge[],
  config: LayoutConfig = DEFAULT_CONFIG
): LayoutResult {
  const dagreGraph = new dagre.graphlib.Graph();
  
  dagreGraph.setGraph({
    rankdir: config.direction,
    nodesep: config.nodeSep,
    ranksep: config.rankSep,
  });
  
  dagreGraph.setDefaultEdgeLabel(() => ({}));
  
  // 添加节点
  nodes.forEach((node) => {
    dagreGraph.setNode(node.id, { width: 200, height: 80 });
  });
  
  // 添加边
  edges.forEach((edge) => {
    dagreGraph.setEdge(edge.source, edge.target);
  });
  
  // 计算布局
  dagre.layout(dagreGraph);
  
  // 获取节点位置
  const layoutedNodes = nodes.map((node) => {
    const nodeWithPosition = dagreGraph.node(node.id);
    return {
      ...node,
      position: {
        x: nodeWithPosition.x - 100, // 居中对齐（节点宽度 200）
        y: nodeWithPosition.y - 40,  // 居中对齐（节点高度 80）
      },
    };
  });
  
  // 计算节点层级（用于检测回边）
  const nodeLevels = new Map<string, number>();
  layoutedNodes.forEach((node) => {
    const nodeWithPosition = dagreGraph.node(node.id);
    nodeLevels.set(node.id, nodeWithPosition.rank ?? 0);
  });
  
  return { nodes: layoutedNodes, nodeLevels };
}

/**
 * 检测边是否为回边（指向层级更低或相同的节点）
 */
export function isBackEdge(
  edge: Edge,
  nodeLevels: Map<string, number>
): boolean {
  const sourceLevel = nodeLevels.get(edge.source) ?? 0;
  const targetLevel = nodeLevels.get(edge.target) ?? 0;
  return targetLevel <= sourceLevel;
}

/**
 * 应用边样式，区分正向边和回边
 * @param edges 边列表
 * @param nodeLevels 节点层级映射
 * @returns 样式化的边列表
 */
export function applyEdgeStyles(
  edges: Edge[],
  nodeLevels: Map<string, number>
): Edge[] {
  return edges.map((edge) => {
    if (isBackEdge(edge, nodeLevels)) {
      return {
        ...edge,
        type: 'smoothstep',
        style: { stroke: '#fa8c16', strokeWidth: 2 },
        markerEnd: {
          type: MarkerType.ArrowClosed,
          width: 20,
          height: 20,
          color: '#fa8c16',
        },
      };
    }
    return {
      ...edge,
      type: 'default',
      style: { stroke: 'var(--color-primary)', strokeWidth: 2 },
      markerEnd: {
        type: MarkerType.ArrowClosed,
        width: 20,
        height: 20,
      },
    };
  });
}

/**
 * Hook: 提供自动布局功能
 */
export function useAutoLayout() {
  return {
    calculateLayout,
    isBackEdge,
    applyEdgeStyles,
  };
}
```

- [ ] **Step 2: 提交新文件**

```bash
git add web/src/pages/Workflow/TeamGraphEditor/useAutoLayout.ts
git commit -m "feat: add useAutoLayout hook for dagre layout"
```

---

### Task 3: 修改 graphUtils.ts 添加布局配置

**Files:**
- Modify: `web/src/pages/Workflow/TeamGraphEditor/graphUtils.ts:1-72`

- [ ] **Step 1: 在 graphUtils.ts 顶部添加布局配置**

在文件顶部（import 语句后）添加：

```typescript
// web/src/pages/Workflow/TeamGraphEditor/graphUtils.ts
import type { Node, Edge } from '@xyflow/react';
import type { AgentConfig, Transition, WorkflowTemplate } from '@/types';

// 布局配置
export const LAYOUT_CONFIG = {
  direction: 'LR' as const,   // Left-to-Right
  nodeSep: 50,                // 节点间距
  rankSep: 100,               // 层级间距
};
```

- [ ] **Step 2: 移除旧的 calculateLayout 函数**

删除旧的 `calculateLayout` 函数（第53-72行），因为新的布局逻辑在 useAutoLayout.ts 中。

保留 `toFlowData` 和 `toWorkflowData` 函数不变。

- [ ] **Step 3: 提交修改**

```bash
git add web/src/pages/Workflow/TeamGraphEditor/graphUtils.ts
git commit -m "refactor: add layout config, remove old calculateLayout"
```

---

### Task 4: 修改 useGraphStore.ts 添加 relayout 和集成布局

**Files:**
- Modify: `web/src/pages/Workflow/TeamGraphEditor/useGraphStore.ts`

- [ ] **Step 1: 导入布局函数**

在文件顶部添加导入：

```typescript
import { calculateLayout, applyEdgeStyles } from './useAutoLayout';
import { MarkerType } from '@xyflow/react';
```

- [ ] **Step 2: 添加 relayout action 到 GraphActions 接口**

在 `GraphActions` 接口中添加（约第43行后）：

```typescript
relayout: () => void;
```

- [ ] **Step 3: 实现 relayout 方法**

在 store 实现中添加 relayout 方法（约第130行后，setHasChanges 之后）：

```typescript
relayout: () => {
  const { nodes, edges } = get();
  if (nodes.length === 0) return;
  
  const { nodes: layoutedNodes, nodeLevels } = calculateLayout(nodes, edges);
  const styledEdges = applyEdgeStyles(edges, nodeLevels);
  
  set({ nodes: layoutedNodes, edges: styledEdges });
},
```

注意：MarkerType 导入已在 Step 1 中添加。

- [ ] **Step 4: 修改 loadData 调用布局**

修改 `loadData` 方法，在设置 nodes 和 edges 后调用布局：

找到约第158-165行的 set 调用，修改为：

```typescript
// 先设置原始数据
const rawNodes = (workflow.agentIds || []).map((agentId: string, index: number) => {
  const agent = agents.find((a: AgentConfig) => a.id === agentId);
  return {
    id: agentId,
    type: 'agentNode',
    position: { x: 100 + index * 150, y: 100 },
    data: { agent: agent || { id: agentId, name: 'Unknown' } },
  };
});

const rawEdges = (workflow.transitions || []).map((t: Transition) => ({
  id: `${t.fromAgentId}-${t.toAgentId}`,
  source: t.fromAgentId,
  target: t.toAgentId,
  data: { triggerHint: t.triggerHint || '' },
}));

// 计算布局并应用边样式
const { nodes: layoutedNodes, nodeLevels } = calculateLayout(rawNodes, rawEdges);
const styledEdges = applyEdgeStyles(rawEdges, nodeLevels);

set({
  teamName: workflow.name || '',
  nodes: layoutedNodes,
  edges: styledEdges,
  allAgents: agents || [],
  hasChanges: false,
  error: null,
});
```

- [ ] **Step 5: 修改 addEdge 触发 relayout**

修改 `addEdge` 方法（约第92-110行），添加边后触发 relayout：

```typescript
addEdge: (sourceId, targetId) => {
  const edges = get().edges;
  const existingEdge = edges.find(
    e => (e.source === sourceId && e.target === targetId) ||
         (e.source === targetId && e.target === sourceId)
  );

  if (existingEdge) {
    set({ error: '该 Agent 之间已存在连线，无需重复添加' });
    return;
  }

  const newEdge: Edge = {
    id: `${sourceId}-${targetId}`,
    source: sourceId,
    target: targetId,
    data: { triggerHint: '' },
  };

  // 添加边后触发 relayout 以应用正确的边样式
  set({ edges: [...edges, newEdge], hasChanges: true, error: null });
  get().relayout();
},
```

注意：这里调用 `get().relayout()` 确保新添加的边获得正确的样式（正向边或回边）。

- [ ] **Step 6: 提交修改**

```bash
git add web/src/pages/Workflow/TeamGraphEditor/useGraphStore.ts
git commit -m "feat: add relayout action and integrate layout in loadData"
```

---

### Task 5: 修改 GraphCanvas.tsx 处理边样式

**Files:**
- Modify: `web/src/pages/Workflow/TeamGraphEditor/GraphCanvas.tsx`

- [ ] **Step 1: 移除 defaultEdgeOptions**

由于边样式已在 useGraphStore 中设置，移除 `defaultEdgeOptions` 配置（约第35-43行）。

删除：
```typescript
const defaultEdgeOptions = {
  type: 'default',
  markerEnd: {
    type: MarkerType.ArrowClosed,
    width: 20,
    height: 20,
  },
};
```

- [ ] **Step 2: 更新 ReactFlow 组件**

移除 `defaultEdgeOptions={defaultEdgeOptions}` 属性（约第139行）。

ReactFlow 组件现在直接使用从 store 中传入的已样式化的 edges。

- [ ] **Step 3: 提交修改**

```bash
git add web/src/pages/Workflow/TeamGraphEditor/GraphCanvas.tsx
git commit -m "refactor: remove defaultEdgeOptions, edge styling moved to store"
```

---

### Task 6: 修改 Toolbar.tsx 添加自动布局按钮

**Files:**
- Modify: `web/src/pages/Workflow/TeamGraphEditor/Toolbar.tsx`

- [ ] **Step 1: 导入图标和 relayout**

修改导入语句：

```typescript
import {
  PlusOutlined,
  SaveOutlined,
  EyeOutlined,
  EditOutlined,
  ApartmentOutlined,
} from '@ant-design/icons';
```

在 useGraphStore 获取中添加 relayout：

```typescript
const {
  mode,
  setMode,
  allAgents,
  nodes,
  addNode,
  hasChanges,
  saveChanges,
  saving,
  relayout,
} = useGraphStore();
```

- [ ] **Step 2: 添加自动布局按钮**

在"添加 Agent" Dropdown 后（约第77行后）添加按钮：

```tsx
<Button
  icon={<ApartmentOutlined />}
  onClick={relayout}
  disabled={nodes.length === 0}
>
  自动布局
</Button>
```

完整修改后的编辑模式区域：

```tsx
{mode === 'edit' && (
  <>
    <Dropdown
      menu={{ items: agentMenuItems }}
      disabled={availableAgents.length === 0}
    >
      <Button icon={<PlusOutlined />}>
        添加 Agent
      </Button>
    </Dropdown>

    <Button
      icon={<ApartmentOutlined />}
      onClick={relayout}
      disabled={nodes.length === 0}
    >
      自动布局
    </Button>

    {hasChanges && (
      <Button
        type="primary"
        icon={<SaveOutlined />}
        onClick={handleSave}
        loading={saving}
      >
        保存
      </Button>
    )}
  </>
)}
```

- [ ] **Step 3: 提交修改**

```bash
git add web/src/pages/Workflow/TeamGraphEditor/Toolbar.tsx
git commit -m "feat: add auto-layout button to Toolbar"
```

---

### Task 7: 测试验证

**Files:**
- Test: 手动验证功能

- [ ] **Step 1: 启动前端开发服务器**

```bash
cd web && npm run dev
```

- [ ] **Step 2: 打开团队图编辑器**

导航到 Agent团队 → 团队管理，选择一个团队进入 TeamGraphEditor。

- [ ] **Step 3: 验证加载时自动布局**

- 观察节点是否按层级排列（从左到右）
- 环路边是否显示为橙色 + smoothstep 样式
- 正向边是否显示为蓝色/默认颜色 + default 样式

- [ ] **Step 4: 验证手动触发布局**

- 添加新 Agent 后，点击"自动布局"按钮
- 验证节点重新排列，边样式正确更新

- [ ] **Step 5: 验证环路检测**

创建一个包含环路的团队（A → B → C → A），验证：
- 回边（C → A）显示为橙色
- 正向边（A → B, B → C）显示为默认颜色

- [ ] **Step 6: 验证新增边样式**

添加一条新边（例如 D → A 形成新环路），验证：
- 新边自动获得正确的样式（正向边或回边）
- 点击"自动布局"后，边样式保持正确

---

## Self-Review Checklist

### Spec Coverage

| Spec Requirement | Task |
|------------------|------|
| 新增 @dagrejs/dagre 依赖 | Task 1 |
| useAutoLayout Hook + applyEdgeStyles | Task 2 |
| 布局配置 LAYOUT_CONFIG | Task 3 |
| relayout 方法 | Task 4 |
| loadData 集成布局 | Task 4 |
| addEdge 触发 relayout | Task 4 |
| 边样式区分回边 | Task 2, Task 4, Task 5 |
| Toolbar 自动布局按钮 | Task 6 |
| 测试验证 | Task 7 |

### Code Quality

- ✅ 边样式代码统一使用 `applyEdgeStyles` 函数，无重复
- ✅ addEdge 添加边后触发 relayout，确保新边获得正确样式

### Placeholder Scan

- ✅ 无 TBD/TODO
- ✅ 所有代码步骤有完整代码块
- ✅ 无"类似 Task N"引用
- ✅ 命令有预期输出

### Type Consistency

- ✅ `calculateLayout` 返回 `{ nodes: Node[], nodeLevels: Map<string, number> }`
- ✅ `isBackEdge` 参数 `(edge: Edge, nodeLevels: Map<string, number>)`
- ✅ `applyEdgeStyles` 参数 `(edges: Edge[], nodeLevels: Map<string, number>)` 返回 `Edge[]`
- ✅ `relayout` 在 GraphActions 接口和实现中签名一致