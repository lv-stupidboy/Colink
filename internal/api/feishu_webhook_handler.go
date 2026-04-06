package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/anthropic/isdp/internal/service/im"
	"github.com/gin-gonic/gin"
)

type FeishuWebhookHandler struct {
	bridgeSvc   *im.FeishuBridgeService
	verifyToken string
}

func NewFeishuWebhookHandler(bridgeSvc *im.FeishuBridgeService, verifyToken string) *FeishuWebhookHandler {
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if event.Header.Token != h.verifyToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	switch event.Header.EventType {
	case "im.message.receive_v1":
		var msgEvent im.FeishuMessageReceivedEvent
		if err := json.Unmarshal(event.Event, &msgEvent); err == nil {
			go h.bridgeSvc.HandleMessageEvent(c.Request.Context(), msgEvent)
		}
	default:
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
