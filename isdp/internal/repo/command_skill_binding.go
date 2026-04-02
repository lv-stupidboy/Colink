// 文件路径: isdp/internal/repo/command_skill_binding.go
package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// CommandSkillBindingRepository Command-Skill绑定数据访问
type CommandSkillBindingRepository struct {
	db *sql.DB
}

// NewCommandSkillBindingRepository 创建CommandSkillBinding Repository
func NewCommandSkillBindingRepository(db *sql.DB) *CommandSkillBindingRepository {
	return &CommandSkillBindingRepository{db: db}
}

// Create 创建绑定
func (r *CommandSkillBindingRepository) Create(ctx context.Context, binding *model.CommandSkillBinding) error {
	query := `
		INSERT INTO command_skill_bindings (id, command_id, skill_id, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		binding.ID.String(), binding.CommandID.String(), binding.SkillID.String(), binding.CreatedAt,
	)
	return err
}

// FindByCommandID 根据Command ID查找绑定的Skill ID列表
func (r *CommandSkillBindingRepository) FindByCommandID(ctx context.Context, commandID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT skill_id FROM command_skill_bindings WHERE command_id = ?`
	rows, err := r.db.QueryContext(ctx, query, commandID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	skillIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var skillIDStr string
		if err := rows.Scan(&skillIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan skill_id: %w", err)
		}
		skillID, _ := uuid.Parse(skillIDStr)
		skillIDs = append(skillIDs, skillID)
	}
	return skillIDs, nil
}

// FindSkillsByCommandID 根据Command ID查找绑定的Skill详情列表
func (r *CommandSkillBindingRepository) FindSkillsByCommandID(ctx context.Context, commandID uuid.UUID) ([]*model.Skill, error) {
	query := `
		SELECT s.id, s.name, s.description, s.tags, s.source_type, s.source_registry_id, s.author_id, s.project_id, s.supported_agents, s.use_count, s.status, s.is_public, s.created_at, s.updated_at
		FROM skills s
		INNER JOIN command_skill_bindings b ON s.id = b.skill_id
		WHERE b.command_id = ?
		ORDER BY s.created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, commandID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find skills: %w", err)
	}
	defer rows.Close()

	skills := make([]*model.Skill, 0)
	for rows.Next() {
		skill, err := scanSkill(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan skill: %w", err)
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

// DeleteByCommandID 删除Command的所有绑定
func (r *CommandSkillBindingRepository) DeleteByCommandID(ctx context.Context, commandID uuid.UUID) error {
	query := `DELETE FROM command_skill_bindings WHERE command_id = ?`
	_, err := r.db.ExecContext(ctx, query, commandID.String())
	return err
}

// DeleteBinding 删除特定绑定
func (r *CommandSkillBindingRepository) DeleteBinding(ctx context.Context, commandID, skillID uuid.UUID) error {
	query := `DELETE FROM command_skill_bindings WHERE command_id = ? AND skill_id = ?`
	_, err := r.db.ExecContext(ctx, query, commandID.String(), skillID.String())
	return err
}

// ExistsBinding 检查绑定是否存在
func (r *CommandSkillBindingRepository) ExistsBinding(ctx context.Context, commandID, skillID uuid.UUID) (bool, error) {
	query := `SELECT COUNT(*) FROM command_skill_bindings WHERE command_id = ? AND skill_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, commandID.String(), skillID.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindBySkillID 根据Skill ID查找绑定的Command ID列表
func (r *CommandSkillBindingRepository) FindBySkillID(ctx context.Context, skillID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT command_id FROM command_skill_bindings WHERE skill_id = ?`
	rows, err := r.db.QueryContext(ctx, query, skillID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	commandIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var commandIDStr string
		if err := rows.Scan(&commandIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan command_id: %w", err)
		}
		commandID, _ := uuid.Parse(commandIDStr)
		commandIDs = append(commandIDs, commandID)
	}
	return commandIDs, nil
}