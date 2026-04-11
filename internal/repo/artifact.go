package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// ArtifactRepository 工作产物数据访问
type ArtifactRepository struct {
	BaseRepository
}

// NewArtifactRepository 创建Artifact Repository
func NewArtifactRepository(db *sql.DB, dbType DBType) *ArtifactRepository {
	return &ArtifactRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// Create 创建产物
func (r *ArtifactRepository) Create(ctx context.Context, artifact *model.Artifact) error {
	query := `
		INSERT INTO artifacts (id, thread_id, type, name, path, content, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	metadata, _ := json.Marshal(artifact.Metadata)
	_, err := r.DB().ExecContext(ctx, query,
		artifact.ID.String(), artifact.ThreadID.String(), artifact.Type, artifact.Name, artifact.Path, artifact.Content, metadata, artifact.CreatedAt,
	)
	return err
}

// FindByID 根据ID查找
func (r *ArtifactRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Artifact, error) {
	query := `
		SELECT id, thread_id, type, name, path, content, metadata, created_at
		FROM artifacts WHERE id = ?
	`
	artifact := &model.Artifact{}
	var idStr, threadIDStr string
	var metadata []byte
	var createdAt SQLiteTimeScanner
	err := r.DB().QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &threadIDStr, &artifact.Type, &artifact.Name, &artifact.Path, &artifact.Content, &metadata, &createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find artifact: %w", err)
	}
	artifact.ID, _ = uuid.Parse(idStr)
	artifact.ThreadID, _ = uuid.Parse(threadIDStr)
	json.Unmarshal(metadata, &artifact.Metadata)
	artifact.CreatedAt = createdAt.Time
	return artifact, nil
}

// FindByThreadID 根据ThreadID查找
func (r *ArtifactRepository) FindByThreadID(ctx context.Context, threadID uuid.UUID) ([]*model.Artifact, error) {
	query := `
		SELECT id, thread_id, type, name, path, content, metadata, created_at
		FROM artifacts WHERE thread_id = ? ORDER BY created_at DESC
	`
	rows, err := r.DB().QueryContext(ctx, query, threadID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find artifacts: %w", err)
	}
	defer rows.Close()

	var artifacts = make([]*model.Artifact, 0) // 初始化为空数组，避免 JSON null
	for rows.Next() {
		artifact := &model.Artifact{}
		var idStr, threadIDStr string
		var metadata []byte
		var createdAt SQLiteTimeScanner
		err := rows.Scan(
			&idStr, &threadIDStr, &artifact.Type, &artifact.Name, &artifact.Path, &artifact.Content, &metadata, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan artifact: %w", err)
		}
		artifact.ID, _ = uuid.Parse(idStr)
		artifact.ThreadID, _ = uuid.Parse(threadIDStr)
		json.Unmarshal(metadata, &artifact.Metadata)
		artifact.CreatedAt = createdAt.Time
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

// Delete 删除产物
func (r *ArtifactRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM artifacts WHERE id = ?`
	_, err := r.DB().ExecContext(ctx, query, id.String())
	return err
}