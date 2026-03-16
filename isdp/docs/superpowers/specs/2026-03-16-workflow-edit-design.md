# Workflow Template Edit Functionality Design

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable editing of existing workflow templates with drag-and-drop agent reordering.

**Architecture:** Reuse existing create Modal for edit mode. Add edit button to each workflow card, toggle Modal into edit mode, pre-fill form with existing data. Use @dnd-kit/sortable for drag-and-drop agent ordering within the Modal.

**Tech Stack:** React, TypeScript, Ant Design, @dnd-kit/sortable

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

**New State Variables:**
```typescript
const [editMode, setEditMode] = useState(false);
const [editingTemplate, setEditingTemplate] = useState<WorkflowTemplate | null>(null);
```

**State Flow:**
```
User clicks "Edit" button
  → setEditMode(true)
  → setEditingTemplate(selectedTemplate)
  → form.setFieldsValue({ name, description, agentIds, checkpoints })
  → setCreateModalVisible(true)
  → Modal opens in edit mode
```

### Modal Reuse Pattern

| Mode | Modal Title | Submit Button | Submit Handler |
|------|-------------|---------------|----------------|
| Create | "自定义工作流" | "创建" | `handleCreateWorkflow` |
| Edit | "编辑工作流" | "保存" | `handleEditWorkflow` |

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
{!template.isDefault && (
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
)}
```

### Modal Title Toggle

```tsx
<Modal
  title={editMode ? "编辑工作流" : "自定义工作流"}
  // ...
>
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
        agentId={agentId}
        agent={agents.find(a => a.id === agentId)}
        onRemove={() => handleRemoveAgent(agentId)}
      />
    ))}
  </SortableContext>
</DndContext>
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
  // Optimistic update
  const previousTemplates = workflowTemplates;
  setWorkflowTemplates(prev =>
    prev.map(t => t.id === editingTemplate.id ? { ...t, ...values } : t)
  );

  try {
    await api.workflows.update(editingTemplate.id, values);
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
    form.resetFields();
  }
};
```

---

## Testing Considerations

### Manual Testing Checklist

- [ ] Edit button appears on workflow cards
- [ ] Clicking edit opens Modal with pre-filled data
- [ ] Modal title shows "编辑工作流" in edit mode
- [ ] Can modify name, description, agents, checkpoints
- [ ] Drag-and-drop reorders agents correctly
- [ ] Agent order persists after save and page reload
- [ ] Can edit system workflows (`isSystem=true`)
- [ ] Validation errors display correctly
- [ ] API errors show toast notification
- [ ] Cancel discards changes without saving

### Edge Cases

1. **Edit system workflow:** Should save successfully (no restrictions)
2. **Remove all agents:** Validation error on submit
3. **Empty name:** Validation error on submit
4. **Network error during save:** Revert optimistic update, show error toast
5. **Concurrent edits:** Last write wins (server-side behavior)

---

## Dependencies

### New Dependency

```json
"@dnd-kit/core": "^6.0.0",
"@dnd-kit/sortable": "^7.0.0"
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