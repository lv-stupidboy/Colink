# TeamGraphEditor 有环图自动布局设计

## 背景

TeamGraphEditor 画布当前使用简单的水平排列布局（每行5个节点），不考虑图拓扑结构，导致：
- 边交叉过多，难以理解工作流
- 环路结构不明显，循环路径不清晰
- 缺乏自动布局，需手动拖拽调整

## 需求

- **图规模**：少于 10 个 Agent（小型团队）
- **环路业务含义**：循环执行（任务流转回到上游）+ 协作回路（两个 Agent 互相协作）
- **布局触发时机**：加载时自动布局 + 手动触发按钮

## 技术方案

采用 **dagre 层级布局**（`@dagrejs/dagre`）。

### 方案对比

| 方案 | 库 | 特点 | 适用性 |
|------|-----|------|--------|
| dagre 层级布局 | `@dagrejs/dagre` | 轻量（~50KB），ReactFlow 官方推荐 | ✅ 采用 |
| elkjs 层级布局 | `elkjs` | 更强大但过重（~400KB），适合大图 | 不采用 |
| 力导向布局 | `d3-force` | 环自然成圆形，小图不稳定 | 不采用 |

### 理由

1. **轻量**：50KB，适合小型团队图
2. **ReactFlow 兼容**：官方文档有集成示例，社区成熟
3. **层级清晰**：主流向从左到右，符合工作流直觉
4. **环路处理**：通过 Bezier 曲线 + 不同样式区分回边

## 设计细节

### 架构

```
TeamGraphEditor/
├── GraphCanvas.tsx          # ReactFlow 画布（现有）
├── useAutoLayout.ts         # 新增：自动布局 Hook
├── Toolbar.tsx              # 工具栏（现有，新增"自动布局"按钮）
├── graphUtils.ts            # 图工具函数（现有，新增布局函数）
└── useGraphStore.ts         # 状态管理（现有）
```

**新增依赖：**
- `@dagrejs/dagre` — 层级布局算法库

### 数据流

1. 加载团队数据 → `loadData()` → 调用 `useAutoLayout` 计算布局
2. 用户点击"自动布局"按钮 → 触发 `relayout()` → 更新节点位置
3. 编辑后手动拖拽 → 用户自行调整（可选）

### 环路可视化策略

**边样式区分：**

| 边类型 | 样式 | 说明 |
|--------|------|------|
| 正向边 | 实线 + 蓝色箭头 | 主流向（从左到右） |
| 回边（环路） | 虚线 + 橙色箭头 + Bezier 曲线 | 循环路径 |

**回边检测逻辑：**
```typescript
// 检测每条边是否指向层级更高的节点
function isBackEdge(edge: Edge, nodeLevels: Map<string, number>): boolean {
  const sourceLevel = nodeLevels.get(edge.source);
  const targetLevel = nodeLevels.get(edge.target);
  return targetLevel <= sourceLevel; // 指向同级或更低级 = 回边
}
```

**边样式配置：**
```typescript
// 正向边（默认样式）
const normalEdgeOptions = {
  type: 'default',
  markerEnd: { type: MarkerType.ArrowClosed },
};

// 回边（环路）
const backEdgeOptions = {
  type: 'smoothstep',        // 平滑阶梯线
  style: { stroke: '#fa8c16', strokeWidth: 2 },
  markerEnd: { type: MarkerType.ArrowClosed, color: '#fa8c16' },
};
```

### UI 组件变更

**Toolbar.tsx 新增按钮：**

- **位置**：在现有按钮组添加"自动布局"按钮
- **图标**：`ApartmentOutlined`（层级结构图标）
- **行为**：点击后调用 `relayout()` 重新计算布局
- **状态**：编辑模式下可用，预览模式下禁用

**布局参数配置（graphUtils.ts）：**
```typescript
const LAYOUT_CONFIG = {
  direction: 'LR',           // Left-to-Right（左到右布局）
  nodeSep: 50,               // 节点间距
  rankSep: 100,              // 层级间距
  edgeSep: 10,               // 边间距
};
```

### 实现要点

1. **useAutoLayout Hook**：
   - 输入：nodes, edges
   - 输出：计算后的节点位置
   - 使用 dagre 算法计算层级布局
   - 返回节点层级映射（用于回边检测）

2. **loadData 集成**：
   - 在 `useGraphStore.loadData()` 完成后调用布局
   - 替换当前的简单水平排列逻辑

3. **relayout 方法**：
   - 添加到 `useGraphStore` 或作为独立 Hook
   - 可由 Toolbar 按钮触发

## 测试要点

1. 加载包含环路的团队图，验证布局正确
2. 正向边和回边样式区分明显
3. "自动布局"按钮功能正常
4. 编辑模式拖拽节点后可重新布局

## 影响范围

- 新增文件：`web/src/pages/Workflow/TeamGraphEditor/useAutoLayout.ts`
- 修改文件：
  - `web/src/pages/Workflow/TeamGraphEditor/graphUtils.ts`
  - `web/src/pages/Workflow/TeamGraphEditor/Toolbar.tsx`
  - `web/src/pages/Workflow/TeamGraphEditor/useGraphStore.ts`
  - `web/src/pages/Workflow/TeamGraphEditor/GraphCanvas.tsx`
- 新增依赖：`@dagrejs/dagre`