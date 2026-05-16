package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/anthropic/isdp/internal/model"
)

// TeamMemoryRepo 团队级记忆数据访问接口
type TeamMemoryRepo interface {
	Create(ctx context.Context, mem *model.TeamMemory) error
	Update(ctx context.Context, id string, content string) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*model.TeamMemory, error)
	ListByTeam(ctx context.Context, teamID string, limit int) ([]*model.TeamMemory, error)
}

// ProjectMemoryRepo 项目级记忆数据访问接口
type ProjectMemoryRepo interface {
	Create(ctx context.Context, mem *model.ProjectMemory) error
	Update(ctx context.Context, id string, content string) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*model.ProjectMemory, error)
	ListByProject(ctx context.Context, projectID string, limit int) ([]*model.ProjectMemory, error)
}

// ========== SQLite Implementations ==========

// SQLiteTeamMemoryRepo 团队级记忆 SQLite 实现
type SQLiteTeamMemoryRepo struct {
	db *sql.DB
}

func NewSQLiteTeamMemoryRepo(db *sql.DB) *SQLiteTeamMemoryRepo {
	return &SQLiteTeamMemoryRepo{db: db}
}

func (r *SQLiteTeamMemoryRepo) Create(ctx context.Context, mem *model.TeamMemory) error {
	if mem.ID == "" {
		mem.ID = uuid.New().String()
	}
	now := time.Now()
	mem.CreatedAt = now
	mem.UpdatedAt = now

	var metadataJSON []byte
	if mem.Metadata != nil {
		metadataJSON, _ = json.Marshal(mem.Metadata)
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO team_memories (id, team_id, content, category, created_at, updated_at, metadata)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		mem.ID, mem.TeamID, mem.Content, mem.Category, mem.CreatedAt, mem.UpdatedAt, metadataJSON,
	)
	return err
}

func (r *SQLiteTeamMemoryRepo) Update(ctx context.Context, id string, content string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE team_memories SET content = ?, updated_at = ? WHERE id = ?`,
		content, now, id,
	)
	return err
}

func (r *SQLiteTeamMemoryRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM team_memories WHERE id = ?`, id)
	return err
}

func (r *SQLiteTeamMemoryRepo) GetByID(ctx context.Context, id string) (*model.TeamMemory, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, team_id, content, category, created_at, updated_at, metadata FROM team_memories WHERE id = ?`,
		id,
	)
	mem := &model.TeamMemory{}
	var metadataJSON []byte
	err := row.Scan(&mem.ID, &mem.TeamID, &mem.Content, &mem.Category, &mem.CreatedAt, &mem.UpdatedAt, &metadataJSON)
	if err != nil {
		return nil, err
	}
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &mem.Metadata)
	}
	return mem, nil
}

func (r *SQLiteTeamMemoryRepo) ListByTeam(ctx context.Context, teamID string, limit int) ([]*model.TeamMemory, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, team_id, content, category, created_at, updated_at, metadata
		 FROM team_memories WHERE team_id = ?
		 ORDER BY updated_at DESC LIMIT ?`,
		teamID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var memories []*model.TeamMemory
	for rows.Next() {
		mem := &model.TeamMemory{}
		var metadataJSON []byte
		err := rows.Scan(&mem.ID, &mem.TeamID, &mem.Content, &mem.Category, &mem.CreatedAt, &mem.UpdatedAt, &metadataJSON)
		if err != nil {
			return nil, err
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &mem.Metadata)
		}
		memories = append(memories, mem)
	}
	return memories, nil
}

// SQLiteProjectMemoryRepo 项目级记忆 SQLite 实现
type SQLiteProjectMemoryRepo struct {
	db *sql.DB
}

func NewSQLiteProjectMemoryRepo(db *sql.DB) *SQLiteProjectMemoryRepo {
	return &SQLiteProjectMemoryRepo{db: db}
}

func (r *SQLiteProjectMemoryRepo) Create(ctx context.Context, mem *model.ProjectMemory) error {
	if mem.ID == "" {
		mem.ID = uuid.New().String()
	}
	now := time.Now()
	mem.CreatedAt = now
	mem.UpdatedAt = now

	var metadataJSON []byte
	if mem.Metadata != nil {
		metadataJSON, _ = json.Marshal(mem.Metadata)
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO project_memories (id, project_id, content, category, created_at, updated_at, metadata)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		mem.ID, mem.ProjectID, mem.Content, mem.Category, mem.CreatedAt, mem.UpdatedAt, metadataJSON,
	)
	return err
}

func (r *SQLiteProjectMemoryRepo) Update(ctx context.Context, id string, content string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE project_memories SET content = ?, updated_at = ? WHERE id = ?`,
		content, now, id,
	)
	return err
}

func (r *SQLiteProjectMemoryRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM project_memories WHERE id = ?`, id)
	return err
}

func (r *SQLiteProjectMemoryRepo) GetByID(ctx context.Context, id string) (*model.ProjectMemory, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, content, category, created_at, updated_at, metadata FROM project_memories WHERE id = ?`,
		id,
	)
	mem := &model.ProjectMemory{}
	var metadataJSON []byte
	err := row.Scan(&mem.ID, &mem.ProjectID, &mem.Content, &mem.Category, &mem.CreatedAt, &mem.UpdatedAt, &metadataJSON)
	if err != nil {
		return nil, err
	}
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &mem.Metadata)
	}
	return mem, nil
}

func (r *SQLiteProjectMemoryRepo) ListByProject(ctx context.Context, projectID string, limit int) ([]*model.ProjectMemory, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, content, category, created_at, updated_at, metadata
		 FROM project_memories WHERE project_id = ?
		 ORDER BY updated_at DESC LIMIT ?`,
		projectID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var memories []*model.ProjectMemory
	for rows.Next() {
		mem := &model.ProjectMemory{}
		var metadataJSON []byte
		err := rows.Scan(&mem.ID, &mem.ProjectID, &mem.Content, &mem.Category, &mem.CreatedAt, &mem.UpdatedAt, &metadataJSON)
		if err != nil {
			return nil, err
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &mem.Metadata)
		}
		memories = append(memories, mem)
	}
	return memories, nil
}