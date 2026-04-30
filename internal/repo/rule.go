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

// RuleRepository Rule数据访问
type RuleRepository struct {
	BaseRepository
}

// NewRuleRepository 创建Rule Repository
func NewRuleRepository(db *sql.DB, dbType DBType) *RuleRepository {
	return &RuleRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// Create 创建Rule
func (r *RuleRepository) Create(ctx context.Context, rule *model.Rule) error {
	query := `
		INSERT INTO rules (id, name, description, supported_agents, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	supportedAgents, _ := json.Marshal(rule.SupportedAgents)
	_, err := r.DB().ExecContext(ctx, query,
		rule.ID.String(), rule.Name, rule.Description, supportedAgents, rule.CreatedAt, rule.UpdatedAt,
	)
	return err
}

// scanRule 辅助函数，扫描Rule行
func scanRule(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.Rule, error) {
	rule := &model.Rule{}
	var idStr string
	var description sql.NullString
	var supportedAgents []byte
	var createdAt, updatedAt SQLiteTimeScanner

	err := scanner.Scan(
		&idStr, &rule.Name, &description, &supportedAgents, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	rule.ID, _ = uuid.Parse(idStr)
	if description.Valid {
		rule.Description = description.String
	}
	json.Unmarshal(supportedAgents, &rule.SupportedAgents)
	rule.CreatedAt = createdAt.Time
	rule.UpdatedAt = updatedAt.Time

	return rule, nil
}

// FindByID 根据ID查找
func (r *RuleRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Rule, error) {
	query := `
		SELECT id, name, description, supported_agents, created_at, updated_at
		FROM rules WHERE id = ?
	`
	rule, err := scanRule(r.DB().QueryRowContext(ctx, query, id.String()))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("rule not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find rule: %w", err)
	}
	return rule, nil
}

// FindByName 根据名称查找
func (r *RuleRepository) FindByName(ctx context.Context, name string) (*model.Rule, error) {
	query := `
		SELECT id, name, description, supported_agents, created_at, updated_at
		FROM rules WHERE name = ?
	`
	rule, err := scanRule(r.DB().QueryRowContext(ctx, query, name))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("rule not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find rule: %w", err)
	}
	return rule, nil
}

// List 列出Rules，支持分页和搜索
func (r *RuleRepository) List(ctx context.Context, query *model.RuleListQuery) ([]*model.Rule, int64, error) {
	var conditions []string
	var args []interface{}

	if query.Search != "" {
		conditions = append(conditions, "(name LIKE ? OR description LIKE ?)")
		searchPattern := "%" + query.Search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	// AgentType 过滤（向后兼容：空数组默认只支持 claude_code）
	if query.AgentType != "" {
		if query.AgentType == "claude_code" {
			conditions = append(conditions, "(supported_agents = '[]' OR supported_agents LIKE ?)")
			args = append(args, `%"claude_code"%`)
		} else {
			conditions = append(conditions, "supported_agents LIKE ?")
			args = append(args, `%"`+query.AgentType+`"%`)
		}
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 计算总数
	countQuery := "SELECT COUNT(*) FROM rules " + whereClause
	var total int64
	err := r.DB().QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count rules: %w", err)
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
		SELECT id, name, description, supported_agents, created_at, updated_at
		FROM rules ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	args = append(args, pageSize, offset)

	rows, err := r.DB().QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list rules: %w", err)
	}
	defer rows.Close()

	rules := make([]*model.Rule, 0)
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan rule: %w", err)
		}
		rules = append(rules, rule)
	}

	return rules, total, nil
}

// Update 更新Rule
func (r *RuleRepository) Update(ctx context.Context, rule *model.Rule) error {
	query := `
		UPDATE rules
		SET name = ?, description = ?, supported_agents = ?, updated_at = ?
		WHERE id = ?
	`
	supportedAgents, _ := json.Marshal(rule.SupportedAgents)
	_, err := r.DB().ExecContext(ctx, query,
		rule.Name, rule.Description, supportedAgents, rule.UpdatedAt, rule.ID.String(),
	)
	return err
}

// Delete 删除Rule
func (r *RuleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM rules WHERE id = ?`
	_, err := r.DB().ExecContext(ctx, query, id.String())
	return err
}