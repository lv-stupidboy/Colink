# Skill 存储路径重构设计

## 需求背景

当前联邦源导入的 skill 存储路径为 `./data/agent-assets/skills/{skillName}/SKILL.md`。由于项目已允许 skill 重名（唯一约束已删除），导入同名 skill会导致 SKILL.md 文件相互覆盖。

**需求**：设计存储路径方案解决同名覆盖问题，并割接历史数据。

## 设计决策

### 决策过程

| 问题 | 决策 | 理由 |
|------|------|------|
| 是否有同名 skill | 无，预防性问题 | 用户确认 A |
| 历史兼容方式 | 激进方案：全部改用 skillId | 用户确认 A，一劳永逸 |
| 方案类型 | 数据库 + 文件割接 | 用户确认 A |
| 是否新增 storage_path 字段 | 否，直接用 skill.ID | 字段冗余，用户质疑后简化 |
|割接脚本集成位置 | installer-tauri 升级流程 | 用户指定 |

## 核心架构变更

### 存储路径策略

**变更前**：`./data/agent-assets/skills/{skillName}/SKILL.md`

**变更后**：`./data/agent-assets/skills/{skillId}/SKILL.md`

使用 UUID 作为目录名，天然唯一，无需运行时检测逻辑。

### 数据库变更

**无需 schema 变更**（简化方案）。

直接使用 `skill.ID.String()` 作为目录名运行时计算。

## 代码变更范围

### 写路径变更（创建/导入/上传）

| 文件 | 函数/位置 | 变更内容 |
|------|----------|---------|
| `internal/service/skill/skill_scanner.go:579` | `ImportSkills()` | `dstDir := filepath.Join(s.storagePath, skill.ID.String())` |
| `internal/service/skill/service.go:273` | `Create()` | `skillDir := filepath.Join(s.storagePath, skillRecord.ID.String())` |
| `internal/api/skill_handler.go:430` | `Upload()` | `skillDir := filepath.Join(storagePath, skillRecord.ID.String())` |
| `internal/api/skill_handler.go:598` | `ImportFromRepo()` | `skillDir := filepath.Join(h.storagePath, skillRecord.ID.String())` |

### 读路径变更（配置生成/导出/导入）

| 文件 | 函数/位置 | 变更内容 |
|------|----------|---------|
| `internal/service/configgen/downloader.go:49` | `DownloadSkill()` | `sourceDir := filepath.Join(d.skillStoragePath, skill.ID.String())` |
| `internal/service/agent/plugins/claude_code/config_generator.go:165` | `copySkill()` | `sourceDir := filepath.Join(g.skillStoragePath, skill.ID.String())` |
| `internal/service/agent/plugins/open_code/config_generator.go:168` | `copySkill()` | `sourceDir := filepath.Join(g.skillStoragePath, skill.ID.String())` |
| `internal/service/assetpackage/service.go:109` | `Export()` | `skillDir := filepath.Join(s.skillStoragePath, skill.ID.String())` |
| `internal/service/assetpackage/service.go:626` | `importSkill()` | `targetDir := filepath.Join(s.skillStoragePath, skill.ID.String())` |
| `internal/service/teampackage/service.go:320` | `Export()` | `skillDir := filepath.Join(s.skillStoragePath, skill.ID.String())` |
| `internal/service/teampackage/service.go:1034` | `importSkill()` | `targetDir := filepath.Join(s.skillStoragePath, skill.ID.String())` |

### 变更模式

所有 `{skill.Name}` 目录名改为 `{skill.ID.String()}`：

```go
// 变更前
skillDir := filepath.Join(storagePath, skill.Name)

// 变更后
skillDir := filepath.Join(storagePath, skill.ID.String())
```

## 割接脚本设计

### 集成位置

割接逻辑集成到 `installer-tauri` 升级流程，在数据库迁移之后执行。

**文件**：`installer-tauri/src-tauri/src/services/installer.rs`

### 集成步骤

在 `run_installation` 函数中，数据库迁移之后新增：

```rust
// Step: Skill storage migration（在 migration 之后，config 之前）
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

migrate_skill_storage(&db_path, &skills_dir)?;

emit_progress(&InstallProgress {
    step: "skillstorage".to_string(),
    status: "success".to_string(),
    progress: Some(80),
    message: Some("Skill 存储路径割接完成".into()),
    details: None,
});
```

###割接函数实现

```rust
/// Migrate skill storage directories from {name} to {id}
fn migrate_skill_storage(db_path: &Path, skills_dir: &Path) -> Result<String> {
    if !db_path.exists() {
        return Ok("数据库不存在，跳过割接".to_string());
    }

    if !skills_dir.exists() {
        return Ok("skills 目录不存在，跳过割接".to_string());
    }

    // 1. 连接 SQLite 数据库
    let conn = rusqlite::Connection::open(db_path)?;

    // 2. 查询所有 skill
    let mut stmt = conn.prepare("SELECT id, name FROM skills")?;
    let skills = stmt.query_map([], |row| {
        Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?))
    })?.collect::<Result<Vec<_>, _>>()?;

    let mut migrated = 0;
    let mut skipped = 0;

    // 3. 遍历每个 skill
    for (id, name) in skills {
        let src_dir = skills_dir.join(&name);
        let dst_dir = skills_dir.join(&id);

        if src_dir.exists() {
            // 目标目录已存在：删除后重命名
            if dst_dir.exists() {
                std::fs::remove_dir_all(&dst_dir)?;
            }
            std::fs::rename(&src_dir, &dst_dir)?;
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

### 边界处理

| 场景 | 处理方式 |
|------|---------|
| 数据库不存在 | 跳过割接，返回提示 |
| skills 目录不存在 | 跳过割接，返回提示 |
| 源目录不存在 | 跳过，记录日志 |
| 目标目录已存在 | 删除旧目录后重命名 |

### 依赖新增

installer-tauri 需新增 `rusqlite` 依赖：

```toml
# installer-tauri/src-tauri/Cargo.toml
[dependencies]
rusqlite = "0.31"
```

## 测试要点

| 测试场景 | 验证内容 |
|---------|---------|
| 新建 skill | 文件存储在 `{skillId}` 目录 |
| 联邦导入 skill | 文件存储在 `{skillId}` 目录 |
| Upload skill | 文件存储在 `{skillId}` 目录 |
| 导入同名 skill | 文件不会覆盖（不同 ID 目录） |
| Agent 配置生成 | 正确从 `{skillId}` 目录读取文件 |
| 资产包导出 | 正确从 `{skillId}` 目录打包文件 |
| 资产包导入 | 正确解压到 `{skillId}` 目录 |
| 团队包导出/导入 | 正确处理 `{skillId}` 目录 |
|割接脚本执行 | 历史 skill 目录正确迁移 |
|割接后访问 skill | 文件路径正确解析 |
| installer-tauri 升级割接 | 自动执行，进度显示正确 |

## 影响范围汇总

| 项目 | 文件 | 变更类型 | 路径类型 |
|------|------|---------|---------|
| isdp | `internal/service/skill/skill_scanner.go` | 修改 | 写路径 |
| isdp | `internal/service/skill/service.go` | 修改 | 写路径 |
| isdp | `internal/api/skill_handler.go` | 修改 | 写路径 |
| isdp | `internal/service/configgen/downloader.go` | 修改 | 读路径 |
| isdp | `internal/service/agent/plugins/claude_code/config_generator.go` | 修改 | 读路径 |
| isdp | `internal/service/agent/plugins/open_code/config_generator.go` | 修改 | 读路径 |
| isdp | `internal/service/assetpackage/service.go` | 修改 | 读路径 |
| isdp | `internal/service/teampackage/service.go` | 修改 | 读路径 |
| installer-tauri | `src-tauri/src/services/installer.rs` | 新增函数 + 集成 | 割接 |
| installer-tauri | `src-tauri/Cargo.toml` | 新增依赖 | 割接 |

**总计**：10 个文件变更，其中 4 个写路径、7 个读路径、2 个割接相关。

## 实施顺序

1. **后端写路径变更** - 修改 4 个文件，改用 skillId 目录名（创建/导入/上传）
2. **后端读路径变更** - 修改 7 个文件，改用 skillId 目录名（配置生成/导出/导入）
3. **installer-tauri 割接逻辑** - 新增函数 + 集成到安装流程
4. **测试验证** - 新建 skill、联邦导入、Agent 配置生成、包导出导入、割接执行

---

<!-- GOVERNANCE_DIGEST_VERSION: v1.3.1 -->