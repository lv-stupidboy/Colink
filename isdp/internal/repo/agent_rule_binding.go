// 文件路径: isdp/internal/repo/agent_rule_binding.go
package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// AgentRuleBindingRepository Agent-Rule绑定数据访问
type AgentRuleBindingRepository struct {
	db *sql.DB
}

// NewAgentRuleBindingRepository 创建AgentRuleBinding Repository
func NewAgentRuleBindingRepository(db *sql.DB) *AgentRuleBindingRepository {
	return &AgentRuleBindingRepository{db: db}
}

// Create 创建绑定
func (r *AgentRuleBindingRepository) Create(ctx context.Context, binding *model.AgentRuleBinding) error {
	query := `
		INSERT INTO agent_rule_bindings (id, agent_role_id, rule_id, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		binding.ID.String(), binding.AgentRoleID.String(), binding.RuleID.String(), binding.CreatedAt,
	)
	return err
}

// FindByAgentRoleID 根据AgentRole ID查找绑定的Rule ID列表
func (r *AgentRuleBindingRepository) FindByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT rule_id FROM agent_rule_bindings WHERE agent_role_id = ?`
	rows, err := r.db.QueryContext(ctx, query, agentRoleID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	ruleIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var ruleIDStr string
		if err := rows.Scan(&ruleIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan rule_id: %w", err)
		}
		ruleID, _ := uuid.Parse(ruleIDStr)
		ruleIDs = append(ruleIDs, ruleID)
	}
	return ruleIDs, nil
}

// FindRulesByAgentRoleID 根据AgentRole ID查找绑定的Rule详情列表
func (r *AgentRuleBindingRepository) FindRulesByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]*model.Rule, error) {
	query := `
		SELECT r.id, r.name, r.description, r.scope, r.created_at, r.updated_at
		FROM rules r
		INNER JOIN agent_rule_bindings b ON r.id = b.rule_id
		WHERE b.agent_role_id = ?
		ORDER BY r.created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, agentRoleID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find rules: %w", err)
	}
	defer rows.Close()

	rules := make([]*model.Rule, 0)
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// DeleteByAgentRoleID 删除AgentRole的所有绑定
func (r *AgentRuleBindingRepository) DeleteByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) error {
	query := `DELETE FROM agent_rule_bindings WHERE agent_role_id = ?`
	_, err := r.db.ExecContext(ctx, query, agentRoleID.String())
	return err
}

// DeleteBinding 删除特定绑定
func (r *AgentRuleBindingRepository) DeleteBinding(ctx context.Context, agentRoleID, ruleID uuid.UUID) error {
	query := `DELETE FROM agent_rule_bindings WHERE agent_role_id = ? AND rule_id = ?`
	_, err := r.db.ExecContext(ctx, query, agentRoleID.String(), ruleID.String())
	return err
}

// ExistsBinding 检查绑定是否存在
func (r *AgentRuleBindingRepository) ExistsBinding(ctx context.Context, agentRoleID, ruleID uuid.UUID) (bool, error) {
	query := `SELECT COUNT(*) FROM agent_rule_bindings WHERE agent_role_id = ? AND rule_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, agentRoleID.String(), ruleID.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindByRuleID 根据Rule ID查找绑定的AgentRole ID列表
func (r *AgentRuleBindingRepository) FindByRuleID(ctx context.Context, ruleID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT agent_role_id FROM agent_rule_bindings WHERE rule_id = ?`
	rows, err := r.db.QueryContext(ctx, query, ruleID.String())
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