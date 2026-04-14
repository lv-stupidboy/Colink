package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/anthropic/isdp/internal/service/im"
	"github.com/gin-gonic/gin"
)

type FeishuWebhookHandler struct {
	bridgeSvc   *im.IMBridgeService
	verifyToken string
}

func NewFeishuWebhookHandler(bridgeSvc *im.IMBridgeService, verifyToken string) *FeishuWebhookHandler {
	return &FeishuWebhookHandler{
		bridgeSvc:   bridgeSvc,
		verifyToken: verifyToken,
	}
}

func (h *FeishuWebhookHandler) RegisterRoutes(r *gin.RouterGroup) {
	feishu := r.Group("/feishu")
	{
		feishu.POST("/webhook", h.HandleWebhook)
	}
}

func (h *FeishuWebhookHandler) HandleWebhook(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}

	var urlVerify im.FeishuURLVerification
	if err := json.Unmarshal(bodyBytes, &urlVerify); err == nil && urlVerify.Type == "url_verification" {
		if urlVerify.Token == h.verifyToken {
			c.JSON(http.StatusOK, gin.H{
				"challenge": urlVerify.Challenge,
				"token":     urlVerify.Token,
				"type":      urlVerify.Type,
			})
			return
		}
	}

	var event im.FeishuWebhookEvent
	if err := json.Unmarshal(bodyBytes, &event); err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}

	if event.Header.Token != h.verifyToken {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}

	switch event.Header.EventType {
	case "im.message.receive_v1":
		var msgEvent im.FeishuMessageReceivedEvent
		if err := json.Unmarshal(event.Event, &msgEvent); err == nil {
			text := msgEvent.Message.ParseTextContent()
			userID := ""
			if msgEvent.Sender.SenderID.OpenID != "" {
				userID = msgEvent.Sender.SenderID.OpenID
			} else if msgEvent.Sender.SenderID.UserID != "" {
				userID = msgEvent.Sender.SenderID.UserID
			}
			detachedCtx := context.WithoutCancel(c.Request.Context())
			go func() {
				_ = h.bridgeSvc.HandleInboundMessage(
					detachedCtx,
					"feishu",
					msgEvent.Message.ChatID,
					msgEvent.Message.ChatType,
					userID,
					"",
					msgEvent.Message.MessageID,
					text,
				)
			}()
		}
	default:
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
