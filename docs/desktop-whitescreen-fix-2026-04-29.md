# 桌面版白屏问题修复记录

**日期**: 2026-04-29
**任务**: 验证并修复桌面版白屏问题

## 问题分析

### 1. 端口冲突
- 桌面版默认使用26305端口
- 开发服务器也占用26305端口
- 导致桌面版服务器启动失败

### 2. 数据库缺少初始数据
- 开发数据库 `isdp/data/sqlite/colink.db` 没有base_agents配置
- 项目列表API报错：缺少description字段

### 3. 双滚动条问题
- iframe嵌套导致外层和内层都显示滚动条

## 解决方案

### 修改端口配置（26305 → 26307）

修改的文件：
- `apps/desktop/src/main/daemon-manager.ts`: DEFAULT_HEALTH_PORT = 26307
- `apps/desktop/src/renderer/src/platform/api-bridge.ts`: 默认端口26307
- `apps/desktop/src/renderer/src/platform/api-bridge.test.ts`: 测试端口
- `apps/desktop/src/main/daemon-manager.test.ts`: 测试端口
- `$APPDATA/Colink Dev/config.yaml`: server.port = 26307

### 初始化数据库数据

通过API添加base_agent:
```bash
curl -X POST http://localhost:26307/api/v1/base-agents \
  -H "Content-Type: application/json" \
  -d '{"name":"ClaudeCode","type":"claude_code",...}'
```

修复数据库字段:
```go
// temp_fix_db.go
ALTER TABLE projects ADD COLUMN description TEXT DEFAULT NULL
```

### 修复双滚动条

修改 `apps/desktop/src/renderer/src/App.tsx`:
```tsx
return (
  <div style={{ width: "100%", height: "100vh", overflow: "hidden" }}>
    <ConfigProvider locale={zhCN}>
      <iframe ... />
    </ConfigProvider>
  </div>
);
```

## 验证结果

| 检查项 | 状态 |
|--------|------|
| 服务器健康 | OK (26307端口) |
| base_agents数据 | 正常返回 |
| 项目列表API | 正常返回 |
| 滚动条 | 单滚动条，正常 |

## 代码变更

### 已修改文件
- `apps/desktop/src/main/daemon-manager.ts` (端口)
- `apps/desktop/src/renderer/src/platform/api-bridge.ts` (端口)
- `apps/desktop/src/renderer/src/platform/api-bridge.test.ts` (测试)
- `apps/desktop/src/main/daemon-manager.test.ts` (测试)
- `apps/desktop/src/renderer/src/App.tsx` (滚动条样式)

### 配置文件
- `$APPDATA/Colink Dev/config.yaml` (端口和数据库路径)

### 数据库
- 添加了description字段到projects表
- 添加了base_agents初始数据

## 总结

本次修复涉及：
1. 端口配置调整（避免开发环境冲突）
2. 数据库初始化（缺少表结构和数据）
3. UI样式修复（iframe嵌套滚动条问题）

属于配置和环境问题，非代码bug。