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

// CommandRepository Command数据访问
type CommandRepository struct {
	BaseRepository
}

// NewCommandRepository 创建Command Repository
func NewCommandRepository(db *sql.DB, dbType DBType) *CommandRepository {
	return &CommandRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// Create 创建Command
func (r *CommandRepository) Create(ctx context.Context, command *model.Command) error {
	query := `
		INSERT INTO commands (id, name, description, supported_agents, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	supportedAgents, _ := json.Marshal(command.SupportedAgents)
	_, err := r.DB().ExecContext(ctx, query,
		command.ID.String(), command.Name, command.Description, supportedAgents, command.CreatedAt, command.UpdatedAt,
	)
	return err
}

// scanCommand 辅助函数，扫描Command行
func scanCommand(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.Command, error) {
	command := &model.Command{}
	var idStr string
	var description sql.NullString
	var supportedAgents []byte
	var createdAt, updatedAt SQLiteTimeScanner

	err := scanner.Scan(
		&idStr, &command.Name, &description, &supportedAgents, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	command.ID, _ = uuid.Parse(idStr)
	if description.Valid {
		command.Description = description.String
	}
	json.Unmarshal(supportedAgents, &command.SupportedAgents)
	command.CreatedAt = createdAt.Time
	command.UpdatedAt = updatedAt.Time

	return command, nil
}

// FindByID 根据ID查找
func (r *CommandRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Command, error) {
	query := `
		SELECT id, name, description, supported_agents, created_at, updated_at
		FROM commands WHERE id = ?
	`
	command, err := scanCommand(r.DB().QueryRowContext(ctx, query, id.String()))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("command not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find command: %w", err)
	}
	return command, nil
}

// FindByName 根据名称查找
func (r *CommandRepository) FindByName(ctx context.Context, name string) (*model.Command, error) {
	query := `
		SELECT id, name, description, supported_agents, created_at, updated_at
		FROM commands WHERE name = ?
	`
	command, err := scanCommand(r.DB().QueryRowContext(ctx, query, name))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("command not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find command: %w", err)
	}
	return command, nil
}

// matchesAgentType 检查资产是否支持指定的Agent类型（向后兼容）
func matchesAgentType(supportedAgents []string, agentType string) bool {
	// 空数组向后兼容：默认只支持 claude_code
	if len(supportedAgents) == 0 {
		return agentType == "claude_code"
	}
	// 非空数组：检查是否包含指定类型
	for _, a := range supportedAgents {
		if a == agentType {
			return true
		}
	}
	return false
}

// List 列出Commands，支持分页和搜索
func (r *CommandRepository) List(ctx context.Context, query *model.CommandListQuery) ([]*model.Command, int64, error) {
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
			// claude_code: 包含空数组（向后兼容）或显式包含 claude_code
			conditions = append(conditions, "(supported_agents = '[]' OR supported_agents LIKE ?)")
			args = append(args, `%"claude_code"%`)
		} else {
			// 其他类型：必须显式包含
			conditions = append(conditions, "supported_agents LIKE ?")
			args = append(args, `%"`+query.AgentType+`"%`)
		}
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 计算总数
	countQuery := "SELECT COUNT(*) FROM commands " + whereClause
	var total int64
	err := r.DB().QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count commands: %w", err)
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
		FROM commands ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	args = append(args, pageSize, offset)

	rows, err := r.DB().QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list commands: %w", err)
	}
	defer rows.Close()

	commands := make([]*model.Command, 0)
	for rows.Next() {
		command, err := scanCommand(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan command: %w", err)
		}
		commands = append(commands, command)
	}

	return commands, total, nil
}

// Update 更新Command
func (r *CommandRepository) Update(ctx context.Context, command *model.Command) error {
	query := `
		UPDATE commands
		SET name = ?, description = ?, supported_agents = ?, updated_at = ?
		WHERE id = ?
	`
	supportedAgents, _ := json.Marshal(command.SupportedAgents)
	_, err := r.DB().ExecContext(ctx, query,
		command.Name, command.Description, supportedAgents, command.UpdatedAt, command.ID.String(),
	)
	return err
}

// Delete 删除Command
func (r *CommandRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM commands WHERE id = ?`
	_, err := r.DB().ExecContext(ctx, query, id.String())
	return err
}