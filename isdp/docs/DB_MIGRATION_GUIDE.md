# ISDP 数据库迁移指南

本文档介绍如何将 ISDP 项目从 SQLite 迁移到华为云 RDS MySQL。

## 1. 迁移前准备

### 1.1 华为云 RDS MySQL 准备

1. 创建华为云 RDS MySQL 实例（最小规格 2核4G 即可）
2. 创建数据库 `isdp_dev`
3. 配置白名单，允许开发者 IP 访问
4. 记录连接信息：
   - Host: `rm-xxx.mysql.rds.huaweicloud.com`
   - Port: `3306`
   - Username: `root`
   - Password: `xxx`

### 1.2 本地准备

确保已安装：
- Go 1.21+
- MySQL 客户端（用于执行 SQL 脚本）

## 2. 配置切换

### 2.1 更新配置文件

编辑 `isdp/configs/config.yaml`：

```yaml
database:
  type: mysql  # 切换到 MySQL

  path: ./data/isdp.db  # SQLite 路径（保留备份）

  mysql:
    host: "rm-xxx.mysql.rds.huaweicloud.com"
    port: 3306
    database: isdp_dev
    username: "root"
    password: "your_password"
    schema: dev_yourname  # 你的个人 schema
    charset: utf8mb4
    max_open_conns: 10
    max_idle_conns: 5
    conn_max_lifetime: 300
```

### 2.2 Schema 说明

MySQL 使用多 Schema 隔离策略：

| Schema | 用途 |
|--------|------|
| `shared` | 团队共享数据，用于联调 |
| `dev_{username}` | 个人开发数据空间 |

## 3. 初始化 MySQL 数据库

### 3.1 创建 Schema

```bash
# 设置环境变量
export MYSQL_HOST="rm-xxx.mysql.rds.huaweicloud.com"
export MYSQL_USER="root"
export MYSQL_PASS="your_password"
export MYSQL_DB="isdp_dev"

# 创建个人 schema
./scripts/schema.sh create dev_yourname

# 或创建共享 schema
./scripts/schema.sh create shared
```

### 3.2 查看现有 Schema

```bash
./scripts/schema.sh list
```

## 4. 数据迁移

### 4.1 从 SQLite 迁移数据到 MySQL

```bash
cd isdp
go run ./cmd/migrate/... ./data/isdp.db dev_yourname
```

输出示例：
```
Starting data migration from SQLite to MySQL...
Source: ./data/isdp.db
Target Schema: dev_yourname
  projects: 5 rows migrated
  base_agents: 2 rows migrated
  agent_configs: 3 rows migrated
  threads: 10 rows migrated
  messages: 156 rows migrated
  ...

Data migration completed successfully!
```

### 4.2 验证数据

连接到 MySQL 验证数据已正确迁移：

```bash
mysql -h $MYSQL_HOST -u $MYSQL_USER -p$MYSQL_PASS $MYSQL_DB -e "USE dev_yourname; SELECT COUNT(*) FROM projects;"
```

## 5. 启动服务

```bash
cd isdp
go run ./cmd/server/...
```

服务将使用 MySQL 数据库启动。

## 6. 回滚到 SQLite

如果需要回滚到 SQLite：

1. 修改 `config.yaml`：
   ```yaml
   database:
     type: sqlite
     path: ./data/isdp.db
   ```

2. 重启服务

## 7. 故障排查

### 7.1 连接失败

- 检查网络连通性：`ping rm-xxx.mysql.rds.huaweicloud.com`
- 检查白名单配置
- 检查用户名密码

### 7.2 Schema 不存在

```bash
./scripts/schema.sh create dev_yourname
```

### 7.3 数据迁移失败

- 确保 SQLite 数据库文件存在
- 确保目标 schema 已创建
- 查看错误日志定位问题

## 8. 清理 SQLite（可选）

验证 MySQL 运行稳定后，可以移除 SQLite 支持：

1. 删除本地 SQLite 文件：`rm ./data/isdp.db`
2. 修改配置，移除 SQLite 相关配置项

注意：建议保留 SQLite 支持一段时间，以便快速回滚。