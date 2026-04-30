# ISDP 测试用例补充设计文档（修订版）

## 1. 概述

为 ISDP 项目补充全面的测试用例，覆盖 UI、API、性能三个维度。所有测试统一迁移到 `auto-test/` 目录，采用混合分层结构。

## 2. 技术栈选型

| 测试类型 | 框架/工具 | 位置 | 说明 |
|----------|-----------|------|------|
| **前端 E2E 测试** | Playwright | `auto-test/e2e/` | 跨浏览器，HTTP request API |
| **前端组件测试** | Vitest + React Testing Library | `auto-test/vitest/` | Vite 生态原生支持 |
| **后端 Handler 测试** | Go + testify + httptest | `auto-test/internal/api/` | 可导入 internal 包 |
| **后端 Service 测试** | Go + testify | `auto-test/internal/service/` | 业务逻辑测试 |
| **后端 Repo 测试** | Go + testify | `auto-test/internal/repo/` | 数据层测试 |
| **性能测试** | Go benchmark + Playwright trace | `auto-test/{e2e,internal}/performance/` | 无需独立工具 |

## 3. 目录结构

```
auto-test/
├── e2e/                          # 前端 E2E 测试
│   ├── agent-dialog/             # Agent 对话模块（P0）
│   │   ├── message-input.spec.ts
│   │   ├── streaming-response.spec.ts
│   │   ├── multi-agent.spec.ts
│   │   └── message-render.spec.ts
│   ├── websocket/                # WebSocket 流式模块（P0）
│   │   ├── connection.spec.ts
│   │   ├── message-flow.spec.ts
│   │   ├── multi-agent-parallel.spec.ts
│   │   └── error-handling.spec.ts
│   ├── team-package/             # 团队包管理模块（P1）
│   │   ├── list-crud.spec.ts
│   │   ├── sync.spec.ts
│   │   └── import.spec.ts
│   ├── thread-workflow/          # 线程/工作流模块（P1）
│   │   ├── thread-crud.spec.ts
│   │   └── workflow-exec.spec.ts
│   ├── api/                      # API HTTP 测试（P1）
│   │   └── backend-api.spec.ts
│   ├── performance/              # 性能测试（P2）
│   │   └── load-render.spec.ts
│   └── fixtures/                 # 测试 fixtures
│       ├── test-fixtures.ts
│       └── mock-data.ts
│
├── internal/                     # 后端内部测试（可导入 internal 包）
│   ├── api/                      # Handler 层（P0）
│   │   ├── agent_handler_test.go
│   │   ├── thread_handler_test.go
│   │   ├── workflow_handler_test.go
│   │   └── team_package_handler_test.go
│   ├── service/                  # Service 层（P0-P1）
│   │   ├── agent/
│   │   ├── a2a/
│   │   ├── teampackagesync/
│   │   └── im/
│   ├── repo/                     # Repo 层（P1）
│   │   ├── db_test.go
│   │   └── im_session_repo_test.go
│   ├── mocks/                    # Mock 对象
│   │   ├── mock_adapter.go
│   │   └── mock_repo.go
│   ├── testdata/                 # 测试数据
│   │   └── test_config.yaml
│   ├── performance/              # Go benchmark（P2）
│   │   └── api_bench_test.go
│   └── setup_test.go             # 测试初始化
│
├── vitest/                       # 前端 Vitest 组件测试
│   ├── components/               # 组件测试（P1-P2）
│   ├── stores/                   # Store 测试（P1）
│   ├── hooks/                    # Hook 测试（P1）
│   └── setup.ts                  # Vitest 配置
│
├── docs/                         # 测试文档
│   └── test-coverage-report.md
│
└── feature-map.yaml              # 特性测试映射配置
```

## 4. 测试用例统计

| 优先级 | 前端 E2E | 后端 Internal | Vitest 组件 | 合计 |
|--------|----------|---------------|-------------|------|
| **P0** | 40 | 60 | 20 | **120** |
| **P1** | 35 | 50 | 25 | **110** |
| **P2** | 25 | 30 | 15 | **70** |
| **P3** | 15 | 20 | 5 | **40** |
| **总计** | **115** | **160** | **65** | **340** |

### 按模块分布

| 模块 | P0 | P1 | P2 | P3 | 合计 |
|------|----|----|----|----|------|
| **Agent 对话** | 25 | 20 | 15 | 10 | 70 |
| **WebSocket 流式** | 15 | 15 | 10 | 5 | 45 |
| **团队包管理** | 10 | 15 | 8 | 5 | 38 |
| **线程/工作流** | 8 | 12 | 6 | 4 | 30 |
| **API HTTP** | 12 | 18 | 6 | 6 | 42 |
| **Service 层** | 20 | 30 | 15 | 10 | 75 |
| **Repo 层** | 5 | 10 | 8 | 5 | 28 |
| **组件/Hook/Store** | 5 | 10 | 12 | 5 | 32 |

### 优先级定义

| 级别 | 定义 | CI 要求 |
|------|------|---------|
| **P0** | 核心路径，阻塞发布 | 必须 100% 通过 |
| **P1** | 重要功能，影响用户体验 | 通过率 ≥ 95% |
| **P2** | 边缘场景、性能优化 | 通过率 ≥ 80% |
| **P3** | 探索性测试、UI细节 | 可选执行 |

## 5. 核心模块详细测试场景

### 5.1 Agent 对话模块（70 用例）

#### AD-01: 消息输入与发送（15 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| AD-01-01 | 输入框正常显示与聚焦 | P0 | E2E |
| AD-01-02 | 输入文本并点击发送成功 | P0 | E2E |
| AD-01-03 | 输入 `@` 触发 Agent 下拉框 | P0 | E2E |
| AD-01-04 | 下拉框显示可用 Agent 列表 | P0 | E2E |
| AD-01-05 | 选择单个 Agent 并发送 | P0 | E2E |
| AD-01-06 | 多 Agent 提及（最多2个） | P1 | E2E |
| AD-01-07 | 快捷键 Ctrl+Enter 发送 | P1 | E2E |
| AD-01-08 | 空消息禁止发送 | P0 | E2E |
| AD-01-09 | 纯空格消息禁止发送 | P2 | E2E |
| AD-01-10 | 超长消息（5000字）截断提示 | P2 | E2E |
| AD-01-11 | 输入框清空后状态恢复 | P1 | E2E |
| AD-01-12 | 中文 Agent 别名触发（如 @后端） | P1 | E2E |
| AD-01-13 | 发送按钮 loading 状态 | P1 | E2E |
| AD-01-14 | 发送失败错误提示 | P0 | E2E |
| AD-01-15 | 输入历史记录（上箭头回溯） | P3 | E2E |

#### AD-02: 流式响应显示（20 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| AD-02-01 | 用户消息卡片正确显示 | P0 | E2E |
| AD-02-02 | Agent 流式内容逐字显示 | P0 | E2E |
| AD-02-03 | ThinkingBlock 折叠/展开 | P1 | E2E |
| AD-02-04 | ToolBlock 显示工具名称 | P1 | E2E |
| AD-02-05 | ToolBlock 输入参数显示 | P2 | E2E |
| AD-02-06 | ToolBlock 输出结果显示 | P1 | E2E |
| AD-02-07 | 流式完成状态切换 | P0 | E2E |
| AD-02-08 | 多 Agent 并行响应分区显示 | P0 | E2E |
| AD-02-09 | 流式中断恢复继续 | P1 | E2E |
| AD-02-10 | 大文本流式（10KB+）完整 | P0 | E2E |
| AD-02-11 | 代码块流式高亮正确 | P1 | E2E |
| AD-02-12 | Markdown 渲染正确 | P0 | E2E |
| AD-02-13 | 表格 Markdown 渲染 | P2 | E2E |
| AD-02-14 | 列表 Markdown 渲染 | P2 | E2E |
| AD-02-15 | 链接 Markdown 可点击 | P1 | E2E |
| AD-02-16 | 图片 Markdown 显示 | P2 | E2E |
| AD-02-17 | 特殊字符转义正确 | P1 | E2E |
| AD-02-18 | 中英文混合流式 | P1 | E2E |
| AD-02-19 | Agent 名称标签显示 | P0 | E2E |
| AD-02-20 | 消息时间戳显示 | P2 | E2E |

#### AD-03: Agent 协作流程（15 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| AD-03-01 | 单 Agent 对话完整流程 | P0 | E2E |
| AD-03-02 | 双 Agent 协作（@backend @architect） | P0 | E2E |
| AD-03-03 | Handoff 切换 Agent | P1 | E2E |
| AD-03-04 | HumanTask 中断等待输入 | P1 | E2E |
| AD-03-05 | HumanTask 用户回复后继续 | P1 | E2E |
| AD-03-06 | Agent 执行超时提示 | P0 | E2E |
| AD-03-07 | Agent 错误重试机制 | P1 | E2E |
| AD-03-08 | 多 Agent 输出交错不混乱 | P0 | E2E |
| AD-03-09 | Agent 输出顺序一致性 | P1 | E2E |
| AD-03-10 | 自己提及自己被过滤 | P1 | E2E |
| AD-03-11 | 代码块内 @ 提及不触发 | P2 | E2E |
| AD-03-12 | 非行首 @ 提及不触发 | P1 | E2E |
| AD-03-13 | 最多 2 个目标限制 | P1 | E2E |
| AD-03-14 | Agent 配置缺失提示 | P2 | E2E |
| AD-03-15 | Agent 状态实时同步 | P1 | E2E |

#### AD-04: 消息状态与错误（10 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| AD-04-01 | 消息发送 loading 状态 | P0 | E2E |
| AD-04-02 | Agent 执行状态显示 | P1 | E2E |
| AD-04-03 | WebSocket 断开提示 | P0 | E2E |
| AD-04-04 | 发送失败重试按钮 | P0 | E2E |
| AD-04-05 | 网络恢复自动重连 | P1 | E2E |
| AD-04-06 | 错误消息样式区分 | P2 | E2E |
| AD-04-07 | 消息删除功能 | P2 | E2E |
| AD-04-08 | 消息复制功能 | P2 | E2E |
| AD-04-09 | 消息重新生成 | P3 | E2E |
| AD-04-10 | 消息编辑历史 | P3 | E2E |

#### AD-05: 消息历史与导航（10 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| AD-05-01 | 滚动加载历史消息 | P1 | E2E |
| AD-05-02 | 自动滚动到最新消息 | P0 | E2E |
| AD-05-03 | 滚动指示器显示 | P1 | E2E |
| AD-05-04 | 点击指示器跳转底部 | P1 | E2E |
| AD-05-05 | 消息列表虚拟滚动 | P2 | E2E |
| AD-05-06 | 500 条消息渲染性能 | P2 | E2E |
| AD-05-07 | 消息搜索功能 | P3 | E2E |
| AD-05-08 | 消息筛选（按 Agent） | P3 | E2E |
| AD-05-09 | 消息导出功能 | P3 | E2E |
| AD-05-10 | 消息分页加载 | P2 | E2E |

### 5.2 WebSocket 流式模块（45 用例）

#### WS-01: 连接管理（12 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| WS-01-01 | 页面加载自动建立连接 | P0 | E2E |
| WS-01-02 | 连接状态指示器显示 | P1 | E2E |
| WS-01-03 | 连接失败自动重试（3次） | P0 | E2E |
| WS-01-04 | 离开页面断开连接 | P1 | E2E |
| WS-01-05 | 多 Tab 连接复用 | P2 | E2E |
| WS-01-06 | 心跳保活机制 | P1 | E2E |
| WS-01-07 | 连接超时提示 | P0 | E2E |
| WS-01-08 | 手动重连按钮 | P1 | E2E |
| WS-01-09 | 连接 ID 正确分配 | P2 | E2E |
| WS-01-10 | 连接日志记录 | P3 | E2E |
| WS-01-11 | WebSocket URL 参数正确 | P1 | E2E |
| WS-01-12 | SSL/WSS 连接支持 | P1 | E2E |

#### WS-02: 流式消息接收（15 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| WS-02-01 | message_start 事件处理 | P0 | E2E |
| WS-02-02 | content_delta 事件处理 | P0 | E2E |
| WS-02-03 | thinking_delta 事件处理 | P1 | E2E |
| WS-02-04 | tool_start 事件处理 | P1 | E2E |
| WS-02-05 | tool_delta 事件处理 | P1 | E2E |
| WS-02-06 | tool_complete 事件处理 | P1 | E2E |
| WS-02-07 | message_complete 事件处理 | P0 | E2E |
| WS-02-08 | 高频消息（100条/秒）不丢失 | P0 | E2E |
| WS-02-09 | 消息顺序一致性 | P0 | E2E |
| WS-02-10 | 增量内容拼接正确 | P0 | E2E |
| WS-02-11 | 事件类型解析错误不崩溃 | P1 | E2E |
| WS-02-12 | 未知事件类型忽略 | P2 | E2E |
| WS-02-13 | 消息去重处理 | P1 | E2E |
| WS-02-14 | 消息缓冲队列管理 | P2 | E2E |
| WS-02-15 | 流式暂停/恢复 | P3 | E2E |

#### WS-03: 多 Agent 并行流式（10 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| WS-03-01 | 多 Agent 输出分区显示 | P0 | E2E |
| WS-03-02 | 各 Agent 流式独立 | P0 | E2E |
| WS-03-03 | 交错消息不混乱 | P0 | E2E |
| WS-03-04 | 各区消息顺序一致 | P1 | E2E |
| WS-03-05 | Agent 标识正确关联 | P0 | E2E |
| WS-03-06 | 并行流式性能 | P1 | E2E |
| WS-03-07 | 单 Agent 完成不影响其他 | P1 | E2E |
| WS-03-08 | 全部完成后状态更新 | P0 | E2E |
| WS-03-09 | 并行数量限制 | P2 | E2E |
| WS-03-10 | 并行流式取消 | P3 | E2E |

#### WS-04: 异常处理（8 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| WS-04-01 | 断网自动重连 | P0 | E2E |
| WS-04-02 | 重连后恢复消息流 | P0 | E2E |
| WS-04-03 | 超时提示显示 | P0 | E2E |
| WS-04-04 | 解析失败不崩溃 | P0 | E2E |
| WS-04-05 | UI 状态恢复 | P1 | E2E |
| WS-04-06 | 错误消息持久化 | P2 | E2E |
| WS-04-07 | 重连次数限制 | P1 | E2E |
| WS-04-08 | 错误上报 | P3 | E2E |

### 5.3 团队包管理模块（38 用例）

#### TP-01: 列表与 CRUD（15 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| TP-01-01 | 团队包列表加载 | P0 | E2E |
| TP-01-02 | 空列表提示显示 | P1 | E2E |
| TP-01-03 | 创建团队包成功 | P0 | E2E |
| TP-01-04 | 创建表单验证 | P1 | E2E |
| TP-01-05 | 更新团队包成功 | P0 | E2E |
| TP-01-06 | 删除团队包成功 | P0 | E2E |
| TP-01-07 | 批量删除功能 | P1 | E2E |
| TP-01-08 | 删除确认弹窗 | P1 | E2E |
| TP-01-09 | 搜索团队包 | P1 | E2E |
| TP-01-10 | 分页加载 | P1 | E2E |
| TP-01-11 | 排序功能 | P2 | E2E |
| TP-01-12 | 状态筛选 | P2 | E2E |
| TP-01-13 | 详情查看 | P0 | E2E |
| TP-01-14 | 复制团队包 | P2 | E2E |
| TP-01-15 | 导出团队包 | P3 | E2E |

#### TP-02: 同步功能（12 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| TP-02-01 | 同步触发成功 | P0 | E2E |
| TP-02-02 | 同步状态显示 | P0 | E2E |
| TP-02-03 | 同步进度条 | P1 | E2E |
| TP-02-04 | 同步失败提示 | P0 | E2E |
| TP-02-05 | 同步冲突处理 | P1 | E2E |
| TP-02-06 | 同步历史记录 | P2 | E2E |
| TP-02-07 | 自动同步触发 | P1 | E2E |
| TP-02-08 | 手动同步按钮 | P0 | E2E |
| TP-02-09 | 同步取消功能 | P2 | E2E |
| TP-02-10 | 同步性能测试 | P2 | E2E |
| TP-02-11 | 批量同步 | P3 | E2E |
| TP-02-12 | 同步日志查看 | P3 | E2E |

#### TP-03: 导入导出（11 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| TP-03-01 | 导入团队包成功 | P0 | E2E |
| TP-03-02 | 导入冲突检测 | P0 | E2E |
| TP-03-03 | 冲突解决策略选择 | P1 | E2E |
| TP-03-04 | 导入进度显示 | P1 | E2E |
| TP-03-05 | 导入失败回滚 | P0 | E2E |
| TP-03-06 | 文件格式验证 | P1 | E2E |
| TP-03-07 | 批量导入 | P1 | E2E |
| TP-03-08 | 导入历史记录 | P2 | E2E |
| TP-03-09 | 导入预览功能 | P2 | E2E |
| TP-03-10 | 从市场导入 | P1 | E2E |
| TP-03-11 | 导入依赖处理 | P2 | E2E |

## 6. 其他模块测试场景

### 6.1 线程/工作流模块（30 用例）

#### TW-01: 线程 CRUD（15 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| TW-01-01 | 线程列表加载 | P0 | E2E |
| TW-01-02 | 创建线程成功 | P0 | E2E |
| TW-01-03 | 线程详情查看 | P0 | E2E |
| TW-01-04 | 更新线程信息 | P1 | E2E |
| TW-01-05 | 删除线程成功 | P0 | E2E |
| TW-01-06 | 线程状态切换 | P1 | E2E |
| TW-01-07 | 线程搜索功能 | P1 | E2E |
| TW-01-08 | 线程分页加载 | P1 | E2E |
| TW-01-09 | 线程排序功能 | P2 | E2E |
| TW-01-10 | 线程筛选（按项目） | P1 | E2E |
| TW-01-11 | 线程消息历史加载 | P0 | E2E |
| TW-01-12 | 线程创建表单验证 | P1 | E2E |
| TW-01-13 | 线程批量操作 | P2 | E2E |
| TW-01-14 | 线程归档功能 | P3 | E2E |
| TW-01-15 | 线程恢复功能 | P3 | E2E |

#### TW-02: 工作流执行（15 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| TW-02-01 | 工作流列表加载 | P0 | E2E |
| TW-02-02 | 创建工作流成功 | P0 | E2E |
| TW-02-03 | 工作流编辑器显示 | P1 | E2E |
| TW-02-04 | 工作流节点添加 | P1 | E2E |
| TW-02-05 | 工作流节点连接 | P1 | E2E |
| TW-02-06 | 工作流保存成功 | P0 | E2E |
| TW-02-07 | 工作流执行触发 | P0 | E2E |
| TW-02-08 | 工作流执行状态显示 | P0 | E2E |
| TW-02-09 | 工作流执行进度 | P1 | E2E |
| TW-02-10 | 工作流执行结果 | P1 | E2E |
| TW-02-11 | 工作流执行失败处理 | P0 | E2E |
| TW-02-12 | 工作流复制功能 | P2 | E2E |
| TW-02-13 | 工作流删除功能 | P1 | E2E |
| TW-02-14 | 工作流版本管理 | P3 | E2E |
| TW-02-15 | 工作流模板导入 | P2 | E2E |

### 6.2 API HTTP 测试（42 用例）

#### API-01: Agent Handler（12 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| API-01-01 | GET /api/v1/agents 列表 | P0 | E2E |
| API-01-02 | GET /api/v1/agents/:id 详情 | P0 | E2E |
| API-01-03 | POST /api/v1/agents 创建 | P0 | E2E |
| API-01-04 | PUT /api/v1/agents/:id 更新 | P0 | E2E |
| API-01-05 | DELETE /api/v1/agents/:id 删除 | P0 | E2E |
| API-01-06 | POST /api/v1/agents/batch-delete 批量删除 | P1 | E2E |
| API-01-07 | POST /api/v1/agents/copy 复制 | P1 | E2E |
| API-01-08 | POST /api/v1/agents/check-references 引用检查 | P1 | E2E |
| API-01-09 | POST /api/v1/agents/debug 调试 | P2 | E2E |
| API-01-10 | POST /api/v1/agents/continue-debug 继续调试 | P2 | E2E |
| API-01-11 | Agent 参数验证 | P0 | Internal |
| API-01-12 | Agent 错误响应格式 | P0 | Internal |

#### API-02: Thread Handler（8 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| API-02-01 | GET /api/v1/threads 列表 | P0 | E2E |
| API-02-02 | GET /api/v1/threads/:id 详情 | P0 | E2E |
| API-02-03 | POST /api/v1/threads 创建 | P0 | E2E |
| API-02-04 | PUT /api/v1/threads/:id 更新 | P1 | E2E |
| API-02-05 | DELETE /api/v1/threads/:id 删除 | P0 | E2E |
| API-02-06 | GET /api/v1/threads/:id/messages 消息列表 | P0 | E2E |
| API-02-07 | POST /api/v1/threads/:id/messages 发送消息 | P0 | E2E |
| API-02-08 | Thread 参数验证 | P0 | Internal |

#### API-03: Team Package Handler（8 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| API-03-01 | GET /api/v1/team-packages 列表 | P0 | E2E |
| API-03-02 | GET /api/v1/team-packages/:id 详情 | P0 | E2E |
| API-03-03 | POST /api/v1/team-packages 创建 | P0 | E2E |
| API-03-04 | PUT /api/v1/team-packages/:id 更新 | P1 | E2E |
| API-03-05 | DELETE /api/v1/team-packages/:id 删除 | P0 | E2E |
| API-03-06 | POST /api/v1/team-packages/sync 同步 | P0 | E2E |
| API-03-07 | POST /api/v1/team-packages/import 导入 | P0 | E2E |
| API-03-08 | Team Package 参数验证 | P0 | Internal |

#### API-04: 其他 Handler（14 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| API-04-01 | GET /api/v1/projects 列表 | P0 | E2E |
| API-04-02 | POST /api/v1/projects 创建 | P0 | E2E |
| API-04-03 | GET /api/v1/workflows 列表 | P0 | E2E |
| API-04-04 | POST /api/v1/workflows 执行 | P1 | E2E |
| API-04-05 | GET /api/v1/base-agents 列表 | P1 | E2E |
| API-04-06 | POST /api/v1/base-agents 创建 | P1 | E2E |
| API-04-07 | GET /api/v1/artifacts 列表 | P1 | E2E |
| API-04-08 | GET /health 健康检查 | P0 | E2E |
| API-04-09 | 404 错误处理 | P1 | E2E |
| API-04-10 | 500 错误处理 | P0 | E2E |
| API-04-11 | 请求超时处理 | P1 | E2E |
| API-04-12 | 响应格式验证（camelCase） | P0 | Internal |
| API-04-13 | 并发请求处理 | P1 | Internal |
| API-04-14 | 请求限流测试 | P2 | Internal |

### 6.3 后端 Service 层测试（75 用例）

#### SV-01: Agent Service（20 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| SV-01-01 | Agent 适配器获取 | P0 | Internal |
| SV-01-02 | Agent 适配器执行 | P0 | Internal |
| SV-01-03 | Agent 输出解析 | P0 | Internal |
| SV-01-04 | Agent 元数据获取 | P1 | Internal |
| SV-01-05 | Agent 插件注册 | P0 | Internal |
| SV-01-06 | 调试线程管理器初始化 | P1 | Internal |
| SV-01-07 | 调试线程创建 | P1 | Internal |
| SV-01-08 | 五层上下文构建 | P0 | Internal |
| SV-01-09 | Governance 摘要生成 | P1 | Internal |
| SV-01-10 | Human Chain 历史管理 | P1 | Internal |
| SV-01-11 | Orchestrator 调试流程 | P1 | Internal |
| SV-01-12 | 项目上下文加载 | P0 | Internal |
| SV-01-13 | Token Budget 计算 | P1 | Internal |
| SV-01-14 | Agent 错误处理 | P0 | Internal |
| SV-01-15 | Agent 状态管理 | P1 | Internal |
| SV-01-16 | Agent 配置验证 | P0 | Internal |
| SV-01-17 | Agent 引用检查 | P1 | Internal |
| SV-01-18 | Agent 批量操作 | P1 | Internal |
| SV-01-19 | Agent 复制逻辑 | P1 | Internal |
| SV-01-20 | Agent 清理逻辑 | P2 | Internal |

#### SV-02: A2A Service（15 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| SV-02-01 | A2A 提及解析 | P0 | Internal |
| SV-02-02 | Invocation 队列管理 | P0 | Internal |
| SV-02-03 | Invocation 注册表 | P0 | Internal |
| SV-02-04 | Session Chain 存储 | P1 | Internal |
| SV-02-05 | 多 Agent 协作流程 | P0 | Internal |
| SV-02-06 | Handoff 机制 | P1 | Internal |
| SV-02-07 | 边界检查（最多2目标） | P0 | Internal |
| SV-02-08 | 自己提及过滤 | P1 | Internal |
| SV-02-09 | 代码块内提及过滤 | P1 | Internal |
| SV-02-10 | 非行首提及过滤 | P1 | Internal |
| SV-02-11 | 中文别名解析 | P1 | Internal |
| SV-02-12 | A2A 错误处理 | P0 | Internal |
| SV-02-13 | A2A 状态同步 | P1 | Internal |
| SV-02-14 | A2A 性能测试 | P2 | Internal |
| SV-02-15 | A2A 并发测试 | P1 | Internal |

#### SV-03: IM Service（20 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| SV-03-01 | 消息分发逻辑 | P0 | Internal |
| SV-03-02 | 消息去重机制 | P0 | Internal |
| SV-03-03 | 飞书适配器初始化 | P1 | Internal |
| SV-03-04 | 飞书消息发送 | P1 | Internal |
| SV-03-05 | 飞书消息接收 | P1 | Internal |
| SV-03-06 | IM Bridge 服务 | P1 | Internal |
| SV-03-07 | 错误处理 | P0 | Internal |
| SV-03-08 | 重试机制 | P1 | Internal |
| SV-03-09 | 限流器 | P1 | Internal |
| SV-03-10 | 会话锁 | P1 | Internal |
| SV-03-11 | 注册表管理 | P1 | Internal |
| SV-03-12 | 边缘场景处理 | P1 | Internal |
| SV-03-13 | 类型转换 | P1 | Internal |
| SV-03-14 | 集成测试 | P1 | Internal |
| SV-03-15 | 消息格式验证 | P0 | Internal |
| SV-03-16 | 消息优先级 | P2 | Internal |
| SV-03-17 | 消息队列管理 | P1 | Internal |
| SV-03-18 | 消息持久化 | P1 | Internal |
| SV-03-19 | IM 性能测试 | P2 | Internal |
| SV-03-20 | IM 并发测试 | P1 | Internal |

#### SV-04: Team Package Sync（10 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| SV-04-01 | Clone 缓存初始化 | P1 | Internal |
| SV-04-02 | Clone 缓存命中 | P1 | Internal |
| SV-04-03 | Clone 缓存过期 | P1 | Internal |
| SV-04-04 | 同步触发逻辑 | P0 | Internal |
| SV-04-05 | 同步状态管理 | P0 | Internal |
| SV-04-06 | 同步冲突检测 | P0 | Internal |
| SV-04-07 | 同步错误处理 | P0 | Internal |
| SV-04-08 | 批量导入逻辑 | P1 | Internal |
| SV-04-09 | 导入依赖处理 | P2 | Internal |
| SV-04-10 | 导入性能测试 | P2 | Internal |

#### SV-05: 其他 Service（10 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| SV-05-01 | Command Service | P1 | Internal |
| SV-05-02 | HumanTask Service | P1 | Internal |
| SV-05-03 | Rule Service | P1 | Internal |
| SV-05-04 | Workflow 执行逻辑 | P0 | Internal |
| SV-05-05 | Workflow 状态管理 | P1 | Internal |
| SV-05-06 | Thread 创建逻辑 | P0 | Internal |
| SV-05-07 | Thread 消息处理 | P0 | Internal |
| SV-05-08 | Project CRUD 逻辑 | P1 | Internal |
| SV-05-09 | Base Agent 管理 | P1 | Internal |
| SV-05-10 | Artifact 管理 | P2 | Internal |

### 6.4 后端 Repo 层测试（28 用例）

#### RP-01: 数据库操作（15 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| RP-01-01 | SQLite 连接创建 | P0 | Internal |
| RP-01-02 | SQLite 类型验证 | P0 | Internal |
| RP-01-03 | 无效类型错误处理 | P0 | Internal |
| RP-01-04 | MySQL 连接创建（过渡期） | P2 | Internal |
| RP-01-05 | IM Session Repo CRUD | P1 | Internal |
| RP-01-06 | 事务处理 | P0 | Internal |
| RP-01-07 | 批量插入性能 | P1 | Internal |
| RP-01-08 | 查询优化测试 | P2 | Internal |
| RP-01-09 | 连接池管理 | P1 | Internal |
| RP-01-10 | 数据库迁移测试 | P1 | Internal |
| RP-01-11 | 数据库备份恢复 | P2 | Internal |
| RP-01-12 | 错误处理 | P0 | Internal |
| RP-01-13 | 连接超时处理 | P1 | Internal |
| RP-01-14 | 数据一致性测试 | P1 | Internal |
| RP-01-15 | 并发写入测试 | P1 | Internal |

#### RP-02: 其他 Repo（13 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| RP-02-01 | Agent Repo CRUD | P0 | Internal |
| RP-02-02 | Thread Repo CRUD | P0 | Internal |
| RP-02-03 | Workflow Repo CRUD | P1 | Internal |
| RP-02-04 | Team Package Repo CRUD | P0 | Internal |
| RP-02-05 | Project Repo CRUD | P1 | Internal |
| RP-02-06 | Base Agent Repo CRUD | P1 | Internal |
| RP-02-07 | Artifact Repo CRUD | P1 | Internal |
| RP-02-08 | 查询条件构建 | P1 | Internal |
| RP-02-09 | 分页查询 | P1 | Internal |
| RP-02-10 | 关联查询 | P2 | Internal |
| RP-02-11 | 批量删除 | P1 | Internal |
| RP-02-12 | 数据验证 | P0 | Internal |
| RP-02-13 | 错误处理 | P0 | Internal |

### 6.5 Vitest 组件/Store/Hook 测试（32 用例）

#### VT-01: 核心组件（15 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| VT-01-01 | AgentConfigCard 渲染 | P1 | Vitest |
| VT-01-02 | AgentConfigCard 交互 | P1 | Vitest |
| VT-01-03 | ThreadList 渲染 | P1 | Vitest |
| VT-01-04 | ThreadList 选择 | P1 | Vitest |
| VT-01-05 | WorkflowEditor 渲染 | P1 | Vitest |
| VT-01-06 | WorkflowEditor 交互 | P2 | Vitest |
| VT-01-07 | TeamGraphEditor 渲染 | P1 | Vitest |
| VT-01-08 | TeamGraphEditor 布局 | P2 | Vitest |
| VT-01-09 | ChatMessageList 渲染 | P0 | Vitest |
| VT-01-10 | ChatMessageList 流式 | P0 | Vitest |
| VT-01-11 | ThemeSwitcher 切换 | P2 | Vitest |
| VT-01-12 | ApiStatusIndicator 状态 | P1 | Vitest |
| VT-01-13 | ErrorBoundary 捕获 | P1 | Vitest |
| VT-01-14 | 消息输入组件 | P0 | Vitest |
| VT-01-15 | Agent 下拉组件 | P0 | Vitest |

#### VT-02: Zustand Store（10 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| VT-02-01 | useAgentStore 初始化 | P1 | Vitest |
| VT-02-02 | useAgentStore CRUD | P1 | Vitest |
| VT-02-03 | useThreadStore 初始化 | P1 | Vitest |
| VT-02-04 | useThreadStore CRUD | P1 | Vitest |
| VT-02-05 | useGraphStore 初始化 | P1 | Vitest |
| VT-02-06 | useGraphStore 更新 | P2 | Vitest |
| VT-02-07 | useWorkflowStore 初始化 | P1 | Vitest |
| VT-02-08 | useWorkflowStore 执行 | P1 | Vitest |
| VT-02-09 | Store 持久化 | P2 | Vitest |
| VT-02-10 | Store 重置 | P2 | Vitest |

#### VT-03: Hooks（7 用例）

| ID | 测试场景 | 优先级 | 类型 |
|----|----------|--------|------|
| VT-03-01 | useTheme 切换 | P2 | Vitest |
| VT-03-02 | useApi 请求 | P1 | Vitest |
| VT-03-03 | useWebSocket 连接 | P0 | Vitest |
| VT-03-04 | useWebSocket 消息 | P0 | Vitest |
| VT-03-05 | useThrottledWebSocket | P1 | Vitest |
| VT-03-06 | useScroll 处理 | P1 | Vitest |
| VT-03-07 | useMention 解析 | P0 | Vitest |

## 7. 性能测试场景（20 用例）

### 7.1 API 性能测试（10 用例）

| ID | 测试场景 | 指标 | 阈值 | 优先级 |
|----|----------|------|------|--------|
| PF-01-01 | Agent List API 响应时间 | 响应时间 | < 200ms | P1 |
| PF-01-02 | Thread List API 响应时间 | 响应时间 | < 200ms | P1 |
| PF-01-03 | Team Package List API 响应时间 | 响应时间 | < 200ms | P1 |
| PF-01-04 | Workflow Execute API 响应时间 | 响应时间 | < 500ms | P1 |
| PF-01-05 | 100 并发创建 Thread | 成功率 | > 99% | P1 |
| PF-01-06 | 50 并发发送消息 | 成功率 | > 99% | P1 |
| PF-01-07 | WebSocket 消息延迟 | 延迟 | < 100ms | P1 |
| PF-01-08 | 大数据量查询（1000条） | 响应时间 | < 1s | P2 |
| PF-01-09 | 批量操作性能 | 响应时间 | < 2s | P2 |
| PF-01-10 | 长连接稳定性（1小时） | 连接保持 | 无断开 | P2 |

### 7.2 前端性能测试（10 用例）

| ID | 测试场景 | 指标 | 阈值 | 优先级 |
|----|----------|------|------|--------|
| PF-02-01 | ThreadList 500条渲染 | LCP | < 1.5s | P2 |
| PF-02-02 | AgentConfigList 100条渲染 | LCP | < 1s | P2 |
| PF-02-03 | 流式消息高频渲染 | FPS | > 30 | P1 |
| PF-02-04 | 1000 条消息内存占用 | 内存增量 | < 50MB | P2 |
| PF-02-05 | 首页加载时间 | LCP | < 2s | P2 |
| PF-02-06 | 深色模式切换时间 | 切换时间 | < 100ms | P3 |
| PF-02-07 | 图表渲染性能 | FPS | > 30 | P2 |
| PF-02-08 | 虚拟滚动性能 | FPS | > 50 | P2 |
| PF-02-09 | WebSocket 消息处理延迟 | 延迟 | < 50ms | P1 |
| PF-02-10 | 长时间运行稳定性（1小时） | 内存增长 | < 100MB | P2 |

## 8. 特性测试场景

### 8.1 特性清单

| 特性 ID | 特性名称 | 关联模块 | 优先级 |
|---------|----------|----------|--------|
| **F001** | Agent 对话核心 | AD, WS, SV-Agent, API-Agent | P0 |
| **F002** | WebSocket 流式 | WS, SV-A2A, VT-WebSocket | P0 |
| **F003** | 多 Agent 协作 (A2A) | AD-03, SV-A2A, VT-Mention | P0 |
| **F004** | 团队包管理 | TP, SV-TP, API-TP | P1 |
| **F005** | 线程管理 | TW, SV-Thread, API-Thread | P1 |
| **F006** | 工作流执行 | TW-02, SV-Workflow, API-Workflow | P1 |
| **F007** | IM 集成 | SV-IM, RP-IM | P1 |
| **F008** | 消息渲染 | AD-02, VT-Components | P1 |
| **F009** | 深色模式 | VT-Theme, PF-02-06 | P2 |
| **F010** | 性能优化 | PF-01, PF-02 | P2 |

### 8.2 特性测试执行命令

```bash
# 执行单个特性所有测试
make test-feature F=F001    # Agent 对话核心
make test-feature F=F002    # WebSocket 流式
make test-feature F=F003    # 多 Agent 协作

# 执行多个特性测试
make test-feature F=F001,F002,F003

# 执行指定优先级的特性
make test-feature-priority P=P0   # 所有 P0 特性
make test-feature-priority P=P0,P1 # 所有 P0 + P1 特性
```

## 9. 测试执行命令

```bash
# ===== 前端测试 =====

# E2E 全量测试（在 web 目录运行）
cd web && npm run test:e2e
# 或指定目录
cd web && npx playwright test auto-test/e2e/

# E2E 按优先级执行
cd web && npx playwright test auto-test/e2e/ --grep "P0"
cd web && npx playwright test auto-test/e2e/ --grep "P0|P1"

# E2E 按模块执行
cd web && npx playwright test auto-test/e2e/agent-dialog/
cd web && npx playwright test auto-test/e2e/websocket/
cd web && npx playwright test auto-test/e2e/team-package/

# Vitest 组件测试
cd web && npm run test:vitest
# 或直接运行
cd web && npx vitest run auto-test/vitest/

# 性能测试（前端）
cd web && npx playwright test --trace on auto-test/e2e/performance/

# ===== 后端测试 =====

# Go 全量测试（在项目根目录运行）
go test ./auto-test/internal/... -v

# Go 按层级执行
go test ./auto-test/internal/api/... -v      # Handler 层
go test ./auto-test/internal/service/... -v  # Service 层
go test ./auto-test/internal/repo/... -v     # Repo 层

# Go 按模块执行
go test ./auto-test/internal/service/agent/... -v
go test ./auto-test/internal/service/a2a/... -v
go test ./auto-test/internal/service/teampackagesync/... -v

# Go 性能测试（benchmark）
go test -bench=. ./auto-test/internal/performance/

# ===== CI 全量测试 =====

# 统一执行（推荐）
make test-all

# 分步执行
make test-frontend    # 前端 E2E + Vitest
make test-backend     # 后端 Go 测试
make test-performance # 性能测试

# 按优先级测试
make test-p0  # 只执行 P0 测试
make test-p1  # 执行 P0 + P1 测试
```

## 10. 测试约束规则

### 10.1 目录约束

| 规则 | 说明 |
|------|------|
| 所有测试统一放在 `auto-test/` | 已有测试需迁移，不再保留原地 |
| 前端 E2E 在 `auto-test/e2e/` | Playwright 测试 |
| 后端测试在 `auto-test/internal/` | 可导入 internal 包 |
| Vitest 测试在 `auto-test/vitest/` | 组件/Store/Hook 测试 |
| 性能测试在各自目录的 `performance/` 子目录 | 不单独建顶层目录 |
| 特性映射文件在 `auto-test/feature-map.yaml` | 特性测试配置 |

### 10.2 命名约束

| 规则 | 示例 |
|------|------|
| E2E 文件命名 `{模块}-{场景}.spec.ts` | `message-input.spec.ts` |
| Go 测试文件命名 `{场景}_test.go` | `agent_handler_test.go` |
| 测试 ID 格式 `{模块}-{类别}-{序号}` | `AD-01-02` |
| 特性 ID 格式 `F{三位数字}` | `F001` |
| 优先级标注在测试 ID 表或注释中 | `// @priority P0` |

### 10.3 数据约束

| 规则 | 说明 |
|------|------|
| API/Service 层用测试数据库 | 独立数据库文件，不影响生产数据 |
| 组件/Hook/Store 用 Mock | 使用 vitest mock 功能 |
| 测试数据放在 `auto-test/internal/testdata/` | 配置文件、SQL 脚本等 |
| Mock 对象放在 `auto-test/internal/mocks/` | 接口 mock 实现 |
| 前端 fixtures 放在 `auto-test/e2e/fixtures/` | 测试固定数据 |

### 10.4 执行约束

| 规则 | 说明 |
|------|------|
| P0 测试阻塞发布 | CI 中 P0 必须 100% 通过 |
| P1 通过率 ≥ 95% | 可允许少量边缘场景失败 |
| P2/P3 不阻塞发布 | 作为补充验证 |
| 特性测试可独立执行 | `make test-feature F=F001` |
| 性能测试单独执行 | `make test-performance` |

## 11. 已有测试迁移计划

### 11.1 需迁移的测试文件

| 原位置 | 迁移目标 |
|--------|----------|
| `web/tests/e2e/*.spec.ts` (12个) | `auto-test/e2e/` 对应模块目录 |
| `internal/**/*_test.go` (33个) | `auto-test/internal/service/` 等对应目录 |
| `web/tests/fixtures/test-fixtures.ts` | `auto-test/e2e/fixtures/` |
| `web/tests/test-runner.ts` | `auto-test/e2e/` |

### 11.2 迁移后清理

- 迁移完成后删除原 `web/tests/e2e/` 目录
- 迁移完成后删除原 `internal/**/*_test.go` 文件
- 更新相关配置文件（playwright.config.ts、vitest.config.ts）

## 12. 实施时间线

### 12.1 分阶段实施计划

| 阶段 | 内容 | 时间 | 产出 |
|------|------|------|------|
| **Phase 1** | 测试基础设施 | 第 1 周前 3 天 | 目录结构、配置文件、CI 工作流 |
| **Phase 2** | P0 核心测试 | 第 1 周后 2 天 | Agent 对话、WebSocket P0 测试 |
| **Phase 3-4** | P0/P1 Internal | 第 2 周 | Service 层、团队包测试 |
| **Phase 5** | 测试迁移 | 第 2 周末 | 迁移 + 验证 + 删除原文件 |
| **Phase 6-7** | Vitest + 性能 | 第 3 周 | 组件测试、性能测试 |
| **Phase 8** | 文档更新 | 第 3 周末 | CLAUDE.md、AGENTS.md |
| **Phase 9** | 最终验证 | 第 4 周 | 全量测试验证 |

### 12.2 P0 测试完成标准

- ✅ 所有 P0 测试文件创建完成
- ✅ CI 工作流配置并运行成功
- ✅ P0 测试在 CI 中 100% 通过
- ✅ 测试覆盖率报告生成

### 12.3 P1 测试完成标准

- ✅ 所有 P1 测试文件创建完成
- ✅ P1 测试通过率 ≥ 95%
- ✅ 特性测试 F001-F007 可独立执行

### 12.4 发布门禁

| 门禁条件 | 检查方式 |
|----------|----------|
| P0 测试 100% 通过 | CI 自动检查，失败阻塞合并 |
| P1 测试 ≥ 95% 通过 | CI 报告，低于阈值警告 |
| 覆盖率 ≥ 60% | codecov 报告 |
| 无新增 P0 测试失败 | PR 检查 |

## 13. 附录：测试工具配置参考

### 13.1 Playwright 配置参考

```typescript
// web/playwright.config.ts
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './auto-test/e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [
    ['html', { outputFolder: 'playwright-report' }],
    ['json', { outputFile: 'test-results.json' }],
  ],
  use: {
    baseURL: 'http://localhost:26306',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
  ],
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:26306',
    reuseExistingServer: !process.env.CI,
  },
});
```

### 13.2 Vitest 配置参考

```typescript
// web/vitest.config.ts
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    include: ['auto-test/vitest/**/*.test.ts'],
    setupFiles: ['auto-test/vitest/setup.ts'],
    environment: 'jsdom',
    globals: true,
    coverage: {
      reporter: ['text', 'json', 'html'],
      include: ['src/**/*.tsx'],
    },
  },
});
```

### 13.3 Go 测试初始化参考

```go
// auto-test/internal/setup_test.go
package internal_test

import (
    "os"
    "testing"
)

func TestMain(m *testing.M) {
    os.Setenv("ISDP_TEST_MODE", "true")
    code := m.Run()
    os.Unsetenv("ISDP_TEST_MODE")
    os.Exit(code)
}
```