package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// CommandRepository Command数据访问
type CommandRepository struct {
	db *sql.DB
}

// NewCommandRepository 创建Command Repository
func NewCommandRepository(db *sql.DB) *CommandRepository {
	return &CommandRepository{db: db}
}

// Create 创建Command
func (r *CommandRepository) Create(ctx context.Context, command *model.Command) error {
	query := `
		INSERT INTO commands (id, name, description, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		command.ID.String(), command.Name, command.Description, command.Version, command.CreatedAt, command.UpdatedAt,
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
	var version sql.NullString

	err := scanner.Scan(
		&idStr, &command.Name, &description, &version, &command.CreatedAt, &command.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	command.ID, _ = uuid.Parse(idStr)
	if description.Valid {
		command.Description = description.String
	}
	if version.Valid {
		command.Version = version.String
	}

	return command, nil
}

// FindByID 根据ID查找
func (r *CommandRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Command, error) {
	query := `
		SELECT id, name, description, version, created_at, updated_at
		FROM commands WHERE id = ?
	`
	command, err := scanCommand(r.db.QueryRowContext(ctx, query, id.String()))
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
		SELECT id, name, description, version, created_at, updated_at
		FROM commands WHERE name = ?
	`
	command, err := scanCommand(r.db.QueryRowContext(ctx, query, name))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("command not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find command: %w", err)
	}
	return command, nil
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

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 计算总数
	countQuery := "SELECT COUNT(*) FROM commands " + whereClause
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
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
		SELECT id, name, description, version, created_at, updated_at
		FROM commands ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
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
		SET name = ?, description = ?, version = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		command.Name, command.Description, command.Version, command.UpdatedAt, command.ID.String(),
	)
	return err
}

// Delete 删除Command
func (r *CommandRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM commands WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}

// FindByNameAndVersion 根据名称和版本查找
func (r *CommandRepository) FindByNameAndVersion(ctx context.Context, name, version string) (*model.Command, error) {
	query := `
		SELECT id, name, description, version, created_at, updated_at
		FROM commands WHERE name = ? AND version = ?
	`
	command, err := scanCommand(r.db.QueryRowContext(ctx, query, name, version))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("command not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find command: %w", err)
	}
	return command, nil
}