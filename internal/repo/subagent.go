package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// SubagentRepository Subagent数据访问
type SubagentRepository struct {
	BaseRepository
}

// NewSubagentRepository 创建Subagent Repository
func NewSubagentRepository(db *sql.DB, dbType DBType) *SubagentRepository {
	return &SubagentRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// Create 创建Subagent（content 存储在文件系统，不写入数据库）
func (r *SubagentRepository) Create(ctx context.Context, subagent *model.Subagent) error {
	query := `
		INSERT INTO subagents (id, name, description, supported_agents, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	supportedAgents, _ := json.Marshal(subagent.SupportedAgents)

	_, err := r.DB().ExecContext(ctx, query,
		subagent.ID.String(), subagent.Name, subagent.Description, supportedAgents, subagent.CreatedAt, subagent.UpdatedAt,
	)
	return err
}

// scanSubagent 辅助函数，扫描Subagent行（不包含 content，由 service 层从文件读取）
func scanSubagent(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.Subagent, error) {
	subagent := &model.Subagent{}
	var idStr string
	var description sql.NullString
	var supportedAgents []byte
	var createdAt, updatedAt SQLiteTimeScanner

	err := scanner.Scan(
		&idStr, &subagent.Name, &description, &supportedAgents, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	subagent.ID, _ = uuid.Parse(idStr)
	if description.Valid {
		subagent.Description = description.String
	}
	json.Unmarshal(supportedAgents, &subagent.SupportedAgents)
	subagent.CreatedAt = createdAt.Time
	subagent.UpdatedAt = updatedAt.Time

	return subagent, nil
}

// FindByID 根据ID查找
func (r *SubagentRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Subagent, error) {
	query := `
		SELECT id, name, description, supported_agents, created_at, updated_at
		FROM subagents WHERE id = ?
	`
	subagent, err := scanSubagent(r.DB().QueryRowContext(ctx, query, id.String()))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("subagent not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find subagent: %w", err)
	}
	return subagent, nil
}

// FindByName 根据名称查找
func (r *SubagentRepository) FindByName(ctx context.Context, name string) (*model.Subagent, error) {
	query := `
		SELECT id, name, description, supported_agents, created_at, updated_at
		FROM subagents WHERE name = ?
	`
	subagent, err := scanSubagent(r.DB().QueryRowContext(ctx, query, name))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("subagent not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find subagent: %w", err)
	}
	return subagent, nil
}

// List 列出Subagents，支持分页和搜索
func (r *SubagentRepository) List(ctx context.Context, query *model.SubagentListQuery) ([]*model.Subagent, int64, error) {
	// 构建查询条件
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
	countQuery := "SELECT COUNT(*) FROM subagents " + whereClause
	var total int64
	err := r.DB().QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count subagents: %w", err)
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
		FROM subagents ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	args = append(args, pageSize, offset)

	rows, err := r.DB().QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list subagents: %w", err)
	}
	defer rows.Close()

	subagents := make([]*model.Subagent, 0)
	for rows.Next() {
		subagent, err := scanSubagent(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan subagent: %w", err)
		}
		subagents = append(subagents, subagent)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating subagent rows: %w", err)
	}

	return subagents, total, nil
}

// Update 更新Subagent（content 存储在文件系统，不更新数据库）
func (r *SubagentRepository) Update(ctx context.Context, subagent *model.Subagent) error {
	now := time.Now()
	subagent.UpdatedAt = now
	query := `
		UPDATE subagents
		SET name = ?, description = ?, supported_agents = ?, updated_at = ?
		WHERE id = ?
	`
	supportedAgents, _ := json.Marshal(subagent.SupportedAgents)

	_, err := r.DB().ExecContext(ctx, query,
		subagent.Name, subagent.Description, supportedAgents, now, subagent.ID.String(),
	)
	return err
}

// Delete 删除Subagent
func (r *SubagentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM subagents WHERE id = ?`
	_, err := r.DB().ExecContext(ctx, query, id.String())
	return err
}