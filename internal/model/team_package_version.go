package model

import (
	"time"

	"github.com/google/uuid"
)

type TeamPackageVersion struct {
	ID           uuid.UUID  `json:"id"`
	WorkflowID   uuid.UUID  `json:"workflowId"`
	Name         string     `json:"name"`
	Category     string     `json:"category"`
	Version      string     `json:"version"`
	Description  string     `json:"description"`
	LastSyncedAt *time.Time `json:"lastSyncedAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

func (t *TeamPackageVersion) TableName() string {
	return "team_package_versions"
}