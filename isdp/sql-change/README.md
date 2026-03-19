# 数据库变更管理说明

本目录用于管理 ISDP 平台的数据库 Schema 变更，确保团队成员协作时数据库结构的一致性。

## 团队数据库使用说明

### 数据库分类

| 数据库名 | 用途 | 权限 |
|---------|------|------|
| `product` | 正式数据库 | 团队共享，需谨慎操作 |
| `dev_xx` | 个人开发数据库 | 按团队人员命名，个人级私有 |

### 命名规范

- **正式数据库**: `product`
- **开发数据库**: `dev_<姓名拼音>`，例如：
  - `dev_zhangsan`
  - `dev_lisi`
  - `dev_wangwu`

### 使用规则

1. **开发阶段**: 在个人开发数据库 (`dev_xx`) 中进行开发和测试
2. **变更同步**: 开发完成后，将变更脚本提交到 `migrations/` 目录
3. **正式发布**: 经审核后，由负责人统一执行到 `product` 数据库
4. **账号管理**: 数据库账号密码通过团队内部渠道私下共享，**严禁提交到代码仓库**

### 连接配置

1. 复制配置模板：
```bash
cp configs/config.yaml.example configs/config.yaml
```

2. 修改 `configs/config.yaml`，填入真实的数据库连接信息：

```yaml
database:
  type: mysql
  mysql:
    host: <MySQL服务器地址>
    port: 3306
    database: <数据库名>  # product 或 dev_xx
    username: <用户名>
    password: <密码>
```

> ⚠️ **安全提醒**:
> - `config.yaml` 已加入 `.gitignore`，不会被提交到 Git
> - 真实的数据库连接信息请通过团队内部渠道获取

## 目录结构

```
sql-change/
├── README.md                          # 本说明文档
├── init_db_mysql.sql                  # MySQL 完整初始化脚本（最新版本）
└── migrations/                        # 增量变更脚本目录
    ├── 20260319_add_workflow_template_id_to_threads.sql
    └── ...
```

## 变更脚本命名规范

文件名格式：`YYYYMMDD_description.sql`

- **YYYYMMDD**: 变更日期（如 20260319）
- **description**: 简短描述，使用下划线分隔单词（如 add_column_xxx）

示例：
```
20260319_add_local_path_to_projects.sql
20260320_create_audit_log_table.sql
20260321_add_index_to_messages_created_at.sql
```

## 工作流程

### 1. 新增数据库变更

当需要修改数据库结构时：

1. 在 `migrations/` 目录下创建新的 SQL 文件
2. 文件名遵循命名规范
3. SQL 文件内容应包含：
   - 变更说明注释
   - 变更 SQL 语句
   - 回滚 SQL 语句（可选但推荐）

示例：
```sql
-- 变更说明：为 projects 表添加工作目录字段
-- 作者：张三
-- 日期：2026-03-19

-- 正向变更
ALTER TABLE projects ADD COLUMN local_path VARCHAR(512) NOT NULL DEFAULT '';

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE projects DROP COLUMN local_path;
```

### 2. 更新初始化脚本

每次新增变更后，需要同步更新 `init_db_mysql.sql`：

1. 将变更内容合并到初始化脚本
2. 更新版本号注释
3. 确保新环境初始化时能获得最新完整结构

### 3. 执行变更

在测试环境验证后，在生产环境执行：

```bash
# 连接 MySQL 执行变更脚本
mysql -h <host> -u <user> -p<password> <database> < migrations/20260319_xxx.sql
```

## 注意事项

1. **向后兼容**：变更应尽量保持向后兼容，避免破坏现有功能
2. **数据安全**：涉及数据迁移的变更，务必先备份
3. **测试验证**：所有变更必须先在测试环境验证
4. **团队同步**：变更完成后及时通知团队成员更新本地数据库

## 当前数据库版本

- **版本**: 2.2
- **最后更新**: 2026-03-19
- **数据库类型**: MySQL 8.0
- **字符集**: utf8mb4

## 表结构概览

| 表名 | 说明 |
|------|------|
| base_agents | 基础 Agent 配置（Claude、OpenAI 等） |
| workflow_templates | 工作流模板 |
| projects | 项目信息 |
| threads | 开发会话 |
| messages | 对话消息 |
| agent_configs | Agent 角色配置 |
| agent_invocations | Agent 调用记录 |
| artifacts | 开发产物 |
| sandboxes | 沙箱容器 |