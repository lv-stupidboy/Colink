# Agent Batch Operations Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add batch operations for Agent roles - batch generate config files and batch update base agent.

**Architecture:** Backend adds two new APIs with parallel processing, frontend adds batch operation buttons with result modal.

**Tech Stack:** Go + Gin (backend), React + Ant Design + TypeScript (frontend)

---

## Task 1: Backend Model Definitions

**Files:**
- Modify: `internal/model/agent_config.go` (add new structs)

**Step 1: Add request/response structs**

Add to `internal/model/agent_config.go` after existing structs:

```go
// BatchGenerateConfigRequest 批量生成配置请求
type BatchGenerateConfigRequest struct {
	AgentIds []string `json:"agentIds" binding:"required"`
	CliType  string   `json:"cliType"`
}

// BatchGenerateResult 批量生成配置结果
type BatchGenerateResult struct {
	Total   int                  `json:"total"`
	Success int                  `json:"success"`
	Failed  int                  `json:"failed"`
	Results []GenerateResultItem `json:"results"`
}

// GenerateResultItem 单个Agent生成结果
type GenerateResultItem struct {
	AgentId        uuid.UUID `json:"agentId"`
	AgentName      string    `json:"agentName"`
	Status         string    `json:"status"`
	SkillsCount    int       `json:"skillsCount"`
	CommandsCount  int       `json:"commandsCount"`
	SubagentsCount int       `json:"subagentsCount"`
	RulesCount     int       `json:"rulesCount"`
	SettingsCount  int       `json:"settingsCount"`
	Error          string    `json:"error,omitempty"`
}

// BatchUpdateBaseAgentRequest 批量修改基础Agent请求
type BatchUpdateBaseAgentRequest struct {
	AgentIds    []string `json:"agentIds" binding:"required"`
	BaseAgentId string   `json:"baseAgentId" binding:"required"`
}

// BatchUpdateResult 批量修改基础Agent结果
type BatchUpdateResult struct {
	Total   int               `json:"total"`
	Success int               `json:"success"`
	Failed  int               `json:"failed"`
	Results []UpdateResultItem `json:"results"`
}

// UpdateResultItem 单个Agent修改结果
type UpdateResultItem struct {
	AgentId       uuid.UUID `json:"agentId"`
	AgentName     string    `json:"agentName"`
	BaseAgentName string    `json:"baseAgentName"`
	Status        string    `json:"status"`
	Error         string    `json:"error,omitempty"`
}
```

**Step 2: Commit**

```bash
git add internal/model/agent_config.go
git commit -m "feat(model): add batch operation request/response structs"
```

---

## Task 2: Backend Service - BatchGenerateConfig

**Files:**
- Modify: `internal/service/agent/config_service.go`

**Step 1: Add BatchGenerateConfig method**

Add to `internal/service/agent/config_service.go`:

```go
// BatchGenerateConfig 批量生成配置
func (s *ConfigService) BatchGenerateConfig(ctx context.Context,
	agentIds []uuid.UUID, cliType string) (*model.BatchGenerateResult, error) {

	result := &model.BatchGenerateResult{
		Total:   len(agentIds),
		Results: make([]model.GenerateResultItem, len(agentIds)),
	}

	if cliType == "" {
		cliType = "claude_code"
	}

	// 使用worker pool限制并发数
	const maxWorkers = 5
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, id := range agentIds {
		wg.Add(1)
		sem <- struct{}{} // 获取信号量

		go func(idx int, agentId uuid.UUID) {
			defer wg.Done()
			defer func() { <-sem }() // 释放信号量

			item := &model.GenerateResultItem{AgentId: agentId}

			config, err := s.GetByID(ctx, agentId)
			if err != nil {
				item.Status = "failed"
				item.Error = "Agent不存在"
			} else {
				item.AgentName = config.Name
				// 调用现有的生成配置逻辑
				genResult, err := s.generateSingleConfig(ctx, config, cliType)
				if err != nil {
					item.Status = "failed"
					item.Error = err.Error()
				} else {
					item.Status = "success"
					item.SkillsCount = genResult.SkillsCount
					item.CommandsCount = genResult.CommandsCount
					item.SubagentsCount = genResult.SubagentsCount
					item.RulesCount = genResult.RulesCount
					item.SettingsCount = genResult.SettingsCount
				}
			}

			mu.Lock()
			result.Results[idx] = *item
			if item.Status == "success" {
				result.Success++
			} else {
				result.Failed++
			}
			mu.Unlock()
		}(i, id)
	}

	wg.Wait()
	return result, nil
}

// generateSingleConfig 为单个Agent生成配置（内部方法）
func (s *ConfigService) generateSingleConfig(ctx context.Context,
	config *model.AgentRoleConfig, cliType string) (*model.ConfigGenerateResult, error) {

	// 查找现有的生成配置服务
	genService := s.configGenService
	if genService == nil {
		return nil, errors.New("配置生成服务未初始化")
	}

	return genService.GenerateConfig(ctx, config.ID, cliType)
}
```

**Step 2: Add import for sync**

Ensure imports include:
```go
import (
	"sync"
	...
)
```

**Step 3: Commit**

```bash
git add internal/service/agent/config_service.go
git commit -m "feat(service): add BatchGenerateConfig method"
```

---

## Task 3: Backend Service - BatchUpdateBaseAgent

**Files:**
- Modify: `internal/service/agent/config_service.go`

**Step 1: Add BatchUpdateBaseAgent method**

Add after BatchGenerateConfig:

```go
// BatchUpdateBaseAgent 批量修改基础Agent
func (s *ConfigService) BatchUpdateBaseAgent(ctx context.Context,
	agentIds []uuid.UUID, baseAgentId uuid.UUID) (*model.BatchUpdateResult, error) {

	// 验证目标BaseAgent存在
	baseAgent, err := s.baseAgentRepo.FindByID(ctx, baseAgentId)
	if err != nil {
		return nil, fmt.Errorf("目标基础Agent不存在")
	}

	result := &model.BatchUpdateResult{
		Total:   len(agentIds),
		Results: make([]model.UpdateResultItem, len(agentIds)),
	}

	for i, id := range agentIds {
		item := &model.UpdateResultItem{AgentId: id}

		config, err := s.GetByID(ctx, id)
		if err != nil {
			item.Status = "failed"
			item.Error = "Agent不存在"
		} else {
			item.AgentName = config.Name
			config.BaseAgentID = baseAgentId
			config.UpdatedAt = time.Now()

			if err := s.repo.Update(ctx, config); err != nil {
				item.Status = "failed"
				item.Error = err.Error()
			} else {
				item.Status = "success"
				item.BaseAgentName = baseAgent.Name

				// 更新缓存
				s.cacheMu.Lock()
				s.cache[id] = config
				s.cacheMu.Unlock()
			}
		}

		result.Results[i] = *item
		if item.Status == "success" {
			result.Success++
		} else {
			result.Failed++
		}
	}

	return result, nil
}
```

**Step 2: Add baseAgentRepo dependency**

Modify ConfigService struct to include baseAgentRepo:

```go
type ConfigService struct {
	repo            *repo.AgentConfigRepository
	baseAgentRepo   *repo.BaseAgentRepository
	configGenService *configgen.Service
	cache           map[uuid.UUID]*model.AgentRoleConfig
	cacheMu         sync.RWMutex
}

func NewConfigService(repo *repo.AgentConfigRepository, baseAgentRepo *repo.BaseAgentRepository, configGenService *configgen.Service) *ConfigService {
	return &ConfigService{
		repo:            repo,
		baseAgentRepo:   baseAgentRepo,
		configGenService: configGenService,
		cache:           make(map[uuid.UUID]*model.AgentRoleConfig),
	}
}
```

**Step 3: Commit**

```bash
git add internal/service/agent/config_service.go
git commit -m "feat(service): add BatchUpdateBaseAgent method"
```

---

## Task 4: Backend Handler - Batch APIs

**Files:**
- Modify: `internal/api/agent_handler.go`

**Step 1: Add handler methods**

Add to `internal/api/agent_handler.go`:

```go
// BatchGenerateConfig 批量生成配置
func (h *AgentHandler) BatchGenerateConfig(c *gin.Context) {
	var req model.BatchGenerateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	// 解析UUID
	agentIds := make([]uuid.UUID, len(req.AgentIds))
	for i, idStr := range req.AgentIds {
		id, err := uuid.Parse(idStr)
		if err != nil {
			BadRequest(c, fmt.Sprintf("无效的Agent ID: %s", idStr))
			return
		}
		agentIds[i] = id
	}

	cliType := req.CliType
	if cliType == "" {
		cliType = "claude_code"
	}

	result, err := h.configService.BatchGenerateConfig(c.Request.Context(), agentIds, cliType)
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, result)
}

// BatchUpdateBaseAgent 批量修改基础Agent
func (h *AgentHandler) BatchUpdateBaseAgent(c *gin.Context) {
	var req model.BatchUpdateBaseAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	// 解析UUID
	agentIds := make([]uuid.UUID, len(req.AgentIds))
	for i, idStr := range req.AgentIds {
		id, err := uuid.Parse(idStr)
		if err != nil {
			BadRequest(c, fmt.Sprintf("无效的Agent ID: %s", idStr))
			return
		}
		agentIds[i] = id
	}

	baseAgentId, err := uuid.Parse(req.BaseAgentId)
	if err != nil {
		BadRequest(c, "无效的基础Agent ID")
		return
	}

	result, err := h.configService.BatchUpdateBaseAgent(c.Request.Context(), agentIds, baseAgentId)
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, result)
}
```

**Step 2: Commit**

```bash
git add internal/api/agent_handler.go
git commit -m "feat(api): add batch generate config and batch update base agent handlers"
```

---

## Task 5: Backend Route Registration

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Add route registration**

Find the agent routes section and add:

```go
// Agent批量操作
agentGroup.POST("/batch-generate-config", agentHandler.BatchGenerateConfig)
agentGroup.POST("/batch-update-base-agent", agentHandler.BatchUpdateBaseAgent)
```

**Step 2: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(routes): register batch operation routes"
```

---

## Task 6: Backend Build Verification

**Step 1: Build and verify**

```bash
cd D:/CoLinkProject/Colink-0412/isdp && go build ./cmd/server
```

Expected: Build succeeds without errors

**Step 2: If build fails, fix issues**

Check for missing imports, undefined references, etc.

---

## Task 7: Frontend Type Definitions

**Files:**
- Modify: `web/src/types/index.ts`

**Step 1: Add result types**

Add to `web/src/types/index.ts`:

```typescript
// 批量生成配置结果
export interface BatchGenerateResult {
  total: number;
  success: number;
  failed: number;
  results: GenerateResultItem[];
}

export interface GenerateResultItem {
  agentId: string;
  agentName: string;
  status: 'success' | 'failed';
  skillsCount: number;
  commandsCount: number;
  subagentsCount: number;
  rulesCount: number;
  settingsCount: number;
  error?: string;
}

// 批量修改基础Agent结果
export interface BatchUpdateResult {
  total: number;
  success: number;
  failed: number;
  results: UpdateResultItem[];
}

export interface UpdateResultItem {
  agentId: string;
  agentName: string;
  baseAgentName: string;
  status: 'success' | 'failed';
  error?: string;
}
```

**Step 2: Commit**

```bash
git add web/src/types/index.ts
git commit -m "feat(types): add batch operation result types"
```

---

## Task 8: Frontend API Client

**Files:**
- Modify: `web/src/api/client.ts`

**Step 1: Add API methods**

Add to agents section in `web/src/api/client.ts`:

```typescript
agents: {
  // ... existing methods ...

  // 批量生成配置
  batchGenerateConfig: (agentIds: string[], cliType?: string) =>
    request.post<BatchGenerateResult>('/api/agents/batch-generate-config', {
      agentIds,
      cliType: cliType || 'claude_code',
    }),

  // 批量修改基础Agent
  batchUpdateBaseAgent: (agentIds: string[], baseAgentId: string) =>
    request.post<BatchUpdateResult>('/api/agents/batch-update-base-agent', {
      agentIds,
      baseAgentId,
    }),
},
```

**Step 2: Add type imports**

Ensure types are imported at the top:

```typescript
import type {
  // ... existing types ...
  BatchGenerateResult,
  BatchUpdateResult,
} from '@/types';
```

**Step 3: Commit**

```bash
git add web/src/api/client.ts
git commit -m "feat(api): add batch operation API methods"
```

---

## Task 9: Frontend UI - Batch Buttons

**Files:**
- Modify: `web/src/pages/AgentRoleList.tsx`

**Step 1: Add state variables**

Add after existing state declarations:

```typescript
// 批量生成配置相关状态
const [batchGenerateLoading, setBatchGenerateLoading] = useState(false);
const [batchResultVisible, setBatchResultVisible] = useState(false);
const [batchResultData, setBatchResultData] = useState<BatchGenerateResult | null>(null);

// 批量修改基础Agent相关状态
const [batchUpdateVisible, setBatchUpdateVisible] = useState(false);
const [batchUpdateLoading, setBatchUpdateLoading] = useState(false);
const [batchUpdateResultVisible, setBatchUpdateResultVisible] = useState(false);
const [batchUpdateResultData, setBatchUpdateResultData] = useState<BatchUpdateResult | null>(null);
const [targetBaseAgentId, setTargetBaseAgentId] = useState<string>('');
```

**Step 2: Add type imports**

Add to imports:

```typescript
import type { BatchGenerateResult, BatchUpdateResult } from '@/types';
```

**Step 3: Add batch operation buttons**

Modify the `extra` section of custom agents Card:

```tsx
extra={
  <Space>
    {selectedRowKeys.length > 0 && (
      <>
        <Button
          icon={<EditOutlined />}
          onClick={() => setBatchUpdateVisible(true)}
        >
          批量修改基础Agent ({selectedRowKeys.length})
        </Button>
        <Button
          icon={<SettingOutlined />}
          onClick={handleBatchGenerateConfig}
          loading={batchGenerateLoading}
        >
          批量生成配置 ({selectedRowKeys.length})
        </Button>
        <Button
          danger
          icon={<DeleteOutlined />}
          loading={batchDeleteLoading}
          onClick={handleBatchDelete}
        >
          批量删除 ({selectedRowKeys.length})
        </Button>
      </>
    )}
    <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
      新建角色
    </Button>
  </Space>
}
```

**Step 4: Commit**

```bash
git add web/src/pages/AgentRoleList.tsx
git commit -m "feat(ui): add batch operation buttons"
```

---

## Task 10: Frontend Handler - BatchGenerateConfig

**Files:**
- Modify: `web/src/pages/AgentRoleList.tsx`

**Step 1: Add handleBatchGenerateConfig function**

Add after handleBatchDelete function:

```typescript
// 批量生成配置
const handleBatchGenerateConfig = () => {
  if (selectedRowKeys.length === 0) return;

  Modal.confirm({
    title: '批量生成配置',
    icon: <ExclamationCircleOutlined />,
    content: (
      <div>
        <p>确定为选中的 {selectedRowKeys.length} 个 Agent 生成配置？</p>
        <p style={{ color: '#faad14' }}>已有配置将被覆盖。</p>
      </div>
    ),
    okText: '确认生成',
    okType: 'primary',
    cancelText: '取消',
    onOk: async () => {
      setBatchGenerateLoading(true);
      try {
        const result = await api.agents.batchGenerateConfig(selectedRowKeys as string[]);
        setBatchResultData(result);
        setBatchResultVisible(true);
        setSelectedRowKeys([]);
        loadConfigs();
      } catch (error) {
        message.error('批量生成失败');
      } finally {
        setBatchGenerateLoading(false);
      }
    },
  });
};
```

**Step 2: Commit**

```bash
git add web/src/pages/AgentRoleList.tsx
git commit -m "feat(ui): add handleBatchGenerateConfig function"
```

---

## Task 11: Frontend Handler - BatchUpdateBaseAgent

**Files:**
- Modify: `web/src/pages/AgentRoleList.tsx`

**Step 1: Add handleBatchUpdateBaseAgent function**

Add after handleBatchGenerateConfig:

```typescript
// 批量修改基础Agent
const handleBatchUpdateBaseAgent = async () => {
  if (!targetBaseAgentId) {
    message.warning('请选择目标基础Agent');
    return;
  }

  setBatchUpdateLoading(true);
  try {
    const result = await api.agents.batchUpdateBaseAgent(
      selectedRowKeys as string[],
      targetBaseAgentId
    );
    setBatchUpdateResultData(result);
    setBatchUpdateVisible(false);
    setBatchUpdateResultVisible(true);
    setSelectedRowKeys([]);
    setTargetBaseAgentId('');
    loadConfigs();
  } catch (error) {
    message.error('批量修改失败');
  } finally {
    setBatchUpdateLoading(false);
  }
};
```

**Step 2: Commit**

```bash
git add web/src/pages/AgentRoleList.tsx
git commit -m "feat(ui): add handleBatchUpdateBaseAgent function"
```

---

## Task 12: Frontend Modal - BatchUpdateBaseAgent Selection

**Files:**
- Modify: `web/src/pages/AgentRoleList.tsx`

**Step 1: Add selection modal**

Add after the existing Modal for create/edit:

```tsx
{/* 批量修改基础Agent选择弹窗 */}
<Modal
  title="批量修改基础Agent"
  open={batchUpdateVisible}
  onCancel={() => {
    setBatchUpdateVisible(false);
    setTargetBaseAgentId('');
  }}
  onOk={handleBatchUpdateBaseAgent}
  confirmLoading={batchUpdateLoading}
  okText="确认修改"
  cancelText="取消"
>
  <Form layout="vertical">
    <Form.Item label="选择目标基础Agent">
      <Select
        placeholder="选择要切换的基础Agent"
        value={targetBaseAgentId}
        onChange={setTargetBaseAgentId}
        style={{ width: '100%' }}
      >
        {baseAgents.map(agent => (
          <Select.Option key={agent.id} value={agent.id}>
            {agent.name} ({agent.type === 'claude_code' ? 'Claude Code' : 'OpenCode'})
          </Select.Option>
        ))}
      </Select>
    </Form.Item>
  </Form>

  <Alert
    type="info"
    message={`将为 ${selectedRowKeys.length} 个 Agent 更换基础Agent`}
    showIcon
    style={{ marginTop: 16 }}
  />
</Modal>
```

**Step 2: Commit**

```bash
git add web/src/pages/AgentRoleList.tsx
git commit -m "feat(ui): add batch update base agent selection modal"
```

---

## Task 13: Frontend Modal - BatchGenerateConfig Result

**Files:**
- Modify: `web/src/pages/AgentRoleList.tsx`

**Step 1: Add result modal**

Add after batch update modal:

```tsx
{/* 批量生成配置结果弹窗 */}
<Modal
  title="批量生成结果"
  open={batchResultVisible}
  onCancel={() => {
    setBatchResultVisible(false);
    setBatchResultData(null);
  }}
  footer={[
    <Button key="close" onClick={() => {
      setBatchResultVisible(false);
      setBatchResultData(null);
    }}>
      关闭
    </Button>,
  ]}
  width={700}
>
  {batchResultData && (
    <>
      <Alert
        type={batchResultData.failed > 0 ? 'warning' : 'success'}
        message={`成功 ${batchResultData.success} 个，失败 ${batchResultData.failed} 个`}
        showIcon
        style={{ marginBottom: 16 }}
      />

      <Table
        dataSource={batchResultData.results}
        rowKey="agentId"
        size="small"
        pagination={false}
        columns={[
          {
            title: 'Agent名称',
            dataIndex: 'agentName',
            width: 150,
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 80,
            render: (status: string) =>
              status === 'success'
                ? <Tag color="green">成功</Tag>
                : <Tag color="red">失败</Tag>,
          },
          {
            title: '详情',
            render: (record: GenerateResultItem) =>
              record.status === 'success'
                ? <Text>{record.skillsCount} Skills, {record.commandsCount} Commands, {record.subagentsCount} Subagents, {record.rulesCount} Rules, {record.settingsCount} Settings</Text>
                : <Text type="danger">{record.error}</Text>,
          },
        ]}
      />
    </>
  )}
</Modal>
```

**Step 2: Commit**

```bash
git add web/src/pages/AgentRoleList.tsx
git commit -m "feat(ui): add batch generate config result modal"
```

---

## Task 14: Frontend Modal - BatchUpdateBaseAgent Result

**Files:**
- Modify: `web/src/pages/AgentRoleList.tsx`

**Step 1: Add result modal**

Add after batch generate result modal:

```tsx
{/* 批量修改基础Agent结果弹窗 */}
<Modal
  title="批量修改结果"
  open={batchUpdateResultVisible}
  onCancel={() => {
    setBatchUpdateResultVisible(false);
    setBatchUpdateResultData(null);
  }}
  footer={[
    <Button key="close" onClick={() => {
      setBatchUpdateResultVisible(false);
      setBatchUpdateResultData(null);
    }}>
      关闭
    </Button>,
  ]}
  width={500}
>
  {batchUpdateResultData && (
    <>
      <Alert
        type={batchUpdateResultData.failed > 0 ? 'warning' : 'success'}
        message={`成功 ${batchUpdateResultData.success} 个，失败 ${batchUpdateResultData.failed} 个`}
        showIcon
        style={{ marginBottom: 16 }}
      />

      <Table
        dataSource={batchUpdateResultData.results}
        rowKey="agentId"
        size="small"
        pagination={false}
        columns={[
          {
            title: 'Agent名称',
            dataIndex: 'agentName',
            width: 150,
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 80,
            render: (status: string) =>
              status === 'success'
                ? <Tag color="green">成功</Tag>
                : <Tag color="red">失败</Tag>,
          },
          {
            title: '目标基础Agent',
            dataIndex: 'baseAgentName',
            render: (name: string, record: UpdateResultItem) =>
              record.status === 'success'
                ? <Tag color="blue">{name}</Tag>
                : <Text type="danger">{record.error}</Text>,
          },
        ]}
      />
    </>
  )}
</Modal>
```

**Step 2: Commit**

```bash
git add web/src/pages/AgentRoleList.tsx
git commit -m "feat(ui): add batch update base agent result modal"
```

---

## Task 15: Final Verification

**Step 1: Build backend**

```bash
cd D:/CoLinkProject/Colink-0412/isdp && go build ./cmd/server
```

Expected: Build succeeds

**Step 2: Build frontend**

```bash
cd D:/CoLinkProject/Colink-0412/isdp/web && npm run build
```

Expected: Build succeeds

**Step 3: Manual test**

1. Start backend: `go run ./cmd/server`
2. Start frontend: `cd web && npm run dev`
3. Navigate to Agent角色页面
4. Select multiple agents
5. Test "批量修改基础Agent" button
6. Test "批量生成配置" button
7. Verify result modals display correctly

---

## Summary

**Total Tasks:** 15

**Backend Changes:**
- Model structs (Task 1)
- Service methods (Tasks 2-3)
- API handlers (Task 4)
- Route registration (Task 5)
- Build verification (Task 6)

**Frontend Changes:**
- Type definitions (Task 7)
- API client (Task 8)
- UI buttons (Task 9)
- Handler functions (Tasks 10-11)
- Modals (Tasks 12-14)
- Final verification (Task 15)