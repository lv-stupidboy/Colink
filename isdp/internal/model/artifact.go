package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Artifact Models ==========

type ArtifactType string

const (
	ArtifactTypeCode       ArtifactType = "code"
	ArtifactTypeDocument   ArtifactType = "document"
	ArtifactTypeReview     ArtifactType = "review"
	ArtifactTypeTest       ArtifactType = "test"
	ArtifactTypeConfig     ArtifactType = "config"
)

// Artifact 产物模型
type Artifact struct {
	ID        uuid.UUID          `json:"id"`
	ThreadID  uuid.UUID          `json:"thread_id"`
	Type      ArtifactType       `json:"type"`
	Name      string             `json:"name"`
	Path      string             `json:"path,omitempty"`
	Content   string             `json:"content,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
}

func (a *Artifact) TableName() string {
	return "artifacts"
}