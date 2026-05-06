# Skill 重名允许 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow Skills with duplicate names across all source types, with dropdowns showing descriptions in brackets to distinguish them.

**Architecture:** Delete the database unique constraint `uk_skills_name`, remove backend name uniqueness checks, and update frontend dropdowns to display `name (description)` format.

**Tech Stack:** Go (backend), SQLite (database), React + Ant Design (frontend), goose (migrations)

---

## Files to Create/Modify

| File | Action | Purpose |
|------|--------|---------|
| `sql-change/v1.2.5/sqlite/00011_drop_skill_name_unique.sql` | Create | Migration to drop unique constraint |
| `internal/service/skill/service.go` | Modify | Remove Create method name check (lines 118-128) |
| `internal/service/skill/skill_scanner.go` | Modify | Remove ImportSkills name check (lines 573-583) |
| `web/src/pages/AgentRoleList.tsx` | Modify | Update Skills dropdown label format (lines 1002-1014) |
| `web/src/pages/SubagentList.tsx` | Modify | Update Skills dropdown label format (lines 669-681) |
| `web/src/pages/CommandList.tsx` | Modify | Update Skills dropdown label format (lines 603-613) |
| `web/src/pages/Workflow/TeamGraphEditor/AgentDetailPanel.tsx` | Modify | Update Skills dropdown to show description (line 228) |

---

### Task 1: Create Database Migration Script

**Files:**
- Create: `sql-change/v1.2.5/sqlite/00011_drop_skill_name_unique.sql`

- [ ] **Step 1: Create the version directory and migration file**

```sql
-- v1.2.5/sqlite/00011_drop_skill_name_unique.sql
-- 删除 skills 表的名称唯一约束，允许 skill 重名

-- +goose Up
ALTER TABLE skills DROP INDEX uk_skills_name;

-- +goose Down
ALTER TABLE skills ADD UNIQUE INDEX uk_skills_name (name);
```

Create the file at: `D:\workspace\isdp\sql-change\v1.2.5\sqlite\00011_drop_skill_name_unique.sql`

- [ ] **Step 2: Verify the migration file structure**

Run: `ls -la sql-change/v1.2.5/sqlite/`
Expected: Directory exists with `00011_drop_skill_name_unique.sql`

- [ ] **Step 3: Commit**

```bash
git add sql-change/v1.2.5/
git commit -m "feat(db): add migration to drop skill name unique constraint"
```

---

### Task 2: Remove Backend Create Method Name Check

**Files:**
- Modify: `internal/service/skill/service.go:118-128`

- [ ] **Step 1: Delete the name uniqueness check in Create method**

Current code at `internal/service/skill/service.go` (lines 117-129):

```go
// Create 创建Skill
func (s *Service) Create(ctx context.Context, req *model.CreateSkillRequest) (*model.Skill, error) {
	// 检查名称是否重复
	existing, err := s.skillRepo.FindByName(ctx, req.Name)
	if err != nil {
		// 如果不是"未找到"错误，返回实际错误
		if !strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("检查技能名称失败: %w", err)
		}
		// 名称不存在，可以创建
	} else if existing != nil {
		return nil, errors.New("技能名称已存在")
	}

	// 只有 personal 类型才能设置私有
```

Replace with:

```go
// Create 创建Skill
func (s *Service) Create(ctx context.Context, req *model.CreateSkillRequest) (*model.Skill, error) {
	// 允许 skill 重名，不再检查名称唯一性

	// 只有 personal 类型才能设置私有
```

- [ ] **Step 2: Verify the backend compiles**

Run: `go build ./cmd/server`
Expected: Build succeeds without errors

- [ ] **Step 3: Commit**

```bash
git add internal/service/skill/service.go
git commit -m "feat(skill): remove name uniqueness check in Create method"
```

---

### Task 3: Remove ImportSkills Name Check

**Files:**
- Modify: `internal/service/skill/skill_scanner.go:573-583`

- [ ] **Step 1: Delete the name check in ImportSkills goroutine**

Current code at `internal/service/skill/skill_scanner.go` (lines 573-584):

```go
			// 检查是否已存在（加锁防止同批次重复）
			nameMu.Lock()
			existing, err := s.skillRepo.FindByName(ctx, item.Name)
			if err == nil && existing != nil {
				nameMu.Unlock()
				skipChan <- model.SkippedSkillInfo{
					Name:   item.Name,
					Reason: "同名技能已存在",
				}
				return
			}

			// 复制技能目录
```

Replace with:

```go
			// 允许 skill 重名，不再检查名称唯一性
			// 加锁防止同批次目录复制冲突（仅用于目录操作）
			nameMu.Lock()

			// 复制技能目录
```

- [ ] **Step 2: Verify the backend compiles**

Run: `go build ./cmd/server`
Expected: Build succeeds without errors

- [ ] **Step 3: Commit**

```bash
git add internal/service/skill/skill_scanner.go
git commit -m "feat(skill): remove name uniqueness check in ImportSkills"
```

---

### Task 4: Update AgentRoleList Skills Dropdown

**Files:**
- Modify: `web/src/pages/AgentRoleList.tsx:1002-1014`

- [ ] **Step 1: Modify the Select options to show description in brackets**

Current code at `web/src/pages/AgentRoleList.tsx` (lines 1002-1014):

```tsx
              options={skills.map(s => ({
                label: s.name,
                value: s.id,
                desc: s.description || '暂无描述',
              }))}
              optionRender={(option) => (
                <div style={{ display: 'flex', flexDirection: 'column' }}>
                  <span style={{ fontWeight: 500 }}>{option.label}</span>
                  <span style={{ fontSize: 12, color: '#999', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 400 }}>
                    {option.data?.desc}
                  </span>
                </div>
              )}
```

Replace with:

```tsx
              options={skills.map(s => ({
                label: `${s.name} (${s.description || '暂无描述'})`,
                value: s.id,
                desc: s.description || '暂无描述',
              }))}
              optionRender={(option) => (
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ fontWeight: 500 }}>{option.data?.label?.split(' (')[0]}</span>
                  <span style={{ fontSize: 12, color: '#999', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 300 }}>
                    ({option.data?.desc})
                  </span>
                </div>
              )}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No type errors

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/AgentRoleList.tsx
git commit -m "feat(ui): show skill description in brackets for AgentRole dropdown"
```

---

### Task 5: Update SubagentList Skills Dropdown

**Files:**
- Modify: `web/src/pages/SubagentList.tsx:669-681`

- [ ] **Step 1: Modify the Select options to show description in brackets**

Current code at `web/src/pages/SubagentList.tsx` (lines 669-681):

```tsx
              options={skills.map(s => ({
                label: s.name,
                value: s.id,
                desc: s.description || '暂无描述',
              }))}
              optionRender={(option) => (
                <div style={{ display: 'flex', flexDirection: 'column' }}>
                  <span style={{ fontWeight: 500 }}>{option.label}</span>
                  <span style={{ fontSize: 12, color: '#999', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 400 }}>
                    {option.data?.desc}
                  </span>
                </div>
              )}
```

Replace with:

```tsx
              options={skills.map(s => ({
                label: `${s.name} (${s.description || '暂无描述'})`,
                value: s.id,
                desc: s.description || '暂无描述',
              }))}
              optionRender={(option) => (
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ fontWeight: 500 }}>{option.data?.label?.split(' (')[0]}</span>
                  <span style={{ fontSize: 12, color: '#999', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 300 }}>
                    ({option.data?.desc})
                  </span>
                </div>
              )}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No type errors

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/SubagentList.tsx
git commit -m "feat(ui): show skill description in brackets for Subagent dropdown"
```

---

### Task 6: Update CommandList Skills Dropdown

**Files:**
- Modify: `web/src/pages/CommandList.tsx:603-613`

- [ ] **Step 1: Modify the Select options to show description in brackets**

Current code at `web/src/pages/CommandList.tsx` (lines 603-613):

```tsx
              options={allSkills.map(s => ({
                label: s.name,
                value: s.id,
                desc: s.description || '暂无描述',
              }))}
              optionRender={(option) => (
                <div style={{ display: 'flex', flexDirection: 'column' }}>
                  <span style={{ fontWeight: 500 }}>{option.label}</span>
                  <span style={{ fontSize: 12, color: '#999', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 400 }}>
                    {option.data?.desc}
                  </span>
```

Replace with:

```tsx
              options={allSkills.map(s => ({
                label: `${s.name} (${s.description || '暂无描述'})`,
                value: s.id,
                desc: s.description || '暂无描述',
              }))}
              optionRender={(option) => (
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ fontWeight: 500 }}>{option.data?.label?.split(' (')[0]}</span>
                  <span style={{ fontSize: 12, color: '#999', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 300 }}>
                    ({option.data?.desc})
                  </span>
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No type errors

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/CommandList.tsx
git commit -m "feat(ui): show skill description in brackets for Command dropdown"
```

---

### Task 7: Update Workflow AgentDetailPanel Skills Dropdown

**Files:**
- Modify: `web/src/pages/Workflow/TeamGraphEditor/AgentDetailPanel.tsx:228`

- [ ] **Step 1: Modify the Select options to show description in brackets**

Current code at `web/src/pages/Workflow/TeamGraphEditor/AgentDetailPanel.tsx` (line 228):

```tsx
                          options={skills.map(s => ({ label: s.name, value: s.id }))}
```

Replace with:

```tsx
                          options={skills.map(s => ({ label: `${s.name} (${s.description || '暂无描述'})`, value: s.id }))}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No type errors

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/Workflow/TeamGraphEditor/AgentDetailPanel.tsx
git commit -m "feat(ui): show skill description in brackets for Workflow AgentDetailPanel"
```

---

### Task 8: Run Migration and Verify

**Files:**
- None (verification task)

- [ ] **Step 1: Build the migrate tool**

Run: `go build -o bin/migrate.exe ./cmd/migrate`
Expected: Build succeeds

- [ ] **Step 2: Run the migration**

Run: `bin/migrate.exe up --db data/sqlite/colink.db --version 1.2.5`
Expected: Migration succeeds, output shows version 1.2.5 applied

- [ ] **Step 3: Verify the constraint is dropped**

Run: `sqlite3 data/sqlite/colink.db "PRAGMA index_list('skills');"`
Expected: Output does NOT contain `uk_skills_name`

- [ ] **Step 4: Final commit (if any remaining changes)**

```bash
git status
# If any uncommitted changes remain:
git add -A
git commit -m "feat(skill): complete skill duplicate name support"
```

---

## Testing Checklist

After implementation, verify:

1. **Backend Create Test**:
   - Create two skills with same name via API → Both succeed
   - API: `POST /api/skills` with same `name` field

2. **Backend Import Test**:
   - Import skill from federated source with name matching local skill → Import succeeds
   - Import creates new skill record with same name

3. **Frontend Dropdown Test**:
   - Open AgentRole binding Skills dropdown
   - Verify: Skills show `name (description)` format
   - Select skill → Correctly binds by ID

4. **Workflow Dropdown Test**:
   - Open Workflow AgentDetailPanel Skills dropdown
   - Verify: Skills show `name (description)` format

---

## Self-Review

**1. Spec Coverage:**
- ✓ Database: Task 1 drops unique constraint
- ✓ Backend Create: Task 2 removes name check
- ✓ Backend Import: Task 3 removes name check
- ✓ Frontend AgentRole: Task 4 updates dropdown
- ✓ Frontend Subagent: Task 5 updates dropdown
- ✓ Frontend Command: Task 6 updates dropdown
- ✓ Frontend Workflow: Task 7 updates dropdown
- ✓ Verification: Task 8 runs migration

**2. Placeholder Scan:**
- No TBD/TODO found
- All code blocks have complete content
- All file paths are exact

**3. Type Consistency:**
- `label` property: string with `name (description)` format
- `value` property: string (skill.id)
- `desc` property: string (description)
- All frontend components use consistent structure

---

## Summary

| Phase | Tasks | Files |
|-------|-------|-------|
| Database | Task 1 | 1 new file |
| Backend | Tasks 2-3 | 2 modified files |
| Frontend | Tasks 4-7 | 4 modified files |
| Verification | Task 8 | Migration execution |

Total: 8 tasks, 7 files touched.