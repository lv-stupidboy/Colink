package ws

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
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

// Handler WebSocket处理器
type Handler struct {
	hub *Hub
}

// NewHandler 创建Handler
func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
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

	// 启动读写协程
	go client.WritePump()
	go client.ReadPump()
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/ws", h.HandleWebSocket)
}