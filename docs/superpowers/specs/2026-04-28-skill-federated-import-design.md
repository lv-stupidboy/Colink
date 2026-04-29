# Skill 联邦源导入功能设计规格

## 概述

支持从外部联邦源（代码仓库）拉取 Skill 的能力，用户可以在新建 Skill 时选择联邦源，扫描仓库中的 Skill 列表，批量导入到本地。

## 需求确认

| 项目 | 决策 |
|------|------|
| Skill 识别规则 | 混合模式（支持单 Skill 仓库和多 Skill 仓库） |
| 导入关系 | 半复制（复制内容到本地，保留来源信息） |
| 认证支持 | 公开仓库 + GitHub Token + GitLab Token（扩展） |
| 实现方案 | Git Clone + 本地扫描 |
| 多选处理 | 表格批量编辑模式 |

---

## Part 1: 整体架构

### 用户交互流程

```
新建 Skill → 选择来源"联邦" → 点击"联邦源下载" → 
选择联邦源 → 点击"导入" → 
[后端: git clone → 扫描 SKILL.md → 返回列表] → 
弹窗展示 Skill 列表 → 用户勾选 → 关闭弹窗 → 
主表单切换为表格模式 → 用户设置属性 → 点击保存 → 完成
```

### 核心模块划分

| 模块 | 职责 | 现有/新增 |
|------|------|-----------|
| 前端 SkillLibrary | 弹窗展示 Skill 列表、勾选、表格批量编辑 | 新增弹窗组件、表格模式 |
| 前端 API Client | 调用扫描 API、批量导入 API | 新增方法 |
| 后端 SkillHandler | 扫描联邦源、批量导入 Skill | 新增 API |
| 后端 SkillScanner | Git Clone + 并发扫描 SKILL.md | 新增模块 |
| 后端 RegistryService | GitLab Token 支持 | 扩展方法 |

### API 设计

#### 1. 扫描联邦源 API

```
POST /api/v1/skills/import/federated/scan
Request: { registryId: string }
Response: { 
  skills: [
    { 
      name: string, 
      description: string, 
      path: string,  // Skill 在仓库中的相对路径
      existsLocally: boolean  // 是否已存在本地
    }
  ],
  registryName: string,
  registryUrl: string
}
```

#### 2. 批量导入 API

```
POST /api/v1/skills/import/federated/batch
Request: { 
  registryId: string,
  skills: [
    {
      name: string,
      path: string,
      description: string,
      tags: string[],
      supportedAgents: string[]
    }
  ]
}
Response: { 
  imported: [Skill],
  skipped: [{ name: string, reason: string }]
}
```

---

## Part 2: 后端实现细节

### Git Clone + Token 注入

根据联邦源类型，注入 Token 到 git clone URL：

| RegistryType | URL 格式 |
|--------------|----------|
| github（有 Token） | `https://{token}@github.com/owner/repo.git` |
| gitlab（有 Token） | `https://oauth2:{token}@gitlab.com/owner/repo.git` |
| 公开仓库 | 直接使用原 URL |

### 临时目录管理

- 目录路径：`{storagePath}/.temp/{uuid}`
- 扫描完成后异步删除（使用后台 goroutine，避免阻塞响应）

### Skill 扫描逻辑（并发优化）

使用 goroutine pool 进行并发扫描（限制 10 个并发）：

```go
func ScanSkillsConcurrent(repoDir string) []SkillInfo {
    // 1. 快速遍历找出所有包含 SKILL.md 的目录
    skillDirs := findSkillDirectories(repoDir)
    
    // 2. 创建并发池（限制 10 个并发）
    pool := make(chan struct{}, 10)
    results := make(chan SkillInfo, len(skillDirs))
    
    // 3. 并发解析每个 SKILL.md
    for _, dir := range skillDirs {
        pool <- struct{}{}  // 获取槽位
        go func(d string) {
            defer func() { <-pool }()  // 释放槽位
            results <- parseSkill(d)
        }(dir)
    }
    
    // 4. 收集结果
    skills := []SkillInfo{}
    for i := 0; i < len(skillDirs); i++ {
        skills = append(skills, <-results)
    }
    return skills
}
```

扫描步骤：
1. 首先快速遍历仓库目录，找出所有包含 `SKILL.md` 的目录路径
2. 使用 goroutine pool（限制 10 并发）并发解析每个 SKILL.md
3. 收集所有解析结果返回

### SKILL.md 解析规则

复用现有 `parseSkillMD` 函数：
- 从 YAML front matter 提取 name、description
- 备选：从 `# 标题` 提取 name，从 `## Description` 提取 description

### GitLab Token 支持扩展

在 `RegistryService` 中实现 `syncFromGitLab`：

```go
func syncFromGitLab(ctx, registry) ([]RemoteSkill, error) {
    // GitLab API: GET /api/v4/projects/{id}/repository/tree
    // 认证: Header "PRIVATE-TOKEN: {token}"
}
```

---

## Part 3: 前端实现细节

### Skill 选择弹窗组件

新增 `SkillSelectModal` 组件：

```tsx
interface SkillSelectModalProps {
  visible: boolean;
  registryId: string;
  onConfirm: (selectedSkills: RemoteSkill[]) => void;
  onCancel: () => void;
}

interface RemoteSkill {
  name: string;
  description: string;
  path: string;
  existsLocally: boolean;
}
```

### 弹窗 UI 设计

```
┌─────────────────────────────────────────────────────┐
│ 从联邦源导入 Skill                              [×] │
├─────────────────────────────────────────────────────┤
│ 联邦源：my-github-skills                            │
│ URL：https://github.com/owner/skills-repo           │
│                                                     │
│ ┌──────────────────────────────────────────────────┤
│ │ [✓] java-coding-standards     已用于 3 个项目    │
│ │     Java 代码规范检查技能                         │
│ │                                                 │
│ │ [✓] python-security-audit     已用于 1 个项目    │
│ │     Python 安全审计技能                          │
│ │                                                 │
│ │ [ ] react-best-practices      (已存在本地)       │
│ │     React 最佳实践技能                           │
│ └───────────────────────────────────────────────────┤
│                                                     │
│ 已选择 2 个 Skill                                   │
│                                                     │
│                    [取消]      [确认导入]           │
└─────────────────────────────────────────────────────┘
```

### 表格批量编辑模式

当用户选择多个 Skill 后，主表单切换为表格模式：

```
┌─────────────────────────────────────────────────────────────┐
│ 新建 Skill - 批量导入                                  [×] │
├─────────────────────────────────────────────────────────────┤
│ 来源：联邦                                                  │
│ 联邦源：my-github-skills                                    │
│                                                             │
│ 已选择 3 个 Skill，请补充属性信息：                          │
│                                                             │
│ ┌───────────────────────────────────────────────────────────┤
│ │ 名称          │ 描述               │ 标签     │ Agent    │
│ ├───────────────────────────────────────────────────────────┤
│ │ java-coding   │ Java代码规范检查    │ [选择]  │ [选择]   │
│ │ standards     │                    │         │          │
│ ├───────────────────────────────────────────────────────────┤
│ │ python-       │ Python安全审计      │ [选择]  │ [选择]   │
│ │ security      │                    │         │          │
│ ├───────────────────────────────────────────────────────────┤
│ │ react-best    │ React最佳实践       │ [选择]  │ [选择]   │
│ │ practices     │                    │         │          │
│ └───────────────────────────────────────────────────────────┘
│                                                             │
│  ☑ 统一设置：为所有 Skill 应用相同的标签和 Agent            │
│     标签：[________________]                                │
│     Agent：[________________]                               │
│                                                             │
│                          [取消]        [保存全部]           │
└─────────────────────────────────────────────────────────────┘
```

### 表格编辑功能

- **单独设置**：每行 Skill 可单独设置 tags、supportedAgents
- **统一设置**：勾选"统一设置"后，为所有 Skill 应用相同的属性
- **名称/描述**：从联邦源解析，用户可修改
- **必填项**：Agent 必选（保留现有校验规则）

### 单选 Skill 的处理

如果用户只选择 1 个 Skill，保持原有表单模式（不切换为表格），简化交互。

### 交互流程完整版

```
新建 Skill → 来源"联邦" → 点击"联邦源下载" → 
选择联邦源 → 点击"导入" → 
[后端扫描] → 弹窗展示列表 → 
用户勾选 Skill → 点击"确认导入" →
关闭弹窗 → 
  - 单选 → 原有表单模式
  - 多选 → 表格批量编辑模式
→
用户设置属性 → 点击"保存" → 批量创建 Skill → 刷新列表
```

---

## 数据模型

### RemoteSkill（扫描结果）

```go
type RemoteSkill struct {
    Name            string   `json:"name"`
    Description     string   `json:"description"`
    Path            string   `json:"path"`            // Skill 在仓库中的相对路径
    ExistsLocally   bool     `json:"existsLocally"`   // 是否已存在本地同名 Skill
}
```

### BatchImportRequest（批量导入请求）

```go
type BatchImportRequest struct {
    RegistryID string                `json:"registryId" binding:"required"`
    Skills     []SkillImportItem     `json:"skills" binding:"required,min=1"`
}

type SkillImportItem struct {
    Name            string   `json:"name" binding:"required"`
    Path            string   `json:"path" binding:"required"`
    Description     string   `json:"description"`
    Tags            []string `json:"tags"`
    SupportedAgents []string `json:"supportedAgents" binding:"required,min=1"`
}
```

---

## 文件变更范围

### 后端新增/修改文件

| 文件 | 变更 |
|------|------|
| `internal/api/skill_handler.go` | 新增 `ScanFederatedSkills`、`BatchImportFederated` API |
| `internal/service/skill/skill_scanner.go` | 新建文件：并发扫描逻辑 |
| `internal/service/skill/registry_service.go` | 扩展 `syncFromGitLab` 方法 |
| `internal/model/skill.go` | 新增 `RemoteSkill`、`BatchImportRequest` 结构体 |

### 前端新增/修改文件

| 文件 | 变更 |
|------|------|
| `web/src/pages/SkillLibrary/index.tsx` | 新增弹窗逻辑、表格批量编辑模式 |
| `web/src/components/SkillSelectModal.tsx` | 新建文件：Skill 选择弹窗组件 |
| `web/src/api/client.ts` | 新增 `scanFederatedSkills`、`batchImportFederated` 方法 |
| `web/src/types/index.ts` | 新增 `RemoteSkill` 类型定义 |

---

## 测试要点

### 后端测试

1. Git Clone 成功/失败场景
2. Token 注入正确性（GitHub/GitLab）
3. 并发扫描性能（10+ Skill 仓库）
4. SKILL.md 解析准确性（front matter / 标题解析）
5. 已存在 Skill 的跳过逻辑
6. 临时目录清理

### 前端测试

1. 弹窗列表展示正确性
2. 多选/单选交互区分
3. 表格批量编辑功能
4. 统一设置功能
5. 必填项校验（Agent）
6. 深色模式适配

---

## 修订历史

| 版本 | 日期 | 变更 |
|------|------|------|
| v1.0 | 2026-04-28 | 初版设计规格 |