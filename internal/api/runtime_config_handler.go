package api

import (
	"net/http"

	"github.com/anthropic/isdp/pkg/config"
	"github.com/gin-gonic/gin"
)

type RuntimeConfigHandler struct {
	cfg *config.Config
}

type RuntimeConfigResponse struct {
	DeploymentType string `json:"deploymentType"`
	WorkspacePath  string `json:"workspacePath"`
	DefaultPath    string `json:"defaultPath"`
}

func NewRuntimeConfigHandler(cfg *config.Config) *RuntimeConfigHandler {
	return &RuntimeConfigHandler{cfg: cfg}
}

func (h *RuntimeConfigHandler) Get(c *gin.Context) {
	c.JSON(http.StatusOK, RuntimeConfigResponse{
		DeploymentType: string(h.cfg.Deployment.Type),
		WorkspacePath:  h.cfg.Deployment.WorkspacePath,
		DefaultPath:    h.cfg.Deployment.WorkspacePath,
	})
}

func (h *RuntimeConfigHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/runtime/config", h.Get)
}
