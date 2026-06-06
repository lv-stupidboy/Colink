package ws

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client WebSocket客户端
type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	Send     chan []byte
	ThreadID string
	UserID   string
	Handler  *Handler // 处理器引用（用于消息处理）
}

// Hub WebSocket中心
type Hub struct {
	clients     map[string]map[*Client]bool // threadID -> clients
	broadcast   chan *BroadcastMessage
	register    chan *Client
	unregister  chan *Client
	clientsMux  sync.RWMutex
}

// BroadcastMessage 广播消息
type BroadcastMessage struct {
	ThreadID string
	Message  []byte
}

// WSMessage WebSocket消息结构
type WSMessage struct {
	Type      string                 `json:"type"`
	ThreadID  string                 `json:"threadId"`
	Timestamp int64                  `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`
}

// NewHub 创建Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		broadcast:  make(chan *BroadcastMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run 运行Hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clientsMux.Lock()
			if _, ok := h.clients[client.ThreadID]; !ok {
				h.clients[client.ThreadID] = make(map[*Client]bool)
			}
			h.clients[client.ThreadID][client] = true
			h.clientsMux.Unlock()

		case client := <-h.unregister:
			h.clientsMux.Lock()
			if clients, ok := h.clients[client.ThreadID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.Send)
					if len(clients) == 0 {
						delete(h.clients, client.ThreadID)
					}
				}
			}
			h.clientsMux.Unlock()

		case message := <-h.broadcast:
			h.clientsMux.RLock()
			clients := h.clients[message.ThreadID]
			h.clientsMux.RUnlock()

			for client := range clients {
				select {
				case client.Send <- message.Message:
				default:
					close(client.Send)
					delete(clients, client)
				}
			}
		}
	}
}

// BroadcastToThread 向指定Thread广播消息
func (h *Hub) BroadcastToThread(threadID string, message WSMessage) {
	data, err := jsonMarshal(message)
	if err != nil {
		return
	}
	h.broadcast <- &BroadcastMessage{
		ThreadID: threadID,
		Message:  data,
	}
}

// BroadcastGlobal 向所有客户端广播消息（用于跨 Thread 事件）
// 注意：与 BroadcastToThread 不同，发送失败时仅跳过而不关闭连接。
// 原因：跨 Thread 事件（如任务通知）对可靠性要求较低，客户端可通过轮询或重连获取数据；
// 关闭连接可能过于激进，影响用户体验。
func (h *Hub) BroadcastGlobal(message WSMessage) {
	data, err := jsonMarshal(message)
	if err != nil {
		return
	}

	h.clientsMux.RLock()
	defer h.clientsMux.RUnlock()

	// 向所有 Thread 的所有客户端广播
	for _, clients := range h.clients {
		for client := range clients {
			select {
			case client.Send <- data:
			default:
				// 发送失败，跳过该客户端
			}
		}
	}
}

// GetClientCount 获取指定Thread的客户端数量
func (h *Hub) GetClientCount(threadID string) int {
	h.clientsMux.RLock()
	defer h.clientsMux.RUnlock()
	return len(h.clients[threadID])
}

func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
// SessionBroadcasterAdapter 将 Hub 适配为 SessionBroadcaster 接口
// 用于 SessionPool 向前端广播 session 状态变化
type SessionBroadcasterAdapter struct {
	hub *Hub
}

// NewSessionBroadcasterAdapter 创建适配器
func NewSessionBroadcasterAdapter(hub *Hub) *SessionBroadcasterAdapter {
	return &SessionBroadcasterAdapter{hub: hub}
}

// BroadcastToThread 实现 SessionBroadcaster 接口
func (a *SessionBroadcasterAdapter) BroadcastToThread(threadID string, eventType string, payload map[string]interface{}) {
	a.hub.BroadcastToThread(threadID, WSMessage{
		Type:      eventType,
		ThreadID:  threadID,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	})
}
