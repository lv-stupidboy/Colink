# 团队包性能优化计划审查

**时间**: 2026-04-22 21:45
**审查者**: Colink计划审查师
**结论**: ✅ 通过（附修正建议）

---

## 审查结论

### 验证通过的点

| 项目 | 状态 | 验证方式 |
|------|------|----------|
| GetTeamPackages 无缓存 | ✅ 正确 | service.go:180-181 每次调用 RefreshMarket |
| 批量预览串行 | ✅ 正确 | TeamPackages.tsx:140-158 for 循环逐个调用 |
| 批量导入串行 | ✅ 正确 | TeamPackages.tsx:176-192 逐个调用 syncPackage |
| 文件路径存在 | ✅ 正确 | service.go, market_handler.go, TeamPackages.tsx, client.ts 都存在 |
| Go 代码片段语法 | ✅ 正确 | cache.go, batch.go 结构合理 |
| TypeScript 代码片段 | ✅ 正确 | teamPackageCache.ts, API 方法语法正确 |

### 需修正的问题

| 问题 | 严重度 | 建议 |
|------|--------|------|
| 缓存键设计不完整 | ⚠️ 中 | GetTeamPackages 返回所有市场聚合结果，应缓存整个结果而非按 marketId 分存 |
| API 方法位置错误 | ⚠️ 中 | previewPackagesBatch/syncPackagesBatch 应在 teamPackages 对象而非 markets |
| 并发数硬编码 | ⚠️ 低 | 建议 5/3 从 config.TeamPackageSyncConfig 读取 |
| 部分成功处理 | ⚠️ 低 | 批量操作失败部分应有明确的前端提示策略 |

---

## 修正建议详情

### 1. 缓存键设计调整

**当前设计**：按 marketId 缓存单个市场数据

**问题**：GetTeamPackages 需要遍历所有 market，聚合后返回。按 marketId 缓存会导致：
- 每次仍需遍历所有 market 检查缓存
- 需要额外的聚合逻辑

**建议**：直接缓存 GetTeamPackages 的完整结果

```go
// 建议的缓存结构
type TeamPackagesCache struct {
    data      []model.MarketPackage
    expiredAt time.Time
}

// 缓存键：固定 key "team-packages"
func (s *Service) GetTeamPackages(ctx context.Context, forceRefresh bool) ([]model.MarketPackage, error) {
    if !forceRefresh {
        cached := s.cache.GetTeamPackages()
        if cached != nil {
            return cached, nil
        }
    }
    // ... 刷新逻辑
    s.cache.SetTeamPackages(packages)
    return packages, nil
}
```

### 2. API 方法位置修正

**计划原位置**：markets 对象
**应放位置**：teamPackages 对象

```typescript
// client.ts - 应放在 teamPackages 对象内
teamPackages = {
  // ...existing methods...
  previewPackagesBatch: (...) => ...,
  syncPackagesBatch: (...) => ...,
};
```

### 3. 并发数配置化

**当前**：硬编码 maxConcurrency := 5
**建议**：从 config 读取

```go
maxConcurrency := s.config.MaxPreviewConcurrency  // 默认 5
```

---

## 实施可行性评估

| 评估项 | 结论 |
|--------|------|
| 改动范围适中 | ✅ 新增2后端文件+1前端文件，修改4文件 |
| 不影响现有功能 | ✅ forceRefresh 默认 false，保持向后兼容 |
| 技术方案可行 | ✅ Go sync.RWMutex + TypeScript localStorage 都成熟 |
| 预期效果合理 | ✅ 缓存命中时 <1秒，并行处理 ~80% 提升 |

---

## 结论

计划整体可行，建议开发工程师实施时参考上述修正建议。

**审查通过，可进入实施阶段。**