# Dashboard Agent Cards Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two preview cards to Dashboard showing Agent团队 and Agent角色 statistics with navigation links.

**Architecture:** Extend existing Dashboard component with new state variables for workflows and agent configs, fetch data alongside existing API calls, render two new Card components between stats row and active threads section.

**Tech Stack:** React, Ant Design (Card, Row, Col, List, Typography), TypeScript, existing API client

---

## File Structure

| File | Action | Purpose |
|------|--------|---------|
| `web/src/pages/Dashboard/index.tsx` | Modify | Add imports, state, API calls, and two card components |

---

### Task 1: Add Imports and State Variables

**Files:**
- Modify: `web/src/pages/Dashboard/index.tsx:1-28`

- [ ] **Step 1: Add new icon imports**

Add `TeamOutlined` and `RobotOutlined` to the icon imports at line 3-10:

```tsx
import {
  ProjectOutlined,
  ThunderboltOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  PlusOutlined,
  ArrowRightOutlined,
  TeamOutlined,
  RobotOutlined,
} from '@ant-design/icons';
```

- [ ] **Step 2: Add new type imports**

Add `WorkflowTemplate` and `AgentConfig` to the type imports at line 13:

```tsx
import type { Project, Thread, WorkflowTemplate, AgentConfig } from '@/types';
```

- [ ] **Step 3: Add state variables for workflows and agents**

Add new state variables after line 28 (after `const [loading, setLoading] = useState(false);`):

```tsx
  const [workflows, setWorkflows] = useState<WorkflowTemplate[]>([]);
  const [agentConfigs, setAgentConfigs] = useState<AgentConfig[]>([]);
  const [loadingAgents, setLoadingAgents] = useState(false);
```

---

### Task 2: Extend Data Loading Function

**Files:**
- Modify: `web/src/pages/Dashboard/index.tsx:34-81`

- [ ] **Step 1: Add API calls to loadDashboardData**

Modify the `loadDashboardData` function to fetch workflows and agent configs in parallel with existing calls. Replace the entire function:

```tsx
  const loadDashboardData = async () => {
    setLoading(true);
    setLoadingAgents(true);
    try {
      // 并行加载所有数据
      const [projectData, workflowsData, agentsData] = await Promise.all([
        api.projects.list(),
        api.workflows.list(),
        api.agents.list(),
      ]);

      // 处理项目列表
      const projectsList = ((projectData as unknown as Project[]) || []);
      setProjects(projectsList.slice(0, 5));

      // 统计
      setStats({
        totalProjects: projectsList.length,
        activeThreads: 3, // 模拟数据
        completedThreads: 12,
        pendingReviews: 2,
      });

      // 设置工作流和Agent数据
      setWorkflows(workflowsData || []);
      setAgentConfigs(agentsData || []);

      // 加载活跃线程
      setActiveThreads([
        {
          id: '1',
          projectId: '1',
          name: '功能开发中',
          status: 'running',
          currentPhase: 'development',
          currentAgent: 'developer',
          depth: 2,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
        {
          id: '2',
          projectId: '2',
          name: '代码审查中',
          status: 'running',
          currentPhase: 'review',
          currentAgent: 'reviewer',
          depth: 1,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
      ]);
    } catch (error) {
      console.error('Failed to load dashboard data:', error);
    } finally {
      setLoading(false);
      setLoadingAgents(false);
    }
  };
```

---

### Task 3: Add Agent团队 Card Component

**Files:**
- Modify: `web/src/pages/Dashboard/index.tsx` (insert after stats Row, before 活跃任务 Title)

- [ ] **Step 1: Add Agent团队 card JSX**

Insert the following JSX after the stats `Row` (after line 172 `</Row>`) and before the 活跃任务 Title:

```tsx
      {/* Agent团队和角色卡片 */}
      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} lg={12}>
          <Card
            hoverable
            onClick={() => navigate('/workflow')}
            style={{ cursor: 'pointer' }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
              <Space>
                <TeamOutlined style={{ fontSize: 18 }} />
                <Text strong>Agent团队</Text>
                <Tag color="blue">{workflows.length}</Tag>
              </Space>
              <ArrowRightOutlined />
            </div>
            {workflows.length > 0 ? (
              <>
                <div style={{ marginBottom: 8 }}>
                  {workflows.slice(0, 3).map((workflow) => (
                    <div key={workflow.id} style={{ padding: '4px 0', color: 'var(--text-secondary)' }}>
                      · {workflow.name} ({workflow.agentIds?.length || 0} Agents)
                    </div>
                  ))}
                </div>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  查看全部 →
                </Text>
              </>
            ) : (
              <Text type="secondary">暂无团队，点击创建</Text>
            )}
          </Card>
        </Col>
```

---

### Task 4: Add Agent角色 Card Component

**Files:**
- Modify: `web/src/pages/Dashboard/index.tsx` (continue from Task 3)

- [ ] **Step 1: Add Agent角色 card JSX**

Add the Agent角色 card in the same Row, right after the Agent团队 card Col:

```tsx
        <Col xs={24} lg={12}>
          <Card
            hoverable
            onClick={() => navigate('/agents')}
            style={{ cursor: 'pointer' }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
              <Space>
                <RobotOutlined style={{ fontSize: 18 }} />
                <Text strong>Agent角色</Text>
                <Tag color="purple">{agentConfigs.length}</Tag>
              </Space>
              <ArrowRightOutlined />
            </div>
            {agentConfigs.length > 0 ? (
              <>
                <div style={{ marginBottom: 8 }}>
                  <div style={{ padding: '4px 0', color: 'var(--text-secondary)' }}>
                    系统预置: {agentConfigs.filter(a => a.isSystem).length}
                  </div>
                  <div style={{ padding: '4px 0', color: 'var(--text-secondary)' }}>
                    自定义: {agentConfigs.filter(a => !a.isSystem).length}
                  </div>
                </div>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  查看全部 →
                </Text>
              </>
            ) : (
              <Text type="secondary">暂无角色，点击创建</Text>
            )}
          </Card>
        </Col>
      </Row>
```

---

### Task 5: Manual Testing

**Files:**
- None (browser testing)

- [ ] **Step 1: Start development server**

Run the frontend dev server:
```bash
cd web && npm run dev
```

- [ ] **Step 2: Open Dashboard in browser**

Navigate to `http://localhost:26306` and verify:
1. Two new cards appear below the stats row
2. Agent团队 card shows team count and team list (or "暂无团队" if empty)
3. Agent角色 card shows counts (or "暂无角色" if empty)
4. Cards are clickable and navigate to correct pages
5. Layout is responsive (cards stack vertically on small screens)

- [ ] **Step 3: Test dark mode**

Toggle dark mode and verify:
1. Card backgrounds use CSS variables (no hardcoded colors)
2. Text colors adapt to theme
3. Tags and icons render correctly

- [ ] **Step 4: Commit changes**

```bash
git add web/src/pages/Dashboard/index.tsx docs/superpowers/specs/2026-04-16-dashboard-agent-cards-design.md
git commit -m "feat(dashboard): add Agent团队 and Agent角色 preview cards

- Add two new cards between stats and active threads
- Agent团队 card: team count + recent 3 teams list
- Agent角色 card: total + system preset + custom counts
- Both cards provide navigation to respective management pages
- Support dark mode via CSS variables
- Responsive layout (xs=24, lg=12)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Self-Review Checklist

**1. Spec Coverage:**
- ✅ Task 1-4: Add imports, state, API calls → covered
- ✅ Task 3: Agent团队 card layout → covered
- ✅ Task 4: Agent角色 card layout → covered  
- ✅ Task 5: Manual testing → covered
- ✅ Empty state handling → covered in card JSX
- ✅ Dark mode CSS variables → using var(--text-secondary)

**2. Placeholder Scan:**
- No TBD, TODO, or "implement later"
- All code blocks contain actual implementation code
- No vague instructions like "add appropriate styling"

**3. Type Consistency:**
- WorkflowTemplate imported from @/types
- AgentConfig imported from @/types
- Using workflow.agentIds (matches type definition)
- Using agent.isSystem (matches type definition)