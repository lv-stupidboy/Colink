# 联邦导入单选模式修复验证报告

**测试时间**: 2026-05-06 10:34
**测试工程师**: SuperPowers测试工程师

## 测试背景

用户报告：从联邦源导入 skill 时，后端返回 201 但未创建 skill.md 文件。

前端修复：`handleSubmit` 函数添加对 `sourceType === 'federated'` 的判断，调用 `api.skills.importFederated`。

## 测试环境

| 组件 | 状态 | 端口 |
|------|------|------|
| 后端服务 | ✅ 运行 | 26305 |
| 前端服务 | ✅ 运行 | 26306 |
| 数据库 | ✅ SQLite | ./data/sqlite/colink.db |

## 测试执行

### 1. 批量导入 API 测试

**请求**:
```bash
POST /api/v1/skills/import/federated/batch
{
  "registryId": "48675343-aaf9-4a7d-8134-a7f92d633ea5",
  "skills": [{"name": "daily-digest", "path": "skills/daily-digest", ...}]
}
```

**结果**: ✅ 成功
- HTTP 200
- Skill 记录创建成功 (ID: 6e077480-11e3-4e5e-bf73-eca786865fe9)
- SKILL.md 文件创建成功

**文件验证**:
```
D:/workspace/isdp/data/agent-assets/skills/daily-digest/SKILL.md
内容: 3088 bytes，包含 YAML frontmatter 和完整 skill 定义
```

### 2. 单选导入 API 测试

**请求**:
```bash
POST /api/v1/skills/import/federated
{
  "registryId": "48675343-aaf9-4a7d-8134-a7f92d633ea5",
  "skillName": "daily-digest"
}
```

**结果**: ❌ 失败
- HTTP 500
- 错误: `下载技能失败: 获取技能元数据失败: HTTP 401`

**根因**: `ImportFromFederated` handler 使用硬编码 URL (`https://skills.sh`)，忽略传入的 registryId。

## 代码审查发现

### 前端修复验证 ✅

文件: `web/src/pages/SkillLibrary/index.tsx`
位置: `handleSubmit` 函数 (第 312-327 行)

```tsx
} else if (sourceType === 'federated' && selectedRegistryId) {
  // 单选联邦导入：下载并创建 skill 文件
  await api.skills.importFederated(selectedRegistryId, values.name);
  // 更新额外字段
  ...
  message.success('联邦导入成功');
}
```

**结论**: 前端逻辑正确，会调用 `importFederated` API。

### 后端问题发现 ❌

文件: `internal/api/skill_handler.go`
位置: `ImportFromFederated` 函数 (第 609-669 行)

```go
// 硬编码 URL，未使用 registryId
federatedURL := "https://skills.sh"
// ...
skillData, err := h.downloadFederatedSkill(federatedURL, req.SkillName)
```

**对比批量导入** (第 519-617 行):

```go
// 正确使用 registryId
registry, err := s.registryRepo.FindByID(ctx, req.RegistryID)
cloneURL := s.buildCloneURL(registry)
// git clone + 复制目录
```

## 结论

| 组件 | 修复状态 | 说明 |
|------|---------|------|
| 前端 | ✅ 已修复 | 正确调用 `importFederated` API |
| 后端单选导入 | ❌ 需修复 | 使用硬编码 URL，应重构为类似批量导入逻辑 |
| 后端批量导入 | ✅ 正常工作 | 正确使用 Git 仓库克隆 |

## 下一步行动

需要后端修复：`ImportFromFederated` handler 需重构以使用 registryId 查询 Git 仓库 URL，执行 git clone 并复制 skill 目录。

---

<a2a-handoff>
### What | 后端 ImportFromFederated handler 需重构
### Why | 当前使用硬编码 URL 忽略 registryId，导致单选联邦导入失败
### Next | 参考批量导入逻辑，使用 registryId 查询 Git 仓库 URL 并执行 git clone
</a2a-handoff>

@SuperPowers全栈开发工程师 请修复后端 ImportFromFederated handler