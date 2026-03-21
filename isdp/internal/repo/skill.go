package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// SkillRepository Skill数据访问
type SkillRepository struct {
	db *sql.DB
}

// NewSkillRepository 创建Skill Repository
func NewSkillRepository(db *sql.DB) *SkillRepository {
	return &SkillRepository{db: db}
}

// Create 创建Skill
func (r *SkillRepository) Create(ctx context.Context, skill *model.Skill) error {
	query := `
		INSERT INTO skills (id, name, description, tags, source_type, source_registry_id, author_id, project_id, supported_agents, version, use_count, status, is_public, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	supportedAgents, _ := json.Marshal(skill.SupportedAgents)
	tags, _ := json.Marshal(skill.Tags)

	var sourceRegistryID, authorID, projectID interface{}
	if skill.SourceRegistryID != uuid.Nil {
		sourceRegistryID = skill.SourceRegistryID.String()
	}
	if skill.AuthorID != uuid.Nil {
		authorID = skill.AuthorID.String()
	}
	if skill.ProjectID != uuid.Nil {
		projectID = skill.ProjectID.String()
	}

	_, err := r.db.ExecContext(ctx, query,
		skill.ID.String(), skill.Name, skill.Description, tags, skill.SourceType, sourceRegistryID, authorID, projectID, supportedAgents, skill.Version, skill.UseCount, skill.Status, skill.IsPublic, skill.CreatedAt, skill.UpdatedAt,
	)
	return err
}

// scanSkill 辅助函数，扫描Skill行
func scanSkill(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.Skill, error) {
	skill := &model.Skill{}
	var idStr string
	var description sql.NullString
	var tags, supportedAgents []byte
	var sourceRegistryID, authorID, projectID sql.NullString

	err := scanner.Scan(
		&idStr, &skill.Name, &description, &tags, &skill.SourceType, &sourceRegistryID, &authorID, &projectID, &supportedAgents, &skill.Version, &skill.UseCount, &skill.Status, &skill.IsPublic, &skill.CreatedAt, &skill.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	skill.ID, _ = uuid.Parse(idStr)
	if description.Valid {
		skill.Description = description.String
	}
	json.Unmarshal(tags, &skill.Tags)
	if sourceRegistryID.Valid {
		skill.SourceRegistryID, _ = uuid.Parse(sourceRegistryID.String)
	}
	if authorID.Valid {
		skill.AuthorID, _ = uuid.Parse(authorID.String)
	}
	if projectID.Valid {
		skill.ProjectID, _ = uuid.Parse(projectID.String)
	}
	json.Unmarshal(supportedAgents, &skill.SupportedAgents)

	return skill, nil
}

// FindByID 根据ID查找
func (r *SkillRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Skill, error) {
	query := `
		SELECT id, name, description, tags, source_type, source_registry_id, author_id, project_id, supported_agents, version, use_count, status, is_public, created_at, updated_at
		FROM skills WHERE id = ?
	`
	skill, err := scanSkill(r.db.QueryRowContext(ctx, query, id.String()))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("skill not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find skill: %w", err)
	}
	return skill, nil
}

// FindByName 根据名称查找
func (r *SkillRepository) FindByName(ctx context.Context, name string) (*model.Skill, error) {
	query := `
		SELECT id, name, description, tags, source_type, source_registry_id, author_id, project_id, supported_agents, version, use_count, status, is_public, created_at, updated_at
		FROM skills WHERE name = ?
	`
	skill, err := scanSkill(r.db.QueryRowContext(ctx, query, name))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("skill not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find skill: %w", err)
	}
	return skill, nil
}

// List 列出Skills，支持分页和过滤
func (r *SkillRepository) List(ctx context.Context, query *model.SkillListQuery) ([]*model.Skill, int64, error) {
	// 构建查询条件
	var conditions []string
	var args []interface{}

	if query.Tag != "" {
		conditions = append(conditions, "JSON_CONTAINS(tags, ?)")
		args = append(args, `"`+query.Tag+`"`)
	}
	if query.SourceType != "" {
		conditions = append(conditions, "source_type = ?")
		args = append(args, query.SourceType)
	}
	if query.Search != "" {
		conditions = append(conditions, "(name LIKE ? OR description LIKE ?)")
		searchPattern := "%" + query.Search + "%"
		args = append(args, searchPattern, searchPattern)
	}
	if query.AgentType != "" {
		conditions = append(conditions, "JSON_CONTAINS(supported_agents, ?)")
		args = append(args, `"`+query.AgentType+`"`)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 计算总数
	countQuery := "SELECT COUNT(*) FROM skills " + whereClause
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count skills: %w", err)
	}

	// 分页
	page := query.Page
	pageSize := query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	// 查询列表
	listQuery := `
		SELECT id, name, description, tags, source_type, source_registry_id, author_id, project_id, supported_agents, version, use_count, status, is_public, created_at, updated_at
		FROM skills ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list skills: %w", err)
	}
	defer rows.Close()

	skills := make([]*model.Skill, 0)
	for rows.Next() {
		skill, err := scanSkill(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan skill: %w", err)
		}
		skills = append(skills, skill)
	}

	return skills, total, nil
}

// Update 更新Skill
func (r *SkillRepository) Update(ctx context.Context, skill *model.Skill) error {
	query := `
		UPDATE skills
		SET name = ?, description = ?, tags = ?, source_type = ?, source_registry_id = ?, author_id = ?, project_id = ?, supported_agents = ?, version = ?, use_count = ?, status = ?, is_public = ?, updated_at = NOW()
		WHERE id = ?
	`
	supportedAgents, _ := json.Marshal(skill.SupportedAgents)
	tags, _ := json.Marshal(skill.Tags)

	var sourceRegistryID, authorID, projectID interface{}
	if skill.SourceRegistryID != uuid.Nil {
		sourceRegistryID = skill.SourceRegistryID.String()
	}
	if skill.AuthorID != uuid.Nil {
		authorID = skill.AuthorID.String()
	}
	if skill.ProjectID != uuid.Nil {
		projectID = skill.ProjectID.String()
	}

	_, err := r.db.ExecContext(ctx, query,
		skill.Name, skill.Description, tags, skill.SourceType, sourceRegistryID, authorID, projectID, supportedAgents, skill.Version, skill.UseCount, skill.Status, skill.IsPublic, skill.ID.String(),
	)
	return err
}

// Delete 删除Skill
func (r *SkillRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM skills WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}

// IncrementUseCount 增加使用次数
func (r *SkillRepository) IncrementUseCount(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE skills SET use_count = use_count + 1, updated_at = NOW() WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}

// UpdateUseCount 更新使用次数（直接设置值）
func (r *SkillRepository) UpdateUseCount(ctx context.Context, skillID string, count int) error {
	query := `UPDATE skills SET use_count = ?, updated_at = NOW() WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, count, skillID)
	return err
}

// GetAllTags 获取所有标签
func (r *SkillRepository) GetAllTags(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT tags FROM skills WHERE tags IS NOT NULL AND tags != '[]'`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	defer rows.Close()

	tagSet := make(map[string]bool)
	for rows.Next() {
		var tagsJSON string
		if err := rows.Scan(&tagsJSON); err != nil {
			continue
		}
		var tags []string
		if err := json.Unmarshal([]byte(tagsJSON), &tags); err == nil {
			for _, tag := range tags {
				tagSet[tag] = true
			}
		}
	}

	result := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		result = append(result, tag)
	}
	return result, nil
}

// ========== Agent-Skill Binding ==========

// AgentSkillBindingRepository Agent-Skill绑定数据访问
type AgentSkillBindingRepository struct {
	db *sql.DB
}

// NewAgentSkillBindingRepository 创建AgentSkillBinding Repository
func NewAgentSkillBindingRepository(db *sql.DB) *AgentSkillBindingRepository {
	return &AgentSkillBindingRepository{db: db}
}

// Create 创建绑定
func (r *AgentSkillBindingRepository) Create(ctx context.Context, binding *model.AgentSkillBinding) error {
	query := `
		INSERT INTO agent_skill_bindings (id, agent_role_id, skill_id, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		binding.ID.String(), binding.AgentRoleID.String(), binding.SkillID.String(), binding.CreatedAt,
	)
	return err
}

// FindByAgentRoleID 根据AgentRole ID查找绑定的Skill ID列表
func (r *AgentSkillBindingRepository) FindByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT skill_id FROM agent_skill_bindings WHERE agent_role_id = ?`
	rows, err := r.db.QueryContext(ctx, query, agentRoleID.String())
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

// FindBySkillID 根据Skill ID查找绑定的AgentRole ID列表
func (r *AgentSkillBindingRepository) FindBySkillID(ctx context.Context, skillID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT agent_role_id FROM agent_skill_bindings WHERE skill_id = ?`
	rows, err := r.db.QueryContext(ctx, query, skillID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	agentRoleIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var agentRoleIDStr string
		if err := rows.Scan(&agentRoleIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan agent_role_id: %w", err)
		}
		agentRoleID, _ := uuid.Parse(agentRoleIDStr)
		agentRoleIDs = append(agentRoleIDs, agentRoleID)
	}
	return agentRoleIDs, nil
}

// DeleteByAgentRoleID 删除AgentRole的所有绑定
func (r *AgentSkillBindingRepository) DeleteByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) error {
	query := `DELETE FROM agent_skill_bindings WHERE agent_role_id = ?`
	_, err := r.db.ExecContext(ctx, query, agentRoleID.String())
	return err
}

// DeleteBinding 删除特定绑定
func (r *AgentSkillBindingRepository) DeleteBinding(ctx context.Context, agentRoleID, skillID uuid.UUID) error {
	query := `DELETE FROM agent_skill_bindings WHERE agent_role_id = ? AND skill_id = ?`
	_, err := r.db.ExecContext(ctx, query, agentRoleID.String(), skillID.String())
	return err
}

// ExistsBinding 检查绑定是否存在
func (r *AgentSkillBindingRepository) ExistsBinding(ctx context.Context, agentRoleID, skillID uuid.UUID) (bool, error) {
	query := `SELECT COUNT(*) FROM agent_skill_bindings WHERE agent_role_id = ? AND skill_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, agentRoleID.String(), skillID.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}