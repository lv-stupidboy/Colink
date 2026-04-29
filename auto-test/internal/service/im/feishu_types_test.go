package im_test

import (
	"encoding/json"
	"testing"
)

func TestFeishuMessage_ParseTextContent(t *testing.T) {
	msg := FeishuMessage{
		Content: `{"text":"Hello World"}`,
	}

	text := msg.ParseTextContent()
	if text != "Hello World" {
		t.Errorf("expected 'Hello World', got '%s'", text)
	}
}

func TestFeishuMessage_ParseTextContent_Empty(t *testing.T) {
	msg := FeishuMessage{
		Content: "",
	}

	text := msg.ParseTextContent()
	if text != "" {
		t.Errorf("expected empty string, got '%s'", text)
	}
}

func TestFeishuMessage_ParseTextContent_InvalidJSON(t *testing.T) {
	msg := FeishuMessage{
		Content: "not json",
	}

	text := msg.ParseTextContent()
	if text != "" {
		t.Errorf("expected empty string for invalid JSON, got '%s'", text)
	}
}

func TestFeishuURLVerification(t *testing.T) {
	data := `{"challenge":"test_challenge","token":"test_token","type":"url_verification"}`
	var verify FeishuURLVerification
	err := json.Unmarshal([]byte(data), &verify)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if verify.Challenge != "test_challenge" {
		t.Errorf("expected challenge 'test_challenge', got '%s'", verify.Challenge)
	}
	if verify.Token != "test_token" {
		t.Errorf("expected token 'test_token', got '%s'", verify.Token)
	}
	if verify.Type != "url_verification" {
		t.Errorf("expected type 'url_verification', got '%s'", verify.Type)
	}
}

func TestFeishuWebhookEvent(t *testing.T) {
	data := `{"schema":"2.0","header":{"event_id":"event_123","event_type":"im.message.receive_v1","token":"test"},"event":{}}`
	var event FeishuWebhookEvent
	err := json.Unmarshal([]byte(data), &event)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if event.Schema != "2.0" {
		t.Errorf("expected schema '2.0', got '%s'", event.Schema)
	}
	if event.Header.EventType != "im.message.receive_v1" {
		t.Errorf("expected event_type 'im.message.receive_v1', got '%s'", event.Header.EventType)
	}
}

func TestFeishuMessageReceivedEvent(t *testing.T) {
	data := `{"sender":{"sender_id":{"open_id":"ou_123"}},"message":{"message_id":"msg_456","chat_id":"chat_789","chat_type":"p2p","message_type":"text","content":"{\"text\":\"test\"}"}}`
	var event FeishuMessageReceivedEvent
	err := json.Unmarshal([]byte(data), &event)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if event.Sender.SenderID.OpenID != "ou_123" {
		t.Errorf("expected open_id 'ou_123', got '%s'", event.Sender.SenderID.OpenID)
	}
	if event.Message.ChatID != "chat_789" {
		t.Errorf("expected chat_id 'chat_789', got '%s'", event.Message.ChatID)
	}

	text := event.Message.ParseTextContent()
	if text != "test" {
		t.Errorf("expected message text 'test', got '%s'", text)
	}
}
