# Agent角色批量操作功能设计

**日期**: 2026-04-17
**状态**: 设计完成，待实现

---

## 概述

为Agent角色管理页面新增两个批量操作功能：
1. **批量生成配置** - 为多个Agent角色一键生成配置文件（Commands、Skills、Rules等）
2. **批量修改基础Agent** - 为多个Agent角色批量更换关联的基础Agent实例

---

## 功能一：批量生成配置

### 1.1 用户需求

- **场景**: 用户希望一次性为多个Agent角色生成配置目录，而不是逐个点击"生成配置"
- **选择方式**: 复选框勾选多个Agent
- **冲突处理**: 强制覆盖已有配置
- **结果反馈**: 详细结果列表，显示每个Agent的成功/失败状态

### 1.2 API设计

**接口**: `POST /api/agents/batch-generate-config`

**请求体**:
```json
{
  "agentIds": ["uuid1", "uuid2", "uuid3"],
  "cliType": "claude_code"
}
```

**响应体**:
```json
{
  "total": 3,
  "success": 2,
  "failed": 1,
  "results": [
    {
      "agentId": "uuid1",
      "agentName": "需求分析师",
      "status": "success",
      "skillsCount": 5,
      "commandsCount": 3,
      "subagentsCount": 2,
      "rulesCount": 4,
      "settingsCount": 1
    },
    {
      "agentId": "uuid2",
      "agentName": "架构师",
      "status": "success",
      ...
    },
    {
      "agentId": "uuid3",
      "agentName": "测试工程师",
      "status": "failed",
      "error": "配置目录创建失败：磁盘空间不足"
    }
  ]
}
```

### 1.3 后端实现

**新增Handler** (`internal/api/agent_handler.go`):
```go
type BatchGenerateConfigRequest struct {
    AgentIds []string `json:"agentIds" binding:"required"`
    CliType  string   `json:"cliType"`
}

func (h *AgentHandler) BatchGenerateConfig(c *gin.Context) {
    var req BatchGenerateConfigRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        BadRequest(c, err.Error())
        return
    }
    
    // 解析UUID
    agentIds := make([]uuid.UUID, len(req.AgentIds))
    for i, idStr := range req.AgentIds {
        agentIds[i], _ = uuid.Parse(idStr)
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
```

**新增Service方法** (`internal/service/agent/config_service.go`):
```go
type BatchGenerateResult struct {
    Total   int                   `json:"total"`
    Success int                   `json:"success"`
    Failed  int                   `json:"failed"`
    Results []GenerateResultItem  `json:"results"`
}

type GenerateResultItem struct {
    AgentId       uuid.UUID `json:"agentId"`
    AgentName     string    `json:"agentName"`
    Status        string    `json:"status"`
    SkillsCount   int       `json:"skillsCount"`
    CommandsCount int       `json:"commandsCount"`
    SubagentsCount int      `json:"subagentsCount"`
    RulesCount    int       `json:"rulesCount"`
    SettingsCount int       `json:"settingsCount"`
    Error         string    `json:"error,omitempty"`
}

func (s *ConfigService) BatchGenerateConfig(ctx context.Context, 
    agentIds []uuid.UUID, cliType string) (*BatchGenerateResult, error) {
    
    result := &BatchGenerateResult{
        Total:   len(agentIds),
        Results: make([]GenerateResultItem, len(agentIds)),
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
            
            item := &GenerateResultItem{AgentId: agentId}
            
            config, err := s.GetByID(ctx, agentId)
            if err != nil {
                item.Status = "failed"
                item.Error = "Agent不存在"
            } else {
                item.AgentName = config.Name
                genResult, err := s.generateConfigForAgent(ctx, config, cliType)
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
            if item.Status == "success" { result.Success++ }
            else { result.Failed++ }
            mu.Unlock()
        }(i, id)
    }
    
    wg.Wait()
    return result, nil
}
```

**并行策略**:
- 使用goroutine并发处理
- 通过semaphore限制并发数为5，避免资源耗尽
- mutex保护共享结果

### 1.4 前端实现

**新增按钮** (`web/src/pages/AgentRoleList.tsx`):
```tsx
const [batchGenerateLoading, setBatchGenerateLoading] = useState(false);
const [batchResultVisible, setBatchResultVisible] = useState(false);
const [batchResultData, setBatchResultData] = useState<BatchGenerateResult | null>(null);

// 批量操作按钮区域
{selectedRowKeys.length > 0 && (
  <Space>
    <Button
      icon={<SettingOutlined />}
      onClick={handleBatchGenerateConfig}
      loading={batchGenerateLoading}
    >
      批量生成配置 ({selectedRowKeys.length})
    </Button>
    <Button danger icon={<DeleteOutlined />} ... />
  </Space>
)}
```

**批量生成函数**:
```tsx
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

**结果弹窗**:
```tsx
<Modal
  title="批量生成结果"
  open={batchResultVisible}
  onCancel={() => setBatchResultVisible(false)}
  footer={<Button onClick={() => setBatchResultVisible(false)}>关闭</Button>}
  width={700}
>
  <Alert
    type={batchResultData?.failed > 0 ? 'warning' : 'success'}
    message={`成功 ${batchResultData?.success} 个，失败 ${batchResultData?.failed} 个`}
    showIcon
    style={{ marginBottom: 16 }}
  />
  
  <Table
    dataSource={batchResultData?.results}
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
            ? <span>{record.skillsCount} Skills, {record.commandsCount} Commands, {record.subagentsCount} Subagents, {record.rulesCount} Rules, {record.settingsCount} Settings</span>
            : <Text type="danger">{record.error}</Text>,
      },
    ]}
  />
</Modal>
```

**新增API调用** (`web/src/api/client.ts`):
```ts
agents: {
  ...
  batchGenerateConfig: (agentIds: string[]) =>
    request.post('/api/agents/batch-generate-config', { agentIds }),
}
```

---

## 功能二：批量修改基础Agent

### 2.1 用户需求

- **场景**: 用户希望一次性为多个Agent角色更换关联的基础Agent实例
- **选择方式**: 复选框勾选多个Agent → 弹窗选择目标BaseAgent
- **结果反馈**: 详细结果列表，显示每个Agent的修改状态

### 2.2 API设计

**接口**: `POST /api/agents/batch-update-base-agent`

**请求体**:
```json
{
  "agentIds": ["uuid1", "uuid2"],
  "baseAgentId": "uuid-target"
}
```

**响应体**:
```json
{
  "total": 2,
  "success": 2,
  "failed": 0,
  "results": [
    { "agentId": "uuid1", "agentName": "需求分析师", "status": "success", "baseAgentName": "Claude Code Pro" },
    { "agentId": "uuid2", "agentName": "架构师", "status": "success", "baseAgentName": "Claude Code Pro" }
  ]
}
```

### 2.3 后端实现

**新增Handler**:
```go
type BatchUpdateBaseAgentRequest struct {
    AgentIds    []string `json:"agentIds" binding:"required"`
    BaseAgentId string   `json:"baseAgentId" binding:"required"`
}

func (h *AgentHandler) BatchUpdateBaseAgent(c *gin.Context) {
    var req BatchUpdateBaseAgentRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        BadRequest(c, err.Error())
        return
    }
    
    agentIds := make([]uuid.UUID, len(req.AgentIds))
    for i, idStr := range req.AgentIds {
        agentIds[i], _ = uuid.Parse(idStr)
    }
    baseAgentId, _ := uuid.Parse(req.BaseAgentId)
    
    result, err := h.configService.BatchUpdateBaseAgent(c.Request.Context(), agentIds, baseAgentId)
    if err != nil {
        InternalError(c, err.Error())
        return
    }
    
    Success(c, result)
}
```

**新增Service方法**:
```go
type BatchUpdateResult struct {
    Total   int                  `json:"total"`
    Success int                  `json:"success"`
    Failed  int                  `json:"failed"`
    Results []UpdateResultItem   `json:"results"`
}

type UpdateResultItem struct {
    AgentId       uuid.UUID `json:"agentId"`
    AgentName     string    `json:"agentName"`
    BaseAgentName string    `json:"baseAgentName"`
    Status        string    `json:"status"`
    Error         string    `json:"error,omitempty"`
}

func (s *ConfigService) BatchUpdateBaseAgent(ctx context.Context,
    agentIds []uuid.UUID, baseAgentId uuid.UUID) (*BatchUpdateResult, error) {
    
    // 验证目标BaseAgent存在
    baseAgent, err := s.baseAgentRepo.FindByID(ctx, baseAgentId)
    if err != nil {
        return nil, fmt.Errorf("目标基础Agent不存在")
    }
    
    result := &BatchUpdateResult{
        Total:   len(agentIds),
        Results: make([]UpdateResultItem, len(agentIds)),
    }
    
    for i, id := range agentIds {
        item := &UpdateResultItem{AgentId: id}
        
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
        if item.Status == "success" { result.Success++ }
        else { result.Failed++ }
    }
    
    return result, nil
}
```

### 2.4 前端实现

**新增状态和按钮**:
```tsx
const [batchUpdateVisible, setBatchUpdateVisible] = useState(false);
const [targetBaseAgentId, setTargetBaseAgentId] = useState<string>('');
const [batchUpdateLoading, setBatchUpdateLoading] = useState(false);
const [batchUpdateResult, setBatchUpdateResult] = useState<BatchUpdateResult | null>(null);

// 批量操作按钮
{selectedRowKeys.length > 0 && (
  <Space>
    <Button icon={<EditOutlined />} onClick={() => setBatchUpdateVisible(true)}>
      批量修改基础Agent ({selectedRowKeys.length})
    </Button>
    <Button icon={<SettingOutlined />} onClick={handleBatchGenerateConfig}>
      批量生成配置 ({selectedRowKeys.length})
    </Button>
    <Button danger icon={<DeleteOutlined />} onClick={handleBatchDelete}>
      批量删除 ({selectedRowKeys.length})
    </Button>
  </Space>
)}
```

**选择弹窗**:
```tsx
<Modal
  title="批量修改基础Agent"
  open={batchUpdateVisible}
  onCancel={() => setBatchUpdateVisible(false)}
  onOk={handleBatchUpdateBaseAgent}
  confirmLoading={batchUpdateLoading}
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
  />
</Modal>
```

**执行修改**:
```tsx
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
    setBatchUpdateResult(result);
    setBatchUpdateVisible(false);
    setBatchUpdateResultVisible(true);
    setSelectedRowKeys([]);
    loadConfigs();
  } catch (error) {
    message.error('批量修改失败');
  } finally {
    setBatchUpdateLoading(false);
  }
};
```

**新增API调用**:
```ts
agents: {
  ...
  batchUpdateBaseAgent: (agentIds: string[], baseAgentId: string) =>
    request.post('/api/agents/batch-update-base-agent', { agentIds, baseAgentId }),
}
```

---

## 错误处理

### 后端错误处理

| 场景 | 处理方式 |
|------|----------|
| Agent不存在 | 单项失败，记录错误 |
| 配置生成失败（磁盘/权限） | 单项失败，记录原始错误 |
| 目标BaseAgent不存在 | 全部失败，返回错误 |
| 数据库更新失败 | 单项失败，记录错误 |

### 前端错误处理

| 场景 | 处理方式 |
|------|----------|
| 未选择任何Agent | 按钮禁用或弹出提示 |
| 未选择目标BaseAgent | 弹窗内提示 |
| API请求失败 | Toast提示"操作失败" |
| 单项失败 | 结果弹窗显示红色Tag和错误 |

---

## 实现文件清单

### 后端新增/修改

| 文件 | 改动 |
|------|------|
| `internal/api/agent_handler.go` | 新增 `BatchGenerateConfig`、`BatchUpdateBaseAgent` Handler |
| `internal/service/agent/config_service.go` | 新增 `BatchGenerateConfig`、`BatchUpdateBaseAgent` 方法 |
| `internal/model/agent_config.go` | 新增请求/响应结构体 |
| `cmd/server/main.go` | 注册新路由 |

### 前端新增/修改

| 文件 | 改动 |
|------|------|
| `web/src/pages/AgentRoleList.tsx` | 新增批量操作按钮、弹窗、处理函数 |
| `web/src/api/client.ts` | 新增 `batchGenerateConfig`、`batchUpdateBaseAgent` API |
| `web/src/types/index.ts` | 新增结果类型定义 |

---

## 测试要点

1. **批量生成配置**:
   - 选中1个Agent → 正常生成
   - 选中多个Agent → 并行生成
   - 包含系统预置Agent → 允许生成
   - 配置目录已存在 → 覆盖

2. **批量修改基础Agent**:
   - 选中1个Agent → 正常修改
   - 选中多个Agent → 批量修改
   - 目标BaseAgent不存在 → 提示错误
   - 系统预置Agent → 允许修改

3. **并发处理**:
   - 后端并发数限制生效
   - 前端loading状态正确显示
   - 大量Agent时响应时间合理