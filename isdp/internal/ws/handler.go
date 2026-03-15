package ws

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 生产环境需要验证来源
	},
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

	fmt.Printf("[WebSocket] Connection request - threadId: %s, userId: %s\n", threadID, userID)

	if threadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "threadId required"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Printf("[WebSocket] Upgrade failed: %v\n", err)
		return
	}

	fmt.Printf("[WebSocket] Connection upgraded successfully - threadId: %s\n", threadID)

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