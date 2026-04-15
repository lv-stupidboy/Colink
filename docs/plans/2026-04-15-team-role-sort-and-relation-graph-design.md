# 团队角色排序与关系图查看设计

**日期**: 2026-04-15
**状态**: 已确认
**作者**: Claude

---

## 需求概述

1. **团队角色排序**: 手动拖拽调整 Agent 在团队内的显示顺序
2. **团队关系图查看**: 基于 transitions 数据展示 Agent 间流转关系，支持卡片/关系图视图切换，仅展示不编辑

---

## 整体架构

### 功能入口

- **排序功能**: 团队卡片内的 Agent 区域，添加拖拽手柄，支持拖拽调整顺序
- **关系图切换**: 团队卡片头部新增「卡片视图 / 关系图视图」切换按钮

### 文件结构

```
web/src/pages/Workflow/
├── index.tsx              # 主页面（现有）
├── Workflow.css           # 样式（现有）
├── TeamCard.tsx           # 团队卡片组件（新建，从 index.tsx 抽取）
├── TeamRelationGraph.tsx  # 关系图组件（新建）
├── AgentAvatar.tsx        # Agent 头像组件（新建，支持拖拽）
└── useAgentDragSort.ts    # 拖拽排序 hook（新建）
```

### 状态管理

使用组件内 state，不涉及全局状态：
- `viewMode: 'card' | 'graph'` — 当前视图模式
- 拖拽排序直接更新 `agentIds` 并调用 API 保存

---

## 角色排序功能

### 交互设计

**拖拽触发:**
- Agent 头像左侧新增拖拽手柄图标（使用 `HolderOutlined` 图标）
- 鼠标悬停手柄时，整个 Agent 卡块高亮显示可拖拽状态
- 拖拽过程中，其他 Agent 卡块自动腾出位置

**拖拽完成:**
- 松开鼠标后，前端更新本地 `agents` 数组顺序
- 调用现有 `api.workflows.update` API，更新 `agentIds` 字段
- transitions 保持不变（关系由触发配置定义，排序不影响）

### 技术实现

使用 CSS 原生拖拽实现:

```typescript
// useAgentDragSort.ts hook 核心逻辑
const useAgentDragSort = (
  teamId: string,
  agents: TeamAgent[],
  onSave: (agentIds: string[]) => void
) => {
  // 记录拖拽状态：draggingIndex, dragOverIndex
  // 拖拽开始：记录起始位置
  // 拖拽经过：计算插入位置，更新 CSS order 属性
  // 拖拽结束：生成新顺序数组，调用 onSave
};
```

**样式处理:**
- 拖拽中的 Agent 添加 `.dragging` 类，设置 `opacity: 0.5`
- 目标位置添加 `.drag-over` 类，预留空间
- 使用 CSS `order` 属性实现实时位置交换效果

### API 复用

复用现有 `handleUpdateTeam` 逻辑:
- 调用 `api.workflows.update(teamId, { agentIds, transitions })`
- transitions 保持原值不变

---

## 关系图查看功能

### 视图切换

**切换按钮位置:**
- 团队卡片头部右侧，与删除按钮并排
- 使用 `SwitcherOutlined` 图标，点击切换「卡片」/「关系图」模式
- 每个团队卡片独立切换（不全局统一）

### 关系图渲染

**节点设计:**
- 复用现有 Agent 头像样式（圆形 + 图标 + 名称）
- 系统角色显示皇冠图标，自定义角色显示用户图标
- 节点按 `agentIds` 顺序横向排列，间距固定

**连线设计:**
- 基于 `transitions` 数据绘制 SVG 路径
- 使用 `path` 元素绘制贝塞尔曲线，避免直线交叉
- 箭头指向目标 Agent
- 连线颜色:
  - `sequence`（顺序）：蓝色实线
  - `parallel`（并行）：绿色实线
  - `merge`（汇聚）：橙色实线

**触发提示显示:**
- 连线中点上方显示 `triggerHint` 文本
- 文字过长时截断 + Tooltip 显示完整内容

### SVG 布局算法

```typescript
// TeamRelationGraph.tsx 核心布局逻辑
const layoutAgents = (agents: TeamAgent[], width: number) => {
  const nodeWidth = 80;
  const nodeHeight = 100;
  const gap = 40;
  const startX = 60;

  return agents.map((agent, index) => ({
    x: startX + index * (nodeWidth + gap),
    y: 80,
    ...agent
  }));
};
```

**连线路径计算:**
- 起点：源节点右侧中心 `(x + nodeWidth/2, y)`
- 终点：目标节点左侧中心 `(x - nodeWidth/2, y)`
- 使用 quadratic bezier 曲线，中点偏移避免重叠

### 样式适配

- 关系图容器设置固定高度 `200px`，支持 overflow 横向滚动
- 深色模式：节点背景、连线颜色使用 CSS 变量
- 响应式：节点过多时自动缩放或分页展示

---

## 实现步骤

**第一阶段：角色拖拽排序**
1. 新建 `useAgentDragSort.ts` hook
2. 新建 `AgentAvatar.tsx` 组件，集成拖拽手柄
3. 抽取 `TeamCard.tsx` 组件，替换现有内联渲染
4. 验证排序 API 调用正确

**第二阶段：关系图视图**
1. 新建 `TeamRelationGraph.tsx` 组件
2. 在 `TeamCard` 添加视图切换按钮
3. 实现 SVG 节点渲染和连线绘制
4. 样式适配深色模式

**第三阶段：整合优化**
1. 合并测试，确保两种视图切换流畅
2. 处理边缘情况（空团队、单 Agent 团队）
3. 添加 loading 状态和错误处理

---

## 测试要点

**排序功能测试:**
- 拖拽单个 Agent 到新位置，验证顺序保存
- 拖拽到边界位置（第一个/最后一个）
- 连续拖拽多次，验证数据一致性
- 系统团队（isSystem=true）不允许排序

**关系图测试:**
- 2 个 Agent 的简单流转关系
- 多个 Agent 的复杂流转（多个 incoming/outgoing）
- 无 transitions 的团队显示空关系图提示
- 触发提示文字截断和 Tooltip 显示

**兼容性测试:**
- 深色模式样式正确
- 视图切换时数据不丢失
- 窗口缩放时关系图布局不错乱

---

## 不实现的功能（YAGNI）

- ❌ 关系图内拖拽节点位置
- ❌ 在关系图中编辑连线
- ❌ 全局角色排序规则
- ❌ 导出关系图为图片