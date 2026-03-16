# Workflow Template Edit Functionality Design

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable editing of existing workflow templates with drag-and-drop agent reordering.

**Architecture:** Reuse existing create Modal for edit mode. Add edit button to each workflow card, toggle Modal into edit mode, pre-fill form with existing data. Use @dnd-kit/sortable for drag-and-drop agent ordering within the Modal.

**Tech Stack:** React, TypeScript, Ant Design, @dnd-kit/sortable

**Required Imports:**
```typescript
import { useState } from 'react';
import { DndContext, DragEndEvent } from '@dnd-kit/core';
import { SortableContext, useSortable, arrayMove } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { Button, message, Select } from 'antd';
import { HolderOutlined, DeleteOutlined } from '@ant-design/icons';
import type { WorkflowTemplate, AgentConfig } from '@/types';
```

---

## Overview

### Problem Statement
Workflow templates in `web/src/pages/Workflow/index.tsx` currently support create, delete, and set-default operations, but lack edit functionality. Users cannot modify workflow name, description, agent configuration, or checkpoints after creation.

### Solution
Add edit capability by reusing the existing create Modal. When user clicks edit on a workflow card, open the Modal in edit mode with pre-filled data. Allow drag-and-drop reordering of agents to control execution order.

---

## Architecture

### Component Changes

**Modified File:** `web/src/pages/Workflow/index.tsx`

**Existing State Variables (reuse):**
- `agents: AgentConfig[]` - Already exists, list of available agent configurations
- `workflowTemplates: WorkflowTemplate[]` - Already exists, list of workflow templates
- `createModalVisible: boolean` - Already exists, controls Modal visibility
- `form: FormInstance` - Already exists, Ant Design form instance

**New State Variables:**
```typescript
const [editMode, setEditMode] = useState(false);
const [editingTemplate, setEditingTemplate] = useState<WorkflowTemplate | null>(null);
const [selectedAgentIds, setSelectedAgentIds] = useState<string[]>([]);
```

**State Synchronization:**
- `selectedAgentIds` is the source of truth for the drag-and-drop list
- On edit open: `setSelectedAgentIds(template.agentIds)`
- On form submit: read from `selectedAgentIds`, not form field
- The traditional Select component is replaced by the sortable list

**State Flow:**
```
User clicks "Edit" button
  → handleEditClick(template)
  → setEditMode(true)
  → setEditingTemplate(template)
  → setSelectedAgentIds(template.agentIds)
  → form.setFieldsValue({ name, description, checkpoints })
  → setCreateModalVisible(true)
  → Modal opens in edit mode
```

**Event Handlers:**

```typescript
// Called when edit button is clicked
const handleEditClick = (template: WorkflowTemplate) => {
  setEditMode(true);
  setEditingTemplate(template);
  setSelectedAgentIds(template.agentIds || []);
  form.setFieldsValue({
    name: template.name,
    description: template.description,
    checkpoints: template.checkpoints,
  });
  setCreateModalVisible(true);
};

// Called when drag ends - reorder agent list
const handleDragEnd = (event: DragEndEvent) => {
  const { active, over } = event;
  if (over && active.id !== over.id) {
    setSelectedAgentIds((prev) => {
      const oldIndex = prev.indexOf(active.id as string);
      const newIndex = prev.indexOf(over.id as string);
      const newList = arrayMove(prev, oldIndex, newIndex);
      return newList;
    });
  }
};

// Called when remove button clicked on agent item
const handleRemoveAgent = (agentId: string) => {
  setSelectedAgentIds((prev) => prev.filter((id) => id !== agentId));
};
```

### Modal Reuse Pattern

| Mode | Modal Title | Submit Button | Submit Handler |
|------|-------------|---------------|----------------|
| Create | "自定义工作流" | "创建" | `handleCreateWorkflow` |
| Edit | "编辑工作流" | "保存" | `handleEditWorkflow` |

**Form Submit Handler Toggle:**

The Form's `onFinish` handler must switch based on `editMode`:

```tsx
<Form
  form={form}
  layout="vertical"
  onFinish={editMode ? handleEditWorkflow : handleCreateWorkflow}
>
```

### Form Validation Strategy

The agent selection uses a custom state (`selectedAgentIds`) outside of Ant Design's Form system. This is intentional because:
1. Drag-and-drop reordering requires array state management
2. The sortable list UI is more intuitive than a multi-select dropdown

**Validation Approach:**
- Name, description, checkpoints: Use Ant Design Form validation (rules in Form.Item)
- Agent selection: Custom validation in submit handler

```tsx
const handleEditWorkflow = async (values: any) => {
  // Custom validation for agents
  if (selectedAgentIds.length === 0) {
    message.error('请选择至少一个Agent实例');
    return;
  }
  // ... rest of handler
};
```

**Note:** The existing `agentIds` Form.Item (line 439-452 in original code) should be replaced with the sortable list UI. Do not keep both.

### Agent Drag-and-Drop

**Library:** `@dnd-kit/sortable`

**Implementation:**
- Replace static Select for agents with sortable list
- Each agent item shows: name, role, drag handle
- User can drag to reorder agent execution sequence
- Order persists in `agentIds` array (order matters)

---

## UI/UX Design

### Edit Button Placement

Location: Inside each workflow card's header area, next to "设为默认" button.

```tsx
<Button
  type="link"
  size="small"
  onClick={(e) => {
    e.stopPropagation();
    handleEditClick(template);
  }}
>
  编辑
</Button>
```

**Note:** Edit button appears on ALL workflow cards, including default and system templates. All workflows should be editable.
```

### Modal Title Toggle

```tsx
<Modal
  title={editMode ? "编辑工作流" : "自定义工作流"}
  okText={editMode ? "保存" : "创建"}
  onCancel={handleModalCancel}
  // ...
>
```

### Modal Cancel Handler

Reset all edit-related state when user cancels:

```typescript
const handleModalCancel = () => {
  setCreateModalVisible(false);
  setEditMode(false);
  setEditingTemplate(null);
  setSelectedAgentIds([]);
  form.resetFields();
};
```

### Hide "基于模板" Field in Edit Mode

The existing create Modal has a "基于模板" (`basedOn`) select field. This is only relevant for creating new workflows and should be hidden in edit mode:

```tsx
{!editMode && (
  <Form.Item name="basedOn" label="基于模板">
    <Select placeholder="选择模板作为基础" allowClear>
      {/* ... options ... */}
    </Select>
  </Form.Item>
)}
```

**Note:** The `basedOn` field behavior in create mode (pre-populating fields when a template is selected) is **out of scope** for this implementation. It's preserved as-is from existing code.

### Agent Selection UI

Replace the existing agent Select with a hybrid approach:

1. **Add Agent Button**: Opens a dropdown/modal to select agents to add
2. **Sortable List**: Shows currently selected agents with drag handles

```tsx
<Form.Item label="Agent实例">
  <div className="agent-selection-container">
    {/* Sortable list of selected agents */}
    {selectedAgentIds.length > 0 && (
      <DndContext onDragEnd={handleDragEnd}>
        <SortableContext items={selectedAgentIds}>
          {selectedAgentIds.map((agentId) => (
            <SortableAgentItem
              key={agentId}
              id={agentId}
              agent={agents.find(a => a.id === agentId)}
              onRemove={() => handleRemoveAgent(agentId)}
            />
          ))}
        </SortableContext>
      </DndContext>
    )}

    {/* Add agent dropdown */}
    <Select
      placeholder="选择Agent添加"
      style={{ width: '100%' }}
      onSelect={(value) => {
        if (!selectedAgentIds.includes(value)) {
          setSelectedAgentIds([...selectedAgentIds, value]);
        }
      }}
    >
      {agents
        .filter(a => !selectedAgentIds.includes(a.id))
        .map(agent => (
          <Select.Option key={agent.id} value={agent.id}>
            {agent.name} ({AgentRoleLabels[agent.role]})
          </Select.Option>
        ))}
    </Select>

    <div className="hint-text">拖拽调整执行顺序</div>
  </div>
</Form.Item>
```

### Drag-and-Drop Agent List

**Visual Design:**
- Each agent item: horizontal card with drag handle (≡) on left
- Agent name and role displayed
- Delete button (X) on right to remove agent
- Reorder hint text: "拖拽调整执行顺序"

**Sortable List Structure:**
```tsx
<DndContext onDragEnd={handleDragEnd}>
  <SortableContext items={selectedAgentIds}>
    {selectedAgentIds.map((agentId) => (
      <SortableAgentItem
        key={agentId}
        id={agentId}
        agent={agents.find(a => a.id === agentId)}
        onRemove={() => handleRemoveAgent(agentId)}
      />
    ))}
  </SortableContext>
</DndContext>
```

**SortableAgentItem Component:**

Create a new component inside the file (or as a separate file):

```tsx
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { HolderOutlined, DeleteOutlined } from '@ant-design/icons';

interface SortableAgentItemProps {
  id: string;
  agent?: AgentConfig;
  onRemove: () => void;
}

const SortableAgentItem: React.FC<SortableAgentItemProps> = ({ id, agent, onRemove }) => {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className="sortable-agent-item"
    >
      <div className="drag-handle" {...attributes} {...listeners}>
        <HolderOutlined />
      </div>
      <div className="agent-info">
        <span className="agent-name">{agent?.name || id}</span>
        <span className="agent-role">{agent?.role}</span>
      </div>
      <Button
        type="text"
        danger
        size="small"
        icon={<DeleteOutlined />}
        onClick={onRemove}
      />
    </div>
  );
};
```

**CSS for SortableAgentItem:**

Add to `web/src/pages/Workflow/index.css` (create if doesn't exist):

```tsx
// Add at top of index.tsx
import './index.css';
```

```css
.sortable-agent-item {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  background: #fafafa;
  border: 1px solid #d9d9d9;
  border-radius: 6px;
  margin-bottom: 8px;
  cursor: grab;
}

.sortable-agent-item:active {
  cursor: grabbing;
}

.sortable-agent-item .drag-handle {
  padding: 4px 8px;
  color: #999;
  cursor: grab;
}

.sortable-agent-item .agent-info {
  flex: 1;
  margin-left: 8px;
}

.sortable-agent-item .agent-name {
  font-weight: 500;
}

.sortable-agent-item .agent-role {
  margin-left: 8px;
  color: #666;
  font-size: 12px;
}
```

### Visual Feedback

| Action | Feedback |
|--------|----------|
| Drag start | Item lifts with shadow, cursor changes to grab |
| Drag over | Target position shows insertion indicator |
| Drop | Item animates into new position |
| Remove agent | Item fades out, list reorders smoothly |

---

## Error Handling and Validation

### Validation Rules

| Field | Rule | Error Message |
|-------|------|---------------|
| name | Required, non-empty | "请输入工作流名称" |
| agentIds | At least one agent required | "请选择至少一个Agent实例" |
| checkpoints | Optional | None |

### Error Scenarios

| Scenario | User Feedback |
|----------|---------------|
| API update fails | Toast notification with error message from server |
| Network timeout | "网络错误，请稍后重试" toast |
| Validation error | Form field highlight with Ant Design's built-in validation |
| Drag-drop fails | Graceful fallback to manual reordering via up/down buttons |

### Optimistic Updates

```typescript
const handleEditWorkflow = async (values: any) => {
  if (!editingTemplate) return;

  // Validate at least one agent selected
  if (selectedAgentIds.length === 0) {
    message.error('请选择至少一个Agent实例');
    return;
  }

  // Prepare update data
  const updateData = {
    name: values.name,
    description: values.description,
    agentIds: selectedAgentIds,
    checkpoints: values.checkpoints || [],
  };

  // Optimistic update
  const previousTemplates = workflowTemplates;
  setWorkflowTemplates(prev =>
    prev.map(t => t.id === editingTemplate.id ? { ...t, ...updateData } : t)
  );

  try {
    await api.workflows.update(editingTemplate.id, updateData);
    message.success('工作流更新成功');
    // Refresh from server to ensure consistency
    fetchWorkflowTemplates();
  } catch (error) {
    // Revert on failure
    setWorkflowTemplates(previousTemplates);
    message.error(error?.response?.data?.error || '工作流更新失败');
  } finally {
    setCreateModalVisible(false);
    setEditMode(false);
    setEditingTemplate(null);
    setSelectedAgentIds([]);
    form.resetFields();
  }
};
```

---

## Testing Considerations

### Manual Testing Checklist

- [ ] Edit button appears on ALL workflow cards (including default and system)
- [ ] Clicking edit opens Modal with pre-filled data
- [ ] Modal title shows "编辑工作流" in edit mode
- [ ] Modal okText shows "保存" in edit mode
- [ ] "基于模板" field is hidden in edit mode
- [ ] Selected agents appear in sortable list
- [ ] Can add new agents via Select dropdown
- [ ] Can remove agents via delete button
- [ ] Drag-and-drop reorders agents correctly
- [ ] Agent order persists after save and page reload
- [ ] Can edit default workflows (isDefault=true)
- [ ] Can edit system workflows (isSystem=true)
- [ ] Validation errors display correctly (empty name, no agents)
- [ ] API errors show toast notification
- [ ] Cancel discards changes without saving
- [ ] Modal cancel resets edit state properly

### Edge Cases

1. **Edit default workflow:** Should save successfully (no restrictions)
2. **Edit system workflow:** Should save successfully (no restrictions)
3. **Remove all agents:** Validation error on submit - "请选择至少一个Agent实例"
4. **Empty name:** Validation error on submit
5. **Network error during save:** Revert optimistic update, show error toast
6. **Concurrent edits:** Last write wins (server-side behavior)
7. **Drag to same position:** No change to order
8. **Cancel then re-edit same workflow:** Form should be reset and re-populated correctly

---

## Dependencies

### New Dependency

**Installation Command:**
```bash
cd web && npm install @dnd-kit/core @dnd-kit/sortable @dnd-kit/utilities
```

**package.json additions:**
```json
"@dnd-kit/core": "^6.0.0",
"@dnd-kit/sortable": "^7.0.0",
"@dnd-kit/utilities": "^3.0.0"
```

### Utility Functions

The `arrayMove` function is needed for drag-and-drop reordering. Either use a library or implement inline:

```typescript
// Option 1: Install dnd-kit's utilities (recommended)
import { arrayMove } from '@dnd-kit/sortable';

// Option 2: Inline implementation
const arrayMove = <T,>(array: T[], from: number, to: number): T[] => {
  const newArray = array.slice();
  newArray.splice(to < 0 ? newArray.length + to : to, 0, newArray.splice(from, 1)[0]);
  return newArray;
};
```

### Existing APIs

```typescript
// Already exists in web/src/api/client.ts
api.workflows.update(id: string, data: Partial<WorkflowTemplate>): Promise<WorkflowTemplate>
```

No backend changes required.

---

## File Structure

| File | Responsibility |
|------|----------------|
| `web/src/pages/Workflow/index.tsx` | Add edit mode state, edit button, handleEditWorkflow, drag-and-drop |
| `web/src/pages/Workflow/index.css` | CSS for sortable agent items (create if doesn't exist) |
| `web/src/api/client.ts` | Already has update method - no changes needed |
| `web/package.json` | Add @dnd-kit dependencies |

---

## Summary

This design adds edit functionality to workflow templates by:
1. Adding edit button to workflow cards
2. Reusing create Modal for edit mode with pre-filled data
3. Implementing drag-and-drop agent reordering with @dnd-kit
4. Handling errors with optimistic updates and toast notifications

The implementation is purely frontend - the backend API already supports updates via `api.workflows.update()`.