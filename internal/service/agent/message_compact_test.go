package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// -----------------------------------------------------------------------------
// CompactMessageContent 三阶段处理逻辑
// -----------------------------------------------------------------------------

func TestCompactMessageContent_HandoffBlockPreferred(t *testing.T) {
	// 有 handoff 块：只保留 handoff 内容，其他全丢
	content := `啰嗦的开头...
<thinking>大量思考</thinking>
<a2a-handoff>
接下来请 @后端 实现登录 API：
- 输入：username / password
- 输出：JWT token
</a2a-handoff>
后面还有很多解释文字，长长长长长长长长长长长长长长长长长长长长长长长长长长长长长长`

	out := CompactMessageContent(content, 800)
	if !strings.Contains(out, "接下来请 @后端") {
		t.Fatalf("handoff 内容应保留: %q", out)
	}
	if strings.Contains(out, "啰嗦的开头") {
		t.Fatalf("handoff 优先时应丢弃前后其他文字: %q", out)
	}
	if strings.Contains(out, "大量思考") {
		t.Fatalf("thinking 也应丢弃: %q", out)
	}
	if strings.Contains(out, "长长长长") {
		t.Fatalf("handoff 后的解释也应丢弃: %q", out)
	}
}

func TestCompactMessageContent_StripsThinkingBlock(t *testing.T) {
	content := `分析结果如下：
<thinking>
用户想要一个登录功能，我需要考虑安全性...
分几步来看：
1. 输入验证
2. 密码加密
</thinking>
所以最终方案是使用 bcrypt + JWT。`

	out := CompactMessageContent(content, 800)
	if strings.Contains(out, "用户想要") || strings.Contains(out, "分几步来看") {
		t.Fatalf("thinking 块应被剥离: %q", out)
	}
	if !strings.Contains(out, "分析结果如下") || !strings.Contains(out, "bcrypt + JWT") {
		t.Fatalf("正文应保留: %q", out)
	}
}

func TestCompactMessageContent_StripsToolBlocks(t *testing.T) {
	content := `我先查一下代码。
<tool_use>
{"name": "Read", "path": "auth.go", "start": 100, "end": 200}
</tool_use>
<tool_result>
line 100: func Login() { ... 500 chars ... }
line 200: return err
</tool_result>
看完后我的结论是 auth.go 需要重构。`

	out := CompactMessageContent(content, 800)
	if strings.Contains(out, "tool_use") || strings.Contains(out, `"name": "Read"`) {
		t.Fatalf("tool_use 应剥离: %q", out)
	}
	if strings.Contains(out, "tool_result") || strings.Contains(out, "line 100:") {
		t.Fatalf("tool_result 应剥离: %q", out)
	}
	if !strings.Contains(out, "auth.go 需要重构") {
		t.Fatalf("结论应保留: %q", out)
	}
}

func TestCompactMessageContent_StripsMarkdownThinkingHeader(t *testing.T) {
	content := `## 思考
先分析用户需求，再考虑技术方案。
这一段是思考。

## 结论
使用 A 方案。`

	out := CompactMessageContent(content, 800)
	// markdown 思考标题带的段落应被剥离
	if strings.Contains(out, "先分析用户需求") {
		t.Fatalf("markdown thinking 段应剥离: %q", out)
	}
	if !strings.Contains(out, "## 结论") || !strings.Contains(out, "使用 A 方案") {
		t.Fatalf("结论段应保留: %q", out)
	}
}

func TestCompactMessageContent_HeadTailTruncateWhenNoHandoff(t *testing.T) {
	// 无 handoff、剥离后仍超长 → head-tail 截断
	body := strings.Repeat("X", 2000)
	out := CompactMessageContent(body, 800)
	if len(out) > 900 {
		t.Fatalf("超长应被截断到 ~800 chars 附近，got len=%d", len(out))
	}
	if !strings.Contains(out, "truncated") {
		t.Fatalf("应带 truncated marker: %q", out[:min(200, len(out))])
	}
}

func TestCompactMessageContent_ShortContentPassThrough(t *testing.T) {
	content := "简短回复"
	out := CompactMessageContent(content, 800)
	if out != "简短回复" {
		t.Fatalf("短内容应原样通过: got %q", out)
	}
}

func TestCompactMessageContent_CollapsesBlankLines(t *testing.T) {
	content := "第一段\n\n\n\n\n第二段"
	out := CompactMessageContent(content, 800)
	if strings.Contains(out, "\n\n\n") {
		t.Fatalf("3+ 空行应折叠为 2: %q", out)
	}
}

// -----------------------------------------------------------------------------
// AssembleIncrementalContext 新增行为：使用 compaction + 新 header 格式
// -----------------------------------------------------------------------------

func TestAssembleIncrementalContext_PicksHandoffFromLegacyStoredContent(t *testing.T) {
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	msgRepo := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	deps := &IncrementalContextDeps{
		MsgRepo:     msgRepo,
		CursorStore: NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite)),
	}
	tid := uuid.New()
	self := uuid.New()
	upstream := uuid.New()

	// 上游 agent 输出：一大段说明 + handoff 块
	upstreamContent := `我作为需求分析师，本轮已经完成需求梳理。
<thinking>
用户要一个登录功能，我需要...
（很长的思考）
</thinking>
分析结果如下（大量正文）：
1. 用户名 + 密码认证
2. JWT token 返回
3. session 过期机制

<a2a-handoff>
@全栈开发 请实现登录 API：
- POST /login
- 输入: {username, password}
- 输出: {token, expiresAt}
</a2a-handoff>

以上是本轮结论，供下游参考。`

	insertMsg(t, msgRepo, tid, model.MessageRoleAgent, upstream.String(), upstreamContent)

	res, err := AssembleIncrementalContext(context.Background(), deps, tid, AssembleOptions{
		SelfAgentID: self,
		MaxTokens:   4000,
	})
	if err != nil {
		t.Fatal(err)
	}

	// handoff 应出现，thinking / 长正文应消失
	if !strings.Contains(res.ContextText, "@全栈开发") {
		t.Fatalf("handoff 应出现: %s", res.ContextText)
	}
	if strings.Contains(res.ContextText, "用户要一个登录功能") {
		t.Fatalf("thinking 应被剥离: %s", res.ContextText)
	}
	if strings.Contains(res.ContextText, "以上是本轮结论") {
		t.Fatalf("handoff 优先时应丢弃 handoff 后的说明: %s", res.ContextText)
	}
	// header 应包含 agentID（sortable id UUID）
	if !strings.Contains(res.ContextText, upstream.String()) {
		t.Fatalf("header 应包含 upstream agentID: %s", res.ContextText)
	}
}

func TestAssembleIncrementalContext_ReducesTotalSizeVsRaw(t *testing.T) {
	// 冒烟测试：验证 compaction 确实缩小了 ContextText 相比于原始 stored content 的总长度
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	msgRepo := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	deps := &IncrementalContextDeps{
		MsgRepo:     msgRepo,
		CursorStore: NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite)),
	}
	tid := uuid.New()
	self := uuid.New()

	// 3 条上游消息，各含大量 thinking 块（模拟真实场景）
	longThinking := strings.Repeat("这是一段冗长的思考推理过程，反复权衡各种可能。", 30)
	for i := 0; i < 3; i++ {
		content := "结论：" + strings.Repeat("要点 ", 5) +
			"\n<thinking>\n" + longThinking + "\n</thinking>\n" +
			"补充说明：本轮完成"
		up := uuid.New()
		insertMsg(t, msgRepo, tid, model.MessageRoleAgent, up.String(), content)
	}

	res, err := AssembleIncrementalContext(context.Background(), deps, tid, AssembleOptions{
		SelfAgentID: self,
		MaxTokens:   4000,
	})
	if err != nil {
		t.Fatal(err)
	}

	// compaction 生效后应显著小于原始 3 * longThinking 总量
	if len(res.ContextText) > 5000 {
		t.Fatalf("compaction 效果不足，ContextText=%d chars (期望 <5000)", len(res.ContextText))
	}
	if !strings.Contains(res.ContextText, "结论") {
		t.Fatalf("正文应保留: %s", res.ContextText[:min(500, len(res.ContextText))])
	}
	if strings.Contains(res.ContextText, "冗长的思考推理过程") {
		t.Fatalf("thinking 应被剥离: %s", res.ContextText[:min(500, len(res.ContextText))])
	}
}
