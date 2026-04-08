package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Sandbox Models ==========

type SandboxStatus string

const (
	SandboxStatusCreated  SandboxStatus = "created"
	SandboxStatusRunning  SandboxStatus = "running"
	SandboxStatusStopped  SandboxStatus = "stopped"
	SandboxStatusComplete SandboxStatus = "complete"
	SandboxStatusError    SandboxStatus = "error"
)

// Sandbox 沙箱容器模型
type Sandbox struct {
	ID          uuid.UUID     `json:"id"`
	ThreadID    uuid.UUID     `json:"threadId"`
	Name        string        `json:"name"`
	Image       string        `json:"image"`
	Status      SandboxStatus `json:"status"`
	ContainerID string        `json:"containerId"`
	Port        int           `json:"port"`
	CreatedAt   time.Time     `json:"createdAt"`
	EndedAt     *time.Time    `json:"endedAt,omitempty"`
}

func (s *Sandbox) TableName() string {
	return "sandboxes"
}