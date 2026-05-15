# 项目描述与仓库地址自动填充实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为新建/编辑项目页面增加描述输入框，并实现选择本地路径后自动读取 git remote url 填充仓库地址。

**Architecture:** 后端新增 `/api/files/git-info` API，前端在选择路径后调用 API 自动填充。遵循现有分层架构（handler → service → model）。

**Tech Stack:** Go (Gin) + React (Ant Design) + TypeScript

---

## 文件结构

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `internal/model/project.go` | 修改 | 新增 `GitInfoResponse` 结构体 |
| `internal/service/project/service.go` | 修改 | 新增 `GetGitInfo` 方法 |
| `internal/api/project_handler.go` | 修改 | 新增 `GetGitInfo` handler |
| `cmd/server/main.go` | 修改 | 注册新路由 |
| `web/src/api/client.ts` | 修改 | files API 增加 `getGitInfo` 方法 |
| `web/src/pages/ProjectList.tsx` | 修改 | 新建 Modal 增加描述 + 自动填充逻辑 |
| `web/src/pages/ProjectDetail/index.tsx` | 修改 | 编辑 Modal 增加路径字段 + 自动填充逻辑 |

---

### Task 1: 后端新增 GitInfoResponse 结构体

**Files:**
- Modify: `internal/model/project.go:161` (文件末尾追加)

- [ ] **Step 1: 添加 GitInfoResponse 结构体**

在 `internal/model/project.go` 文件末尾添加：

```go
// GitInfoResponse Git 信息响应
type GitInfoResponse struct {
	HasGit    bool   `json:"hasGit"`    // 是否有 git 仓库
	RemoteUrl string `json:"remoteUrl"` // origin remote URL
	Branch    string `json:"branch"`    // 当前分支
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/model/project.go
git commit -m "feat(model): add GitInfoResponse struct for git info API"
```

---

### Task 2: 后端 Service 实现 GetGitInfo 方法

**Files:**
- Modify: `internal/service/project/service.go:634` (文件末尾追加)

- [ ] **Step 1: 实现 GetGitInfo 方法**

在 `internal/service/project/service.go` 文件末尾添加：

```go
// GetGitInfo 获取路径的 Git 信息
func (s *Service) GetGitInfo(ctx context.Context, path string) (*model.GitInfoResponse, error) {
	resp := &model.GitInfoResponse{
		HasGit:    false,
		RemoteUrl: "",
		Branch:    "",
	}

	if path == "" {
		return resp, nil
	}

	// 规范化路径
	path = filepath.Clean(path)

	// Windows 驱动器路径处理
	if runtime.GOOS == "windows" && len(path) == 2 && path[1] == ':' {
		path = path + string(filepath.Separator)
	}

	// 检查 .git 目录是否存在
	gitDir := filepath.Join(path, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return resp, nil
	}

	resp.HasGit = true

	// 解析 .git/config 获取 remote url
	configPath := filepath.Join(gitDir, "config")
	content, err := os.ReadFile(configPath)
	if err != nil {
		return resp, nil
	}

	// 解析 config 文件获取 remote "origin" url
	lines := strings.Split(string(content), "\n")
	inRemoteOrigin := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "[remote \"origin\"]" {
			inRemoteOrigin = true
			continue
		}
		if inRemoteOrigin && strings.HasPrefix(line, "url = ") {
			resp.RemoteUrl = strings.TrimPrefix(line, "url = ")
			break
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") && line != "[remote \"origin\"]" {
			inRemoteOrigin = false
		}
	}

	// 获取当前分支（从 HEAD 文件）
	headPath := filepath.Join(gitDir, "HEAD")
	headContent, err := os.ReadFile(headPath)
	if err == nil {
		headStr := strings.TrimSpace(string(headContent))
		if strings.HasPrefix(headStr, "ref: refs/heads/") {
			resp.Branch = strings.TrimPrefix(headStr, "ref: refs/heads/")
		} else if len(headStr) == 40 {
			// detached HEAD，显示 commit hash 前 7 位
			resp.Branch = headStr[:7]
		}
	}

	return resp, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/service/project/service.go
git commit -m "feat(service): add GetGitInfo method to read git remote from local path"
```

---

### Task 3: 后端 Handler 新增 GetGitInfo 接口

**Files:**
- Modify: `internal/api/project_handler.go:253` (RegisterRoutes 方法中)

- [ ] **Step 1: 新增 GetGitInfo handler 方法**

在 `internal/api/project_handler.go` 的 `RegisterRoutes` 方法前添加：

```go
// GetGitInfo 获取路径的 Git 信息
func (h *ProjectHandler) GetGitInfo(c *gin.Context) {
	path := c.Query("path")
	result, err := h.service.GetGitInfo(c.Request.Context(), path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
```

- [ ] **Step 2: 注册路由**

修改 `RegisterRoutes` 方法，在 `files` group 中添加新路由：

```go
// RegisterRoutes 注册路由
func (h *ProjectHandler) RegisterRoutes(r *gin.RouterGroup) {
	projects := r.Group("/projects")
	{
		projects.GET("", h.List)
		projects.POST("", h.Create)
		// Note: /:id/files must be registered BEFORE /:id to ensure proper matching
		projects.GET("/:id/files", h.ListFiles)
		projects.GET("/:id", h.Get)
		projects.PUT("/:id", h.Update)
		projects.DELETE("/:id", h.Delete)
	}
	// 文件浏览 API
	files := r.Group("/files")
	{
		files.GET("/browse", h.BrowsePath)
		files.GET("/validate", h.ValidatePath)
		files.GET("/git-info", h.GetGitInfo) // 新增
		files.GET("/content", h.GetFileContent)
		files.GET("/image", h.GetFileImage)
		files.POST("/folder", h.CreateFolder)
	}
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/api/project_handler.go
git commit -m "feat(api): add GetGitInfo handler and route for git remote detection"
```

---

### Task 4: 前端 API Client 新增 getGitInfo 方法

**Files:**
- Modify: `web/src/api/client.ts:265` (files 对象中)

- [ ] **Step 1: 在 files API 中添加 getGitInfo 方法**

修改 `web/src/api/client.ts` 的 `files` 对象，在 `getContent` 方法后添加：

```typescript
  // 文件浏览 API（用于路径选择）
  files = {
    // ... 现有方法保持不变 ...
    // 获取文件内容（用于文件预览）
    getContent: (basePath: string, path: string): Promise<{
      content: string;
      size: number;
      truncated: boolean;
      path: string;
      isBinary: boolean;
    }> => {
      const url = `/files/content?basePath=${encodeURIComponent(basePath)}&path=${encodeURIComponent(path)}`;
      return this.request(url, 'GET');
    },
    // 获取路径的 Git 信息（用于自动填充仓库地址）
    getGitInfo: (path: string): Promise<{
      hasGit: boolean;
      remoteUrl: string;
      branch: string;
    }> => {
      return this.request(`/files/git-info?path=${encodeURIComponent(path)}`, 'GET');
    },
  };
```

- [ ] **Step 2: Commit**

```bash
git add web/src/api/client.ts
git commit -m "feat(api): add getGitInfo method to files API client"
```

---

### Task 5: ProjectList 新建 Modal 增加描述输入框

**Files:**
- Modify: `web/src/pages/ProjectList.tsx:185-216` (新建 Modal 表单)

- [ ] **Step 1: 添加描述输入框**

修改 `web/src/pages/ProjectList.tsx` 的新建 Modal 表单，在 `workflowTemplateId` 字段后添加：

```tsx
      <Modal
        title="新建项目"
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
      >
        <Form form={form} layout="vertical" onFinish={handleCreate}>
          <Form.Item name="name" label="项目名称" rules={[{ required: true, message: '请输入项目名称' }]}>
            <Input placeholder="请输入项目名称" autoComplete="off" />
          </Form.Item>
          <Form.Item name="description" label="项目描述">
            <Input.TextArea rows={3} placeholder="请输入项目描述（可选）" />
          </Form.Item>
          <Form.Item
            name="localPath"
            label="本地路径"
            rules={[{ required: true, message: '请选择本地路径' }]}
          >
            <Input
              placeholder="点击选择或输入本地路径"
              addonAfter={
                <Button
                  icon={<FolderOpenOutlined />}
                  onClick={() => setPathSelectorVisible(true)}
                  style={{ border: 'none', background: 'transparent' }}
                >
                  浏览
                </Button>
              }
            />
          </Form.Item>
          {/* ... 后续字段保持不变 ... */}
        </Form>
      </Modal>
```

- [ ] **Step 2: Commit**

```bash
git add web/src/pages/ProjectList.tsx
git commit -m "feat(ui): add description textarea to project creation modal"
```

---

### Task 6: ProjectList 实现自动填充仓库地址

**Files:**
- Modify: `web/src/pages/ProjectList.tsx:62-65` (handlePathSelect 方法)

- [ ] **Step 1: 添加 repositoryUrl 表单字段**

在新建 Modal 表单中添加 repositoryUrl 字段（在 workflowTemplateId 之后）：

```tsx
          <Form.Item name="workflowTemplateId" label="绑定团队" rules={[{ required: true, message: '请选择团队' }]}>
            <Select placeholder="选择Agent团队" loading={workflowTemplates.length === 0}>
              {workflowTemplates.map(t => (
                <Select.Option key={t.id} value={t.id}>
                  {t.name} {t.isDefault ? '(默认)' : ''} {t.isSystem ? '[系统]' : ''}
                </Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name="repositoryUrl" label="仓库地址">
            <Input placeholder="Git 仓库地址（选择路径后自动填充）" />
          </Form.Item>
```

- [ ] **Step 2: 修改 handlePathSelect 方法**

修改 `handlePathSelect` 方法，添加自动填充逻辑：

```tsx
  // 处理路径选择
  const handlePathSelect = async (path: string) => {
    form.setFieldsValue({ localPath: path });
    setPathSelectorVisible(false);
    
    // 自动填充仓库地址
    try {
      const gitInfo = await api.files.getGitInfo(path);
      if (gitInfo.hasGit && gitInfo.remoteUrl) {
        form.setFieldsValue({ repositoryUrl: gitInfo.remoteUrl });
      }
    } catch (error) {
      console.error('获取 Git 信息失败:', error);
    }
  };
```

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/ProjectList.tsx
git commit -m "feat(ui): auto-fill repository URL when selecting local path in project creation"
```

---

### Task 7: ProjectDetail 编辑 Modal 增加本地路径字段

**Files:**
- Modify: `web/src/pages/ProjectDetail/index.tsx:391-432` (编辑 Modal 表单)

- [ ] **Step 1: 添加本地路径输入框**

修改编辑 Modal 表单，在 `name` 字段后添加 `localPath` 字段：

```tsx
      <Modal
        title="编辑项目"
        open={editModalVisible}
        onOk={() => form.submit()}
        onCancel={() => setEditModalVisible(false)}
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={handleUpdateProject}>
          <Form.Item name="name" label="项目名称" rules={[{ required: true }]}>
            <Input placeholder="请输入项目名称" />
          </Form.Item>
          <Form.Item name="description" label="项目描述">
            <Input.TextArea rows={3} placeholder="请输入项目描述" />
          </Form.Item>
          <Form.Item name="localPath" label="本地路径" rules={[{ required: true, message: '请输入本地路径' }]}>
            <Input placeholder="项目本地路径" />
          </Form.Item>
          {/* ... 后续字段保持不变 ... */}
        </Form>
      </Modal>
```

- [ ] **Step 2: 添加 PathSelector 相关状态和组件**

在 `ProjectDetail/index.tsx` 中添加：

```tsx
// 在 state 定义区域添加
const [pathSelectorVisible, setPathSelectorVisible] = useState(false);

// 在 handleUpdateProject 方法后添加
const handlePathSelect = async (path: string) => {
  form.setFieldsValue({ localPath: path });
  setPathSelectorVisible(false);
  
  // 自动填充仓库地址
  try {
    const gitInfo = await api.files.getGitInfo(path);
    if (gitInfo.hasGit && gitInfo.remoteUrl) {
      form.setFieldsValue({ repositoryUrl: gitInfo.remoteUrl });
    }
  } catch (error) {
    console.error('获取 Git 信息失败:', error);
  }
};
```

- [ ] **Step 3: 添加 PathSelector 组件**

在 Modal 区域末尾添加 PathSelector：

```tsx
      {/* 路径选择器 */}
      <PathSelector
        visible={pathSelectorVisible}
        onSelect={handlePathSelect}
        onCancel={() => setPathSelectorVisible(false)}
        title="选择项目本地路径"
      />
```

- [ ] **Step 4: 导入 PathSelector 和 api**

确保顶部有必要的导入（已有 `api` 的导入，需确认 `PathSelector`）：

```tsx
import PathSelector from '@/components/PathSelector';
```

- [ ] **Step 5: 修改 localPath Input 添加浏览按钮**

```tsx
          <Form.Item name="localPath" label="本地路径" rules={[{ required: true, message: '请输入本地路径' }]}>
            <Input
              placeholder="项目本地路径"
              addonAfter={
                <Button
                  icon={<FolderOpenOutlined />}
                  onClick={() => setPathSelectorVisible(true)}
                  style={{ border: 'none', background: 'transparent' }}
                >
                  浏览
                </Button>
              }
            />
          </Form.Item>
```

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/ProjectDetail/index.tsx
git commit -m "feat(ui): add local path field with auto-fill to project edit modal"
```

---

### Task 8: 手动测试验证

**Files:**
- Test: 手动功能测试

- [ ] **Step 1: 启动后端服务**

```bash
go run ./cmd/server
```

Expected: 服务启动成功，监听 26305 端口

- [ ] **Step 2: 启动前端服务**

```bash
cd web && npm run dev
```

Expected: 前端启动成功，监听 26306 端口

- [ ] **Step 3: 测试新建项目描述输入**

1. 打开项目管理页面
2. 点击"新建项目"
3. 输入项目名称和描述
4. 选择本地路径（选择一个有 git 的目录）
5. 验证仓库地址自动填充
6. 点击创建
7. 进入项目详情页，验证描述显示正确

- [ ] **Step 4: 测试编辑项目自动填充**

1. 进入项目详情页
2. 点击"编辑"
3. 修改本地路径为另一个有 git 的目录
4. 验证仓库地址自动更新
5. 点击保存，验证更新成功

- [ ] **Step 5: 测试无 git 目录**

1. 新建项目
2. 选择一个无 git 的空目录
3. 验证仓库地址字段为空
4. 手动输入仓库地址
5. 创建成功

---

## 自审清单

**Spec Coverage:**
- ✅ 新建项目添加描述输入框 → Task 5
- ✅ 自动填充仓库地址（新建） → Task 6
- ✅ 自动填充仓库地址（编辑） → Task 7
- ✅ 后端 GitInfo API → Task 1-3
- ✅ 前端 API 方法 → Task 4

**Placeholder Scan:**
- ✅ 无 TBD/TODO
- ✅ 所有步骤包含完整代码
- ✅ 无模糊描述

**Type Consistency:**
- ✅ `GitInfoResponse` 结构体字段名一致：`hasGit`, `remoteUrl`, `branch`
- ✅ 前端 API 返回类型与后端 JSON 字段一致
- ✅ 表单字段名 `repositoryUrl` 与后端模型一致