# Agent 批量删除功能实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现前后端 Agent 批量删除功能，支持勾选多个自定义角色一键删除

**Architecture:** 后端新增 `POST /api/v1/agents/batch-delete` API，前端在表格添加勾选和批量删除按钮

**Tech Stack:** Go + Gin (后端), React + Ant Design (前端)

---

## Task 1: 后端 Handler 层添加 BatchDelete 方法

**Files:**
- Modify: `internal/api/agent_handler.go:165` (在 Delete 方法后添加)

**Step 1: 在 agent_handler.go 添加 BatchDelete 方法**

```go
// BatchDeleteRequest 批量删除请求
type BatchDeleteRequest struct {
	IDs []string `json:"ids" binding:"required"`
}

// ReferencedAgentInfo 被引用的Agent信息
type ReferencedAgentInfo struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	WorkflowNames []string `json:"workflowNames"`
}

// BatchDelete 批量删除配置
func (h *AgentHandler) BatchDelete(c *gin.Context) {
	var req BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids is required"})
		return
	}

	ctx := c.Request.Context()
	var referencedAgents []ReferencedAgentInfo
	var validIDs []uuid.UUID

	// 1. 解析并验证所有 ID，检查系统角色和引用状态
	for _, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue // 忽略无效 ID
		}

		// 获取配置检查是否为系统角色
		config, err := h.configSvc.GetByID(ctx, id)
		if err != nil {
			continue // 忽略不存在的 ID
		}

		if config.IsSystem {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":            "系统预置角色不可删除",
				"hasSystemAgent":   true,
				"systemAgentName":  config.Name,
			})
			return
		}

		// 检查工作流引用
		templates, err := h.workflowRepo.FindByAgentID(ctx, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check references"})
			return
		}

		if len(templates) > 0 {
			var names []string
			for _, t := range templates {
				names = append(names, t.Name)
			}
			referencedAgents = append(referencedAgents, ReferencedAgentInfo{
				ID:            idStr,
				Name:          config.Name,
				WorkflowNames: names,
			})
		} else {
			validIDs = append(validIDs, id)
		}
	}

	// 2. 任一有引用则拒绝整个操作
	if len(referencedAgents) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":            "部分Agent被工作流引用，无法删除",
			"referencedAgents": referencedAgents,
		})
		return
	}

	// 3. 执行批量删除
	for _, id := range validIDs {
		if err := h.configSvc.Delete(ctx, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// 清理绑定关系
		if h.agentSkillBindingRepo != nil {
			h.agentSkillBindingRepo.DeleteByAgentRoleID(ctx, id)
		}
		if h.agentSubagentBindingRepo != nil {
			h.agentSubagentBindingRepo.DeleteByAgentRoleID(ctx, id)
		}
		if h.agentCommandBindingRepo != nil {
			h.agentCommandBindingRepo.DeleteByAgentRoleID(ctx, id)
		}
		if h.agentRuleBindingRepo != nil {
			h.agentRuleBindingRepo.DeleteByAgentRoleID(ctx, id)
		}
		if h.agentSettingsBindingRepo != nil {
			h.agentSettingsBindingRepo.DeleteByAgentRoleID(ctx, id)
		}
	}

	c.Status(http.StatusNoContent)
}
```

**Step 2: 在 RegisterRoutes 方法添加路由**

在 `agents.POST("/:id/copy", h.Copy)` 前添加：

```go
			agents.POST("/batch-delete", h.BatchDelete)
```

**Step 3: 验证后端编译**

Run: `go build ./cmd/server`
Expected: 编译成功无错误

**Step 4: Commit**

```bash
git add internal/api/agent_handler.go
git commit -m "feat: add batch-delete API for agents"
```

---

## Task 2: 前端 API 添加 batchDelete 方法

**Files:**
- Modify: `web/src/api/client.ts:327` (在 agents 对象 submitQuestionAnswer 后添加)

**Step 1: 在 agents 对象添加 batchDelete 方法**

```typescript
    // 批量删除
    batchDelete: (ids: string[]): Promise<void> =>
      this.request('/agents/batch-delete', 'POST', { ids }),
```

**Step 2: 验证 TypeScript 编译**

Run: `cd web && npm run build`
Expected: 编译成功无错误

**Step 3: Commit**

```bash
git add web/src/api/client.ts
git commit -m "feat: add batchDelete API method"
```

---

## Task 3: 前端 AgentRoleList 添加勾选和批量删除 UI

**Files:**
- Modify: `web/src/pages/AgentRoleList.tsx`

**Step 1: 添加状态变量**

在 `const [systemPageSize, setSystemPageSize] = useState(5);` 后添加：

```typescript
  // 批量删除相关状态
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [batchDeleteLoading, setBatchDeleteLoading] = useState(false);
```

**Step 2: 添加批量删除处理函数**

在 `handleDelete` 函数后添加：

```typescript
  // 批量删除
  const handleBatchDelete = () => {
    if (selectedRowKeys.length === 0) return;

    // 获取选中项的名称列表
    const selectedNames = customAgents
      .filter(a => selectedRowKeys.includes(a.id))
      .map(a => a.name);

    Modal.confirm({
      title: '批量删除确认',
      icon: <ExclamationCircleOutlined />,
      content: (
        <div>
          <p>确定要删除以下 {selectedRowKeys.length} 个 Agent 角色吗？此操作不可恢复。</p>
          <ul style={{ marginTop: 8, paddingLeft: 20, maxHeight: 200, overflow: 'auto' }}>
            {selectedNames.map(name => <li key={name}>{name}</li>)}
          </ul>
        </div>
      ),
      okText: '确认删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        setBatchDeleteLoading(true);
        try {
          await api.agents.batchDelete(selectedRowKeys as string[]);
          message.success(`成功删除 ${selectedRowKeys.length} 个 Agent 角色`);
          setSelectedRowKeys([]);
          loadConfigs();
        } catch (error: any) {
          const errorData = error.response?.data;
          if (errorData?.referencedAgents) {
            Modal.error({
              title: '无法删除',
              content: (
                <div>
                  <p>以下 Agent 角色被团队引用，无法删除：</p>
                  <ul style={{ marginTop: 8, paddingLeft: 20 }}>
                    {errorData.referencedAgents.map((agent: any) => (
                      <li key={agent.id}>
                        <strong>{agent.name}</strong> - 被 {agent.workflowNames.join('、')} 引用
                      </li>
                    ))}
                  </ul>
                  <p style={{ marginTop: 8 }}>请先从团队中移除这些 Agent，再进行删除。</p>
                </div>
              ),
            });
          } else if (errorData?.hasSystemAgent) {
            Modal.error({
              title: '无法删除',
              content: <p>系统预置角色「{errorData.systemAgentName}」不可删除</p>,
            });
          } else {
            message.error('批量删除失败');
          }
        } finally {
          setBatchDeleteLoading(false);
        }
      },
    });
  };
```

**Step 3: 在自定义角色表格添加 rowSelection**

找到自定义角色表格的 `<Table` 标签，添加 rowSelection 配置：

```typescript
            rowSelection={{
              selectedRowKeys,
              onChange: setSelectedRowKeys,
              getCheckboxProps: (record: AgentConfig) => ({
                disabled: record.isSystem,
              }),
            }}
```

**Step 4: 在自定义角色 Card 的 extra 添加批量删除按钮**

修改 extra 属性：

```typescript
        extra={
          <Space>
            {selectedRowKeys.length > 0 && (
              <Button
                danger
                icon={<DeleteOutlined />}
                loading={batchDeleteLoading}
                onClick={handleBatchDelete}
              >
                批量删除 ({selectedRowKeys.length})
              </Button>
            )}
            <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
              新建角色
            </Button>
          </Space>
        }
```

**Step 5: 验证前端编译**

Run: `cd web && npm run build`
Expected: 编译成功无错误

**Step 6: Commit**

```bash
git add web/src/pages/AgentRoleList.tsx
git commit -m "feat: add batch delete UI for agents"
```

---

## Task 4: 端到端验证

**Step 1: 启动前后端服务**

Run: `go run ./cmd/server` (后端)
Run: `cd web && npm run dev` (前端)

**Step 2: 测试场景**

1. 打开 http://localhost:26306，进入 Agent 角色页面
2. 创建 2-3 个自定义 Agent 角色
3. 勾选多个角色，点击"批量删除"按钮
4. 确认删除后验证角色列表更新

**Step 3: 测试引用拦截**

1. 创建一个团队（工作流），添加自定义 Agent
2. 回到 Agent 角色页面，勾选被引用的 Agent
3. 点击批量删除，验证显示引用错误信息

**Step 4: Commit 最终验证**

```bash
git add docs/plans/2026-04-17-agent-batch-delete-design.md
git commit -m "docs: add agent batch delete design document"
```