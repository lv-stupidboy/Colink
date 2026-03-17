# ISDP 数据库版本控制与团队协作方案

## 概述

本方案旨在解决ISDP项目团队在数据库协作开发中遇到的问题，提供一种标准化的数据库变更管理方法，确保团队成员之间数据库结构的一致性和可追溯性。

## 方案核心理念

- **基线一致**：以远程数据库作为统一的基准
- **变更追踪**：将所有数据库变更记录为迁移脚本
- **顺序执行**：按时间顺序应用迁移脚本
- **版本归档**：迁移脚本永久存档，便于追溯

## 数据库迁移脚本规范

### 命名规范

迁移脚本使用以下命名格式：

```
YYYYMMDDHHMMSS_description.sql
```

例如：
- `202403170001_add_workflow_templates_table.sql`
- `202403170002_update_tables_for_a2a_interaction.sql`

### 内容规范

每个迁移脚本应包含以下部分：

```sql
-- Migration: YYYYMMDDHHMMSS_description
-- Description: 描述迁移的目的和影响
-- Author: 开发者姓名或团队
-- Based on: 基于哪个版本或对比哪个版本

-- UP (应用此迁移)
-- 正向SQL语句

-- DOWN (回滚此迁移)
/*
-- 逆向SQL语句
-- 对于SQLite，某些操作不可逆，需谨慎考虑
*/
```

## 使用指南

### 1. 团队成员初始化

当新成员加入团队或需要同步数据库状态时：

```bash
# 确保你有最新的代码
git pull origin main

# 应用所有数据库迁移
bash scripts/migrate.sh up
```

### 2. 执行数据库变更

当你需要对数据库结构进行变更时：

1. **备份当前数据库**
```bash
cp data/isdp.db data/isdp.db.backup.$(date +%Y%m%d_%H%M%S)
```

2. **创建新的迁移脚本**
```bash
bash scripts/migrate.sh new add_new_feature_columns
```

3. **编辑迁移脚本**，填写UP和DOWN操作

4. **在开发环境中测试迁移**
```bash
bash scripts/migrate.sh up
```

5. **提交迁移脚本到版本控制系统**
```bash
git add scripts/YYYYMMDDHHMMSS_*.sql
git commit -m "db: add migration for new feature"
git push origin main
```

### 3. 查看数据库状态

```bash
bash scripts/migrate.sh status
```

## 迁移脚本示例

```sql
-- Migration: 202403170001_add_workflow_templates_table
-- Description: 添加工作流模板表以支持可配置的工作流
-- Author: ISDP Team

-- UP
CREATE TABLE workflow_templates (
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    agent_ids BLOB NOT NULL,
    checkpoints BLOB,
    estimated_time TEXT,
    is_system BOOLEAN NOT NULL DEFAULT 0,
    is_default BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- Add workflow_template_id column to projects table
ALTER TABLE projects ADD COLUMN workflow_template_id TEXT;
ALTER TABLE projects ADD CONSTRAINT fk_projects_workflow_template
    FOREIGN KEY (workflow_template_id) REFERENCES workflow_templates(id) ON DELETE SET NULL;

-- DOWN (reverse migration)
/*
-- 注意：SQLite不支持直接删除列和外键约束
-- 要完全回滚此操作需要重建表
DROP TABLE workflow_templates;
*/
```

## 最佳实践

### 对于DDL变更（表结构修改）

- 始终在迁移脚本中使用事务
- 仔细考虑添加外键约束
- 在DROP操作前考虑是否应该使用SET NULL或CASCADE
- 对于大型表的结构修改，考虑对业务的影响

### 对于DML变更（数据修改）

- 将数据迁移与结构迁移分开
- 编写可重复执行的数据迁移脚本
- 对重要的数据迁移操作进行充分测试

### 通用原则

- 迁移脚本应向后兼容
- 每个迁移脚本只做一件事
- 在应用迁移前先备份数据库
- 在团队环境中，迁移应经过代码审查

## 工具支持

当前的迁移工具支持以下命令：

- `migrate.sh status`: 显示数据库状态
- `migrate.sh up`: 应用所有待处理的迁移
- `migrate.sh new <name>`: 创建新的迁移脚本

## 注意事项

- SQLite在ALTER TABLE方面的功能有限，某些复杂的表结构变更可能需要特殊处理
- 团队协作时，确保在应用迁移前与团队成员同步
- 定期备份数据库，特别是在应用重大变更前
- 在生产环境中应用迁移前，务必在预发布环境充分测试