# Team Graph Editor Test Plan

## Test Scope
- React Flow graph rendering
- Preview/Edit mode switching
- Node operations (add/remove/view)
- Edge operations (create/delete/edit trigger hint)
- API integration
- Dark mode compatibility
- Error handling

## Test Cases

### TC-01: Page Loading
| ID | Description | Steps | Expected Result |
|----|-------------|-------|-----------------|
| 01-01 | Load team with agents | Navigate to graph editor with valid teamId | Nodes and edges render correctly |
| 01-02 | Load empty team | Navigate to graph editor with team having no agents | Empty state with guidance shown |
| 01-03 | Load with invalid teamId | Navigate with non-existent teamId | Error alert displayed |

### TC-02: Preview Mode
| ID | Description | Steps | Expected Result |
|----|-------------|-------|-----------------|
| 02-01 | Click node | Click on Agent node | Side panel shows agent details (readonly) |
| 02-02 | Click edge | Click on connection line | Side panel shows edge details (readonly) |
| 02-03 | Try drag node | Attempt to drag node | Node doesn't move |
| 02-04 | Try create connection | Drag from handle | Connection not created |

### TC-03: Edit Mode
| ID | Description | Steps | Expected Result |
|----|-------------|-------|-----------------|
| 03-01 | Switch to edit | Click "编辑" button | Mode switches, controls appear |
| 03-02 | Add agent | Click "添加 Agent" dropdown, select agent | New node appears on canvas |
| 03-03 | Create connection | Drag from source handle to target handle | New edge created |
| 03-04 | Duplicate connection | Try to create same connection twice | Error shown: "该 Agent 之间已存在连线" |
| 03-05 | Edit trigger hint | Click edge, modify trigger hint text, save | Trigger hint updated |
| 03-06 | Delete node | Click node, click "从团队移除" | Node and connected edges removed |
| 03-07 | Delete edge | Click edge, click "删除连线" | Edge removed |

### TC-04: Save Operations
| ID | Description | Steps | Expected Result |
|----|-------------|-------|-----------------|
| 04-01 | Save changes | Make changes, click "保存" | API called, success message shown |
| 04-02 | Save without changes | No changes, no save button | Save button not shown |
| 04-03 | Save on error | Trigger API error | Error alert shown, hasChanges remains true |

### TC-05: Dark Mode
| ID | Description | Steps | Expected Result |
|----|-------------|-------|-----------------|
| 05-01 | Toggle dark mode | Switch theme to dark | All elements use CSS variables correctly |
| 05-02 | Check canvas | View background, nodes, edges | No hardcoded colors visible |

## Automated Test Scripts (Optional)

### Unit Tests for useGraphStore
```typescript
// Tests for store actions
describe('useGraphStore', () => {
  it('should prevent duplicate edges', () => {
    // addEdge with same source/target twice
    // expect error state to be set
  });

  it('should clear connected edges when removing node', () => {
    // removeNode with connected edges
    // expect edges to be filtered
  });

  it('should track hasChanges correctly', () => {
    // addNode, addEdge, updateEdgeTriggerHint
    // expect hasChanges = true
  });
});
```

## Test Execution Checklist

- [ ] TC-01: Page Loading (3 cases)
- [ ] TC-02: Preview Mode (4 cases)
- [ ] TC-03: Edit Mode (7 cases)
- [ ] TC-04: Save Operations (3 cases)
- [ ] TC-05: Dark Mode (2 cases)

## Test Environment
- Browser: Chrome/Firefox latest
- Node version: per project requirements
- API: mock or real backend