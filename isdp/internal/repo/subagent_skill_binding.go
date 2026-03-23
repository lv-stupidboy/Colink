// 文件路径: isdp/internal/repo/subagent_skill_binding.go
package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// SubagentSkillBindingRepository Subagent-Skill绑定数据访问
type SubagentSkillBindingRepository struct {
	db *sql.DB
}

// NewSubagentSkillBindingRepository 创建SubagentSkillBinding Repository
func NewSubagentSkillBindingRepository(db *sql.DB) *SubagentSkillBindingRepository {
	return &SubagentSkillBindingRepository{db: db}
}

// Create 创建绑定
func (r *SubagentSkillBindingRepository) Create(ctx context.Context, binding *model.SubagentSkillBinding) error {
	query := `
		INSERT INTO subagent_skill_bindings (id, subagent_id, skill_id, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		binding.ID.String(), binding.SubagentID.String(), binding.SkillID.String(), binding.CreatedAt,
	)
	return err
}

// FindBySubagentID 根据Subagent ID查找绑定的Skill ID列表
func (r *SubagentSkillBindingRepository) FindBySubagentID(ctx context.Context, subagentID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT skill_id FROM subagent_skill_bindings WHERE subagent_id = ?`
	rows, err := r.db.QueryContext(ctx, query, subagentID.String())
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

// FindSkillsBySubagentID 根据Subagent ID查找绑定的Skill详情列表
func (r *SubagentSkillBindingRepository) FindSkillsBySubagentID(ctx context.Context, subagentID uuid.UUID) ([]*model.Skill, error) {
	query := `
		SELECT s.id, s.name, s.description, s.tags, s.source_type, s.source_registry_id, s.author_id, s.project_id, s.supported_agents, s.version, s.use_count, s.status, s.is_public, s.created_at, s.updated_at
		FROM skills s
		INNER JOIN subagent_skill_bindings b ON s.id = b.skill_id
		WHERE b.subagent_id = ?
		ORDER BY s.created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, subagentID.String())
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

// DeleteBySubagentID 删除Subagent的所有绑定
func (r *SubagentSkillBindingRepository) DeleteBySubagentID(ctx context.Context, subagentID uuid.UUID) error {
	query := `DELETE FROM subagent_skill_bindings WHERE subagent_id = ?`
	_, err := r.db.ExecContext(ctx, query, subagentID.String())
	return err
}

// DeleteBinding 删除特定绑定
func (r *SubagentSkillBindingRepository) DeleteBinding(ctx context.Context, subagentID, skillID uuid.UUID) error {
	query := `DELETE FROM subagent_skill_bindings WHERE subagent_id = ? AND skill_id = ?`
	_, err := r.db.ExecContext(ctx, query, subagentID.String(), skillID.String())
	return err
}

// ExistsBinding 检查绑定是否存在
func (r *SubagentSkillBindingRepository) ExistsBinding(ctx context.Context, subagentID, skillID uuid.UUID) (bool, error) {
	query := `SELECT COUNT(*) FROM subagent_skill_bindings WHERE subagent_id = ? AND skill_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, subagentID.String(), skillID.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindBySkillID 根据Skill ID查找绑定的Subagent ID列表
func (r *SubagentSkillBindingRepository) FindBySkillID(ctx context.Context, skillID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT subagent_id FROM subagent_skill_bindings WHERE skill_id = ?`
	rows, err := r.db.QueryContext(ctx, query, skillID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	subagentIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var subagentIDStr string
		if err := rows.Scan(&subagentIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan subagent_id: %w", err)
		}
		subagentID, _ := uuid.Parse(subagentIDStr)
		subagentIDs = append(subagentIDs, subagentID)
	}
	return subagentIDs, nil
}