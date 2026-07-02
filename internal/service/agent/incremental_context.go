// Package agent — Incremental Context Assembler
//
// 借鉴 clowder-ai packages/api/src/domains/cats/services/agents/routing/route-helpers.ts
// 的 assembleIncrementalContext + fetchAfterCursor + upsertMaxBoundary：
//
//   1) 读 cursor
//   2) msg store 拉 sortable_id > cursor 的所有消息
//   3) 内存过滤（system / own / 空内容）
//   4) 按 token 预算尾部保留（保尾丢头）
//   5) 组装文本供 prompt 拼接
//   6) 返回 boundaryID —— 供 invocation 结束后统一 ack
//
// deferred ack 语义（借鉴 clowder-ai messages.ts:1048 + AgentRouter.ts:1722）：
//   - assemble 时不 ack，只返回 boundary
//   - invocation 结束（success/abort/fail 三条路径）时统一 ackCollectedCursors
//   - "错误时也 ack" —— messages 已进入 prompt，不 ack 会导致重复投递
//     （clowder-ai 注释："砚砚每次都疯狂回之前的消息"）
package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// IncrementalContextDeps 组装增量上下文需要的依赖
// 单独抽出便于测试注入 mock
type IncrementalContextDeps struct {
	MsgRepo     *repo.MessageRepository
	CursorStore *DeliveryCursorStore
}

// IncrementalContextResult 组装结果
type IncrementalContextResult struct {
	// ContextText 拼接好的 <incremental-context> 块；空表示无未读
	ContextText string

	// BoundaryID 本次拉到的最大 sortable_id
	// 空表示无消息可 ack；调用方 invocation 结束时应 ack 此值
	BoundaryID string

	// UnreadCount 拉到的原始未读消息数（过滤前）
	UnreadCount int

	// DeliveredCount 实际拼进 prompt 的消息数
	DeliveredCount int

	// Truncated 是否因预算裁剪丢弃了部分未读
	Truncated bool

	// TruncationNote 裁剪情况的简要说明（用于日志）
	TruncationNote string
}

// AssembleOptions 可选参数
type AssembleOptions struct {
	// MaxTokens 组装内容的 token 预算（估算，非精确 tokenizer）
	// 默认 4000 —— 与 clowder-ai ContextAssembler.ts:34 DEFAULT_MAX_TOTAL_TOKENS 对齐留有余地
	MaxTokens int

	// MaxMessages 拉取时的 SQL LIMIT，防止未读堆积炸内存
	// 默认 200
	MaxMessages int

	// SelfAgentID 当前 Agent ID —— 过滤自己发的消息（不再拉自己上一轮的输出）
	SelfAgentID uuid.UUID

	// IncludeSystem 是否包含 role=system 的消息（默认 false）
	IncludeSystem bool
}

// AssembleIncrementalContext 拉 cursor 之后的未读消息并组装 prompt 片段
//
// 返回值：
//   - ContextText 可直接拼进 prompt 前缀
//   - BoundaryID 供 caller 在 invocation 结束时 ack
//
// 错误处理：
//   - cursor 读失败 → 视作 cursor="" 从头拉（保守语义）
//   - msg 拉失败 → 返回错误，caller 决定是否阻塞
//   - filter 全过滤空 → 依然返回 BoundaryID 供 ack（防止无限重投）
func AssembleIncrementalContext(
	ctx context.Context,
	deps *IncrementalContextDeps,
	threadID uuid.UUID,
	opts AssembleOptions,
) (*IncrementalContextResult, error) {
	if deps == nil || deps.MsgRepo == nil || deps.CursorStore == nil {
		return &IncrementalContextResult{}, nil
	}
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 4000
	}
	if opts.MaxMessages <= 0 {
		opts.MaxMessages = 200
	}

	// 1) 读 cursor（失败降级为从头拉，保守）
	cursor, err := deps.CursorStore.GetCursor(ctx, threadID, opts.SelfAgentID)
	if err != nil {
		cursor = ""
	}

	// 2) 拉未读
	unread, err := deps.MsgRepo.GetByThreadAfter(ctx, threadID, cursor, opts.MaxMessages)
	if err != nil {
		return nil, fmt.Errorf("GetByThreadAfter: %w", err)
	}
	if len(unread) == 0 {
		return &IncrementalContextResult{}, nil
	}

	// 3) 过滤（对齐 clowder-ai route-helpers.ts:715-730）
	relevant := make([]*model.Message, 0, len(unread))
	for _, m := range unread {
		if !opts.IncludeSystem && m.Role == model.MessageRoleSystem {
			continue
		}
		// 自己发的不再拉（防止 Agent 看到自己上一轮的输出重复触发）
		if opts.SelfAgentID != uuid.Nil && m.AgentID == opts.SelfAgentID.String() {
			continue
		}
		// 空内容跳过（可能是占位消息）
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		relevant = append(relevant, m)
	}

	// 4) 边界值一定要取到 —— 即使 relevant 为空也要推进 cursor
	// 借鉴 clowder-ai 的关键 bug 教训："不 ack 导致无限重复投递"
	boundaryID := unread[len(unread)-1].SortableID

	if len(relevant) == 0 {
		return &IncrementalContextResult{
			BoundaryID:  boundaryID,
			UnreadCount: len(unread),
		}, nil
	}

	// 5) 按 token 预算裁剪（保尾丢头 —— 越新越重要）
	//    先对每条 content 做 CompactMessageContent 精简（handoff 优先 / 剥 thinking / 头尾截断），
	//    再基于精简后的长度做 token 预算裁剪。避免长 thinking 块吃掉全部预算。
	compacted := make([]compactedMessage, len(relevant))
	for i, m := range relevant {
		compacted[i] = compactedMessage{
			m:       m,
			content: CompactMessageContent(m.Content, 800),
		}
	}
	kept, truncated := trimCompactedByTokenBudget(compacted, opts.MaxTokens)

	// 6) 组装 <incremental-context> 块
	var sb strings.Builder
	sb.WriteString("<incremental-context>\n")
	if truncated {
		sb.WriteString(fmt.Sprintf("<!-- 已裁剪 %d 条较早未读消息（token 预算） -->\n", len(compacted)-len(kept)))
	}
	for _, c := range kept {
		sender := "用户"
		if c.m.Role == model.MessageRoleAgent {
			sender = c.m.AgentID
			if sender == "" {
				sender = "agent"
			}
		}
		// 精简 header：省掉秒级时间戳（保留分钟即可，节省 ~3 chars/条）
		sb.WriteString(fmt.Sprintf("[%s @ %s]\n%s\n\n", sender, c.m.CreatedAt.Format("15:04"), c.content))
	}
	sb.WriteString("</incremental-context>\n")

	note := ""
	if truncated {
		note = fmt.Sprintf("kept %d of %d relevant unread within %d tokens", len(kept), len(compacted), opts.MaxTokens)
	}

	return &IncrementalContextResult{
		ContextText:    sb.String(),
		BoundaryID:     boundaryID,
		UnreadCount:    len(unread),
		DeliveredCount: len(kept),
		Truncated:      truncated,
		TruncationNote: note,
	}, nil
}

// compactedMessage 消息 + 已精简后的 content
// 用于避免在预算裁剪循环里重复计算 CompactMessageContent
type compactedMessage struct {
	m       *model.Message
	content string
}

// trimCompactedByTokenBudget 保尾丢头版本
// 与 trimMessagesByTokenBudget 语义相同，只是 body 用精简后的 content 计算 tokens。
func trimCompactedByTokenBudget(msgs []compactedMessage, maxTokens int) (kept []compactedMessage, truncated bool) {
	if len(msgs) == 0 || maxTokens <= 0 {
		return msgs, false
	}
	used := 0
	start := len(msgs)
	for i := len(msgs) - 1; i >= 0; i-- {
		tk := EstimateTokens(msgs[i].content) + 20 // +20 for header overhead
		if used+tk > maxTokens {
			break
		}
		used += tk
		start = i
	}
	if start == 0 {
		return msgs, false
	}
	return msgs[start:], true
}

// trimMessagesByTokenBudget 按预算裁剪，保尾丢头
//
// 语义：从最新一条往前累加 EstimateTokens，直到超预算就停止；
// 保留的一定是尾部（最近的）消息序列。
func trimMessagesByTokenBudget(msgs []*model.Message, maxTokens int) (kept []*model.Message, truncated bool) {
	if len(msgs) == 0 || maxTokens <= 0 {
		return msgs, false
	}
	used := 0
	start := len(msgs)
	for i := len(msgs) - 1; i >= 0; i-- {
		tk := EstimateTokens(msgs[i].Content) + 20 // +20 for header overhead
		if used+tk > maxTokens {
			break
		}
		used += tk
		start = i
	}
	if start == 0 {
		return msgs, false
	}
	return msgs[start:], true
}

// ============================================================================
// upsertMaxBoundary + AckCollectedCursors — deferred ack buffer
//
// 借鉴 clowder-ai route-helpers.ts:227-232 upsertMaxBoundary：
//   同一 Agent 一次 invocation 内可能被多次 assembleIncrementalContext
//   （A2A re-entry），必须取所有 boundary 的最大值，防止较小 boundary 回退。
// ============================================================================

// CursorBoundaryBuffer 单次 invocation 生命周期的 boundary 累积器
// 一个 invocation 一份，最后统一 ack
type CursorBoundaryBuffer struct {
	mu         sync.Mutex
	boundaries map[uuid.UUID]string // agentID → max boundary observed
}

// NewCursorBoundaryBuffer 构造
func NewCursorBoundaryBuffer() *CursorBoundaryBuffer {
	return &CursorBoundaryBuffer{
		boundaries: make(map[uuid.UUID]string),
	}
}

// UpsertMax 只前进不回退地记录 boundary
// 借鉴 clowder-ai upsertMaxBoundary L227-232：if !current || boundaryID > current
func (b *CursorBoundaryBuffer) UpsertMax(agentID uuid.UUID, boundaryID string) {
	if boundaryID == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if prev, ok := b.boundaries[agentID]; !ok || boundaryID > prev {
		b.boundaries[agentID] = boundaryID
	}
}

// Snapshot 返回当前所有记录的副本（用于 flush）
func (b *CursorBoundaryBuffer) Snapshot() map[uuid.UUID]string {
	b.mu.Lock()
	defer b.mu.Unlock()
	snap := make(map[uuid.UUID]string, len(b.boundaries))
	for k, v := range b.boundaries {
		snap[k] = v
	}
	return snap
}

// Size 当前 buffer 中的 agent 数量
func (b *CursorBoundaryBuffer) Size() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.boundaries)
}

// AckAll 一次性 ack 所有累积的 boundary
//
// 借鉴 clowder-ai AgentRouter.ts:1722-1730 ackCollectedCursors：
//   for (const [catId, boundaryId] of boundaries) {
//     try { await this.deliveryCursorStore.ackCursor(...); }
//     catch (err) { log.error(...); }
//   }
//
// 语义：每个 ack 失败不影响其它 ack；返回第一个错误供 caller 打日志
func (b *CursorBoundaryBuffer) AckAll(ctx context.Context, store *DeliveryCursorStore, threadID uuid.UUID) error {
	snap := b.Snapshot()
	var firstErr error
	for agentID, boundary := range snap {
		if err := store.AckCursor(ctx, threadID, agentID, boundary); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
