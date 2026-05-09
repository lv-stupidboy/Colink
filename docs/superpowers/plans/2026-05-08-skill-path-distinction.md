# Skill Path Distinction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `SourcePath` field to distinguish federated skills from different directories in the same registry.

**Architecture:** Add a new optional field `source_path` to the Skill model. Import and sync operations will use `(Name, SourceRegistryID, SourcePath)` triple for exact matching. Frontend displays path in skill cards for federated skills.

**Tech Stack:** Go backend (Gin + SQLite), React frontend (Ant Design), SQL migrations

---

## File Structure

| File | Purpose |
|------|---------|
| `sql-change/v1.2.6/sqlite/00013_add_source_path.sql` | Database migration - add column |
| `internal/model/skill.go` | Add `SourcePath` field to Skill struct |
| `internal/repo/skill.go` | Add `FindBySourcePath` method, update scan/Create/Update |
| `internal/service/skill/skill_scanner.go` | Store `SourcePath` during import |
| `internal/service/skill/registry_service.go` | Use `FindBySourcePath` in SyncPreview/SyncConfirm |
| `web/src/types/index.ts` | Add `sourcePath` to Skill interface |
| `web/src/pages/SkillLibrary/index.tsx` | Display path in card and conflict modal |

---

### Task 1: Database Migration

**Files:**
- Create: `sql-change/v1.2.6/sqlite/00013_add_source_path.sql`

- [ ] **Step 1: Create migration directory**

```bash
mkdir -p sql-change/v1.2.6/sqlite
```

- [ ] **Step 2: Write migration SQL file**

```sql
-- +goose Up
-- +goose StatementBegin
ALTER TABLE skills ADD COLUMN source_path TEXT DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE skills DROP COLUMN source_path;
-- +goose StatementEnd
```

Save to: `sql-change/v1.2.6/sqlite/00013_add_source_path.sql`

- [ ] **Step 3: Create MySQL migration (optional)**

If MySQL support is needed, create `sql-change/v1.2.6/mysql/00013_add_source_path.sql`:

```sql
-- +goose Up
-- +goose StatementBegin
ALTER TABLE skills ADD COLUMN source_path VARCHAR(255) DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE skills DROP COLUMN source_path;
-- +goose StatementEnd
```

- [ ] **Step 4: Commit migration files**

```bash
git add sql-change/v1.2.6/
git commit -m "feat(skill): add source_path column migration"
```

---

### Task 2: Go Model Update

**Files:**
- Modify: `internal/model/skill.go:29-53`

- [ ] **Step 1: Add SourcePath field to Skill struct**

Find the Skill struct definition and add the new field after `SourceRegistryID`:

```go
// Skill 技能模型
type Skill struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Tags        []string  `json:"tags,omitempty"`

	// 来源信息
	SourceType       SkillSourceType `json:"sourceType"`
	SourceRegistryID uuid.UUID       `json:"sourceRegistryId,omitempty"`
	SourcePath       string          `json:"sourcePath,omitempty"` // 联邦源仓库相对路径
	AuthorID         uuid.UUID       `json:"authorId,omitempty"`
	ProjectID        uuid.UUID       `json:"projectId,omitempty"`
	// ... rest of fields unchanged
}
```

- [ ] **Step 2: Run backend build to verify**

```bash
cd D:/workspace/isdp && go build ./cmd/server
```

Expected: Build succeeds

- [ ] **Step 3: Commit model change**

```bash
git add internal/model/skill.go
git commit -m "feat(skill): add SourcePath field to Skill model"
```

---

### Task 3: Repo Layer Update

**Files:**
- Modify: `internal/repo/skill.go`

- [ ] **Step 1: Update scanSkill helper function**

Modify `scanSkill` to include `source_path` column. Add `sourcePath` variable and scan it:

```go
func scanSkill(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.Skill, error) {
	skill := &model.Skill{}
	var idStr string
	var description sql.NullString
	var tags, supportedAgents []byte
	var sourceRegistryID, authorID, projectID sql.NullString
	var sourcePath sql.NullString  // NEW: source_path column
	var createdAt, updatedAt SQLiteTimeScanner

	err := scanner.Scan(
		&idStr, &skill.Name, &description, &tags, &skill.SourceType, &sourceRegistryID, &authorID, &projectID, &supportedAgents, &skill.UseCount, &skill.Status, &skill.IsPublic, &createdAt, &updatedAt, &sourcePath,  // NEW: add sourcePath
	)
	if err != nil {
		return nil, err
	}

	skill.ID, _ = uuid.Parse(idStr)
	if description.Valid {
		skill.Description = description.String
	}
	json.Unmarshal(tags, &skill.Tags)
	if sourceRegistryID.Valid {
		skill.SourceRegistryID, _ = uuid.Parse(sourceRegistryID.String)
	}
	if sourcePath.Valid {
		skill.SourcePath = sourcePath.String  // NEW: set SourcePath
	}
	// ... rest unchanged
	
	return skill, nil
}
```

- [ ] **Step 2: Update SELECT queries to include source_path**

Update all SELECT queries in the file to include `source_path`:

```go
// FindByID - add source_path to SELECT
query := `
	SELECT id, name, description, tags, source_type, source_registry_id, author_id, project_id, supported_agents, use_count, status, is_public, created_at, updated_at, source_path
	FROM skills WHERE id = ?
`

// FindByName - add source_path to SELECT
query := `
	SELECT id, name, description, tags, source_type, source_registry_id, author_id, project_id, supported_agents, use_count, status, is_public, created_at, updated_at, source_path
	FROM skills WHERE name = ?
`

// List - add source_path to SELECT (line 178-179)
listQuery := `
	SELECT id, name, description, tags, source_type, source_registry_id, author_id, project_id, supported_agents, use_count, status, is_public, created_at, updated_at, source_path
	FROM skills ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?
`
```

- [ ] **Step 3: Update Create INSERT query**

Add `source_path` to INSERT statement:

```go
func (r *SkillRepository) Create(ctx context.Context, skill *model.Skill) error {
	query := `
		INSERT INTO skills (id, name, description, tags, source_type, source_registry_id, source_path, author_id, project_id, supported_agents, use_count, status, is_public, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	supportedAgents, _ := json.Marshal(skill.SupportedAgents)
	tags, _ := json.Marshal(skill.Tags)

	var sourceRegistryID, authorID, projectID interface{}
	if skill.SourceRegistryID != uuid.Nil {
		sourceRegistryID = skill.SourceRegistryID.String()
	}
	if skill.AuthorID != uuid.Nil {
		authorID = skill.AuthorID.String()
	}
	if skill.ProjectID != uuid.Nil {
		projectID = skill.ProjectID.String()
	}

	_, err := r.DB().ExecContext(ctx, query,
		skill.ID.String(), skill.Name, skill.Description, tags, skill.SourceType, sourceRegistryID, skill.SourcePath, authorID, projectID, supportedAgents, skill.UseCount, skill.Status, skill.IsPublic, skill.CreatedAt, skill.UpdatedAt,
	)
	return err
}
```

- [ ] **Step 4: Update Update query**

Add `source_path` to UPDATE statement:

```go
func (r *SkillRepository) Update(ctx context.Context, skill *model.Skill) error {
	now := time.Now()
	skill.UpdatedAt = now
	query := `
		UPDATE skills
		SET name = ?, description = ?, tags = ?, source_type = ?, source_registry_id = ?, source_path = ?, author_id = ?, project_id = ?, supported_agents = ?, use_count = ?, status = ?, is_public = ?, updated_at = ?
		WHERE id = ?
	`
	supportedAgents, _ := json.Marshal(skill.SupportedAgents)
	tags, _ := json.Marshal(skill.Tags)

	var sourceRegistryID, authorID, projectID interface{}
	if skill.SourceRegistryID != uuid.Nil {
		sourceRegistryID = skill.SourceRegistryID.String()
	}
	if skill.AuthorID != uuid.Nil {
		authorID = skill.AuthorID.String()
	}
	if skill.ProjectID != uuid.Nil {
		projectID = skill.ProjectID.String()
	}

	_, err := r.DB().ExecContext(ctx, query,
		skill.Name, skill.Description, tags, skill.SourceType, sourceRegistryID, skill.SourcePath, authorID, projectID, supportedAgents, skill.UseCount, skill.Status, skill.IsPublic, now, skill.ID.String(),
	)
	return err
}
```

- [ ] **Step 5: Add FindBySourcePath method**

Add new method after `FindByName`:

```go
// FindBySourcePath 根据名称 + 联邦源ID + 路径查找（精确匹配）
func (r *SkillRepository) FindBySourcePath(ctx context.Context, name string, registryID uuid.UUID, path string) (*model.Skill, error) {
	query := `
		SELECT id, name, description, tags, source_type, source_registry_id, author_id, project_id, supported_agents, use_count, status, is_public, created_at, updated_at, source_path
		FROM skills 
		WHERE name = ? AND source_registry_id = ? AND source_path = ?
	`
	skill, err := scanSkill(r.DB().QueryRowContext(ctx, query, name, registryID.String(), path))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("skill not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find skill: %w", err)
	}
	return skill, nil
}
```

- [ ] **Step 6: Run backend build to verify**

```bash
cd D:/workspace/isdp && go build ./cmd/server
```

Expected: Build succeeds

- [ ] **Step 7: Commit repo changes**

```bash
git add internal/repo/skill.go
git commit -m "feat(skill): add SourcePath support in repo layer with FindBySourcePath"
```

---

### Task 4: Import Service Update

**Files:**
- Modify: `internal/service/skill/skill_scanner.go:676-690`

- [ ] **Step 1: Update Skill creation to include SourcePath**

Find the skill creation block in `ImportSkills` goroutine and add `SourcePath`:

```go
// 创建 Skill 记录
skill := &model.Skill{
	ID:               uuid.New(),
	Name:             item.Name,
	Description:      item.Description,
	Tags:             item.Tags,
	SourceType:       model.SkillSourceFederated,
	SourceRegistryID: registry.ID,
	SourcePath:       item.Path,  // NEW: 存储仓库相对路径
	SupportedAgents:  item.SupportedAgents,
	IsPublic:         true,
	Status:           model.SkillStatusActive,
	UseCount:         0,
	CreatedAt:        time.Now(),
	UpdatedAt:        time.Now(),
}
```

- [ ] **Step 2: Update skill update block to include SourcePath**

In the update mode block (around line 638-644), add SourcePath update:

```go
// 更新元数据（替换策略）
existing.Description = item.Description
existing.Tags = item.Tags
existing.SupportedAgents = item.SupportedAgents
existing.SourceType = model.SkillSourceFederated
existing.SourceRegistryID = registry.ID
existing.SourcePath = item.Path  // NEW: 更新路径
existing.UpdatedAt = time.Now()
```

- [ ] **Step 3: Run backend build to verify**

```bash
cd D:/workspace/isdp && go build ./cmd/server
```

Expected: Build succeeds

- [ ] **Step 4: Commit service changes**

```bash
git add internal/service/skill/skill_scanner.go
git commit -m "feat(skill): store SourcePath during federated import"
```

---

### Task 5: Sync Service Update

**Files:**
- Modify: `internal/service/skill/registry_service.go`

- [ ] **Step 1: Update SyncPreview to use FindBySourcePath**

Replace `FindByName` calls with `FindBySourcePath` for precise matching. In `SyncPreview` method (line 222-262):

```go
// 分析冲突情况
for _, remoteSkill := range remoteSkills {
	// 使用精确匹配：Name + SourceRegistryID + SourcePath
	existing, err := s.skillRepo.FindBySourcePath(ctx, remoteSkill.Name, registry.ID, remoteSkill.Path)
	if err != nil {
		// 本地无精确匹配 → 检查是否有同名但不同路径的
		existingByName, err2 := s.skillRepo.FindByName(ctx, remoteSkill.Name)
		if err2 != nil {
			// 本地完全无同名 → newSkills
			result.NewSkills = append(result.NewSkills, &model.RemoteSkill{
				Name:        remoteSkill.Name,
				Description: remoteSkill.Description,
				Path:        remoteSkill.Path,
			})
			continue
		}
		// 同名但路径不同 → 异源冲突
		conflictSkill := &model.SyncConflictSkill{
			Name:        remoteSkill.Name,
			Description: remoteSkill.Description,
			Path:        remoteSkill.Path,  // NEW: 包含路径信息
			LocalSkill: &model.LocalSkillInfo{
				ID:          existingByName.ID,
				SourceType:  string(existingByName.SourceType),
				Description: existingByName.Description,
				SourcePath:  existingByName.SourcePath,  // NEW: 本地路径
			},
		}
		if existingByName.SourceType == model.SkillSourceFederated && existingByName.SourceRegistryID != uuid.Nil {
			sourceRegistry, _ := s.registryRepo.FindByID(ctx, existingByName.SourceRegistryID)
			if sourceRegistry != nil {
				conflictSkill.LocalSkill.SourceRegistryID = existingByName.SourceRegistryID
				conflictSkill.LocalSkill.SourceRegistryName = sourceRegistry.Name
			}
		}
		result.ConflictSkills = append(result.ConflictSkills, conflictSkill)
		continue
	}

	// 精确匹配成功 → 同源同名，自动更新
	result.AutoUpdateSkills = append(result.AutoUpdateSkills, &model.SyncPreviewSkill{
		Name:         remoteSkill.Name,
		LocalSkillID: existing.ID,
		Description:  remoteSkill.Description,
		Path:         remoteSkill.Path,  // NEW: 包含路径信息
	})
}
```

- [ ] **Step 2: Update LocalSkillInfo model to include SourcePath**

In `internal/model/skill.go`, update `LocalSkillInfo`:

```go
// LocalSkillInfo 本地同名 Skill 信息（用于冲突展示）
type LocalSkillInfo struct {
	ID               uuid.UUID `json:"id"`
	SourceType       string    `json:"sourceType"`
	SourceRegistryID uuid.UUID `json:"sourceRegistryId,omitempty"`
	SourceRegistryName string  `json:"sourceRegistryName,omitempty"`
	SourcePath       string    `json:"sourcePath,omitempty"`  // NEW: 本地路径
	Description      string    `json:"description"`
}
```

- [ ] **Step 3: Update SyncConflictSkill model to include Path**

In `internal/model/skill.go`, update `SyncConflictSkill`:

```go
// SyncConflictSkill 同步冲突 skill（异源）
type SyncConflictSkill struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Path        string          `json:"path,omitempty"`  // NEW: 远程路径
	LocalSkill  *LocalSkillInfo `json:"localSkill"`
}
```

- [ ] **Step 4: Update SyncPreviewSkill model to include Path**

In `internal/model/skill.go`, update `SyncPreviewSkill`:

```go
// SyncPreviewSkill 同步预览 skill（同源）
type SyncPreviewSkill struct {
	Name         string    `json:"name"`
	LocalSkillID uuid.UUID `json:"localSkillId"`
	Description  string    `json:"description"`
	Path         string    `json:"path,omitempty"`  // NEW: 远程路径
}
```

- [ ] **Step 5: Run backend build to verify**

```bash
cd D:/workspace/isdp && go build ./cmd/server
```

Expected: Build succeeds

- [ ] **Step 6: Commit sync service changes**

```bash
git add internal/service/skill/registry_service.go internal/model/skill.go
git commit -m "feat(skill): use FindBySourcePath for precise sync matching"
```

---

### Task 6: Frontend TypeScript Types

**Files:**
- Modify: `web/src/types/index.ts:604-640`

- [ ] **Step 1: Add sourcePath to Skill interface**

```typescript
export interface Skill {
  id: string;
  name: string;
  description?: string;
  tags?: string[];
  sourceType: SkillSourceType;
  sourceRegistryId?: string;
  sourcePath?: string;  // NEW: 联邦源仓库相对路径
  authorId?: string;
  projectId?: string;
  supportedAgents?: string[];
  useCount: number;
  status: 'active' | 'deprecated';
  isPublic: boolean;
  createdAt: string;
  updatedAt: string;
}
```

- [ ] **Step 2: Add sourcePath to LocalSkillInfo interface**

Find `LocalSkillInfo` interface and add `sourcePath`:

```typescript
export interface LocalSkillInfo {
  id: string;
  sourceType: string;
  sourceRegistryId?: string;
  sourceRegistryName?: string;
  sourcePath?: string;  // NEW: 本地路径
  description: string;
}
```

- [ ] **Step 3: Add path to SyncConflictSkill interface**

Find `SyncConflictSkill` interface and add `path`:

```typescript
export interface SyncConflictSkill {
  name: string;
  description: string;
  path?: string;  // NEW: 远程路径
  localSkill: LocalSkillInfo;
}
```

- [ ] **Step 4: Add path to SyncPreviewSkill interface**

Find `SyncPreviewSkill` interface and add `path`:

```typescript
export interface SyncPreviewSkill {
  name: string;
  localSkillId: string;
  description: string;
  path?: string;  // NEW: 远程路径
}
```

- [ ] **Step 5: Run frontend build to verify**

```bash
cd D:/workspace/isdp/web && npm run build
```

Expected: Build succeeds

- [ ] **Step 6: Commit type changes**

```bash
git add web/src/types/index.ts
git commit -m "feat(skill): add sourcePath to frontend Skill types"
```

---

### Task 7: Frontend Skill Card Path Display

**Files:**
- Modify: `web/src/pages/SkillLibrary/index.tsx:1120-1148`

- [ ] **Step 1: Add path display in skill card**

Find the skill card content area (after the description paragraph, before tags). Add path display:

```tsx
{/* 描述区域 */}
<Paragraph
  ellipsis={{ rows: 2 }}
  style={{ marginBottom: 4, fontSize: 13, minHeight: 44, maxHeight: 44 }}
>
  {skill.description || '暂无描述'}
</Paragraph>

{/* 路径区域 - 仅联邦类型显示 */}
{skill.sourceType === 'federated' && skill.sourcePath && (
  <div style={{ marginBottom: 4 }}>
    <Text type="secondary" style={{ fontSize: 12 }}>
      路径: {skill.sourcePath}
    </Text>
  </div>
)}

{/* 标签区域 */}
<div style={{ height: 32, marginBottom: 4, overflow: 'hidden' }}>
  {skill.tags && skill.tags.length > 0 && (
    // ... existing tags code
  )}
</div>
```

- [ ] **Step 2: Run frontend build to verify**

```bash
cd D:/workspace/isdp/web && npm run build
```

Expected: Build succeeds

- [ ] **Step 3: Commit card display changes**

```bash
git add web/src/pages/SkillLibrary/index.tsx
git commit -m "feat(skill): display sourcePath in skill cards for federated skills"
```

---

### Task 8: Frontend Conflict Modal Path Display

**Files:**
- Modify: `web/src/pages/SkillLibrary/index.tsx` (conflict modal)
- Modify: `web/src/pages/RegistryManagement/index.tsx` (sync conflict modal)

- [ ] **Step 1: Add path column to SkillLibrary conflict modal**

Find the conflict modal table columns (around line 1422-1495) and add path column:

```tsx
<Table
  dataSource={conflictItems}
  columns={[
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 120,
    },
    {
      title: '路径',
      dataIndex: 'path',
      key: 'path',
      width: 150,
      render: (path: string) => <Text type="secondary" style={{ fontSize: 12 }}>{path || '-'}</Text>,
    },
    {
      title: '本地来源',
      key: 'localSource',
      width: 120,
      render: (_, record) => {
        // ... existing code
      },
    },
    // ... rest of columns
  ]}
/>
```

- [ ] **Step 2: Add path column to RegistryManagement sync conflict modal**

Find the sync conflict modal table columns and add path column:

```tsx
columns={[
  {
    title: '名称',
    dataIndex: 'name',
    key: 'name',
    width: 120,
  },
  {
    title: '路径',
    dataIndex: 'path',
    key: 'path',
    width: 150,
    render: (path: string) => <Text type="secondary" style={{ fontSize: 12 }}>{path || '-'}</Text>,
  },
  {
    title: '本地来源',
    // ... existing code
  },
  // ... rest of columns
]}
```

- [ ] **Step 3: Run frontend build to verify**

```bash
cd D:/workspace/isdp/web && npm run build
```

Expected: Build succeeds

- [ ] **Step 4: Commit modal changes**

```bash
git add web/src/pages/SkillLibrary/index.tsx web/src/pages/RegistryManagement/index.tsx
git commit -m "feat(skill): add path column to conflict modals"
```

---

### Task 9: Integration Test

- [ ] **Step 1: Apply database migration**

Run the migration on the local database:

```bash
cd D:/workspace/isdp
go build -o bin/migrate.exe ./cmd/migrate
bin/migrate.exe up --db ./data/sqlite/colink.db --version 1.2.6
```

Expected: Migration succeeds, `source_path` column added

- [ ] **Step 2: Restart backend service**

```bash
go run ./cmd/server
```

- [ ] **Step 3: Test API with curl**

Test that sourcePath is returned in API responses:

```bash
curl http://localhost:26305/api/v1/skills | jq '.data[0].sourcePath'
```

Expected: Returns `sourcePath` field (empty string for existing skills)

- [ ] **Step 4: Test FindBySourcePath**

Import a skill with a specific path, then verify it can be found by the triple:

```bash
# After importing, test sync-preview
curl -X POST http://localhost:26305/api/v1/registries/{id}/sync-preview | jq '.autoUpdateSkills[0].path'
```

Expected: Returns the path for matched skills

- [ ] **Step 5: Commit test verification**

No code change needed - verification complete.

---

## Self-Review Checklist

**1. Spec coverage:**
- [x] Database migration - Task 1
- [x] Go model SourcePath field - Task 2
- [x] Repo FindBySourcePath - Task 3
- [x] Import stores SourcePath - Task 4
- [x] Sync uses FindBySourcePath - Task 5
- [x] Frontend types - Task 6
- [x] Card path display - Task 7
- [x] Modal path display - Task 8
- [x] Integration test - Task 9

**2. Placeholder scan:**
- No TBD, TODO, or vague instructions found
- All code steps include complete implementation

**3. Type consistency:**
- `SourcePath` (Go) matches `sourcePath` (JSON/TypeScript)
- `FindBySourcePath` method signature matches usage
- All SELECT queries include `source_path`
- All INSERT/UPDATE include `source_path`