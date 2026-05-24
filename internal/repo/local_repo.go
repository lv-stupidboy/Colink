package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// LocalRepoRepository 本地代码仓数据访问
type LocalRepoRepository struct {
	BaseRepository
}

// NewLocalRepoRepository 创建本地代码仓Repository
func NewLocalRepoRepository(db *sql.DB, dbType DBType) *LocalRepoRepository {
	return &LocalRepoRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// Create 创建本地代码仓
func (r *LocalRepoRepository) Create(ctx context.Context, repo *model.LocalRepo) error {
	query := `
		INSERT INTO local_repos (id, name, git_url, local_path, branch, last_commit, status, error_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()

	var branch interface{}
	if repo.Branch != nil {
		branch = *repo.Branch
	}

	var lastCommit interface{}
	if repo.LastCommit != nil {
		lastCommit = *repo.LastCommit
	}

	var errorMessage interface{}
	if repo.ErrorMessage != nil {
		errorMessage = *repo.ErrorMessage
	}

	_, err := r.DB().ExecContext(ctx, query,
		repo.ID.String(),
		repo.Name,
		repo.GitUrl,
		repo.LocalPath,
		branch,
		lastCommit,
		repo.Status,
		errorMessage,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to create local repo: %w", err)
	}
	repo.CreatedAt = now
	repo.UpdatedAt = now
	return nil
}

// FindByID 根据ID查找本地代码仓
func (r *LocalRepoRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.LocalRepo, error) {
	query := `
		SELECT id, name, git_url, local_path, branch, last_commit, status, error_message, created_at, updated_at
		FROM local_repos WHERE id = ?
	`
	repo := &model.LocalRepo{}
	var idStr string
	var branch sql.NullString
	var lastCommit sql.NullString
	var errorMessage sql.NullString
	var createdAt, updatedAt SQLiteTimeScanner
	err := r.DB().QueryRowContext(ctx, query, id.String()).Scan(
		&idStr,
		&repo.Name,
		&repo.GitUrl,
		&repo.LocalPath,
		&branch,
		&lastCommit,
		&repo.Status,
		&errorMessage,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find local repo: %w", err)
	}

	repo.ID, _ = uuid.Parse(idStr)

	if branch.Valid {
		repo.Branch = &branch.String
	}
	if lastCommit.Valid {
		repo.LastCommit = &lastCommit.String
	}
	if errorMessage.Valid {
		repo.ErrorMessage = &errorMessage.String
	}

	repo.CreatedAt = createdAt.Time
	repo.UpdatedAt = updatedAt.Time
	return repo, nil
}

// FindAll 查找所有本地代码仓
func (r *LocalRepoRepository) FindAll(ctx context.Context) ([]*model.LocalRepo, error) {
	query := `
		SELECT id, name, git_url, local_path, branch, last_commit, status, error_message, created_at, updated_at
		FROM local_repos ORDER BY updated_at DESC
	`
	rows, err := r.DB().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find local repos: %w", err)
	}
	defer rows.Close()

	repos := make([]*model.LocalRepo, 0)
	for rows.Next() {
		repo := &model.LocalRepo{}
		var idStr string
		var branch sql.NullString
		var lastCommit sql.NullString
		var errorMessage sql.NullString
		var createdAt, updatedAt SQLiteTimeScanner
		err := rows.Scan(
			&idStr,
			&repo.Name,
			&repo.GitUrl,
			&repo.LocalPath,
			&branch,
			&lastCommit,
			&repo.Status,
			&errorMessage,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan local repo: %w", err)
		}
		repo.ID, _ = uuid.Parse(idStr)

		if branch.Valid {
			repo.Branch = &branch.String
		}
		if lastCommit.Valid {
			repo.LastCommit = &lastCommit.String
		}
		if errorMessage.Valid {
			repo.ErrorMessage = &errorMessage.String
		}

		repo.CreatedAt = createdAt.Time
		repo.UpdatedAt = updatedAt.Time
		repos = append(repos, repo)
	}
	return repos, nil
}

// Update 更新本地代码仓
func (r *LocalRepoRepository) Update(ctx context.Context, repo *model.LocalRepo) error {
	query := `
		UPDATE local_repos
		SET name = ?, git_url = ?, local_path = ?, branch = ?, last_commit = ?, status = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`
	repo.UpdatedAt = time.Now()

	var branch interface{}
	if repo.Branch != nil {
		branch = *repo.Branch
	}

	var lastCommit interface{}
	if repo.LastCommit != nil {
		lastCommit = *repo.LastCommit
	}

	var errorMessage interface{}
	if repo.ErrorMessage != nil {
		errorMessage = *repo.ErrorMessage
	}

	_, err := r.DB().ExecContext(ctx, query,
		repo.Name,
		repo.GitUrl,
		repo.LocalPath,
		branch,
		lastCommit,
		repo.Status,
		errorMessage,
		repo.UpdatedAt,
		repo.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to update local repo: %w", err)
	}
	return nil
}

// Delete 删除本地代码仓
func (r *LocalRepoRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM local_repos WHERE id = ?`
	_, err := r.DB().ExecContext(ctx, query, id.String())
	if err != nil {
		return fmt.Errorf("failed to delete local repo: %w", err)
	}
	return nil
}
