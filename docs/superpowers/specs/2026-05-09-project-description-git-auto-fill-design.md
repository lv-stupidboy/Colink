# 项目描述与仓库地址自动填充设计

## 概述

为新建/编辑项目页面增加两个功能：
1. 新建项目时提供描述输入框
2. 选择本地路径后自动从 git 读取仓库地址

## 需求分析

### 现状

| 功能 | 新建表单 | 编辑表单 | 详情展示 |
|------|----------|----------|----------|
| 描述输入 | ❌ 缺失 | ✅ 有 TextArea | ✅ 已显示 |
| 仓库地址 | ❌ 缺失 | ✅ 有 Input | ✅ 已显示 |
| 自动填充 | ❌ 无 | ❌ 无 | - |

**后端状态**：`description` 和 `repositoryUrl` 字段均已存在于模型层和 API 层，无需改动。

### 目标

- 新建 Modal 增加"项目描述"输入框
- 新建/编辑时，选择本地路径后自动填充仓库地址

## 设计方案

### 功能 1：新建项目添加描述输入框

**改动文件**：`web/src/pages/ProjectList.tsx`

**改动内容**：在新建 Modal 表单中增加描述字段

```tsx
<Form.Item name="description" label="项目描述">
  <Input.TextArea rows={3} placeholder="请输入项目描述（可选）" />
</Form.Item>
```

**后端**：无需改动，`CreateProjectRequest` 已支持 `description` 字段。

### 功能 2：自动填充仓库地址

#### 后端 API

**新增接口**：`GET /api/files/git-info?path=<localPath>`

**返回结构**：
```json
{
  "hasGit": true,
  "remoteUrl": "https://github.com/user/repo.git",
  "branch": "main"
}
```

**实现逻辑**：
1. 检查 `path/.git` 目录是否存在
2. 若存在，执行 `git remote get-url origin`（或解析 `.git/config`）
3. 返回结果，无 git 或无 remote 时返回空值

**改动文件**：
- `internal/model/project.go` - 新增 `GitInfoResponse` 结构体
- `internal/api/project_handler.go` - 新增 `GetGitInfo` handler
- `internal/service/project/service.go` - 新增 `GetGitInfo` 方法
- `cmd/server/main.go` - 注册新路由

#### 前端交互

**新建项目**（`ProjectList.tsx`）：
```
handlePathSelect(path)
  ↓
await api.files.getGitInfo(path)
  ↓
if (data.hasGit && data.remoteUrl) {
  form.setFieldsValue({ repositoryUrl: data.remoteUrl })
}
```

**编辑项目**（`ProjectDetail/index.tsx`）：
- 在编辑 Modal 中增加本地路径输入框（当前缺失）
- 路径变更时调用 `getGitInfo` 自动填充

**改动文件**：
- `web/src/pages/ProjectList.tsx` - 调用 getGitInfo
- `web/src/pages/ProjectDetail/index.tsx` - 增加路径字段 + 调用 getGitInfo
- `web/src/api/client.ts` - 新增 `getGitInfo` API 方法

## 实施步骤

1. 后端新增 GitInfo API（model + handler + service + route）
2. 前端 API client 新增 `getGitInfo` 方法
3. ProjectList.tsx 新建 Modal 增加描述 + 自动填充逻辑
4. ProjectDetail/index.tsx 编辑 Modal 增加路径字段 + 自动填充逻辑

## 测试要点

- 新建项目：输入描述后保存，进入详情页能看到描述
- 新建项目：选择有 git 的目录，仓库地址自动填充
- 新建项目：选择无 git 的目录，仓库地址字段为空（可手动填写）
- 编辑项目：修改本地路径后，仓库地址自动更新
- 边界：路径不存在、路径非目录、git 无 remote origin