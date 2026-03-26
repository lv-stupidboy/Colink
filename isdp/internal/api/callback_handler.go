package api

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/a2a"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/service/mention"
	"github.com/anthropic/isdp/internal/service/message"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CallbackHandler MCP Callback 路由处理器
type CallbackHandler struct {
	registry      *a2a.InvocationRegistry
	mcpAuth       *a2a.MCPAuthService
	messageSvc    *message.Service
	msgRepo       *repo.MessageRepository
	wsHub         *ws.Hub
	orchestrator  *agent.Orchestrator
	baseAgentRepo *repo.BaseAgentRepository
	queue         *a2a.InvocationQueue
	queueProcessor *a2a.QueueProcessor

	// Mention 解析器（支持动态 patterns）
	mentionParser *mention.Parser
}

// NewCallbackHandler 创建 Callback 处理器
func NewCallbackHandler(
	registry *a2a.InvocationRegistry,
	mcpAuth *a2a.MCPAuthService,
	messageSvc *message.Service,
	msgRepo *repo.MessageRepository,
	wsHub *ws.Hub,
	orchestrator *agent.Orchestrator,
	baseAgentRepo *repo.BaseAgentRepository,
	queue *a2a.InvocationQueue,
	queueProcessor *a2a.QueueProcessor,
	mentionParser *mention.Parser,
) *CallbackHandler {
	return &CallbackHandler{
		registry:       registry,
		mcpAuth:        mcpAuth,
		messageSvc:     messageSvc,
		msgRepo:        msgRepo,
		wsHub:          wsHub,
		orchestrator:   orchestrator,
		baseAgentRepo:  baseAgentRepo,
		queue:          queue,
		queueProcessor: queueProcessor,
		mentionParser:  mentionParser,
	}
}

// PostMessageRequest post-message 请求
type PostMessageRequest struct {
	InvocationID     string `json:"invocationId" binding:"required"`
	CallbackToken    string `json:"callbackToken" binding:"required"`
	Content          string `json:"content" binding:"required,max=50000"`
	ThreadID         string `json:"threadId"`          // 可选：跨线程发送
	ReplyTo          string `json:"replyTo"`           // 可选：回复的消息 ID
	ClientMessageID  string `json:"clientMessageId"`   // 可选：客户端消息 ID（幂等性）
	TargetCats       []string `json:"targetCats"`       // 可选：显式指定目标 Agent
}

// PostMessageResponse post-message 响应
type PostMessageResponse struct {
	Status       string `json:"status"`
	ThreadID     string `json:"threadId,omitempty"`
	MessageID    string `json:"messageId,omitempty"`
	ReplyTo      string `json:"replyTo,omitempty"`
	ClientMessageID string `json:"clientMessageId,omitempty"`
}

// PostMessage Agent 主动发消息
// POST /api/callbacks/post-message
func (h *CallbackHandler) PostMessage(c *gin.Context) {
	var req PostMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	invocationID, err := uuid.Parse(req.InvocationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid invocationId"})
		return
	}

	// 验证调用身份
	record := h.registry.Verify(invocationID, req.CallbackToken)
	if record == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "expired_credentials",
			"message": "Invocation ID or callback token is invalid or expired",
		})
		return
	}

	// 过期调用保护
	if !h.registry.IsLatest(invocationID) {
		c.JSON(http.StatusOK, PostMessageResponse{
			Status: "stale_ignored",
		})
		return
	}

	// 确定目标线程
	effectiveThreadID := record.ThreadID
	if req.ThreadID != "" && req.ThreadID != record.ThreadID.String() {
		targetThreadID, err := uuid.Parse(req.ThreadID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid threadId"})
			return
		}
		// TODO: 验证用户对目标线程的访问权限
		effectiveThreadID = targetThreadID
	}

	// 幂等性检查
	if req.ClientMessageID != "" {
		// TODO: 实现客户端消息 ID 去重
		// 可以使用 Redis 或内存缓存
	}

	// 解析 A2A mentions（使用动态 MentionParser）
	var mentions []string
	if h.mentionParser != nil {
		var err error
		mentions, err = h.mentionParser.Parse(c.Request.Context(), req.Content, record.CatID)
		if err != nil {
			// 解析失败，记录错误，使用空列表
			// 不再回退到硬编码的静态解析
		}
	}

	// 合并显式指定的目标
	allMentions := mergeMentions(mentions, req.TargetCats)

	// 存储消息
	msg := &model.Message{
		ThreadID:  effectiveThreadID,
		Role:      model.MessageRoleAgent,
		AgentID:   record.CatID,
		Content:   req.Content,
		Mentions:  allMentions,
		Origin:    "callback",
	}

	// 处理回复
	if req.ReplyTo != "" {
		replyToID, err := uuid.Parse(req.ReplyTo)
		if err == nil {
			msg.ReplyTo = &replyToID
		}
	}

	if err := h.msgRepo.Create(c.Request.Context(), msg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 广播消息到 WebSocket
	if h.wsHub != nil {
		h.wsHub.BroadcastToThread(effectiveThreadID.String(), ws.WSMessage{
			Type:      "agent_message",
			ThreadID:  effectiveThreadID.String(),
			Timestamp: msg.CreatedAt.UnixMilli(),
			Payload: map[string]interface{}{
				"messageId": msg.ID.String(),
				"agentId":   record.CatID,
				"content":   req.Content,
				"origin":    "callback",
				"mentions":  allMentions,
			},
		})
	}

	// 触发 A2A
	if len(allMentions) > 0 && h.orchestrator != nil {
		go h.triggerA2A(context.Background(), effectiveThreadID, allMentions, req.Content, record)
	}

	c.JSON(http.StatusOK, PostMessageResponse{
		Status:          "ok",
		ThreadID:        effectiveThreadID.String(),
		MessageID:       msg.ID.String(),
		ReplyTo:         req.ReplyTo,
		ClientMessageID: req.ClientMessageID,
	})
}

// PendingMentionsRequest pending-mentions 请求
type PendingMentionsRequest struct {
	InvocationID  string `form:"invocationId" binding:"required"`
	CallbackToken string `form:"callbackToken" binding:"required"`
	IncludeAcked  string `form:"includeAcked"` // "true" or "1"
}

// PendingMention 待处理的 mention
type PendingMention struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// PendingMentions 获取待处理的 @mentions
// GET /api/callbacks/pending-mentions
func (h *CallbackHandler) PendingMentions(c *gin.Context) {
	var req PendingMentionsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	invocationID, err := uuid.Parse(req.InvocationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid invocationId"})
		return
	}

	// 验证调用身份
	record := h.registry.Verify(invocationID, req.CallbackToken)
	if record == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "expired_credentials",
			"message": "Invocation ID or callback token is invalid or expired",
		})
		return
	}

	// 获取该 Agent 被 mention 的消息
	messages, err := h.msgRepo.FindMentionsForAgent(c.Request.Context(), record.ThreadID, record.CatID, 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 转换为响应格式
	mentions := make([]PendingMention, 0, len(messages))
	for _, msg := range messages {
		mentions = append(mentions, PendingMention{
			ID:        msg.ID.String(),
			From:      getFrom(msg),
			Message:   msg.Content,
			Timestamp: msg.CreatedAt.UnixMilli(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"mentions": mentions,
	})
}

// ThreadContextRequest thread-context 请求
type ThreadContextRequest struct {
	InvocationID  string `form:"invocationId" binding:"required"`
	CallbackToken string `form:"callbackToken" binding:"required"`
	ThreadID      string `form:"threadId"`  // 可选：读取其他线程
	CatID         string `form:"catId"`     // 可选：过滤特定 Agent 的消息
	Keyword       string `form:"keyword"`   // 可选：关键词搜索
	Limit         int    `form:"limit"`     // 可选：消息数量限制
}

// ThreadContextMessage 线程上下文消息
type ThreadContextMessage struct {
	ID        string `json:"id"`
	UserID    string `json:"userId,omitempty"`
	CatID     string `json:"catId,omitempty"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

// ThreadContext 获取线程上下文
// GET /api/callbacks/thread-context
func (h *CallbackHandler) ThreadContext(c *gin.Context) {
	var req ThreadContextRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	invocationID, err := uuid.Parse(req.InvocationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid invocationId"})
		return
	}

	// 验证调用身份
	record := h.registry.Verify(invocationID, req.CallbackToken)
	if record == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "expired_credentials",
			"message": "Invocation ID or callback token is invalid or expired",
		})
		return
	}

	// 确定目标线程
	threadID := record.ThreadID
	if req.ThreadID != "" {
		targetThreadID, err := uuid.Parse(req.ThreadID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid threadId"})
			return
		}
		// TODO: 验证用户对目标线程的访问权限
		threadID = targetThreadID
	}

	// 获取消息
	limit := req.Limit
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	messages, err := h.msgRepo.FindByThreadID(c.Request.Context(), threadID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 转换为响应格式
	contextMessages := make([]ThreadContextMessage, 0, len(messages))
	for _, msg := range messages {
		// 可选过滤
		if req.CatID != "" {
			if req.CatID == "user" && msg.Role != model.MessageRoleUser {
				continue
			}
			if req.CatID != "user" && msg.AgentID != req.CatID {
				continue
			}
		}

		// 关键词过滤
		if req.Keyword != "" && !strings.Contains(strings.ToLower(msg.Content), strings.ToLower(req.Keyword)) {
			continue
		}

		contextMessages = append(contextMessages, ThreadContextMessage{
			ID:        msg.ID.String(),
			CatID:     msg.AgentID,
			Content:   msg.Content,
			Timestamp: msg.CreatedAt.UnixMilli(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"threadId": threadID.String(),
		"messages": contextMessages,
	})
}

// triggerA2A 触发 A2A 协作
// 参考 Clowder AI 的 enqueueA2ATargets 实现
func (h *CallbackHandler) triggerA2A(ctx context.Context, threadID uuid.UUID, mentions []string, content string, record *a2a.InvocationRecord) {
	if len(mentions) == 0 {
		return
	}

	// 构建触发消息
	triggerMsg := &model.Message{
		ID:        uuid.New(),
		ThreadID:  threadID,
		Content:   content,
		AgentID:   record.CatID,
		Mentions:  mentions,
		Origin:    "callback",
	}

	// 构建依赖
	deps := &a2a.A2ATriggerDeps{
		Registry:     h.registry,
		Orchestrator: h.orchestrator,
		WSHub:        h.wsHub,
		Queue:        h.queue,
	}

	// 构建选项
	opts := &a2a.A2ATriggerOptions{
		TargetCats:     mentions,
		Content:        content,
		UserID:         record.UserID,
		ThreadID:       threadID,
		TriggerMessage: triggerMsg,
		CallerCatID:    record.CatID,
	}

	// 调用 A2A 触发
	result, err := a2a.EnqueueA2ATargets(ctx, deps, opts)
	if err != nil {
		return
	}

	// 如果有入队的条目且 QueueProcessor 可用，触发自动执行
	if len(result.Enqueued) > 0 && h.queueProcessor != nil {
		_ = h.queueProcessor.TryAutoExecute(ctx, threadID)
	}
}

// RegisterRoutes 注册路由
func (h *CallbackHandler) RegisterRoutes(r *gin.RouterGroup) {
	callbacks := r.Group("/callbacks")
	{
		callbacks.POST("/post-message", h.PostMessage)
		callbacks.GET("/pending-mentions", h.PendingMentions)
		callbacks.GET("/thread-context", h.ThreadContext)
	}
}

// 辅助函数

func mergeMentions(parsed []string, explicit []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, m := range parsed {
		if !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}

	for _, m := range explicit {
		if !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}

	return result
}

func getFrom(msg *model.Message) string {
	if msg.AgentID != "" {
		return msg.AgentID
	}
	return "user"
}

// 代码块正则表达式
var codeBlockRegex = regexp.MustCompile("```[\\s\\S]*?```")