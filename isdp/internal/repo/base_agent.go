package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// BaseAgentRepository 基础Agent数据访问
type BaseAgentRepository struct {
	db *sql.DB
}

// NewBaseAgentRepository 创建基础Agent Repository
func NewBaseAgentRepository(db *sql.DB) *BaseAgentRepository {
	return &BaseAgentRepository{db: db}
}

// Create 创建基础Agent
func (r *BaseAgentRepository) Create(ctx context.Context, agent *model.BaseAgent) error {
	query := `
		INSERT INTO base_agents (id, name, type, api_url, api_token, default_model, cli_path, git_bash_path, max_tokens, timeout_minutes, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		agent.ID.String(), agent.Name, agent.Type, agent.ApiURL, agent.ApiToken, agent.DefaultModel, agent.CliPath, agent.GitBashPath, agent.MaxTokens, agent.TimeoutMinutes, agent.IsDefault, agent.CreatedAt, agent.UpdatedAt,
	)
	return err
}

// FindByID 根据ID查找
func (r *BaseAgentRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.BaseAgent, error) {
	query := `
		SELECT id, name, type, api_url, api_token, default_model, cli_path, git_bash_path, max_tokens, timeout_minutes, is_default, created_at, updated_at
		FROM base_agents WHERE id = ?
	`
	agent := &model.BaseAgent{}
	var idStr string
	var apiURL, apiToken, defaultModel, cliPath, gitBashPath sql.NullString
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &agent.Name, &agent.Type, &apiURL, &apiToken, &defaultModel, &cliPath, &gitBashPath, &agent.MaxTokens, &agent.TimeoutMinutes, &agent.IsDefault, &agent.CreatedAt, &agent.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find base agent: %w", err)
	}
	agent.ID, _ = uuid.Parse(idStr)
	agent.ApiURL = apiURL.String
	agent.ApiToken = apiToken.String
	agent.DefaultModel = defaultModel.String
	agent.CliPath = cliPath.String
	agent.GitBashPath = gitBashPath.String
	return agent, nil
}

// FindByType 根据类型查找
func (r *BaseAgentRepository) FindByType(ctx context.Context, agentType model.BaseAgentType) ([]*model.BaseAgent, error) {
	query := `
		SELECT id, name, type, api_url, api_token, default_model, cli_path, git_bash_path, max_tokens, timeout_minutes, is_default, created_at, updated_at
		FROM base_agents WHERE type = ? ORDER BY is_default DESC, name
	`
	rows, err := r.db.QueryContext(ctx, query, agentType)
	if err != nil {
		return nil, fmt.Errorf("failed to find base agents: %w", err)
	}
	defer rows.Close()

	var agents = make([]*model.BaseAgent, 0)
	for rows.Next() {
		agent := &model.BaseAgent{}
		var idStr string
		var apiURL, apiToken, defaultModel, cliPath, gitBashPath sql.NullString
		err := rows.Scan(
			&idStr, &agent.Name, &agent.Type, &apiURL, &apiToken, &defaultModel, &cliPath, &gitBashPath, &agent.MaxTokens, &agent.TimeoutMinutes, &agent.IsDefault, &agent.CreatedAt, &agent.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan base agent: %w", err)
		}
		agent.ID, _ = uuid.Parse(idStr)
		agent.ApiURL = apiURL.String
		agent.ApiToken = apiToken.String
		agent.DefaultModel = defaultModel.String
		agent.CliPath = cliPath.String
		agent.GitBashPath = gitBashPath.String
		agents = append(agents, agent)
	}
	return agents, nil
}

// List 列出所有基础Agent
func (r *BaseAgentRepository) List(ctx context.Context) ([]*model.BaseAgent, error) {
	query := `
		SELECT id, name, type, api_url, api_token, default_model, cli_path, git_bash_path, max_tokens, timeout_minutes, is_default, created_at, updated_at
		FROM base_agents ORDER BY is_default DESC, type, name
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list base agents: %w", err)
	}
	defer rows.Close()

	var agents = make([]*model.BaseAgent, 0)
	for rows.Next() {
		agent := &model.BaseAgent{}
		var idStr string
		var apiURL, apiToken, defaultModel, cliPath, gitBashPath sql.NullString
		err := rows.Scan(
			&idStr, &agent.Name, &agent.Type, &apiURL, &apiToken, &defaultModel, &cliPath, &gitBashPath, &agent.MaxTokens, &agent.TimeoutMinutes, &agent.IsDefault, &agent.CreatedAt, &agent.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan base agent: %w", err)
		}
		agent.ID, _ = uuid.Parse(idStr)
		agent.ApiURL = apiURL.String
		agent.ApiToken = apiToken.String
		agent.DefaultModel = defaultModel.String
		agent.CliPath = cliPath.String
		agent.GitBashPath = gitBashPath.String
		agents = append(agents, agent)
	}
	return agents, nil
}

// ListActive 列出所有基础Agent（移除了启用/禁用功能后，相当于列出所有代理）
func (r *BaseAgentRepository) ListActive(ctx context.Context) ([]*model.BaseAgent, error) {
	return r.List(ctx)
}

// Update 更新基础Agent
func (r *BaseAgentRepository) Update(ctx context.Context, agent *model.BaseAgent) error {
	query := `
		UPDATE base_agents
		SET name = ?, type = ?, api_url = ?, api_token = ?, default_model = ?, cli_path = ?, git_bash_path = ?, max_tokens = ?, timeout_minutes = ?, is_default = ?, updated_at = ?
		WHERE id = ?
	`
	agent.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, query,
		agent.Name, agent.Type, agent.ApiURL, agent.ApiToken, agent.DefaultModel, agent.CliPath, agent.GitBashPath, agent.MaxTokens, agent.TimeoutMinutes, agent.IsDefault, agent.UpdatedAt, agent.ID.String(),
	)
	return err
}

// Delete 删除基础Agent
func (r *BaseAgentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM base_agents WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}

// FindDefault 查找默认基础Agent
func (r *BaseAgentRepository) FindDefault(ctx context.Context) (*model.BaseAgent, error) {
	query := `
		SELECT id, name, type, api_url, api_token, default_model, cli_path, git_bash_path, max_tokens, timeout_minutes, is_default, created_at, updated_at
		FROM base_agents WHERE is_default = true LIMIT 1
	`
	agent := &model.BaseAgent{}
	var idStr string
	var apiURL, apiToken, defaultModel, cliPath, gitBashPath sql.NullString
	err := r.db.QueryRowContext(ctx, query).Scan(
		&idStr, &agent.Name, &agent.Type, &apiURL, &apiToken, &defaultModel, &cliPath, &gitBashPath, &agent.MaxTokens, &agent.TimeoutMinutes, &agent.IsDefault, &agent.CreatedAt, &agent.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // 没有默认的基础Agent
		}
		return nil, fmt.Errorf("failed to find default base agent: %w", err)
	}
	agent.ID, _ = uuid.Parse(idStr)
	agent.ApiURL = apiURL.String
	agent.ApiToken = apiToken.String
	agent.DefaultModel = defaultModel.String
	agent.CliPath = cliPath.String
	agent.GitBashPath = gitBashPath.String
	return agent, nil
}

// SetDefault 设置默认基础Agent（事务操作：先清除所有默认，再设置指定为默认）
func (r *BaseAgentRepository) SetDefault(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 先清除所有默认
	if _, err := tx.ExecContext(ctx, "UPDATE base_agents SET is_default = false, updated_at = ? WHERE is_default = true", time.Now()); err != nil {
		return fmt.Errorf("failed to clear default: %w", err)
	}

	// 设置指定为默认
	if _, err := tx.ExecContext(ctx, "UPDATE base_agents SET is_default = true, updated_at = ? WHERE id = ?", time.Now(), id.String()); err != nil {
		return fmt.Errorf("failed to set default: %w", err)
	}

	return tx.Commit()
}