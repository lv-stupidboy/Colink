package ws

import (
	"encoding/json"
	"sync"

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

// GetClientCount 获取指定Thread的客户端数量
func (h *Hub) GetClientCount(threadID string) int {
	h.clientsMux.RLock()
	defer h.clientsMux.RUnlock()
	return len(h.clients[threadID])
}

func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}