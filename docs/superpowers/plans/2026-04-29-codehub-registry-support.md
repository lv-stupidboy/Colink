# CodeHub 联邦源支持实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 扩展联邦技能源支持华为内网 CodeHub 平台，实现 SSH Key 和 HTTPS 账号密码两种认证方式。

**Architecture:** 新增 `codehub` 类型到 RegistryType 枚举，扩展 buildCloneURL 方法支持 SSH/HTTPS URL 处理，前端扩展类型选择和认证配置表单。

**Tech Stack:** Go backend + React frontend + Ant Design

---

## File Structure

### 后端变更
| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/model/skill.go:116-121` | Modify | 新增 RegistryTypeCodeHub 常量 |
| `internal/service/skill/skill_scanner.go:183-221` | Modify | 扩展 buildCloneURL 方法 |

### 前端变更
| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `web/src/types/index.ts:684` | Modify | 扩展 RegistryType 类型 |
| `web/src/pages/RegistryManagement/index.tsx:140-149` | Modify | getTypeTag 新增 codehub |
| `web/src/pages/RegistryManagement/index.tsx:334-340` | Modify | 类型下拉框新增 codehub 选项 |
| `web/src/pages/RegistryManagement/index.tsx` | Modify | 新增 CodeHub 认证配置表单 |

---

### Task 1: 后端 RegistryType 扩展

**Files:**
- Modify: `internal/model/skill.go:116-121`

- [ ] **Step 1: 新增 RegistryTypeCodeHub 常量**

在 `internal/model/skill.go` 第 121 行后添加新常量：

```go
const (
    RegistryTypeGitHub  RegistryType = "github"
    RegistryTypeGitLab  RegistryType = "gitlab"
    RegistryTypeAPI     RegistryType = "api"
    RegistryTypeCustom  RegistryType = "custom"
    RegistryTypeCodeHub RegistryType = "codehub"  // 新增：华为内网 CodeHub
)
```

- [ ] **Step 2: 验证 Go 构建**

Run: `cd D:/workspace/isdp && go build ./cmd/server`
Expected: 构建成功，无编译错误

- [ ] **Step 3: Commit**

```bash
git add internal/model/skill.go
git commit -m "feat: add RegistryTypeCodeHub constant for CodeHub platform support"
```

---

### Task 2: 后端 buildCloneURL 扩展

**Files:**
- Modify: `internal/service/skill/skill_scanner.go:183-221`

- [ ] **Step 1: 扩展 buildCloneURL 方法**

在 `buildCloneURL` 方法的 switch 语句中（第 217-220 行的 default 前）添加 CodeHub case：

```go
case model.RegistryTypeCodeHub:
    // 华为内网 CodeHub 支持 SSH 和 HTTPS 认证
    url := registry.URL

    // SSH 格式：直接使用，依赖系统 SSH Key
    if strings.HasPrefix(url, "git@") {
        return url
    }

    // HTTPS 格式：注入账号密码
    if strings.HasPrefix(url, "https://") {
        username := ""
        password := ""
        if registry.AuthConfig != nil {
            username = registry.AuthConfig["username"]
            password = registry.AuthConfig["password"]
        }

        if username != "" && password != "" {
            // https://{username}:{password}@codehub-g.huawei.com/xxx.git
            url = strings.Replace(url, "https://",
                fmt.Sprintf("https://%s:%s@", username, password), 1)
        }
        return url
    }

    return url
```

- [ ] **Step 2: 验证 Go 构建**

Run: `cd D:/workspace/isdp && go build ./cmd/server`
Expected: 构建成功，无编译错误

- [ ] **Step 3: Commit**

```bash
git add internal/service/skill/skill_scanner.go
git commit -m "feat: extend buildCloneURL to support CodeHub SSH/HTTPS authentication"
```

---

### Task 3: 前端 RegistryType 类型扩展

**Files:**
- Modify: `web/src/types/index.ts:684`

- [ ] **Step 1: 扩展 RegistryType 类型定义**

修改 `web/src/types/index.ts` 第 684 行：

```typescript
// 注册表类型
export type RegistryType = 'github' | 'gitlab' | 'api' | 'custom' | 'codehub';
```

- [ ] **Step 2: 验证 TypeScript 编译**

Run: `cd D:/workspace/isdp/web && npx tsc --noEmit`
Expected: 编译成功，无类型错误

- [ ] **Step 3: Commit**

```bash
git add web/src/types/index.ts
git commit -m "feat: add codehub to RegistryType type definition"
```

---

### Task 4: 前端 getTypeTag 扩展

**Files:**
- Modify: `web/src/pages/RegistryManagement/index.tsx:140-149`

- [ ] **Step 1: 扩展 getTypeTag 函数**

修改 `getTypeTag` 函数（第 140-149 行），添加 codehub 配置：

```typescript
const getTypeTag = (type: RegistryType) => {
  const typeConfig: Record<RegistryType, { color: string; text: string }> = {
    github: { color: 'blue', text: 'GitHub' },
    gitlab: { color: 'orange', text: 'GitLab' },
    api: { color: 'green', text: 'API' },
    custom: { color: 'purple', text: '自定义' },
    codehub: { color: 'cyan', text: 'CodeHub' },
  };
  const config = typeConfig[type] || { color: 'default', text: type };
  return <Tag color={config.color}>{config.text}</Tag>;
};
```

- [ ] **Step 2: 验证 TypeScript 编译**

Run: `cd D:/workspace/isdp/web && npx tsc --noEmit`
Expected: 编译成功，无类型错误

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/RegistryManagement/index.tsx
git commit -m "feat: add codehub type tag display in RegistryManagement"
```

---

### Task 5: 前端类型下拉框扩展

**Files:**
- Modify: `web/src/pages/RegistryManagement/index.tsx:334-340`

- [ ] **Step 1: 新增 CodeHub 类型选项**

修改类型下拉框（第 334-340 行），添加 codehub 选项：

```typescript
<Form.Item name="type" label="类型" rules={[{ required: true }]}>
  <Select disabled={!!editingRegistry}>
    <Option value="github">GitHub</Option>
    <Option value="gitlab">GitLab</Option>
    <Option value="api">API</Option>
    <Option value="custom">自定义</Option>
    <Option value="codehub">CodeHub 代码托管服务</Option>
  </Select>
</Form.Item>
```

- [ ] **Step 2: 验证 TypeScript 编译**

Run: `cd D:/workspace/isdp/web && npx tsc --noEmit`
Expected: 编译成功，无类型错误

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/RegistryManagement/index.tsx
git commit -m "feat: add codehub option to registry type selector"
```

---

### Task 6: 前端 CodeHub 认证配置表单

**Files:**
- Modify: `web/src/pages/RegistryManagement/index.tsx`

- [ ] **Step 1: 新增类型监听状态**

在组件状态定义区域（第 43 行后）添加类型监听状态：

```typescript
const [selectedType, setSelectedType] = useState<RegistryType>('github');
```

- [ ] **Step 2: 修改类型下拉框添加 onChange**

修改类型下拉框（第 333-340 行），添加 onChange 事件：

```typescript
<Form.Item name="type" label="类型" rules={[{ required: true }]}>
  <Select
    disabled={!!editingRegistry}
    onChange={(value) => setSelectedType(value as RegistryType)}
  >
    <Option value="github">GitHub</Option>
    <Option value="gitlab">GitLab</Option>
    <Option value="api">API</Option>
    <Option value="custom">自定义</Option>
    <Option value="codehub">CodeHub 代码托管服务</Option>
  </Select>
</Form.Item>
```

- [ ] **Step 3: 新增 CodeHub 认证配置表单**

在 URL 表单项后（第 343 行后）添加 CodeHub 认证配置表单：

```typescript
{selectedType === 'codehub' && (
  <>
    <Form.Item
      name={['authConfig', 'username']}
      label="用户名"
      extra="HTTPS 认证账号（SSH 格式 URL 可不填）"
    >
      <Input placeholder="CodeHub 用户名" />
    </Form.Item>
    <Form.Item
      name={['authConfig', 'password']}
      label="密码"
      extra="HTTPS 认证密码（SSH 格式 URL 可不填）"
    >
      <Input.Password placeholder="CodeHub 密码" />
    </Form.Item>
    <div style={{ marginBottom: 16, padding: '8px 12px', background: '#f5f5f5', borderRadius: 4 }}>
      <Text type="secondary">
        SSH 格式 URL 将使用系统全局 SSH Key 认证，无需配置账号密码
      </Text>
    </div>
  </>
)}
```

- [ ] **Step 4: 修改 handleEdit 函数**

修改 `handleEdit` 函数（第 71-82 行），添加 authConfig 和 type 的初始化：

```typescript
const handleEdit = (registry: SkillRegistry) => {
  setEditingRegistry(registry);
  setSelectedType(registry.type);
  form.setFieldsValue({
    name: registry.name,
    displayName: registry.displayName,
    type: registry.type,
    url: registry.url,
    syncInterval: registry.syncInterval,
    status: registry.status,
    authConfig: registry.authConfig || {},
  });
  setModalVisible(true);
};
```

- [ ] **Step 5: 修改 handleCreate 函数**

修改 `handleCreate` 函数（第 65-69 行），添加类型初始化：

```typescript
const handleCreate = () => {
  setEditingRegistry(null);
  setSelectedType('github');
  form.resetFields();
  setModalVisible(true);
};
```

- [ ] **Step 6: 验证 TypeScript 编译**

Run: `cd D:/workspace/isdp/web && npx tsc --noEmit`
Expected: 编译成功，无类型错误

- [ ] **Step 7: Commit**

```bash
git add web/src/pages/RegistryManagement/index.tsx
git commit -m "feat: add CodeHub authentication config form (username/password)"
```

---

### Task 7: 集成验证

**Files:**
- None (验证任务)

- [ ] **Step 1: 启动后端服务**

Run: `cd D:/workspace/isdp && go run ./cmd/server`
Expected: 服务正常启动，监听配置端口

- [ ] **Step 2: 启动前端开发服务器**

Run: `cd D:/workspace/isdp/web && npm run dev`
Expected: 前端正常启动

- [ ] **Step 3: 验证前端 UI**

在浏览器中访问联邦源管理页面：
- 新建注册表弹窗应显示 "CodeHub 代码托管服务" 类型选项
- 选择 CodeHub 类型后应显示用户名、密码输入框
- 提示信息应显示 "SSH 格式 URL 将使用系统全局 SSH Key 认证"

- [ ] **Step 4: Final Commit (如果有遗漏变更)**

```bash
git status
# 如有未提交变更，执行：
git add -A
git commit -m "feat: complete CodeHub registry support implementation"
```

---

## Self-Review Checklist

**1. Spec Coverage:**
- ✅ Task 1: 后端 RegistryType 扩展（规格 1.1）
- ✅ Task 2: 后端 buildCloneURL 逻辑（规格 2）
- ✅ Task 3: 前端 RegistryType 类型扩展（规格 1.3）
- ✅ Task 4: 前端 getTypeTag 扩展（规格 3.1）
- ✅ Task 5: 前端类型下拉框扩展（规格 3.1）
- ✅ Task 6: 前端认证配置表单（规格 3.2-3.4）
- ✅ Task 7: 集成验证（规格 测试要点）

**2. Placeholder Scan:**
- ✅ 无 TBD/TODO
- ✅ 无 "类似 Task N" 引用
- ✅ 所有代码步骤包含完整代码

**3. Type Consistency:**
- ✅ `RegistryTypeCodeHub` 在后端 Task 1 定义，Task 2 使用
- ✅ `'codehub'` 在前端 Task 3 定义，Task 4-6 使用
- ✅ `selectedType` 状态在 Task 6 定义并使用