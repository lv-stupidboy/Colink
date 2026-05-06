# 修复计划：联邦源单选导入认证失败

## 问题定位

### 前端问题（已修复 ✅）

**根因**：前端单选联邦导入流程错误调用 `skills.create` API（只创建数据库记录），未调用 `skills.importFederated` API（下载并创建文件）。

**问题位置**：`web/src/pages/SkillLibrary/index.tsx:308-315`

```tsx
// handleModalOk 中判断逻辑
if (editingSkill?.id) {
  await api.skills.update(editingSkill.id, values);
} else {
  // 单选联邦导入错误地走这里
  await api.skills.create(values);  // ❌ 只创建数据库记录
}
```

**修复状态**：已在 `handleSubmit` 中添加 `sourceType === 'federated'` 判断。

### 后端问题（待修复 ❌）

**根因**：`ImportFromFederated` handler 使用硬编码 URL `https://skills.sh`，忽略传入的 `registryId`，导致认证失败 (HTTP 401)。

**问题位置**：`internal/api/skill_handler.go:619-621`

```go
// TODO: 从数据库查询 Registry 信息
// 这里先硬编码支持 skills.sh
federatedURL := "https://skills.sh"
```

**对比批量导入**（正确实现）：
```go
// BatchImportFederated → scanner.ImportSkills
registry, err := s.registryRepo.FindByID(ctx, req.RegistryID)
// 使用 registry 的 Git URL 和认证信息
```

## 修复方案

### 前端修复（已完成）

在 `handleModalOk` 中添加对 `sourceType === 'federated'` 的判断：

```tsx
if (isAfterUpload && pendingZipBlobRef.current) {
  // 上传 zip 创建
  ...
} else if (editingSkill?.id) {
  // 编辑现有记录
  await api.skills.update(editingSkill.id, values);
} else if (sourceType === 'federated' && selectedRegistryId) {
  // 单选联邦导入 - 使用正确的 API
  await api.skills.importFederated(selectedRegistryId, values.name);
} else {
  // 手动创建
  await api.skills.create(values);
}
```

### 后端修复方案

重构 `ImportFromFederated` 函数，参考批量导入的实现：

1. 使用 `registryId` 从数据库查询 Registry 信息
2. 使用 Registry 的 Git URL 执行 git clone
3. 复制指定 skill 目录到存储路径

**代码结构参考**：
```go
func (h *SkillHandler) ImportFromFederated(c *gin.Context) {
    var req struct {
        RegistryID string `json:"registryId" binding:"required"`
        SkillName  string `json:"skillName" binding:"required"`
    }
    
    // 1. 查询 Registry
    registry, err := h.registryRepo.FindByID(ctx, req.RegistryID)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "联邦源不存在"})
        return
    }
    
    // 2. Git clone（参考 ImportFromRepo）
    tempDir := filepath.Join(h.storagePath, ".temp", uuid.New().String())
    cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", registry.CloneURL, tempDir)
    
    // 3. 复制指定 skill 目录
    srcPath := filepath.Join(tempDir, "skills", req.SkillName)
    destPath := filepath.Join(h.storagePath, req.SkillName)
    copyDirectory(srcPath, destPath)
    
    // 4. 创建数据库记录
    ...
}
```

## 修改文件

| 文件 | 修改内容 | 状态 |
|------|----------|------|
| `web/src/pages/SkillLibrary/index.tsx` | handleModalOk 添加 federated 分支判断 | ✅ 已完成 |
| `internal/api/skill_handler.go` | ImportFromFederated 重构使用 registryId | ❌ 待修复 |

## 验证步骤

1. 启动后端和前端服务
2. 配置一个需要认证的联邦源（如私有 Git 仓库）
3. 选择联邦源，单选一个 skill 导入
4. 确认无 HTTP 401 错误
5. 确认 `data/agent-assets/skills/{skillName}/SKILL.md` 文件已创建
6. 批量导入验证原有功能未受影响

## 风险评估

- **低风险**：修改仅影响单选联邦导入，不影响批量导入和其他创建方式
- **向后兼容**：无破坏性变更

---
创建时间：2026-05-06
更新时间：2026-05-06（添加后端修复）