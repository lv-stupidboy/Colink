# 项目绑定工作流功能实现计划

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在项目设置中支持绑定工作流，任务开发时自动使用绑定的工作流。

**Architecture:** 数据模型新增字段，后端新增 API 端点和校验逻辑，前端在项目设置页面增加工作流绑定 UI，工作流页面增加默认标记功能。

**Tech Stack:** Go (Gin), SQLite, React, TypeScript, Ant Design

---

## Chunk 1: 后端数据模型与数据库迁移

### Task 1: 更新数据模型

**Files:**
- Modify: `isdp/internal/model/project.go`
- Modify: `isdp/internal/model/workflow_template.go`
- Modify: `isdp/internal/model/thread.go`

- [ ] **Step 1: 更新 Project 模型**

在 `isdp/internal/model/project.go` 的 Project 结构体中添加字段：

```go
// Project 项目模型
type Project struct {
	ID                  uuid.UUID       `json:"id"`
	Name                string          `json:"name"`
	Type                ProjectType     `json:"type"`
	Mode                ProjectMode     `json:"mode"`
	Status              ProjectStatus   `json:"status"`
	GitRepo             string          `json:"git_repo,omitempty"`
	Config              json.RawMessage `json:"config,omitempty"`
	WorkflowTemplateID  *uuid.UUID      `json:"workflow_template_id,omitempty"` // 新增
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}
```

- [ ] **Step 2: 更新 WorkflowTemplate 模型**

在 `isdp/internal/model/workflow_template.go` 的 WorkflowTemplate 结构体中添加字段：

```go
// WorkflowTemplate 工作流模板
type WorkflowTemplate struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	AgentIDs      json.RawMessage `json:"agent_ids"`
	Checkpoints   json.RawMessage `json:"checkpoints"`
	EstimatedTime string          `json:"estimated_time"`
	IsSystem      bool            `json:"is_system"`
	IsDefault     bool            `json:"is_default"` // 新增
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}
```

- [ ] **Step 3: 更新 Thread 模型**

在 `isdp/internal/model/thread.go` 的 Thread 结构体中添加字段：

```go
// Thread 开发会话模型
type Thread struct {
	ID                  uuid.UUID    `json:"id"`
	ProjectID           uuid.UUID    `json:"project_id"`
	Status              ThreadStatus `json:"status"`
	CurrentPhase        Phase        `json:"current_phase"`
	CurrentAgent        string       `json:"current_agent"`
	Depth               int          `json:"depth"`
	AbortToken          *string      `json:"abort_token,omitempty"`
	WorkflowTemplateID  *uuid.UUID   `json:"workflow_template_id,omitempty"` // 新增
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
}
```

- [ ] **Step 4: 添加 UpdateProjectRequest 类型**

在 `isdp/internal/model/project.go` 中添加请求类型：

```go
// UpdateProjectRequest 更新项目请求
type UpdateProjectRequest struct {
	Name              *string      `json:"name"`
	Type              *ProjectType `json:"type"`
	Mode              *ProjectMode `json:"mode"`
	Status            *ProjectStatus `json:"status"`
	GitRepo           *string      `json:"git_repo"`
	WorkflowTemplateID *uuid.UUID  `json:"workflow_template_id"`
}
```

- [ ] **Step 5: 提交数据模型变更**

```bash
cd D:/Tools/ASDP_gittee/isdp
git add isdp/internal/model/project.go isdp/internal/model/workflow_template.go isdp/internal/model/thread.go
git commit -m "feat(models): 添加工作流绑定相关字段

- Project 新增 WorkflowTemplateID 字段
- WorkflowTemplate 新增 IsDefault 字段
- Thread 新增 WorkflowTemplateID 字段
- 新增 UpdateProjectRequest 类型

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

### Task 2: 更新数据库初始化与迁移

**Files:**
- Modify: `isdp/cmd/server/main.go`

- [ ] **Step 1: 更新 initDatabase 函数中的 schema**

在 `isdp/cmd/server/main.go` 的 `initDatabase` 函数中，修改表结构：

```go
// initDatabase 初始化数据库表结构
func initDatabase(db *sql.DB) error {
	schema := `
-- 项目表
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    mode TEXT NOT NULL,
    status TEXT DEFAULT 'draft',
    git_repo TEXT,
    config TEXT,
    workflow_template_id TEXT,  -- 新增
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 开发会话表
CREATE TABLE IF NOT EXISTS threads (
    id TEXT PRIMARY KEY,
    project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
    status TEXT DEFAULT 'idle',
    current_phase TEXT,
    current_agent TEXT,
    depth INTEGER DEFAULT 0,
    abort_token TEXT,
    workflow_template_id TEXT,  -- 新增
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ... 其他表保持不变 ...

-- 工作流模板表
CREATE TABLE IF NOT EXISTS workflow_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    agent_ids TEXT DEFAULT '[]',
    checkpoints TEXT DEFAULT '[]',
    estimated_time TEXT,
    is_system INTEGER DEFAULT 0,
    is_default INTEGER DEFAULT 0,  -- 新增
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_threads_project_id ON threads(project_id);
CREATE INDEX IF NOT EXISTS idx_messages_thread_id ON messages(thread_id);
CREATE INDEX IF NOT EXISTS idx_agent_invocations_thread_id ON agent_invocations(thread_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_thread_id ON artifacts(thread_id);
`
	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	// 迁移：检查并添加新列
	migrations := []struct {
		table  string
		column string
		typ    string
	}{
		{"projects", "workflow_template_id", "TEXT"},
		{"threads", "workflow_template_id", "TEXT"},
		{"workflow_templates", "is_default", "INTEGER DEFAULT 0"},
	}

	for _, m := range migrations {
		var count int
		row := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name='%s'", m.table, m.column))
		if err := row.Scan(&count); err == nil && count == 0 {
			_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", m.table, m.column, m.typ))
			if err != nil {
				fmt.Printf("Warning: could not add %s column to %s: %v\n", m.column, m.table, err)
			}
		}
	}

	return nil
}
```

- [ ] **Step 2: 提交数据库迁移变更**

```bash
cd D:/Tools/ASDP_gittee/isdp
git add isdp/cmd/server/main.go
git commit -m "feat(db): 添加工作流绑定相关列和迁移逻辑

- projects 表新增 workflow_template_id 列
- threads 表新增 workflow_template_id 列
- workflow_templates 表新增 is_default 列
- 添加自动迁移检测逻辑

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Chunk 2: 后端 Repository 层变更

### Task 3: 更新 WorkflowTemplateRepository

**Files:**
- Modify: `isdp/internal/repo/workflow_template.go`

- [ ] **Step 1: 更新 Create 方法**

修改 `isdp/internal/repo/workflow_template.go` 中的 Create 方法：

```go
// Create 创建工作流模板
func (r *WorkflowTemplateRepository) Create(ctx context.Context, template *model.WorkflowTemplate) error {
	query := `
		INSERT INTO workflow_templates (id, name, description, agent_ids, checkpoints, estimated_time, is_system, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	var isDefault int
	if template.IsDefault {
		isDefault = 1
	}
	_, err := r.db.ExecContext(ctx, query,
		template.ID.String(),
		template.Name,
		template.Description,
		[]byte(template.AgentIDs),
		[]byte(template.Checkpoints),
		template.EstimatedTime,
		template.IsSystem,
		isDefault,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to create workflow template: %w", err)
	}
	template.CreatedAt = now
	template.UpdatedAt = now
	return nil
}
```

- [ ] **Step 2: 更新 FindByID 方法**

修改 FindByID 方法以读取 is_default 字段：

```go
// FindByID 根据ID查找工作流模板
func (r *WorkflowTemplateRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.WorkflowTemplate, error) {
	query := `
		SELECT id, name, description, agent_ids, checkpoints, estimated_time, is_system, is_default, created_at, updated_at
		FROM workflow_templates WHERE id = ?
	`
	template := &model.WorkflowTemplate{}
	var idStr string
	var agentIDs, checkpoints []byte
	var isSystem, isDefault int
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr,
		&template.Name,
		&template.Description,
		&agentIDs,
		&checkpoints,
		&template.EstimatedTime,
		&isSystem,
		&isDefault,
		&template.CreatedAt,
		&template.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow template: %w", err)
	}
	template.ID, _ = uuid.Parse(idStr)
	template.AgentIDs = json.RawMessage(agentIDs)
	template.Checkpoints = json.RawMessage(checkpoints)
	template.IsSystem = isSystem == 1
	template.IsDefault = isDefault == 1
	return template, nil
}
```

- [ ] **Step 3: 更新 FindAll 方法**

修改 FindAll 方法以读取 is_default 字段：

```go
// FindAll 查找所有工作流模板
func (r *WorkflowTemplateRepository) FindAll(ctx context.Context) ([]*model.WorkflowTemplate, error) {
	query := `
		SELECT id, name, description, agent_ids, checkpoints, estimated_time, is_system, is_default, created_at, updated_at
		FROM workflow_templates ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow templates: %w", err)
	}
	defer rows.Close()

	var templates []*model.WorkflowTemplate
	for rows.Next() {
		template := &model.WorkflowTemplate{}
		var idStr string
		var agentIDs, checkpoints []byte
		var isSystem, isDefault int
		err := rows.Scan(
			&idStr,
			&template.Name,
			&template.Description,
			&agentIDs,
			&checkpoints,
			&template.EstimatedTime,
			&isSystem,
			&isDefault,
			&template.CreatedAt,
			&template.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workflow template: %w", err)
		}
		template.ID, _ = uuid.Parse(idStr)
		template.AgentIDs = json.RawMessage(agentIDs)
		template.Checkpoints = json.RawMessage(checkpoints)
		template.IsSystem = isSystem == 1
		template.IsDefault = isDefault == 1
		templates = append(templates, template)
	}
	return templates, nil
}
```

- [ ] **Step 4: 添加 GetDefault 方法**

在文件末尾添加新方法：

```go
// GetDefault 获取默认工作流模板
func (r *WorkflowTemplateRepository) GetDefault(ctx context.Context) (*model.WorkflowTemplate, error) {
	query := `
		SELECT id, name, description, agent_ids, checkpoints, estimated_time, is_system, is_default, created_at, updated_at
		FROM workflow_templates WHERE is_default = 1 LIMIT 1
	`
	template := &model.WorkflowTemplate{}
	var idStr string
	var agentIDs, checkpoints []byte
	var isSystem, isDefault int
	err := r.db.QueryRowContext(ctx, query).Scan(
		&idStr,
		&template.Name,
		&template.Description,
		&agentIDs,
		&checkpoints,
		&template.EstimatedTime,
		&isSystem,
		&isDefault,
		&template.CreatedAt,
		&template.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("no default workflow template found: %w", err)
	}
	template.ID, _ = uuid.Parse(idStr)
	template.AgentIDs = json.RawMessage(agentIDs)
	template.Checkpoints = json.RawMessage(checkpoints)
	template.IsSystem = isSystem == 1
	template.IsDefault = isDefault == 1
	return template, nil
}
```

- [ ] **Step 5: 添加 SetDefault 方法**

```go
// SetDefault 设置默认工作流模板
func (r *WorkflowTemplateRepository) SetDefault(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. 清除所有工作流的默认标记
	_, err = tx.ExecContext(ctx, "UPDATE workflow_templates SET is_default = 0")
	if err != nil {
		return fmt.Errorf("failed to clear default flags: %w", err)
	}

	// 2. 设置指定工作流为默认
	result, err := tx.ExecContext(ctx, "UPDATE workflow_templates SET is_default = 1 WHERE id = ?", id.String())
	if err != nil {
		return fmt.Errorf("failed to set default: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("workflow template not found: %s", id.String())
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
```

- [ ] **Step 6: 添加 CountProjectReferences 方法**

```go
// CountProjectReferences 统计引用该工作流的项目数量
func (r *WorkflowTemplateRepository) CountProjectReferences(ctx context.Context, id uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM projects WHERE workflow_template_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count project references: %w", err)
	}
	return count, nil
}
```

- [ ] **Step 7: 提交 Repository 变更**

```bash
cd D:/Tools/ASDP_gittee/isdp
git add isdp/internal/repo/workflow_template.go
git commit -m "feat(repo): 工作流模板 Repository 支持默认工作流

- 更新 CRUD 方法支持 is_default 字段
- 新增 GetDefault 方法获取默认工作流
- 新增 SetDefault 方法设置默认工作流（事务保护）
- 新增 CountProjectReferences 方法统计项目引用

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

### Task 4: 更新 ProjectRepository

**Files:**
- Modify: `isdp/internal/repo/project.go`

- [ ] **Step 1: 更新 Create 方法**

修改 Create 方法以支持 workflow_template_id：

```go
// Create 创建项目
func (r *ProjectRepository) Create(ctx context.Context, project *model.Project) error {
	query := `
		INSERT INTO projects (id, name, type, mode, status, git_repo, config, workflow_template_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	var workflowTemplateID interface{}
	if project.WorkflowTemplateID != nil {
		workflowTemplateID = project.WorkflowTemplateID.String()
	}
	_, err := r.db.ExecContext(ctx, query,
		project.ID.String(),
		project.Name,
		project.Type,
		project.Mode,
		project.Status,
		project.GitRepo,
		project.Config,
		workflowTemplateID,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	project.CreatedAt = now
	project.UpdatedAt = now
	return nil
}
```

- [ ] **Step 2: 更新 FindByID 方法**

修改 FindByID 方法以读取 workflow_template_id：

```go
// FindByID 根据ID查找项目
func (r *ProjectRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	query := `
		SELECT id, name, type, mode, status, git_repo, config, workflow_template_id, created_at, updated_at
		FROM projects WHERE id = ?
	`
	project := &model.Project{}
	var idStr string
	var config []byte
	var workflowTemplateID sql.NullString
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr,
		&project.Name,
		&project.Type,
		&project.Mode,
		&project.Status,
		&project.GitRepo,
		&config,
		&workflowTemplateID,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find project: %w", err)
	}
	project.ID, _ = uuid.Parse(idStr)
	if config != nil {
		project.Config = config
	}
	if workflowTemplateID.Valid {
		wid, _ := uuid.Parse(workflowTemplateID.String)
		project.WorkflowTemplateID = &wid
	}
	return project, nil
}
```

- [ ] **Step 3: 更新 FindAll 方法**

修改 FindAll 方法：

```go
// FindAll 查找所有项目
func (r *ProjectRepository) FindAll(ctx context.Context, limit, offset int) ([]*model.Project, error) {
	query := `
		SELECT id, name, type, mode, status, git_repo, config, workflow_template_id, created_at, updated_at
		FROM projects ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to find projects: %w", err)
	}
	defer rows.Close()

	var projects []*model.Project
	for rows.Next() {
		project := &model.Project{}
		var idStr string
		var config []byte
		var workflowTemplateID sql.NullString
		err := rows.Scan(
			&idStr,
			&project.Name,
			&project.Type,
			&project.Mode,
			&project.Status,
			&project.GitRepo,
			&config,
			&workflowTemplateID,
			&project.CreatedAt,
			&project.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		project.ID, _ = uuid.Parse(idStr)
		if config != nil {
			project.Config = config
		}
		if workflowTemplateID.Valid {
			wid, _ := uuid.Parse(workflowTemplateID.String)
			project.WorkflowTemplateID = &wid
		}
		projects = append(projects, project)
	}
	return projects, nil
}
```

- [ ] **Step 4: 更新 Update 方法**

修改 Update 方法：

```go
// Update 更新项目
func (r *ProjectRepository) Update(ctx context.Context, project *model.Project) error {
	query := `
		UPDATE projects
		SET name = ?, type = ?, mode = ?, status = ?, git_repo = ?, config = ?, workflow_template_id = ?, updated_at = ?
		WHERE id = ?
	`
	project.UpdatedAt = time.Now()
	var workflowTemplateID interface{}
	if project.WorkflowTemplateID != nil {
		workflowTemplateID = project.WorkflowTemplateID.String()
	}
	_, err := r.db.ExecContext(ctx, query,
		project.Name,
		project.Type,
		project.Mode,
		project.Status,
		project.GitRepo,
		project.Config,
		workflowTemplateID,
		project.UpdatedAt,
		project.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: 提交 ProjectRepository 变更**

```bash
cd D:/Tools/ASDP_gittee/isdp
git add isdp/internal/repo/project.go
git commit -m "feat(repo): Project Repository 支持工作流绑定

- Create/Update 方法支持 workflow_template_id
- 查询方法读取 workflow_template_id 字段

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

### Task 5: 更新 ThreadRepository

**Files:**
- Modify: `isdp/internal/repo/thread.go`

- [ ] **Step 1: 更新 Create 方法**

修改 `isdp/internal/repo/thread.go` 中的 Create 方法以支持 workflow_template_id：

```go
// Create 创建Thread
func (r *ThreadRepository) Create(ctx context.Context, thread *model.Thread) error {
	query := `
		INSERT INTO threads (id, project_id, status, current_phase, current_agent, depth, workflow_template_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	var workflowTemplateID interface{}
	if thread.WorkflowTemplateID != nil {
		workflowTemplateID = thread.WorkflowTemplateID.String()
	}
	_, err := r.db.ExecContext(ctx, query,
		thread.ID.String(), thread.ProjectID.String(), thread.Status, thread.CurrentPhase, thread.CurrentAgent, thread.Depth, workflowTemplateID, now, now,
	)
	if err != nil {
		return fmt.Errorf("failed to create thread: %w", err)
	}
	thread.CreatedAt = now
	thread.UpdatedAt = now
	return nil
}
```

- [ ] **Step 2: 更新 FindByID 方法**

修改 FindByID 方法以读取 workflow_template_id：

```go
// FindByID 根据ID查找Thread
func (r *ThreadRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Thread, error) {
	query := `
		SELECT id, project_id, status, current_phase, current_agent, depth, abort_token, workflow_template_id, created_at, updated_at
		FROM threads WHERE id = ?
	`
	thread := &model.Thread{}
	var idStr string
	var projectID sql.NullString
	var workflowTemplateID sql.NullString
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &projectID, &thread.Status, &thread.CurrentPhase, &thread.CurrentAgent,
		&thread.Depth, &thread.AbortToken, &workflowTemplateID, &thread.CreatedAt, &thread.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find thread: %w", err)
	}
	thread.ID, _ = uuid.Parse(idStr)
	if projectID.Valid {
		thread.ProjectID, _ = uuid.Parse(projectID.String)
	}
	if workflowTemplateID.Valid {
		wid, _ := uuid.Parse(workflowTemplateID.String)
		thread.WorkflowTemplateID = &wid
	}
	return thread, nil
}
```

- [ ] **Step 3: 更新 FindByProjectID 方法**

修改 FindByProjectID 方法以读取 workflow_template_id：

```go
// FindByProjectID 根据项目ID查找Thread列表
func (r *ThreadRepository) FindByProjectID(ctx context.Context, projectID uuid.UUID) ([]*model.Thread, error) {
	query := `
		SELECT id, project_id, status, current_phase, current_agent, depth, abort_token, workflow_template_id, created_at, updated_at
		FROM threads WHERE project_id = ? ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, projectID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find threads: %w", err)
	}
	defer rows.Close()

	var threads []*model.Thread
	for rows.Next() {
		thread := &model.Thread{}
		var idStr string
		var projID sql.NullString
		var workflowTemplateID sql.NullString
		err := rows.Scan(
			&idStr, &projID, &thread.Status, &thread.CurrentPhase, &thread.CurrentAgent,
			&thread.Depth, &thread.AbortToken, &workflowTemplateID, &thread.CreatedAt, &thread.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan thread: %w", err)
		}
		thread.ID, _ = uuid.Parse(idStr)
		if projID.Valid {
			thread.ProjectID, _ = uuid.Parse(projID.String)
		}
		if workflowTemplateID.Valid {
			wid, _ := uuid.Parse(workflowTemplateID.String)
			thread.WorkflowTemplateID = &wid
		}
		threads = append(threads, thread)
	}
	return threads, nil
}
```

- [ ] **Step 4: 提交 ThreadRepository 变更**

```bash
cd D:/Tools/ASDP_gittee/isdp
git add isdp/internal/repo/thread.go
git commit -m "feat(repo): Thread Repository 支持工作流绑定

- Create 方法支持 workflow_template_id
- FindByID 方法读取 workflow_template_id 字段
- FindByProjectID 方法读取 workflow_template_id 字段

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Chunk 3: 后端 Service 与 API 层变更

### Task 6: 更新 Workflow Service

**Files:**
- Modify: `isdp/internal/service/workflow/service.go`

- [ ] **Step 1: 添加 SetDefault 方法**

在 `isdp/internal/service/workflow/service.go` 中添加：

```go
// SetDefault 设置默认工作流模板
func (s *Service) SetDefault(ctx context.Context, id uuid.UUID) (*model.WorkflowTemplate, error) {
	if err := s.repo.SetDefault(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}
```

- [ ] **Step 2: 增强 Delete 方法**

修改 Delete 方法添加校验：

```go
// Delete 删除工作流模板
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// 检查是否为默认工作流
	template, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if template.IsDefault {
		return fmt.Errorf("该工作流是系统默认工作流，请先设置其他工作流为默认")
	}

	// 检查是否被项目引用
	count, err := s.repo.CountProjectReferences(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("该工作流已被 %d 个项目绑定，无法删除", count)
	}

	return s.repo.Delete(ctx, id)
}
```

- [ ] **Step 3: 添加 GetDefault 方法**

```go
// GetDefault 获取默认工作流模板
func (s *Service) GetDefault(ctx context.Context) (*model.WorkflowTemplate, error) {
	return s.repo.GetDefault(ctx)
}
```

- [ ] **Step 4: 提交 Service 变更**

```bash
cd D:/Tools/ASDP_gittee/isdp
git add isdp/internal/service/workflow/service.go
git commit -m "feat(service): 工作流 Service 支持默认工作流

- 新增 SetDefault 方法
- 新增 GetDefault 方法
- Delete 方法增加默认和引用校验

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

### Task 7: 更新 Workflow API Handler

**Files:**
- Modify: `isdp/internal/api/workflow_handler.go`

- [ ] **Step 1: 添加 SetDefault 处理方法**

在 `isdp/internal/api/workflow_handler.go` 中添加：

```go
// SetDefault 设置默认工作流模板
func (h *WorkflowHandler) SetDefault(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	template, err := h.service.SetDefault(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, template)
}
```

- [ ] **Step 2: 更新 Delete 方法返回更详细的错误**

```go
// Delete 删除工作流模板
func (h *WorkflowHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		// 根据错误类型返回不同状态码
		if strings.Contains(err.Error(), "默认工作流") || strings.Contains(err.Error(), "项目绑定") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
```

- [ ] **Step 3: 更新 RegisterRoutes 添加新路由**

```go
// RegisterRoutes 注册路由
func (h *WorkflowHandler) RegisterRoutes(r *gin.RouterGroup) {
	workflows := r.Group("/workflows")
	{
		workflows.GET("", h.List)
		workflows.POST("", h.Create)
		workflows.GET("/:id", h.Get)
		workflows.PUT("/:id", h.Update)
		workflows.DELETE("/:id", h.Delete)
		workflows.PUT("/:id/default", h.SetDefault) // 新增
	}
}
```

- [ ] **Step 4: 添加 strings 包导入**

确保文件顶部导入了 strings 包：

```go
import (
	"net/http"
	"strings"  // 新增

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/workflow"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)
```

- [ ] **Step 5: 提交 Handler 变更**

```bash
cd D:/Tools/ASDP_gittee/isdp
git add isdp/internal/api/workflow_handler.go
git commit -m "feat(api): 工作流 API 新增设置默认端点

- PUT /workflows/:id/default 设置默认工作流
- Delete 返回更详细的错误信息

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

### Task 8: 更新 Project Service

**Files:**
- Modify: `isdp/internal/service/project/service.go`

- [ ] **Step 1: 查看 Project Service 结构**

先读取文件：

```bash
cat isdp/internal/service/project/service.go
```

- [ ] **Step 2: 更新 Service 结构体和构造函数**

假设 Service 结构如下，添加 workflowRepo 依赖：

```go
package project

import (
	"context"
	"errors"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// Service 项目服务
type Service struct {
	repo           *repo.ProjectRepository
	workflowRepo   *repo.WorkflowTemplateRepository // 新增依赖
}

// NewService 创建项目服务
func NewService(repo *repo.ProjectRepository, workflowRepo *repo.WorkflowTemplateRepository) *Service {
	return &Service{
		repo:         repo,
		workflowRepo: workflowRepo,
	}
}
```

- [ ] **Step 3: 更新 Update 方法添加工作流校验**

在 Update 方法中添加工作流存在性校验：

```go
// Update 更新项目
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateProjectRequest) (*model.Project, error) {
	project, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 如果设置了工作流ID，验证工作流是否存在
	if req.WorkflowTemplateID != nil && *req.WorkflowTemplateID != uuid.Nil {
		_, err := s.workflowRepo.FindByID(ctx, *req.WorkflowTemplateID)
		if err != nil {
			return nil, errors.New("指定的工作流模板不存在")
		}
	}

	// 更新字段
	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Type != nil {
		project.Type = *req.Type
	}
	if req.Mode != nil {
		project.Mode = *req.Mode
	}
	if req.Status != nil {
		project.Status = *req.Status
	}
	if req.GitRepo != nil {
		project.GitRepo = *req.GitRepo
	}
	project.WorkflowTemplateID = req.WorkflowTemplateID

	if err := s.repo.Update(ctx, project); err != nil {
		return nil, err
	}

	return project, nil
}
```

- [ ] **Step 4: 提交 Service 变更**

```bash
cd D:/Tools/ASDP_gittee/isdp
git add isdp/internal/service/project/service.go
git commit -m "feat(service): Project Service 支持工作流绑定

- Update 方法添加工作流存在性校验
- 新增 workflowRepo 依赖

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

### Task 9: 更新 Thread Service - 工作流选择逻辑

**Files:**
- Modify: `isdp/internal/service/thread/service.go`

**这是核心功能实现：创建任务时自动选择工作流**

- [ ] **Step 1: 更新 Service 结构体添加依赖**

修改 `isdp/internal/service/thread/service.go`，添加 projectRepo 和 workflowRepo 依赖：

```go
package thread

import (
	"context"
	"errors"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// Service Thread服务
type Service struct {
	repo         *repo.ThreadRepository
	projectRepo  *repo.ProjectRepository         // 新增依赖
	workflowRepo *repo.WorkflowTemplateRepository // 新增依赖
}

// NewService 创建Thread服务
func NewService(repo *repo.ThreadRepository, projectRepo *repo.ProjectRepository, workflowRepo *repo.WorkflowTemplateRepository) *Service {
	return &Service{
		repo:         repo,
		projectRepo:  projectRepo,
		workflowRepo: workflowRepo,
	}
}
```

- [ ] **Step 2: 更新 Create 方法实现工作流选择逻辑**

修改 Create 方法，实现自动选择工作流的逻辑：

```go
// Create 创建Thread
func (s *Service) Create(ctx context.Context, projectID uuid.UUID) (*model.Thread, error) {
	// 1. 获取项目信息
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	var workflowID *uuid.UUID

	// 2. 检查项目是否绑定了工作流
	if project.WorkflowTemplateID != nil {
		// 验证工作流是否存在
		_, err := s.workflowRepo.FindByID(ctx, *project.WorkflowTemplateID)
		if err != nil {
			return nil, errors.New("项目绑定的工作流不存在，请重新配置")
		}
		workflowID = project.WorkflowTemplateID
	} else {
		// 3. 查询默认工作流
		defaultWorkflow, err := s.workflowRepo.GetDefault(ctx)
		if err != nil {
			return nil, errors.New("请先在项目设置中绑定工作流，或设置系统默认工作流")
		}
		workflowID = &defaultWorkflow.ID
	}

	// 4. 创建 Thread 并关联工作流
	thread := &model.Thread{
		ID:                  uuid.New(),
		ProjectID:           projectID,
		Status:              model.ThreadStatusIdle,
		CurrentPhase:        model.PhaseRequirement,
		WorkflowTemplateID:  workflowID,
	}

	if err := s.repo.Create(ctx, thread); err != nil {
		return nil, err
	}
	return thread, nil
}
```

- [ ] **Step 3: 提交 Thread Service 变更**

```bash
cd D:/Tools/ASDP_gittee/isdp
git add isdp/internal/service/thread/service.go
git commit -m "feat(service): Thread Service 实现工作流自动选择

- Create 方法实现工作流选择逻辑
- 优先使用项目绑定的工作流
- 未绑定时使用系统默认工作流
- 新增 projectRepo 和 workflowRepo 依赖

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

### Task 10: 更新 main.go 依赖注入

**Files:**
- Modify: `isdp/cmd/server/main.go`

- [ ] **Step 1: 更新 Project Service 初始化**

修改 main.go 中 projectService 的初始化，注入 workflowRepo：

```go
// 初始化Services
projectService := project.NewService(projectRepo, workflowRepo)
```

- [ ] **Step 2: 更新 Thread Service 初始化**

修改 main.go 中 threadService 的初始化，注入 projectRepo 和 workflowRepo：

```go
threadService := thread.NewService(threadRepo, projectRepo, workflowRepo)
```

- [ ] **Step 3: 提交变更**

```bash
cd D:/Tools/ASDP_gittee/isdp
git add isdp/cmd/server/main.go
git commit -m "feat: Service 依赖注入更新

- Project Service 注入 workflowRepo 依赖
- Thread Service 注入 projectRepo 和 workflowRepo 依赖

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Chunk 4: 前端类型定义与 API 变更

### Task 11: 更新前端类型定义

**Files:**
- Modify: `isdp/web/src/types/index.ts`

- [ ] **Step 1: 更新 Project 接口**

```typescript
// Project
export interface Project {
  id: string;
  name: string;
  description: string;
  type?: 'service' | 'app' | 'task';
  mode?: 'new' | 'enhance';
  repositoryUrl?: string;
  status: 'active' | 'archived';
  workflowTemplateId?: string;      // 新增
  workflowTemplate?: WorkflowTemplate; // 新增
  createdAt: string;
  updatedAt: string;
}
```

- [ ] **Step 2: 更新 WorkflowTemplate 接口**

```typescript
// 工作流模板
export interface WorkflowTemplate {
  id: string;
  name: string;
  description: string;
  agentIds: string[];
  checkpoints: string[];
  estimatedTime: string;
  isSystem: boolean;
  isDefault: boolean; // 新增
  createdAt: string;
  updatedAt: string;
}
```

- [ ] **Step 3: 提交类型变更**

```bash
cd D:/Tools/ASDP_gittee/isdp
git add isdp/web/src/types/index.ts
git commit -m "feat(types): 前端类型支持工作流绑定

- Project 新增 workflowTemplateId 和 workflowTemplate 字段
- WorkflowTemplate 新增 isDefault 字段

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

### Task 12: 更新 API Client

**Files:**
- Modify: `isdp/web/src/api/client.ts`
- Modify: `isdp/web/src/api/transform.ts`

- [ ] **Step 1: 添加 setDefaultWorkflow API 方法**

在 `isdp/web/src/api/client.ts` 的 workflows 对象中添加：

```typescript
// 工作流模板 API
workflows = {
  list: (): Promise<WorkflowTemplate[]> => this.request('/workflows', 'GET'),
  get: (id: string): Promise<WorkflowTemplate> => this.request(`/workflows/${id}`, 'GET'),
  create: (data: Partial<WorkflowTemplate>): Promise<WorkflowTemplate> =>
    this.request('/workflows', 'POST', data),
  update: (id: string, data: Partial<WorkflowTemplate>): Promise<WorkflowTemplate> =>
    this.request(`/workflows/${id}`, 'PUT', data),
  delete: (id: string): Promise<void> => this.request(`/workflows/${id}`, 'DELETE'),
  setDefault: (id: string): Promise<WorkflowTemplate> =>  // 新增
    this.request(`/workflows/${id}/default`, 'PUT'),
};
```

- [ ] **Step 2: 更新 transform.ts 添加 isDefault 转换**

读取并更新 `isdp/web/src/api/transform.ts`：

在 transformWorkflowTemplate 和 transformWorkflowTemplates 函数中添加 isDefault 字段的转换。

- [ ] **Step 3: 提交 API 变更**

```bash
cd D:/Tools/ASDP_gittee/isdp
git add isdp/web/src/api/client.ts isdp/web/src/api/transform.ts
git commit -m "feat(api): 前端 API 支持设置默认工作流

- 新增 workflows.setDefault 方法
- 更新 transform 支持 isDefault 字段

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Chunk 5: 前端 UI 变更

### Task 13: 更新项目详情页 - 工作流绑定

**Files:**
- Modify: `isdp/web/src/pages/ProjectDetail/index.tsx`

- [ ] **Step 1: 添加工作流列表状态和加载**

在组件中添加状态：

```typescript
const [workflows, setWorkflows] = useState<WorkflowTemplate[]>([]);
const [loadingWorkflows, setLoadingWorkflows] = useState(false);
```

- [ ] **Step 2: 加载工作流列表**

在 loadProjectData 函数中添加工作流加载：

```typescript
const loadProjectData = async () => {
  setLoading(true);
  try {
    const projectData = await api.projects.get(projectId!);
    setProject(projectData as unknown as Project);

    // 加载该项目的 Thread 列表
    const threadsData = await api.threads.list(projectId!);
    setThreads((threadsData as unknown as Thread[]) || []);

    // 加载工作流列表
    setLoadingWorkflows(true);
    const workflowsData = await api.workflows.list();
    setWorkflows(workflowsData);
  } catch (error) {
    message.error('加载数据失败');
    console.error(error);
  } finally {
    setLoading(false);
    setLoadingWorkflows(false);
  }
};
```

- [ ] **Step 3: 更新编辑项目表单添加工作流选择**

在编辑项目 Modal 的 Form 中添加工作流选择：

```tsx
<Form.Item name="workflowTemplateId" label="绑定工作流">
  <Select
    placeholder="选择工作流"
    allowClear
    loading={loadingWorkflows}
  >
    <Option value={null}>不绑定（使用系统默认）</Option>
    {workflows.map((wf) => (
      <Option key={wf.id} value={wf.id}>
        {wf.name} {wf.isDefault ? '(默认)' : ''}
      </Option>
    ))}
  </Select>
</Form.Item>
```

- [ ] **Step 4: 更新项目设置 Tab 显示工作流绑定**

在项目设置 Tab 中添加工作流绑定显示：

```tsx
<TabPane
  tab={<span><SettingOutlined />项目设置</span>}
  key="settings"
>
  <Card>
    <Descriptions title="项目配置" column={2} bordered>
      <Descriptions.Item label="项目 ID">
        <Text code>{project.id}</Text>
      </Descriptions.Item>
      <Descriptions.Item label="项目名称">
        {project.name}
      </Descriptions.Item>
      <Descriptions.Item label="项目类型">
        {projectTypeConfig[project.type || 'service']?.label || project.type}
      </Descriptions.Item>
      <Descriptions.Item label="开发模式">
        {projectModeConfig[project.mode || 'new']?.label || project.mode}
      </Descriptions.Item>
      <Descriptions.Item label="绑定工作流" span={2}>
        {project.workflowTemplate ? (
          <Space>
            <Tag color="blue">{project.workflowTemplate.name}</Tag>
            {project.workflowTemplate.isDefault && <Tag color="gold">默认</Tag>}
          </Space>
        ) : (
          <Text type="secondary">未绑定（使用系统默认）</Text>
        )}
      </Descriptions.Item>
    </Descriptions>

    <div style={{ marginTop: 24 }}>
      <Button type="primary" icon={<EditOutlined />} onClick={() => {
        form.setFieldsValue(project);
        setEditModalVisible(true);
      }}>
        编辑项目信息
      </Button>
    </div>
  </Card>
</TabPane>
```

- [ ] **Step 5: 更新创建任务弹窗显示工作流**

在创建任务 Modal 中显示工作流信息：

```tsx
<Modal
  title="新建开发任务"
  open={createThreadModalVisible}
  onOk={() => threadForm.submit()}
  onCancel={() => {
    setCreateThreadModalVisible(false);
    threadForm.resetFields();
  }}
  width={500}
>
  <Form form={threadForm} layout="vertical" onFinish={handleCreateThread}>
    <Form.Item name="name" label="任务名称（可选）">
      <Input placeholder="为任务起个名字" />
    </Form.Item>
    {/* 新增：显示工作流信息 */}
    <Form.Item label="使用工作流">
      {project?.workflowTemplate ? (
        <Space direction="vertical">
          <Tag color="blue">{project.workflowTemplate.name}</Tag>
          <Text type="secondary" style={{ fontSize: 12 }}>（来自项目绑定）</Text>
        </Space>
      ) : workflows.find(w => w.isDefault) ? (
        <Space direction="vertical">
          <Tag color="blue">{workflows.find(w => w.isDefault)?.name}</Tag>
          <Text type="secondary" style={{ fontSize: 12 }}>（系统默认）</Text>
        </Space>
      ) : (
        <Text type="warning">未配置工作流，请先在项目设置中绑定</Text>
      )}
    </Form.Item>
  </Form>
</Modal>
```

- [ ] **Step 6: 添加 WorkflowTemplate 类型导入**

确保在文件顶部导入 WorkflowTemplate 类型：

```typescript
import type { Project, Thread, WorkflowTemplate } from '@/types';
```

- [ ] **Step 7: 提交项目详情页变更**

```bash
cd D:/Tools/ASDP_gittee/isdp
git add isdp/web/src/pages/ProjectDetail/index.tsx
git commit -m "feat(ui): 项目详情页支持工作流绑定

- 项目设置 Tab 显示绑定的工作流
- 编辑项目可选择绑定工作流
- 创建任务显示将使用的工作流

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

### Task 14: 更新工作流页面 - 默认标记

**Files:**
- Modify: `isdp/web/src/pages/Workflow/index.tsx`

- [ ] **Step 1: 在模板卡片上添加默认标记**

修改 renderTemplateCard 函数：

```tsx
const renderTemplateCard = (template: WorkflowTemplate) => {
  const templateAgents = agents.filter(a => template.agentIds?.includes(a.id));

  return (
    <Card
      hoverable
      className={`workflow-template-card ${selectedTemplate?.id === template.id ? 'selected' : ''}`}
      onClick={() => setSelectedTemplate(template)}
      style={{
        marginBottom: 16,
        border: selectedTemplate?.id === template.id ? '2px solid #1890ff' : undefined,
      }}
    >
      <Space direction="vertical" style={{ width: '100%' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Space>
            <Title level={5} style={{ margin: 0 }}>{template.name}</Title>
            {template.isDefault && <Tag color="gold">默认</Tag>}
            {template.isSystem && <Tag color="purple">系统预设</Tag>}
          </Space>
          <Space>
            <Tag color="blue">{template.estimatedTime}</Tag>
            {!template.isSystem && (
              <Popconfirm
                title="确定删除此工作流？"
                onConfirm={(e) => {
                  e?.stopPropagation();
                  handleDeleteWorkflow(template.id);
                }}
                onCancel={(e) => e?.stopPropagation()}
              >
                <Button
                  type="text"
                  danger
                  size="small"
                  icon={<DeleteOutlined />}
                  onClick={(e) => e.stopPropagation()}
                />
              </Popconfirm>
            )}
          </Space>
        </div>
        {/* ... 其余内容保持不变 ... */}
      </Space>
    </Card>
  );
};
```

- [ ] **Step 2: 添加设为默认功能**

添加处理函数：

```tsx
// 设置默认工作流
const handleSetDefault = async (id: string) => {
  try {
    await api.workflows.setDefault(id);
    message.success('已设为默认工作流');
    fetchWorkflowTemplates(); // 刷新列表
  } catch (error: any) {
    console.error('Failed to set default workflow:', error);
    message.error(error?.response?.data?.error || '设置默认失败');
  }
};
```

- [ ] **Step 3: 在非默认工作流上添加"设为默认"按钮**

更新卡片中的操作按钮部分：

```tsx
<Space>
  <Tag color="blue">{template.estimatedTime}</Tag>
  {!template.isDefault && (
    <Button
      type="link"
      size="small"
      onClick={(e) => {
        e.stopPropagation();
        handleSetDefault(template.id);
      }}
    >
      设为默认
    </Button>
  )}
  {!template.isSystem && (
    <Popconfirm
      title="确定删除此工作流？"
      onConfirm={(e) => {
        e?.stopPropagation();
        handleDeleteWorkflow(template.id);
      }}
      onCancel={(e) => e?.stopPropagation()}
    >
      <Button
        type="text"
        danger
        size="small"
        icon={<DeleteOutlined />}
        onClick={(e) => e.stopPropagation()}
      />
    </Popconfirm>
  )}
</Space>
```

- [ ] **Step 4: 提交工作流页面变更**

```bash
cd D:/Tools/ASDP_gittee/isdp
git add isdp/web/src/pages/Workflow/index.tsx
git commit -m "feat(ui): 工作流页面支持默认标记

- 默认工作流显示标记
- 非默认工作流可设为默认
- 使用 workflows.setDefault API

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Chunk 6: 集成测试与最终提交

### Task 15: 验证功能完整性

- [ ] **Step 1: 编译后端代码**

```bash
cd D:/Tools/ASDP_gittee/isdp
go build -o bin/isdp-server.exe ./isdp/cmd/server
```

预期：编译成功，无错误。

- [ ] **Step 2: 编译前端代码**

```bash
cd D:/Tools/ASDP_gittee/isdp/isdp/web
npm run build
```

预期：编译成功，无错误。

- [ ] **Step 3: 手动测试功能**

启动服务后测试：
1. 工作流页面可以设置默认工作流
2. 项目设置页面可以选择绑定工作流
3. 创建任务时显示正确的工作流信息
4. 删除被绑定的工作流时显示错误提示
5. 删除默认工作流时显示错误提示

- [ ] **Step 4: 最终提交（如有遗漏）**

```bash
cd D:/Tools/ASDP_gittee/isdp
git add -A
git status
# 如有未提交的变更
git commit -m "fix: 修复遗漏的变更

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## 文件变更汇总

### 后端
| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `internal/model/project.go` | 修改 | 新增 WorkflowTemplateID 字段、UpdateProjectRequest 类型 |
| `internal/model/workflow_template.go` | 修改 | 新增 IsDefault 字段 |
| `internal/model/thread.go` | 修改 | 新增 WorkflowTemplateID 字段 |
| `internal/repo/workflow_template.go` | 修改 | 新增 GetDefault、SetDefault、CountProjectReferences 方法 |
| `internal/repo/project.go` | 修改 | CRUD 方法支持 workflow_template_id |
| `internal/repo/thread.go` | 修改 | Create/FindByID/FindByProjectID 方法支持 workflow_template_id |
| `internal/service/workflow/service.go` | 修改 | 新增 SetDefault、GetDefault 方法，增强 Delete 校验 |
| `internal/service/project/service.go` | 修改 | Update 方法添加工作流校验，新增 workflowRepo 依赖 |
| `internal/service/thread/service.go` | 修改 | Create 方法实现工作流选择逻辑，新增 projectRepo/workflowRepo 依赖 |
| `internal/api/workflow_handler.go` | 修改 | 新增 SetDefault 端点，增强 Delete 校验 |
| `cmd/server/main.go` | 修改 | 数据库 schema 更新，Service 依赖注入 |

### 前端
| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `web/src/types/index.ts` | 修改 | Project 新增 workflowTemplateId，WorkflowTemplate 新增 isDefault |
| `web/src/api/client.ts` | 修改 | 新增 workflows.setDefault 方法 |
| `web/src/api/transform.ts` | 修改 | 新增 isDefault 字段转换 |
| `web/src/pages/ProjectDetail/index.tsx` | 修改 | 项目设置工作流绑定 UI，创建任务显示工作流 |
| `web/src/pages/Workflow/index.tsx` | 修改 | 默认工作流标记和设为默认按钮 |