# 联邦技能源同步按钮报错修复

**时间**: 2026-05-07
**问题**: 点击同步按钮时浏览器报错 `Cannot read properties of null (reading 'length')`
**根因**: Go nil 切片 JSON 序列化为 null，前端未防御性检查

## 修复内容

### 1. 后端修复 (`internal/service/skill/registry_service.go:208-215`)

初始化所有切片字段为空切片，避免 JSON 序列化为 null：

```go
result := &model.SyncPreviewResult{
    RegistryID:       registry.ID,
    RegistryName:     registry.Name,
    AutoUpdateSkills: []*model.SyncPreviewSkill{},  // 新增
    ConflictSkills:   []*model.SyncConflictSkill{},  // 新增
    NewSkills:        []*model.RemoteSkill{},        // 新增
    SkippedSkills:    []*model.RemoteSkill{},        // 新增
}
```

### 2. 前端修复 (`web/src/pages/RegistryManagement/index.tsx:155`)

增加可选链防护，兼容 null/undefined：

```typescript
// 修复前
if (preview.conflictSkills.length === 0) {

// 修复后
if ((preview.conflictSkills?.length ?? 0) === 0) {
```

## 验证结果

- ✅ 前端构建成功 (vite build)
- ✅ 后端编译成功 (go build ./cmd/server)

## 相关文件

- `internal/service/skill/registry_service.go`
- `internal/model/skill.go` (结构体定义)
- `web/src/pages/RegistryManagement/index.tsx`