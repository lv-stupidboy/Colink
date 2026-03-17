# Workflow-Bound Agent Mentions and Left-Right Message Layout

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Filter @-mentionable agents based on project's workflow template binding, and display user messages on the left with agent messages on the right.

**Architecture:** Extend Zustand store to manage project workflow context. ThreadView loads project data on mount, filters agents by workflow's agentIds for @-mention dropdown. CSS uses flexbox to align user messages left and agent messages right.

**Tech Stack:** React, TypeScript, Zustand, Ant Design, CSS Flexbox

---

## File Structure

| File | Responsibility |
|------|----------------|
| `web/src/store/index.ts` | Add project/workflow state and loading methods |
| `web/src/pages/ThreadView.tsx` | Use workflow-filtered agents for @mention, pass sender info to message render |
| `web/src/pages/ThreadView.css` | Left-right message alignment styles |

---

## Chunk 1: Extend Zustand Store for Workflow Context

### Task 1: Add Project and Workflow State to Store

**Files:**
- Modify: `web/src/store/index.ts`

- [ ] **Step 1: Add new state fields to Store interface**

Find the `Store` interface (around line 10-30) and add the following fields:

```typescript
interface Store {
  // ... existing fields ...

  // Add these new fields
  currentProject: Project | null;
  currentWorkflowTemplate: WorkflowTemplate | null;
  loadingProjectContext: boolean;

  // ... existing methods ...

  // Add these new methods
  loadProjectContext: (projectId: string) => Promise<void>;
  clearProjectContext: () => void;
  getFilteredAgents: () => AgentConfig[];
}
```

- [ ] **Step 2: Add imports for new types**

At the top of the file, ensure these types are imported:

```typescript
import type { Project, WorkflowTemplate, AgentConfig } from '@/types';
```

- [ ] **Step 3: Initialize new state in create() call**

Find the `create<Store>((set, get) => ({` section and add:

```typescript
  // Add after existing state initializations
  currentProject: null,
  currentWorkflowTemplate: null,
  loadingProjectContext: false,
```

- [ ] **Step 4: Implement loadProjectContext method**

Add the method implementation:

```typescript
  loadProjectContext: async (projectId: string) => {
    set({ loadingProjectContext: true });
    try {
      // Load project to get workflowTemplateId
      const project = await api.projects.get(projectId);

      // Load workflow template if project has one bound
      let workflowTemplate: WorkflowTemplate | null = null;
      if (project.workflowTemplateId) {
        workflowTemplate = await api.workflows.get(project.workflowTemplateId);
      }

      set({
        currentProject: project as unknown as Project,
        currentWorkflowTemplate: workflowTemplate,
        loadingProjectContext: false,
      });
    } catch (error) {
      console.error('Failed to load project context:', error);
      set({
        loadingProjectContext: false,
        currentProject: null,
        currentWorkflowTemplate: null,
      });
    }
  },
```

- [ ] **Step 5: Implement clearProjectContext method**

```typescript
  clearProjectContext: () => {
    set({
      currentProject: null,
      currentWorkflowTemplate: null,
    });
  },
```

- [ ] **Step 6: Implement getFilteredAgents method**

Add this method that filters agents based on the current workflow template:

```typescript
  getFilteredAgents: () => {
    const { currentWorkflowTemplate, agentConfigs } = get();

    // If no workflow template or no agentIds, return all agents
    if (!currentWorkflowTemplate || !currentWorkflowTemplate.agentIds?.length) {
      return agentConfigs;
    }

    // Filter agents that are in the workflow's agentIds
    return agentConfigs.filter(agent =>
      currentWorkflowTemplate.agentIds.includes(agent.id)
    );
  },
```

- [ ] **Step 7: Verify the store compiles**

Run: `cd web && npm run build`
Expected: No TypeScript errors

- [ ] **Step 8: Commit store changes**

```bash
git add web/src/store/index.ts
git commit -m "feat(store): add project context for workflow-bound agent filtering"
```

---

## Chunk 2: Update ThreadView for Workflow-Filtered @Mentions

### Task 2: Load Project Context in ThreadView

**Files:**
- Modify: `web/src/pages/ThreadView.tsx`

- [ ] **Step 1: Import useAppStore at the top if not already present**

Check the imports section (around line 1-30) and ensure:

```typescript
import { useAppStore } from '@/store';
```

- [ ] **Step 2: Get store methods and state in component**

Find the `ThreadView` component function (around line 70-100) and add store access:

```typescript
const ThreadView: React.FC = () => {
  // ... existing state declarations ...

  // Add store access
  const {
    currentProject,
    currentWorkflowTemplate,
    loadingProjectContext,
    loadProjectContext,
    clearProjectContext,
    getFilteredAgents,
    agentConfigs,
  } = useAppStore();
```

- [ ] **Step 3: Load project context when thread loads**

Find the `useEffect` that loads thread data (around line 180-220) and add project context loading:

```typescript
  useEffect(() => {
    if (threadId) {
      loadThreadData();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [threadId]);

  // Add new useEffect for project context
  useEffect(() => {
    if (thread?.projectId) {
      loadProjectContext(thread.projectId);
    }
    return () => {
      clearProjectContext();
    };
  }, [thread?.projectId, loadProjectContext, clearProjectContext]);
```

### Task 3: Replace Hardcoded AgentRoleLabels with Filtered Agents

- [ ] **Step 4: Find the @mention dropdown implementation**

Locate the mention dropdown code (around line 650-700). The current code uses `Object.entries(AgentRoleLabels)`.

- [ ] **Step 5: Create filtered agents list for @mention**

Before the return statement, compute the filtered agents:

```typescript
  // Get agents available for @mention from workflow template
  const mentionableAgents = getFilteredAgents();

  // Create a map of agent id -> display info for @mention
  const agentOptions = mentionableAgents.map(agent => ({
    id: agent.id,
    role: agent.role,
    name: agent.name,
    label: `${agent.name} (${AgentRoleLabels[agent.role] || agent.role})`,
  }));
```

- [ ] **Step 6: Update @mention dropdown to use filtered agents**

Replace the existing `mentionListVisible` dropdown block. Find and replace:

```tsx
{mentionListVisible && (
  <Card className="mention-dropdown" size="small" style={{ position: 'absolute', bottom: '100%', left: 0, marginBottom: 8, zIndex: 1000, maxHeight: 200, overflow: 'auto' }}>
    <List
      size="small"
      dataSource={agentOptions.filter(opt =>
        !mentionFilter ||
        opt.label.toLowerCase().includes(mentionFilter.toLowerCase()) ||
        opt.role.toLowerCase().includes(mentionFilter.toLowerCase())
      )}
      renderItem={(opt) => (
        <List.Item
          style={{ cursor: 'pointer' }}
          onClick={() => selectMention(opt.role, opt.name)}
        >
          <Space>
            <Avatar size="small" icon={<RobotOutlined />} />
            <span>{opt.label}</span>
          </Space>
        </List.Item>
      )}
    />
  </Card>
)}
```

- [ ] **Step 7: Add loading indicator for @mention when context is loading**

Add a conditional check before the dropdown:

```tsx
{mentionListVisible && (
  <Card className="mention-dropdown" size="small" style={{ position: 'absolute', bottom: '100%', left: 0, marginBottom: 8, zIndex: 1000, maxHeight: 200, overflow: 'auto' }}>
    {loadingProjectContext ? (
      <div style={{ padding: 16, textAlign: 'center' }}>
        <Spin size="small" />
        <span style={{ marginLeft: 8 }}>Loading agents...</span>
      </div>
    ) : agentOptions.length === 0 ? (
      <div style={{ padding: 16, textAlign: 'center', color: '#999' }}>
        No agents available in current workflow
      </div>
    ) : (
      <List
        size="small"
        dataSource={agentOptions.filter(opt =>
          !mentionFilter ||
          opt.label.toLowerCase().includes(mentionFilter.toLowerCase()) ||
          opt.role.toLowerCase().includes(mentionFilter.toLowerCase())
        )}
        renderItem={(opt) => (
          <List.Item
            style={{ cursor: 'pointer' }}
            onClick={() => selectMention(opt.role, opt.name)}
          >
            <Space>
              <Avatar size="small" icon={<RobotOutlined />} />
              <span>{opt.label}</span>
            </Space>
          </List.Item>
        )}
      />
    )}
  </Card>
)}
```

- [ ] **Step 8: Add Spin import from antd if not present**

Check imports at top of file:

```typescript
import { Spin } from 'antd';  // Add Spin to existing antd imports
```

- [ ] **Step 9: Commit ThreadView changes**

```bash
git add web/src/pages/ThreadView.tsx
git commit -m "feat(ThreadView): filter @mention agents by workflow template"
```

---

## Chunk 3: Implement Left-Right Message Layout

### Task 4: Update Message Rendering to Include Sender Type

**Files:**
- Modify: `web/src/pages/ThreadView.tsx`

- [ ] **Step 1: Find the message rendering function**

Locate the message rendering section (around line 400-500). Look for the `messages.map()` or similar message iteration.

- [ ] **Step 2: Identify message sender type**

The message object should have a `role` field. Check if it's 'user' or 'assistant'/'agent':

```typescript
// In the message rendering, determine the sender
const isUserMessage = message.role === 'user';
const messageClassName = isUserMessage ? 'message-user' : 'message-agent';
const containerClassName = isUserMessage ? 'message-container-user' : 'message-container-agent';
```

- [ ] **Step 3: Update message container structure**

Find the message container div and update it:

```tsx
<div className={`message-container ${containerClassName}`}>
  <div className={`message ${messageClassName}`}>
    {/* Message content */}
  </div>
</div>
```

- [ ] **Step 4: Update renderMessageItem function**

Find the `renderMessageItem` function (or equivalent message rendering) and update:

```tsx
const renderMessageItem = (message: Message) => {
  const isUserMessage = message.role === 'user';

  return (
    <div
      key={message.id}
      className={`message-container ${isUserMessage ? 'message-container-user' : 'message-container-agent'}`}
    >
      {!isUserMessage && (
        <Avatar
          className="message-avatar"
          icon={<RobotOutlined />}
          style={{ backgroundColor: '#1890ff' }}
        />
      )}
      <div className={`message ${isUserMessage ? 'message-user' : 'message-agent'}`}>
        {/* Message content here */}
        <div className="message-content">
          {message.content}
        </div>
      </div>
      {isUserMessage && (
        <Avatar
          className="message-avatar"
          icon={<UserOutlined />}
          style={{ backgroundColor: '#52c41a' }}
        />
      )}
    </div>
  );
};
```

- [ ] **Step 5: Ensure UserOutlined icon is imported**

Add to imports:

```typescript
import { UserOutlined, RobotOutlined } from '@ant-design/icons';
```

- [ ] **Step 6: Commit ThreadView rendering changes**

```bash
git add web/src/pages/ThreadView.tsx
git commit -m "feat(ThreadView): add sender-aware message container classes"
```

### Task 5: Add CSS for Left-Right Message Layout

**Files:**
- Modify: `web/src/pages/ThreadView.css`

- [ ] **Step 1: Add message container styles for alignment**

Add to the CSS file:

```css
/* Message container alignment */
.message-container {
  display: flex;
  align-items: flex-start;
  margin-bottom: 16px;
  width: 100%;
}

.message-container-user {
  flex-direction: row;
  justify-content: flex-start;
}

.message-container-agent {
  flex-direction: row-reverse;
  justify-content: flex-start;
}

/* User message - aligned left */
.message-container-user .message {
  background: linear-gradient(135deg, #f6ffed 0%, #f9f0ff 100%);
  border-left: 4px solid #52c41a;
  border-radius: 8px 8px 8px 0;
}

/* Agent message - aligned right */
.message-container-agent .message {
  background: linear-gradient(135deg, #e6f7ff 0%, #f0f5ff 100%);
  border-right: 4px solid #1890ff;
  border-radius: 8px 8px 0 8px;
}

/* Avatar positioning */
.message-avatar {
  flex-shrink: 0;
  margin: 0 12px;
}

/* Message content box */
.message {
  max-width: 70%;
  padding: 12px 16px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.message-content {
  word-break: break-word;
  line-height: 1.6;
}
```

- [ ] **Step 2: Remove or update conflicting existing styles**

Check for existing `.message-user` and `.message-agent` styles that may conflict. Update them to work with the new container structure.

- [ ] **Step 3: Add responsive styles for mobile**

```css
/* Responsive adjustments */
@media (max-width: 768px) {
  .message {
    max-width: 85%;
  }
}
```

- [ ] **Step 4: Commit CSS changes**

```bash
git add web/src/pages/ThreadView.css
git commit -m "feat(ThreadView.css): add left-right message layout styles"
```

---

## Chunk 4: Testing and Verification

### Task 6: Manual Testing

- [ ] **Step 1: Start the development server**

Run: `cd web && npm run dev`
Expected: Development server starts without errors

- [ ] **Step 2: Test @mention agent filtering**

1. Create a project and bind it to a workflow template with specific agents
2. Create a thread in that project
3. Type `@` in the message input
4. Verify only agents from the workflow template appear
5. Test with a project that has no workflow bound - should show all agents

- [ ] **Step 3: Test left-right message layout**

1. Send a user message - verify it appears on the LEFT with green accent
2. Receive an agent response - verify it appears on the RIGHT with blue accent
3. Test with long messages - verify they don't overflow
4. Test on mobile viewport - verify responsive behavior

- [ ] **Step 4: Test edge cases**

1. Project with workflow that has empty agentIds - should show all agents
2. Project with workflow that has non-existent agentIds - should handle gracefully
3. Rapid message sending - layout should remain stable

- [ ] **Step 5: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: address any issues found in testing"
```

---

## Summary

**Changes Made:**
1. Extended Zustand store with project/workflow context management
2. Added `loadProjectContext`, `clearProjectContext`, and `getFilteredAgents` methods
3. Updated ThreadView to load project context and filter @mention agents
4. Restructured message rendering with sender-aware container classes
5. Added CSS for left-right message alignment

**Files Modified:**
- `web/src/store/index.ts` - State management for workflow context
- `web/src/pages/ThreadView.tsx` - @mention filtering and message layout
- `web/src/pages/ThreadView.css` - Left-right alignment styles