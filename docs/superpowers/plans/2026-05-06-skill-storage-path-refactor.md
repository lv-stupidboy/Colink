# Skill 存储路径重构实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 skill 存储路径从 `{skillName}` 改为 `{skillId}`，解决同名 skill 文件覆盖问题，并割接历史数据。

**Architecture:** 后端 11 个文件改用 `skill.ID.String()` 作为目录名（4 个写路径、7 个读路径）；installer-tauri 新增割接函数在升级流程中自动迁移历史文件。

**Tech Stack:** Go (后端), Rust (installer-tauri), SQLite (数据查询)

---

## File Structure

| 项目 | 文件 | 变更类型 | 路径类型 |
|------|------|---------|---------|
| isdp | `internal/service/skill/skill_scanner.go:579` | 修改目录名 | 写路径 |
| isdp | `internal/service/skill/service.go:273` | 修改删除路径 | 写路径 |
| isdp | `internal/api/skill_handler.go:430` | 修改上传路径 | 写路径 |
| isdp | `internal/api/skill_handler.go:598` | 修改仓库导入路径 | 写路径 |
| isdp | `internal/service/configgen/downloader.go:49` | 修改技能源目录 | 读路径 |
| isdp | `internal/service/agent/plugins/claude_code/config_generator.go:165` | 修改技能源目录 | 读路径 |
| isdp | `internal/service/agent/plugins/open_code/config_generator.go:168` | 修改技能源目录 | 读路径 |
| isdp | `internal/service/assetpackage/service.go:109` | 修改导出路径 | 读路径 |
| isdp | `internal/service/assetpackage/service.go:626` | 修改导入路径 | 读路径 |
| isdp | `internal/service/teampackage/service.go:320` | 修改导出路径 | 读路径 |
| isdp | `internal/service/teampackage/service.go:1034` | 修改导入路径 | 读路径 |
| installer-tauri | `src-tauri/src/services/installer.rs` | 新增割接函数 + 集成 | 割接 |
| installer-tauri | `src-tauri/Cargo.toml` | 新增 rusqlite 依赖 | 割接 |

---

### Task 1: 修改 SkillScanner ImportSkills 目录名

**Files:**
- Modify: `internal/service/skill/skill_scanner.go:579`

- [ ] **Step 1: 修改 ImportSkills 中 dstDir 变量**

当前代码（第 579 行）：
```go
dstDir := filepath.Join(s.storagePath, item.Name)
```

修改为：
```go
dstDir := filepath.Join(s.storagePath, skill.ID.String())
```

注意：skill 变量在第 588 行创建，需要将 dstDir 的计算移到 skill 创建之后。

**完整变更**：将第 577-585 行的目录复制逻辑移到第 588-601 行的 skill 创建之后：

```go
// 创建 Skill 记录
skill := &model.Skill{
    ID:               uuid.New(),
    Name:             item.Name,
    Description:      item.Description,
    Tags:             item.Tags,
    SourceType:       model.SkillSourceFederated,
    SourceRegistryID: registry.ID,
    SupportedAgents:  item.SupportedAgents,
    IsPublic:         true,
    Status:           model.SkillStatusActive,
    UseCount:         0,
    CreatedAt:        time.Now(),
    UpdatedAt:        time.Now(),
}

// 复制技能目录（使用 skill.ID 作为目录名）
srcDir := filepath.Join(tempDir, item.Path)
dstDir := filepath.Join(s.storagePath, skill.ID.String())

if err := s.copySkillDirectory(srcDir, dstDir); err != nil {
    nameMu.Unlock()
    errChan <- fmt.Errorf("复制技能目录 %s 失败: %w", item.Name, err)
    return
}
```

- [ ] **Step 2: 构建后端验证**

Run: `go build ./cmd/server`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交变更**

```bash
git add internal/service/skill/skill_scanner.go
git commit -m "refactor(skill): ImportSkills 使用 skill.ID 作为目录名"
```

---

### Task 2: 修改 Service Delete 目录路径

**Files:**
- Modify: `internal/service/skill/service.go:273`

- [ ] **Step 1: 修改 Delete 方法中的目录路径**

当前代码（第 273 行）：
```go
skillDir := filepath.Join(s.storagePath, skillRecord.Name)
```

修改为：
```go
skillDir := filepath.Join(s.storagePath, skillRecord.ID.String())
```

- [ ] **Step 2: 构建后端验证**

Run: `go build ./cmd/server`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交变更**

```bash
git add internal/service/skill/service.go
git commit -m "refactor(skill): Delete 使用 skill.ID 作为目录名"
```

---

### Task 3: 修改 SkillHandler Upload 目录路径

**Files:**
- Modify: `internal/api/skill_handler.go:430`

- [ ] **Step 1: 修改 Upload 方法中的目录路径**

当前代码（第 430 行）：
```go
skillDir := filepath.Join(storagePath, skillRecord.Name)
```

修改为：
```go
skillDir := filepath.Join(storagePath, skillRecord.ID.String())
```

- [ ] **Step 2: 构建后端验证**

Run: `go build ./cmd/server`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交变更**

```bash
git add internal/api/skill_handler.go
git commit -m "refactor(skill): Upload 使用 skill.ID 作为目录名"
```

---

### Task 4: 修改 SkillHandler ImportFromRepo 目录路径

**Files:**
- Modify: `internal/api/skill_handler.go:598`

- [ ] **Step 1: 修改 ImportFromRepo 方法中的目录路径**

当前代码（第 598 行）：
```go
skillDir := filepath.Join(h.storagePath, skillRecord.Name)
```

修改为：
```go
skillDir := filepath.Join(h.storagePath, skillRecord.ID.String())
```

- [ ] **Step 2: 构建后端验证**

Run: `go build ./cmd/server`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交变更**

```bash
git add internal/api/skill_handler.go
git commit -m "refactor(skill): ImportFromRepo 使用 skill.ID 作为目录名"
```

---

### Task 5: 修改 Downloader DownloadSkill 源目录路径

**Files:**
- Modify: `internal/service/configgen/downloader.go:49`

- [ ] **Step 1: 修改 DownloadSkill 方法中的源目录路径**

当前代码（第 49 行）：
```go
sourceDir := filepath.Join(d.skillStoragePath, skill.Name)
```

修改为：
```go
sourceDir := filepath.Join(d.skillStoragePath, skill.ID.String())
```

- [ ] **Step 2: 构建后端验证**

Run: `go build ./cmd/server`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交变更**

```bash
git add internal/service/configgen/downloader.go
git commit -m "refactor(configgen): DownloadSkill 使用 skill.ID 作为源目录名"
```

---

### Task 6: 修改 ClaudeCode ConfigGenerator copySkill 源目录路径

**Files:**
- Modify: `internal/service/agent/plugins/claude_code/config_generator.go:165`

- [ ] **Step 1: 修改 copySkill 方法中的源目录路径**

当前代码（第 165 行）：
```go
sourceDir := filepath.Join(g.skillStoragePath, skill.Name)
```

修改为：
```go
sourceDir := filepath.Join(g.skillStoragePath, skill.ID.String())
```

- [ ] **Step 2: 构建后端验证**

Run: `go build ./cmd/server`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交变更**

```bash
git add internal/service/agent/plugins/claude_code/config_generator.go
git commit -m "refactor(claude_code): copySkill 使用 skill.ID 作为源目录名"
```

---

### Task 7: 修改 OpenCode ConfigGenerator copySkill 源目录路径

**Files:**
- Modify: `internal/service/agent/plugins/open_code/config_generator.go:168`

- [ ] **Step 1: 修改 copySkill 方法中的源目录路径**

当前代码（第 168 行）：
```go
sourceDir := filepath.Join(g.skillStoragePath, skill.Name)
```

修改为：
```go
sourceDir := filepath.Join(g.skillStoragePath, skill.ID.String())
```

- [ ] **Step 2: 构建后端验证**

Run: `go build ./cmd/server`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交变更**

```bash
git add internal/service/agent/plugins/open_code/config_generator.go
git commit -m "refactor(open_code): copySkill 使用 skill.ID 作为源目录名"
```

---

### Task 8: 修改 AssetPackage Export 技能目录路径

**Files:**
- Modify: `internal/service/assetpackage/service.go:109`

- [ ] **Step 1: 修改 Export 方法中的技能目录路径**

当前代码（第 109 行）：
```go
skillDir := filepath.Join(s.skillStoragePath, skill.Name)
```

修改为：
```go
skillDir := filepath.Join(s.skillStoragePath, skill.ID.String())
```

- [ ] **Step 2: 构建后端验证**

Run: `go build ./cmd/server`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交变更**

```bash
git add internal/service/assetpackage/service.go
git commit -m "refactor(assetpackage): Export 使用 skill.ID 作为目录名"
```

---

### Task 9: 修改 AssetPackage importSkill 目标目录路径

**Files:**
- Modify: `internal/service/assetpackage/service.go:626`

- [ ] **Step 1: 修改 importSkill 方法中的目标目录路径**

当前代码（第 626 行）：
```go
targetDir := filepath.Join(s.skillStoragePath, item.Name)
```

修改为：
```go
targetDir := filepath.Join(s.skillStoragePath, skill.ID.String())
```

注意：skill 变量在第 639 行创建，需要将 targetDir 的计算移到 skill 创建之后。

- [ ] **Step 2: 构建后端验证**

Run: `go build ./cmd/server`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交变更**

```bash
git add internal/service/assetpackage/service.go
git commit -m "refactor(assetpackage): importSkill 使用 skill.ID 作为目录名"
```

---

### Task 10: 修改 TeamPackage Export 技能目录路径

**Files:**
- Modify: `internal/service/teampackage/service.go:320`

- [ ] **Step 1: 修改 Export 方法中的技能目录路径**

当前代码（第 320 行）：
```go
skillDir := filepath.Join(s.skillStoragePath, skill.Name)
```

修改为：
```go
skillDir := filepath.Join(s.skillStoragePath, skill.ID.String())
```

- [ ] **Step 2: 构建后端验证**

Run: `go build ./cmd/server`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交变更**

```bash
git add internal/service/teampackage/service.go
git commit -m "refactor(teampackage): Export 使用 skill.ID 作为目录名"
```

---

### Task 11: 修改 TeamPackage importSkill 目标目录路径

**Files:**
- Modify: `internal/service/teampackage/service.go:1034`

- [ ] **Step 1: 修改 importSkill 方法中的目标目录路径**

当前代码（第 1034 行）：
```go
targetDir := filepath.Join(s.skillStoragePath, item.Name)
```

修改为：
```go
targetDir := filepath.Join(s.skillStoragePath, skill.ID.String())
```

注意：skill 变量在第 1043 行创建，需要将 targetDir 的计算移到 skill 创建之后。

- [ ] **Step 2: 构建后端验证**

Run: `go build ./cmd/server`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交变更**

```bash
git add internal/service/teampackage/service.go
git commit -m "refactor(teampackage): importSkill 使用 skill.ID 作为目录名"
```

---

### Task 12: installer-tauri 新增 rusqlite 依赖

**Files:**
- Modify: `installer-tauri/src-tauri/Cargo.toml`

- [ ] **Step 1: 添加 rusqlite 依赖**

在 `[dependencies]` 部分末尾添加：
```toml
rusqlite = "0.31"
```

- [ ] **Step 2: 构建验证**

Run: `cd installer-tauri && pnpm build`
Expected: 编译成功，rusqlite 依赖下载完成

- [ ] **Step 3: 提交变更**

```bash
git add installer-tauri/src-tauri/Cargo.toml
git commit -m "feat(installer): 新增 rusqlite 依赖用于割接 skill 存储"
```

---

### Task 13: installer-tauri 新增割接函数

**Files:**
- Modify: `installer-tauri/src-tauri/src/services/installer.rs`

- [ ] **Step 1: 添加 migrate_skill_storage 函数**

在文件末尾（`run_installation` 函数之后）添加：

```rust
/// Migrate skill storage directories from {name} to {id}
fn migrate_skill_storage(db_path: &Path, skills_dir: &Path) -> Result<String> {
    if !db_path.exists() {
        return Ok("数据库不存在，跳过割接".to_string());
    }

    if !skills_dir.exists() {
        return Ok("skills 目录不存在，跳过割接".to_string());
    }

    // 连接 SQLite 数据库
    let conn = rusqlite::Connection::open(db_path)
        .map_err(|e| InstallerError::Io {
            context: "open database for skill migration".to_string(),
            source: std::io::Error::new(std::io::ErrorKind::Other, e),
        })?;

    // 查询所有 skill
    let mut stmt = conn.prepare("SELECT id, name FROM skills")
        .map_err(|e| InstallerError::Io {
            context: "prepare skill query".to_string(),
            source: std::io::Error::new(std::io::ErrorKind::Other, e),
        })?;

    let skills = stmt.query_map([], |row| {
        Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?))
    })
        .map_err(|e| InstallerError::Io {
            context: "query skills".to_string(),
            source: std::io::Error::new(std::io::ErrorKind::Other, e),
        })?
        .collect::<Result<Vec<_>, _>>()
        .map_err(|e| InstallerError::Io {
            context: "collect skill results".to_string(),
            source: std::io::Error::new(std::io::ErrorKind::Other, e),
        })?;

    let mut migrated = 0;
    let mut skipped = 0;

    // 遍历每个 skill
    for (id, name) in skills {
        let src_dir = skills_dir.join(&name);
        let dst_dir = skills_dir.join(&id);

        if src_dir.exists() {
            // 目标目录已存在：删除后重命名
            if dst_dir.exists() {
                std::fs::remove_dir_all(&dst_dir)
                    .map_err(|e| InstallerError::Io {
                        context: format!("remove existing dst dir: {}", dst_dir.display()),
                        source: e,
                    })?;
            }
            std::fs::rename(&src_dir, &dst_dir)
                .map_err(|e| InstallerError::Io {
                    context: format!("rename skill dir: {} -> {}", src_dir.display(), dst_dir.display()),
                    source: e,
                })?;
            migrated += 1;
            log::info!("Migrated skill directory: {} -> {}", name, id);
        } else {
            skipped += 1;
            log::info!("Skipped skill (directory not found): {}", name);
        }
    }

    Ok(format!("割接完成: {} 个迁移, {} 个跳过", migrated, skipped))
}
```

- [ ] **Step 2: 构建验证**

Run: `cd installer-tauri && pnpm build`
Expected: 编译成功，无类型错误

- [ ] **Step 3: 提交变更**

```bash
git add installer-tauri/src-tauri/src/services/installer.rs
git commit -m "feat(installer): 新增 migrate_skill_storage 函数"
```

---

### Task 14: installer-tauri 集成割接到安装流程

**Files:**
- Modify: `installer-tauri/src-tauri/src/services/installer.rs`

- [ ] **Step 1: 在 copy_files_and_complete 函数中集成割接步骤**

找到数据库迁移完成的位置（约第 732-742 行），在 migration 成功后、config 步骤前添加：

```rust
// 在 migration 成功分支（第 726-732 行）之后添加：
// Step: Skill storage migration
emit_progress(&InstallProgress {
    step: "skillstorage".to_string(),
    status: "running".to_string(),
    progress: Some(75),
    message: Some("割接 Skill 存储路径...".into()),
    details: None,
});

let skills_dir = install_dir
    .join("data")
    .join("agent-assets")
    .join("skills");

let db_path = install_dir
    .join("data")
    .join("sqlite")
    .join("colink.db");

let migration_result = migrate_skill_storage(&db_path, &skills_dir)?;

emit_progress(&InstallProgress {
    step: "skillstorage".to_string(),
    status: "success".to_string(),
    progress: Some(80),
    message: Some("Skill 存储路径割接完成".into()),
    details: Some(migration_result),
});

// 注意：后续 config 步骤的 progress 需从 80 开始调整
```

同时调整后续步骤的 progress 值：
- config: 80 → 85
- shortcut: 85 → 90
- registry: 95 → 95
- complete: 100 → 100

- [ ] **Step 2: 构建验证**

Run: `cd installer-tauri && pnpm build`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交变更**

```bash
git add installer-tauri/src-tauri/src/services/installer.rs
git commit -m "feat(installer): 集成 skill 存储割接到升级流程"
```

---

### Task 15: 测试验证

- [ ] **Step 1: 启动后端服务**

Run: `go run ./cmd/server`
Expected: 服务启动成功，监听 26305 端口

- [ ] **Step 2: 测试新建 skill**

API 测试：
```bash
curl -X POST http://localhost:26305/api/v1/skills \
  -H "Content-Type: application/json" \
  -d '{"name":"test-skill-id-path","description":"测试 ID 路径","sourceType":"personal","supportedAgents":["claude"]}'
```

Expected: 返回 201，包含 skill ID

验证目录：
```bash
ls ./data/agent-assets/skills/{skillId}/
```
Expected: 目录使用 UUID 作为名称，而非 "test-skill-id-path"

- [ ] **Step 3: 测试联邦导入**

通过前端 UI 测试联邦导入，验证目录使用 UUID。

- [ ] **Step 4: 测试 Agent 配置生成**

创建一个 Agent 角色并绑定 skill，然后执行配置生成：
```bash
curl -X POST http://localhost:26305/api/v1/config/generate-agent \
  -H "Content-Type: application/json" \
  -d '{"agentRoleId":"{agentRoleId}","baseAgentType":"claude_code","cleanExisting":true}'
```

验证生成的配置目录中 skill 文件正确复制：
```bash
ls ./data/{agentRoleId}/skills/{skillId}/
```
Expected: skill 文件存在于 UUID 目录下

- [ ] **Step 5: 测试资产包导出/导入**

导出包含 skill 的资产包，验证文件正确打包。导入资产包，验证解压到 UUID 目录。

- [ ] **Step 6: 测试割接脚本**

构建 installer-tauri 后，在升级场景测试割接是否执行。

---

## Summary

| Task | 文件 | 变更 | 路径类型 |
|------|------|------|---------|
| 1 | `skill_scanner.go` | ImportSkills 使用 skill.ID | 写路径 |
| 2 | `service.go` | Delete 使用 skill.ID | 写路径 |
| 3 | `skill_handler.go` | Upload 使用 skill.ID | 写路径 |
| 4 | `skill_handler.go` | ImportFromRepo 使用 skill.ID | 写路径 |
| 5 | `downloader.go` | DownloadSkill 使用 skill.ID | 读路径 |
| 6 | `claude_code/config_generator.go` | copySkill 使用 skill.ID | 读路径 |
| 7 | `open_code/config_generator.go` | copySkill 使用 skill.ID | 读路径 |
| 8 | `assetpackage/service.go` | Export 使用 skill.ID | 读路径 |
| 9 | `assetpackage/service.go` | importSkill 使用 skill.ID | 读路径 |
| 10 | `teampackage/service.go` | Export 使用 skill.ID | 读路径 |
| 11 | `teampackage/service.go` | importSkill 使用 skill.ID | 读路径 |
| 12 | `Cargo.toml` | 新增 rusqlite 依赖 | 割接 |
| 13 | `installer.rs` | 新增割接函数 | 割接 |
| 14 | `installer.rs` | 集成到升级流程 | 割接 |
| 15 | - | 测试验证 | - |

---

<!-- GOVERNANCE_DIGEST_VERSION: v1.3.1 -->