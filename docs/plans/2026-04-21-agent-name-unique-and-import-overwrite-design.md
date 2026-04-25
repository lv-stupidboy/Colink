# Agent 角色名称唯一性校验与导入覆盖机制

## 背景

当前系统存在以下问题：
1. 新建角色时无名称唯一性校验，导致出现同名角色
2. 导入角色/团队时，同名角色会新增而非覆盖

## 需求确认

- **角色名称全局唯一**：任何角色不允许同名（包括系统预置与自定义角色）
- **导入直接覆盖**：导入时发现同名角色，自动覆盖而非新增

## 设计方案

### 一、后端：添加名称唯一性校验

#### 1. Repository 层新增方法

**文件**: `internal/repo/agent_config.go`

```go
// FindByName 根据名称查找配置
func (r *AgentConfigRepository) FindByName(ctx context.Context, name string) (*model.AgentRoleConfig, error) {
    query := `
        SELECT id, name, role, description, system_prompt, max_tokens, temperature, 
               base_agent_id, is_default, is_system, requires_human, mention_patterns, 
               config_generated_at, config_path, created_at, updated_at
        FROM agent_configs WHERE name = ?
    `
    // ... 扫描并返回结果
}
```

#### 2. Service 层校验逻辑

**文件**: `internal/service/agent/config_service.go`

**Create 方法修改**:
```go
func (s *ConfigService) Create(ctx context.Context, req *model.CreateAgentRequest) (*model.AgentRoleConfig, error) {
    // 1. 校验名称唯一性
    existing, err := s.repo.FindByName(ctx, req.Name)
    if err == nil && existing != nil {
        return nil, fmt.Errorf("角色名称「%s」已存在，请使用其他名称", req.Name)
    }
    
    // 2. 继续原有创建流程...
}
```

**Update 方法修改**:
```go
func (s *ConfigService) Update(ctx context.Context, id uuid.UUID, req *model.CreateAgentRequest) (*model.AgentRoleConfig, error) {
    // 1. 校验名称唯一性（排除自身）
    existing, err := s.repo.FindByName(ctx, req.Name)
    if err == nil && existing != nil && existing.ID != id {
        return nil, fmt.Errorf("角色名称「%s」已被其他角色使用", req.Name)
    }
    
    // 2. 继续原有更新流程...
}
```

### 二、后端：导入时覆盖同名角色

**文件**: `internal/service/teampackage/service.go`

**importRole 方法修改**:

当前逻辑（按ID匹配）改为按名称匹配：

```go
func (s *Service) importRole(ctx context.Context, role model.TeamPackageRole, overwrite bool) (uuid.UUID, string, model.ImportDetail) {
    detail := model.ImportDetail{
        AssetType: "role",
        Name:      role.Name,
    }

    // 按名称查找已存在的角色
    existing, err := s.agentRepo.FindByName(ctx, role.Name)
    if err == nil && existing != nil {
        // 存在同名角色：覆盖更新
        if !overwrite {
            detail.Status = "skipped"
            detail.Message = "已存在同名角色，跳过导入"
            return existing.ID, role.ID, detail
        }
        
        // 删除旧角色的绑定关系
        if err := s.deleteRoleBindings(ctx, existing.ID); err != nil {
            s.logger.Warn("删除旧角色绑定关系失败", zap.Error(err))
        }
        
        // 更新角色属性（保留原ID）
        existing.Role = model.AgentRole(role.Role)
        existing.Description = role.Description
        existing.SystemPrompt = role.SystemPrompt
        existing.MaxTokens = role.MaxTokens
        existing.Temperature = role.Temperature
        existing.RequiresHuman = role.RequiresHuman
        existing.MentionPatterns = role.MentionPatterns
        existing.UpdatedAt = time.Now()
        
        if err := s.agentRepo.Update(ctx, existing); err != nil {
            detail.Status = "failed"
            detail.Message = fmt.Sprintf("更新角色失败: %v", err)
            return uuid.Nil, role.ID, detail
        }
        
        detail.Status = "success"
        detail.ID = existing.ID.String()
        detail.Message = "已覆盖同名角色"
        return existing.ID, role.ID, detail
    }

    // 不存在同名角色：新建
    // ... 原有新建逻辑
}
```

**ImportPreview 方法修改**:

预览时按名称匹配而非ID：

```go
// 检查角色是否已存在（改为按名称匹配）
for _, role := range manifest.Roles {
    previewRole := model.TeamPackagePreviewRole{
        Name:   role.Name,
        Exists: false,
    }
    existing, err := s.agentRepo.FindByName(ctx, role.Name)
    if err == nil && existing != nil {
        previewRole.Exists = true
        previewRole.LocalID = existing.ID.String()
    }
    preview.Roles = append(preview.Roles, previewRole)
}
```

### 三、前端：错误提示优化

**文件**: `web/src/pages/AgentRoleList.tsx`

**handleSubmit 方法修改**:

```typescript
const handleSubmit = async (values: Partial<AgentConfig>) => {
  try {
    if (editingConfig) {
      await api.agents.update(editingConfig.id, updateData);
      // ... 绑定关系更新
      message.success('更新成功');
    } else {
      const newAgent = await api.agents.create(createData);
      // ... 绑定关系更新
      message.success('创建成功');
    }
    setModalVisible(false);
    loadConfigs();
  } catch (error: any) {
    const errorData = error.response?.data;
    if (errorData?.error) {
      // 显示后端返回的具体错误信息
      message.error(errorData.error);
    } else {
      message.error('操作失败');
    }
  }
};
```

### 四、数据库：添加唯一索引（可选）

**文件**: `sql-change/v1.2.x/sqlite/000xx_add_name_unique.sql`

```sql
-- +goose Up
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_configs_name ON agent_configs(name);

-- +goose Down
DROP INDEX IF EXISTS idx_agent_configs_name;
```

**注意**: 添加唯一索引前需先清理现有重复数据。

## 实施任务清单

### Phase 1: 清理重复数据
1. 编写脚本检测并删除重复角色（保留最新创建的）
2. 执行清理脚本

### Phase 2: 后端校验
1. `internal/repo/agent_config.go` 新增 `FindByName` 方法
2. `internal/service/agent/config_service.go` Create 方法添加校验
3. `internal/service/agent/config_service.go` Update 方法添加校验
4. 测试新建/更新场景

### Phase 3: 导入覆盖机制
1. `internal/service/teampackage/service.go` importRole 改为按名称匹配
2. `internal/service/teampackage/service.go` ImportPreview 改为按名称匹配
3. 测试导入场景

### Phase 4: 数据库唯一索引（可选）
1. 添加唯一索引迁移脚本
2. 执行迁移

### Phase 5: 前端优化
1. 优化错误提示显示

## 影响分析

- **新建角色**: 名称重复时返回错误，用户需修改名称
- **更新角色**: 名称重复时返回错误（排除自身）
- **导入角色**: 同名角色自动覆盖，保留原ID和绑定关系
- **复制角色**: 复制时添加 "(副本)" 后缀，若仍重复需用户手动修改

## 风险点

1. 系统预置角色名称可能被导入覆盖 → 建议：导入时跳过系统角色（is_system=true）
2. 复制功能生成的 "(副本)" 可能重复 → 建议：复制时检测并生成唯一名称