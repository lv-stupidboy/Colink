# 项目绑定工作流功能设计

## 概述

在项目空间中，项目设置支持绑定工作流，在进行任务开发时使用该工作流进行开发。

## 需求

- 一个项目绑定一个默认工作流
- 未绑定时使用系统默认工作流（通过 `is_default` 标记）
- 创建任务时自动使用对应工作流

## 数据模型变更

### 1. Project 模型

`internal/model/project.go`

```go
type Project struct {
    // ... 现有字段 ...
    WorkflowTemplateID *uuid.UUID `json:"workflow_template_id,omitempty"` // 绑定的工作流模板ID
    // ... 其他字段 ...
}
```

### 2. WorkflowTemplate 模型

`internal/model/workflow_template.go`

```go
type WorkflowTemplate struct {
    // ... 现有字段 ...
    IsDefault bool `json:"is_default"` // 是否为默认工作流
    // ... 其他字段 ...
}
```

### 3. 数据库迁移

- `projects` 表新增 `workflow_template_id` 列（UUID，可为空，外键关联 `workflow_templates.id`）
- `workflow_templates` 表新增 `is_default` 列（BOOLEAN，默认 false）

## 后端 API 变更

### 1. 工作流模板 API

#### 设置默认工作流

```
PUT /api/workflows/:id/default
```

**逻辑：**
- 将指定工作流的 `is_default` 设为 true
- 自动将其他所有工作流的 `is_default` 设为 false
- 返回更新后的工作流信息

**响应：**
```json
{
  "id": "uuid",
  "name": "标准 TDD 开发流程",
  "is_default": true,
  ...
}
```

#### 删除工作流校验

`DELETE /api/workflows/:id` 增加校验：

1. 检查是否被项目引用
   ```sql
   SELECT COUNT(*) FROM projects WHERE workflow_template_id = ?
   ```
   若存在引用，返回 400 错误：
   ```json
   {
     "error": "该工作流已被 N 个项目绑定，无法删除",
     "project_count": N
   }
   ```

2. 检查是否为默认工作流
   若 `is_default=true`，返回 400 错误：
   ```json
   {
     "error": "该工作流是系统默认工作流，请先设置其他工作流为默认"
   }
   ```

### 2. 项目 API

#### 更新项目请求

`internal/model/project.go`

```go
type UpdateProjectRequest struct {
    Name              *string      `json:"name"`
    Description       *string      `json:"description"`
    Type              *ProjectType `json:"type"`
    Mode              *ProjectMode `json:"mode"`
    Status            *ProjectStatus `json:"status"`
    RepositoryUrl     *string      `json:"repository_url"`
    WorkflowTemplateID *uuid.UUID  `json:"workflow_template_id"` // 新增，可为null表示解绑
}
```

#### 获取项目详情

返回项目信息时，若 `workflow_template_id` 不为空，在响应中包含关联的工作流模板信息：

```json
{
  "id": "project-uuid",
  "name": "我的项目",
  "workflow_template_id": "workflow-uuid",
  "workflow_template": {
    "id": "workflow-uuid",
    "name": "标准 TDD 开发流程",
    "agent_ids": ["agent-1", "agent-2"],
    "checkpoints": ["需求确认", "方案确认"]
  },
  ...
}
```

### 3. 任务创建逻辑

创建 Thread 时的工作流选择逻辑：

```go
func (s *ThreadService) Create(ctx context.Context, projectID uuid.UUID) (*Thread, error) {
    // 1. 获取项目信息
    project, err := s.projectRepo.GetByID(ctx, projectID)
    if err != nil {
        return nil, err
    }

    var workflowID *uuid.UUID

    // 2. 检查项目是否绑定了工作流
    if project.WorkflowTemplateID != nil {
        // 验证工作流是否存在
        _, err := s.workflowRepo.GetByID(ctx, *project.WorkflowTemplateID)
        if err != nil {
            return nil, errors.New("项目绑定的工作流不存在，请重新配置")
        }
        workflowID = project.WorkflowTemplateID
    } else {
        // 3. 查询默认工作流
        defaultWorkflow, err := s.workflowRepo.GetDefault(ctx)
        if err != nil {
            return nil, errors.New("请先在项目设置中绑定工作流，或设置系统默认工作流")
        }
        workflowID = &defaultWorkflow.ID
    }

    // 4. 创建 Thread 并关联工作流
    thread := &model.Thread{
        ProjectID:         projectID,
        WorkflowTemplateID: workflowID,
        Status:            model.ThreadStatusIdle,
        CurrentPhase:      model.PhaseRequirement,
    }
    // ...
}
```

## 前端变更

### 1. 项目详情页 - 项目设置 Tab

在"项目设置"标签页中新增工作流绑定配置：

**位置：** `isdp/web/src/pages/ProjectDetail/index.tsx`

**UI 结构：**
```
┌─────────────────────────────────────────────────────┐
│ 项目配置                                             │
├─────────────────────────────────────────────────────┤
│ 项目 ID:    xxx                                     │
│ 项目名称:   xxx                                     │
│ 项目类型:   服务                                    │
│ 开发模式:   全新开发                                │
│                                                     │
│ 绑定工作流: [下拉选择框 ▼]                          │
│             - 标准 TDD 开发流程                     │
│             - 快速开发流程                          │
│             - 不绑定（使用系统默认）                │
│                                                     │
│             工作流信息:                             │
│             - Agent: 需求分析师 → 架构师 → ...     │
│             - 检查点: 需求确认, 方案确认            │
│                                                     │
│             [编辑项目信息]                          │
└─────────────────────────────────────────────────────┘
```

**交互：**
- 下拉框列出所有工作流模板
- 选择"不绑定"时 `workflow_template_id` 设为 null
- 选中工作流时显示其 Agent 列表和检查点信息
- 更新后自动刷新显示

### 2. 工作流页面 - 默认标记

**位置：** `isdp/web/src/pages/Workflow/index.tsx`

**变更：**
- 工作流模板卡片上，默认工作流显示 `<Tag color="gold">默认</Tag>` 标签
- 非默认工作流增加"设为默认"操作按钮
- 调用 `PUT /api/workflows/:id/default` 设置默认

**UI 示例：**
```
┌─────────────────────────────────────────────────────┐
│ 标准 TDD 开发流程  [默认] [系统预设]                │
│ 预计耗时: 2-3小时                                   │
│ Agent: 需求分析师 → 架构师 → 开发者 → 评审者        │
│ 检查点: 需求确认, 方案确认, 代码合入                │
│                                      [删除]         │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│ 快速开发流程                                        │
│ 预计耗时: 1小时                                     │
│ Agent: 开发者                                       │
│ 检查点: 代码合入                                    │
│                          [设为默认]  [删除]         │
└─────────────────────────────────────────────────────┘
```

### 3. 创建任务弹窗

**位置：** `isdp/web/src/pages/ProjectDetail/index.tsx` - 创建任务 Modal

**变更：**
- 显示将使用的工作流名称
- 标注来源（项目绑定 / 系统默认）

**UI 示例：**
```
┌─────────────────────────────────────────────────────┐
│ 新建开发任务                                         │
├─────────────────────────────────────────────────────┤
│ 任务名称（可选）: [________________]                │
│                                                     │
│ 使用工作流: 标准 TDD 开发流程                       │
│            （来自项目绑定）                         │
│                                                     │
│                              [取消]  [创建任务]     │
└─────────────────────────────────────────────────────┘
```

### 4. 类型定义更新

`isdp/web/src/types/index.ts`

```typescript
// Project 接口新增字段
export interface Project {
  // ... 现有字段 ...
  workflowTemplateId?: string;
  workflowTemplate?: WorkflowTemplate;
}

// WorkflowTemplate 接口新增字段
export interface WorkflowTemplate {
  // ... 现有字段 ...
  isDefault: boolean;
}
```

## 错误处理与边界情况

### 1. 工作流删除校验

| 场景 | 处理 |
|------|------|
| 工作流被项目引用 | 返回 400 错误，列出引用的项目数量 |
| 工作流是默认工作流 | 返回 400 错误，提示先设置其他工作流为默认 |

### 2. 任务创建边界情况

| 场景 | 处理 |
|------|------|
| 项目绑定的工作流已删除 | 提示"项目绑定的工作流不存在，请重新配置" |
| 项目未绑定，无默认工作流 | 提示"请先在项目设置中绑定工作流，或设置系统默认工作流" |
| 项目未绑定，有默认工作流 | 正常使用默认工作流 |

### 3. 设置默认工作流

- 同一时间只能有一个默认工作流
- 设置新默认时自动清除旧标记
- 系统预设和用户自定义工作流均可设为默认

## 文件变更清单

### 后端
- `internal/model/project.go` - 新增 WorkflowTemplateID 字段
- `internal/model/workflow_template.go` - 新增 IsDefault 字段
- `internal/repo/workflow_template.go` - 新增 GetDefault、SetDefault 方法
- `internal/api/workflow_handler.go` - 新增 SetDefault 端点，增强 Delete 校验
- `internal/repo/project.go` - 更新查询包含工作流信息
- `internal/service/thread/` - 创建任务时的工作流选择逻辑
- 数据库迁移文件

### 前端
- `web/src/types/index.ts` - 更新类型定义
- `web/src/pages/ProjectDetail/index.tsx` - 项目设置工作流绑定
- `web/src/pages/Workflow/index.tsx` - 默认工作流标记和设置
- `web/src/api/client.ts` - 新增 setDefaultWorkflow API