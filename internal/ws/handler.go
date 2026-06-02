package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// 心跳保活配置
const (
	// pingInterval 后端发送 ping 的间隔
	pingInterval = 30 * time.Second
	// pongTimeout 等待 pong 响应的超时时间（超过此时间认为连接断开）
	pongTimeout = 60 * time.Second
	// writeTimeout 写操作超时
	writeTimeout = 10 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 生产环境需要验证来源
	},
}

// wsLogger WebSocket 日志记录器
var wsLogger *zap.Logger

// SetWSLogger 设置 WebSocket 日志记录器
func SetWSLogger(logger *zap.Logger) {
	wsLogger = logger
}

// RunningAgentsGetter 获取运行中 Agent 状态的接口
type RunningAgentsGetter interface {
	GetRunningAgentsForThread(threadID uuid.UUID) any
}

// InvocationRecoverer 恢复 invocation 状态的接口（后台执行支持）
type InvocationRecoverer interface {
	// GetRunningInvocationsWithContentBlocks 获取运行中的 invocation 及其内容块
	GetRunningInvocationsWithContentBlocks(ctx context.Context, threadID uuid.UUID) []InvocationRecoveryData
	// GetRecentlyCompletedInvocations 获取最近完成的 invocation（用于状态同步）
	GetRecentlyCompletedInvocations(ctx context.Context, threadID uuid.UUID, sinceMinutes int) []InvocationRecoveryData
}

// InvocationRecoveryData invocation 恢复数据
type InvocationRecoveryData struct {
	InvocationID  string      `json:"invocationId"`
	AgentID       string      `json:"agentId"`
	AgentName     string      `json:"agentName"`
	Status        string      `json:"status"`
	ContentBlocks interface{} `json:"contentBlocks"`
}

// Handler WebSocket处理器
type Handler struct {
	hub                 *Hub
	runningAgentsGetter RunningAgentsGetter
	invocationRecoverer InvocationRecoverer
	cancelAgentFunc     func(ctx context.Context, invocationID uuid.UUID) error
}

// NewHandler 创建Handler
func NewHandler(hub *Hub, runningAgentsGetter RunningAgentsGetter, invocationRecoverer InvocationRecoverer) *Handler {
	return &Handler{
		hub:                 hub,
		runningAgentsGetter: runningAgentsGetter,
		invocationRecoverer: invocationRecoverer,
	}
}

// SetCancelAgentFunc 设置取消Agent的回调函数
func (h *Handler) SetCancelAgentFunc(fn func(ctx context.Context, invocationID uuid.UUID) error) {
	h.cancelAgentFunc = fn
}

// HandleWebSocket 处理WebSocket连接
func (h *Handler) HandleWebSocket(c *gin.Context) {
	threadID := c.Query("threadId")
	userID := c.Query("userId")

	if wsLogger != nil {
		wsLogger.Info("WebSocket connection request", zap.String("threadId", threadID), zap.String("userId", userID))
	}

	if threadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "threadId required"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		if wsLogger != nil {
			wsLogger.Error("WebSocket upgrade failed", zap.Error(err))
		}
		return
	}

	if wsLogger != nil {
		wsLogger.Info("WebSocket connected", zap.String("threadId", threadID))
	}

	client := &Client{
		Hub:      h.hub,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		ThreadID: threadID,
		UserID:   userID,
		Handler:  h,
	}

	h.hub.register <- client

	// 发送连接成功消息
	connectedMsg := WSMessage{
		Type:      "connected",
		ThreadID:  threadID,
		Timestamp: time.Now().UnixMilli(),
		Payload: map[string]interface{}{
			"message": "WebSocket connected successfully",
		},
	}
	data, _ := jsonMarshal(connectedMsg)
	client.Send <- data

	// 发送运行中 Agent 的状态恢复消息
	if h.runningAgentsGetter != nil {
		threadUUID, err := uuid.Parse(threadID)
		if err == nil {
			runningAgents := h.runningAgentsGetter.GetRunningAgentsForThread(threadUUID)
			if runningAgents != nil {
				restoreMsg := WSMessage{
					Type:      "agent_state_restore",
					ThreadID:  threadID,
					Timestamp: time.Now().UnixMilli(),
					Payload: map[string]interface{}{
						"runningAgents": runningAgents,
					},
				}
				restoreData, _ := jsonMarshal(restoreMsg)
				client.Send <- restoreData
				if wsLogger != nil {
					wsLogger.Info("Sent agent state restore", zap.String("threadId", threadID))
				}
			}

			// 发送最近完成的 invocation 状态（用于重连时同步丢失的完成状态）
			if h.invocationRecoverer != nil {
				completedInvocations := h.invocationRecoverer.GetRecentlyCompletedInvocations(context.Background(), threadUUID, 30) // 30分钟内完成的
				if len(completedInvocations) > 0 {
					completedMsg := WSMessage{
						Type:      "invocation_recovery",
						ThreadID:  threadID,
						Timestamp: time.Now().UnixMilli(),
						Payload: map[string]interface{}{
							"recentlyCompleted": completedInvocations,
						},
					}
					completedData, _ := jsonMarshal(completedMsg)
					client.Send <- completedData
					if wsLogger != nil {
						wsLogger.Info("Sent recently completed invocations",
							zap.String("threadId", threadID),
							zap.Int("count", len(completedInvocations)))
					}
				}
			}
		}
	}

	// 启动读写协程
	go client.WritePump()
	go client.ReadPump()
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/ws", h.HandleWebSocket)
}

// ClientMessage 客户端发送的消息
type ClientMessage struct {
	Type         string `json:"type"`
	ThreadID     string `json:"threadId"`
	InvocationID string `json:"invocationId,omitempty"`
}

// ReadPump 读取客户端消息（支持处理客户端请求）
// 配合 WritePump 的 ping/pong 心跳机制，检测连接健康状态
// 如果 pongTimeout 内没有收到任何消息（包括 pong），则断开连接
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	// 设置 pong 响应处理：收到 pong 时重置读超时
	// 浏览器 WebSocket 收到 ping 会自动回复 pong
	c.Conn.SetPongHandler(func(string) error {
		if wsLogger != nil {
			wsLogger.Debug("Received pong from client", zap.String("threadId", c.ThreadID))
		}
		// 重置读超时（连接仍然活跃）
		return c.Conn.SetReadDeadline(time.Now().Add(pongTimeout))
	})

	// 设置初始读超时
	c.Conn.SetReadDeadline(time.Now().Add(pongTimeout))

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			// 1005 (No Status) 是客户端调用 close() 不传参数时的正常关闭
			// 1000 (Normal) 是显式正常关闭
			// 1001 (GoingAway) 是浏览器关闭页面/导航
			// 1006 (Abnormal) 是异常关闭（无 close frame）
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
				if wsLogger != nil {
					wsLogger.Warn("Read error", zap.Error(err), zap.String("threadId", c.ThreadID))
				}
			} else if wsLogger != nil {
				wsLogger.Info("Connection closed normally", zap.String("threadId", c.ThreadID), zap.Error(err))
			}
			break
		}

		// 重置读超时（收到消息表示连接活跃）
		c.Conn.SetReadDeadline(time.Now().Add(pongTimeout))

		// 解析消息
		var msg ClientMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		// 处理不同类型的消息
		switch msg.Type {
		case "recover_invocation_state":
			c.handleRecoverInvocationState(msg.ThreadID)
		case "cancel_invocation":
			c.handleCancelInvocation(msg.ThreadID, msg.InvocationID)
		}
	}
}

// handleRecoverInvocationState 处理恢复 invocation 状态请求
func (c *Client) handleRecoverInvocationState(threadID string) {
	if c.Handler == nil || c.Handler.invocationRecoverer == nil {
		return
	}

	threadUUID, err := uuid.Parse(threadID)
	if err != nil {
		return
	}

	// 获取运行中的 invocation 及其内容块
	recoveryData := c.Handler.invocationRecoverer.GetRunningInvocationsWithContentBlocks(context.Background(), threadUUID)
	if len(recoveryData) == 0 {
		return
	}

	// 发送恢复消息
	for _, data := range recoveryData {
		recoveryMsg := WSMessage{
			Type:      "invocation_recovery",
			ThreadID:  threadID,
			Timestamp: time.Now().UnixMilli(),
			Payload: map[string]interface{}{
				"invocationId":  data.InvocationID,
				"agentId":       data.AgentID,
				"agentName":     data.AgentName,
				"status":        data.Status,
				"contentBlocks": data.ContentBlocks,
			},
		}
		recoveryData, _ := jsonMarshal(recoveryMsg)
		c.Send <- recoveryData

		if wsLogger != nil {
			wsLogger.Info("Sent invocation recovery",
				zap.String("threadId", threadID),
				zap.String("invocationId", data.InvocationID))
		}
	}
}

// handleCancelInvocation 处理取消 invocation 请求
func (c *Client) handleCancelInvocation(threadID, invocationID string) {
	if c.Handler == nil || c.Handler.cancelAgentFunc == nil {
		return
	}

	// 验证用户在 thread room 中
	if c.ThreadID != threadID {
		return
	}

	invocationUUID, err := uuid.Parse(invocationID)
	if err != nil {
		if wsLogger != nil {
			wsLogger.Warn("Invalid invocationId", zap.String("invocationId", invocationID))
		}
		return
	}

	if err := c.Handler.cancelAgentFunc(context.Background(), invocationUUID); err != nil {
		if wsLogger != nil {
			wsLogger.Warn("CancelAgent failed", zap.Error(err), zap.String("invocationId", invocationID))
		}
	} else {
		if wsLogger != nil {
			wsLogger.Info("Invocation cancelled", zap.String("threadId", threadID), zap.String("invocationId", invocationID))
		}
	}
}

// WritePump 写入消息到客户端（带心跳保活）
// 定期发送 ping 消息，检测连接健康状态
func (c *Client) WritePump() {
	defer c.Conn.Close()

	// 创建定时器：定期发送 ping
	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()

	for {
		select {
		case message, ok := <-c.Send:
			// 设置写超时
			c.Conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if !ok {
				// Send channel 关闭，服务端主动关闭连接
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				if wsLogger != nil {
					wsLogger.Warn("Write message failed", zap.Error(err), zap.String("threadId", c.ThreadID))
				}
				return
			}

		case <-pingTicker.C:
			// 发送 ping 消息，保持连接活跃
			c.Conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.Conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				if wsLogger != nil {
					wsLogger.Warn("Ping failed, closing connection", zap.Error(err), zap.String("threadId", c.ThreadID))
				}
				return
			}
			if wsLogger != nil {
				wsLogger.Debug("Sent ping to client", zap.String("threadId", c.ThreadID))
			}
		}
	}
}
