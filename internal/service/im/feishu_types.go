package im

import "encoding/json"

// FeishuWebhookEvent 飞书 webhook 事件（顶层）
type FeishuWebhookEvent struct {
	Schema string            `json:"schema"`
	Header FeishuEventHeader `json:"header"`
	Event  json.RawMessage   `json:"event"`
}

// FeishuEventHeader 事件头
type FeishuEventHeader struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	Token     string `json:"token"`
	AppID     string `json:"app_id"`
	Timestamp string `json:"create_time"`
}

// FeishuURLVerification URL验证挑战
type FeishuURLVerification struct {
	Challenge string `json:"challenge"`
	Token     string `json:"token"`
	Type      string `json:"type"`
}

// FeishuMessageReceivedEvent im.message.receive_v1 消息事件（nested format）
type FeishuMessageReceivedEvent struct {
	Sender  FeishuSender  `json:"sender"`
	Message FeishuMessage `json:"message"`
}

// FeishuSender 发送者信息
type FeishuSender struct {
	SenderID struct {
		OpenID  string `json:"open_id"`
		UserID  string `json:"user_id"`
		UnionID string `json:"union_id"`
	} `json:"sender_id"`
}

// FeishuMessage 消息内容
type FeishuMessage struct {
	MessageID   string `json:"message_id"`
	ChatID      string `json:"chat_id"`
	ChatType    string `json:"chat_type"`    // "p2p" or "group"
	MessageType string `json:"message_type"` // "text"
	Content     string `json:"content"`      // JSON-encoded: {"text":"hello"}
}

// ParseTextContent 解析飞书文本消息的 content 字段
func (m *FeishuMessage) ParseTextContent() string {
	var contentObj struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(m.Content), &contentObj); err != nil {
		return ""
	}
	return contentObj.Text
}
