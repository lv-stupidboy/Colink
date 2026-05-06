# 单选联邦导入 HTTP 401 错误测试报告

**测试日期**: 2026-05-06
**测试时间**: 11:48
**测试执行者**: SuperPowers 测试工程师
**测试ID**: TF-2026-0506-002

---

## 测试结果：问题仍然存在

### 核心发现

**HTTP 401 错误未修复！** 直接调用 ImportFromFederated API 返回：

```json
{"error":"获取联邦技能列表失败: HTTP 401"}
HTTP_CODE: 500
```

---

## 测试详情

### API 直接测试

**请求**:
```bash
curl -X POST "http://127.0.0.1:26305/api/v1/skills/import/federated" \
  -H "Content-Type: application/json" \
  -d '{"registryId": "48675343-aaf9-4a7d-8134-a7f92d633ea5", "name": "stale-pr-reminder"}'
```

**响应**:
```
{"error":"获取联邦技能列表失败: HTTP 401"}
HTTP_CODE: 500
```

### 前端测试结果

**测试步骤**:
1. 新建 Skill → 选择联邦来源 → 选择联邦源下载 ✅
2. 选择 oschina 联邦源（registryId: 48675343-aaf9-4a7d-8134-a7f92d633ea5） ✅
3. 点击导入 → 技能扫描成功（显示 12 个技能） ✅
4. 选择 stale-pr-reminder 技能 → 点击确认导入 ✅
5. 表单填充 → 选择兼容 Agent（表单验证阻止保存） ⚠️
6. 点击保存按钮 → 前端表单验证阻止（无 HTTP 401） ⚠️

**控制台日志**:
- 无 HTTP 401 错误（前端层面）
- 无 HTTP 500/502/503/504 错误
- 有 React Router 警告（不影响功能）

---

## 问题分析

### 错误来源定位

**错误消息**: "获取联邦技能列表失败: HTTP 401"

**错误位置**: ImportFromFederated handler (`internal/api/skill_handler.go:608`)

**错误原因**: ImportFromFederated 在尝试获取技能元数据时，使用了错误的 URL 或认证方式，导致 Git 仓库返回 HTTP 401 认证失败。

### 对比分析

| API | 结果 | 说明 |
|-----|------|------|
| ScanFederatedSkills `/api/v1/skills/import/federated/scan` | ✅ 成功 | 扫描联邦源技能列表成功 |
| ImportFromFederated `/api/v1/skills/import/federated` | ❌ HTTP 401 | 单选导入失败 |
| BatchImportFederated `/api/v1/skills/import/federated/batch` | ？未测试 | 批量导入（需验证） |

**关键对比**: ScanFederatedSkills 成功扫描 12 个技能，但 ImportFromFederated 导入单个技能失败。

### 可能原因

1. **ScanFederatedSkills 使用 SkillScanner.ImportSkills**，正确处理了 registryId
2. **ImportFromFederated 可能未正确使用 SkillScanner.ImportSkills**，仍然使用硬编码 URL
3. **ImportFromFederated 在获取技能元数据时，可能直接访问 Git 仓库原始 URL**，未使用 registryId 配置的认证

---

## 调用链分析

### ImportFromFederated 代码流程

```
ImportFromFederated handler (skill_handler.go:608)
  → 获取技能元数据（HTTP 401 失败点）
  → 下载技能内容
  → 创建 Skill 记录
```

**失败点**: "获取技能元数据" 阶段遇到 HTTP 401

### ScanFederatedSkills 代码流程

```
ScanFederatedSkills handler (skill_handler.go:?)
  → scanner.ImportSkills(registryId)
  → git clone Git 仓库 URL（使用 registryId 配置的认证）
  → 扫描技能目录
  → 返回技能列表
```

**成功点**: SkillScanner.ImportSkills 正确使用 registryId

---

## 建议修复方案

### 方案：ImportFromFederated 应使用 SkillScanner.ImportSkills

参考 ScanFederatedSkills 和 BatchImportFederated 的实现方式：

1. ImportFromFederated 应调用 SkillScanner.ImportSkills 获取技能内容
2. 使用 registryId 查询 Git 仓库 URL 和认证配置
3. 不应直接访问硬编码 URL

---

## 测试结论

### ❌ 问题未修复

**核心问题**: ImportFromFederated handler 在获取技能元数据时遇到 HTTP 401 认证失败。

**用户报告的错误仍然存在**: "下载技能失败: 获取技能元数据失败: HTTP 401"

---

## 下一步

需要开发工程师修复 ImportFromFederated handler，使其正确使用 registryId 获取技能内容。

---

**测试完成时间**: 2026-05-06 11:48
**测试状态**: ❌ 问题未修复