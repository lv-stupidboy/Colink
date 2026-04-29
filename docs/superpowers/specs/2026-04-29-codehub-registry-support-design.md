# 联邦技能源支持 CodeHub 平台设计文档

## 概述

扩展联邦技能源（Skill Registry）支持华为内网 CodeHub 代码托管平台，实现 SSH Key 和 HTTPS 账号密码两种认证方式。

## 需求背景

- **平台**: 华为内网 CodeHub 代码托管服务
- **域名**: `codehub-g.huawei.com`
- **URL 格式**: 支持 HTTPS 和 SSH 两种格式
- **认证优先级**: SSH Key 优先，HTTPS 账号密码为备用
- **SSH Key 管理**: 使用系统全局 SSH Key（`~/.ssh/id_rsa`），无需在联邦源中配置

## 设计方案

### 1. 数据模型变更

#### 后端 RegistryType 扩展

**文件**: `internal/model/skill.go`

```go
const (
    RegistryTypeGitHub  RegistryType = "github"
    RegistryTypeGitLab  RegistryType = "gitlab"
    RegistryTypeAPI     RegistryType = "api"
    RegistryTypeCustom  RegistryType = "custom"
    RegistryTypeCodeHub RegistryType = "codehub"  // 新增
)
```

#### AuthConfig 字段扩展

CodeHub 类型的 `authConfig` 字段支持：

| 字段 | 类型 | 说明 |
|------|------|------|
| `username` | string | HTTPS 认证账号 |
| `password` | string | HTTPS 认证密码 |

SSH Key 使用系统全局配置（`~/.ssh/id_rsa`），无需存储到 authConfig。

#### 前端 RegistryType 扩展

**文件**: `web/src/types/index.ts`

```typescript
export type RegistryType = 'github' | 'gitlab' | 'api' | 'custom' | 'codehub';
```

### 2. 后端 buildCloneURL 逻辑

**文件**: `internal/service/skill/skill_scanner.go`

扩展 `buildCloneURL` 方法，支持 CodeHub 类型：

```go
case model.RegistryTypeCodeHub:
    url := registry.URL
    
    // SSH 格式：直接使用，依赖系统 SSH Key
    if strings.HasPrefix(url, "git@") {
        return url
    }
    
    // HTTPS 格式：注入账号密码
    if strings.HasPrefix(url, "https://") {
        username := registry.AuthConfig["username"]
        password := registry.AuthConfig["password"]
        
        if username != "" && password != "" {
            // https://{username}:{password}@codehub-g.huawei.com/xxx.git
            url = strings.Replace(url, "https://", 
                fmt.Sprintf("https://%s:%s@", username, password), 1)
        }
        return url
    }
    
    return url
```

#### URL 处理策略

| URL 格式 | 认证方式 | 处理逻辑 |
|----------|----------|----------|
| `git@codehub-g.huawei.com:xxx.git` | SSH Key | 直接使用原 URL，git 自动使用系统 SSH Key |
| `https://codehub-g.huawei.com/xxx.git` | HTTPS 账号密码 | 注入 `username:password` 到 URL |
| HTTPS 无 authConfig | 公开仓库 | 直接使用原 URL |

### 3. 前端联邦源管理页面

**文件**: `web/src/pages/skills/RegistryManagement.tsx`

扩展联邦源创建/编辑表单，支持 CodeHub 类型：

#### 类型选择下拉

新增 `codehub` 选项，显示名称为 "CodeHub 代码托管服务"。

#### 认证配置表单

当选择 `codehub` 类型时，显示：

| 字段 | 类型 | 说明 |
|------|------|------|
| URL | text | 仓库地址（支持 SSH 和 HTTPS 格式） |
| 用户名 | text | HTTPS 认证账号（可选，SSH 格式时可不填） |
| 密码 | password | HTTPS 认证密码（可选，SSH 格式时可不填） |

#### 提示信息

显示提示："SSH 格式 URL 将使用系统全局 SSH Key 认证，无需配置账号密码"

### 4. 密码安全存储

authConfig 中的 password 字段应加密存储，使用现有的敏感数据加密机制。

## 实施范围

### 后端变更

1. `internal/model/skill.go` - 新增 `RegistryTypeCodeHub` 常量
2. `internal/service/skill/skill_scanner.go` - 扩展 `buildCloneURL` 方法

### 前端变更

1. `web/src/types/index.ts` - 扩展 `RegistryType` 类型
2. `web/src/pages/skills/RegistryManagement.tsx` - 新增 CodeHub 类型选项和认证配置表单

### 无需变更

- 数据库表结构（`authConfig` 字段已支持 map 类型）
- API 接口（复用现有联邦源创建/扫描/导入 API）

## 测试要点

1. **SSH URL 测试**: 配置 SSH 格式 URL，验证 git clone 使用系统 SSH Key
2. **HTTPS URL 测试**: 配置 HTTPS 格式 URL + 账号密码，验证注入认证信息
3. **公开仓库测试**: HTTPS URL 无 authConfig，验证公开仓库访问
4. **错误处理测试**: 认证失败时返回清晰错误信息

## 风险与注意事项

1. **SSH Key 依赖**: SSH 认证依赖服务器上已配置的 SSH Key，需确保运维文档说明
2. **密码加密**: authConfig 中 password 需加密存储，避免明文泄露
3. **URL 格式验证**: 前端应验证 URL 格式是否为 `codehub-g.huawei.com` 域名