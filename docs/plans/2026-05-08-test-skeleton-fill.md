---
topics: [test, P0, skeleton, backlog]
doc_kind: plan
created: 2026-05-08
---

# 测试骨架补齐计划

> 紧急补齐 P0 骨架测试的实际逻辑，消除"标记为 P0 但无验证"的假安全感。

## 背景

已有详细设计文档 `docs/plans/2026-04-29-auto-test-supplement-design.md`（340 用例规划）。
但当前核心测试存在骨架问题：

| 文件 | 状态 | 问题 |
|------|------|------|
| `auto-test/internal/api/agent_handler_test.go` | 骨架 | 7 个 `TODO: Initialize router` |
| `auto-test/internal/service/agent/` | 空 | 只有 .gitkeep |
| `auto-test/internal/service/im/` | 空 | 只有 .gitkeep |
| `auto-test/internal/service/teampackagesync/` | 空 | 只有 .gitkeep |
| `auto-test/internal/repo/` | 空 | 目录存在无测试 |

**风险**：Feature Map 显示 F001 有 P0 测试（API-01-*），但实际无验证逻辑。

---

## Phase 1: API Handler 骨架补齐（P0）

### Task 1: agent_handler_test.go 实现真实测试

**Why**: P0 核心测试，当前是空壳。

**Files**: Modify `auto-test/internal/api/agent_handler_test.go`

**Steps**:

1. 引入测试依赖
   ```go
   import (
       "bytes"
       "encoding/json"
       "net/http"
       "net/http/httptest"
       "testing"

       "github.com/stretchr/testify/assert"
       "github.com/anthropic/isdp/internal/repo"
       "github.com/anthropic/isdp/internal/service/agent"
   )
   ```

2. 创建测试 Router 初始化函数
   ```go
   func setupTestRouter(t *testing.T) http.Handler {
       // 使用内存数据库
       db, err := repo.NewSQLiteDB(":memory:")
       if err != nil {
           t.Fatalf("Failed to create test database: %v", err)
       }

       // 初始化测试数据（见 init-sqlite.sql）

       // 创建 Handler
       agentRepo := repo.NewAgentRepo(db)
       agentService := agent.NewService(agentRepo)
       handler := NewAgentHandler(agentService)

       // 注册路由
       router := gin.New()
       router.GET("/api/v1/agents", handler.List)
       router.GET("/api/v1/agents/:id", handler.GetByID)
       router.POST("/api/v1/agents", handler.Create)
       router.PUT("/api/v1/agents/:id", handler.Update)
       router.DELETE("/api/v1/agents/:id", handler.Delete)

       return router
   }
   ```

3. 实现每个测试函数的真实逻辑
   - API-01-01: List 返回 200 + 验证 JSON 结构
   - API-01-02: GetByID 存在返回 200，不存在返回 404
   - API-01-03: Create 成功返回 201 + 验证 ID 生成
   - API-01-04: Update 成功返回 200 + 验证字段更新
   - API-01-05: Delete 成功返回 204
   - API-01-11: 参数验证（空名称返回 400）
   - API-01-12: 错误响应格式验证（统一 JSON 结构）

**Acceptance**: 所有测试通过 `go test ./auto-test/internal/api/... -v -run API-01`

**Time**: ~2-3 小时

---

## Phase 2: Repo 层基础测试（P0）

### Task 2: db_test.go 补充

**Why**: 数据库层是所有操作的基座。

**Files**: Create `auto-test/internal/repo/db_test.go`

**Key Tests**:
- RP-01-01: SQLite 内存连接创建成功
- RP-01-02: 类型验证（SQLite vs MySQL）
- RP-01-06: 事务提交/回滚
- RP-01-12: 错误处理（连接失败返回正确错误）

**Time**: ~1-2 小时

### Task 3: agent_repo_test.go 补充

**Files**: Create `auto-test/internal/repo/agent_repo_test.go`

**Key Tests**:
- RP-02-01: AgentRepo CRUD 完整流程
- RP-02-12: 数据验证（必填字段）

**Time**: ~1 小时

---

## Phase 3: Service 层核心测试（P1）

### Task 4: agent/execution_service_test.go

**Files**: Create `auto-test/internal/service/agent/execution_service_test.go`

**Why**: Agent 执行是核心流程，需验证 timeout/depth/session 逻辑。

**Key Tests**:
- SV-01-02: Adapter 执行（Mock Adapter）
- SV-01-14: 错误处理（超时返回正确错误）

**Time**: ~2 小时

### Task 5: a2a/mention_parser_test.go（已存在，需迁移）

**Files**: Copy from `internal/service/a2a/mention_parser_test.go`

**Time**: ~30 分钟（复制 + 验证）

---

## Phase 4: E2E 改进（P1）

### Task 6: E2E 测试改为固定 fixtures

**Why**: 当前 E2E 依赖项目存在，可能假阴性。

**Files**: Modify `auto-test/e2e/fixtures/test-fixtures.ts`

**Change**: 增加 `beforeAll` 创建测试项目/线程，`afterAll` 清理。

**Time**: ~1 小时

---

## Summary

| Phase | Task | Priority | Time | Status |
|-------|------|----------|------|--------|
| 1 | agent_handler_test.go | P0 | 2-3h | pending |
| 2 | db_test.go | P0 | 1-2h | **completed** |
| 2 | agent_repo_test.go | P0 | 1h | **completed** |
| 3 | parser mention_parser_test.go | P1 | 30m | **completed** |
| 3 | execution_service_test.go | P1 | 2h | pending |
| 4 | E2E fixtures 改进 | P1 | 1h | pending |

**Completed tests**: 31 real tests (repo: 14 + parser: 9 + humantask: 8)
**Skeleton tests**: 14 in agent_handler_test.go (pending implementation)

---

## Acceptance Criteria

- [ ] `make test-backend` 所有 P0 测试通过
- [ ] 无 `TODO.*Initialize router` 骨架注释
- [ ] 覆盖率报告生成成功
- [ ] E2E 测试不依赖现有项目数据

---

## References

- 详细设计: `docs/plans/2026-04-29-auto-test-supplement-design.md`
- 实施计划: `docs/plans/2026-04-29-auto-test-supplement-plan.md`
- Feature Map: `auto-test/feature-map.yaml`