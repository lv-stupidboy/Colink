package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// AgentConfigRepository Agent配置数据访问
type AgentConfigRepository struct {
	db *sql.DB
}

// NewAgentConfigRepository 创建Agent配置Repository
func NewAgentConfigRepository(db *sql.DB) *AgentConfigRepository {
	return &AgentConfigRepository{db: db}
}

// Create 创建配置
func (r *AgentConfigRepository) Create(ctx context.Context, config *model.AgentConfig) error {
	query := `
		INSERT INTO agent_configs (id, name, role, description, system_prompt, model_name, max_tokens, temperature, routing_config, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	routingConfig, _ := json.Marshal(config.RoutingConfig)
	_, err := r.db.ExecContext(ctx, query,
		config.ID.String(), config.Name, config.Role, config.Description, config.SystemPrompt, config.ModelName, config.MaxTokens, config.Temperature, routingConfig, config.IsDefault, config.CreatedAt, config.UpdatedAt,
	)
	return err
}

// FindByID 根据ID查找
func (r *AgentConfigRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.AgentConfig, error) {
	query := `
		SELECT id, name, role, description, system_prompt, model_name, max_tokens, temperature, routing_config, is_default, created_at, updated_at
		FROM agent_configs WHERE id = ?
	`
	config := &model.AgentConfig{}
	var idStr string
	var routingConfig []byte
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &config.Name, &config.Role, &config.Description, &config.SystemPrompt, &config.ModelName, &config.MaxTokens, &config.Temperature, &routingConfig, &config.IsDefault, &config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find agent config: %w", err)
	}
	config.ID, _ = uuid.Parse(idStr)
	json.Unmarshal(routingConfig, &config.RoutingConfig)
	return config, nil
}

// FindByRole 根据角色查找
func (r *AgentConfigRepository) FindByRole(ctx context.Context, role model.AgentRole) ([]*model.AgentConfig, error) {
	query := `
		SELECT id, name, role, description, system_prompt, model_name, max_tokens, temperature, routing_config, is_default, created_at, updated_at
		FROM agent_configs WHERE role = ? ORDER BY is_default DESC, name
	`
	rows, err := r.db.QueryContext(ctx, query, role)
	if err != nil {
		return nil, fmt.Errorf("failed to find agent configs: %w", err)
	}
	defer rows.Close()

	var configs []*model.AgentConfig
	for rows.Next() {
		config := &model.AgentConfig{}
		var idStr string
		var routingConfig []byte
		err := rows.Scan(
			&idStr, &config.Name, &config.Role, &config.Description, &config.SystemPrompt, &config.ModelName, &config.MaxTokens, &config.Temperature, &routingConfig, &config.IsDefault, &config.CreatedAt, &config.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent config: %w", err)
		}
		config.ID, _ = uuid.Parse(idStr)
		json.Unmarshal(routingConfig, &config.RoutingConfig)
		configs = append(configs, config)
	}
	return configs, nil
}

// List 列出所有配置
func (r *AgentConfigRepository) List(ctx context.Context) ([]*model.AgentConfig, error) {
	query := `
		SELECT id, name, role, description, system_prompt, model_name, max_tokens, temperature, routing_config, is_default, created_at, updated_at
		FROM agent_configs ORDER BY role, name
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent configs: %w", err)
	}
	defer rows.Close()

	var configs []*model.AgentConfig
	for rows.Next() {
		config := &model.AgentConfig{}
		var idStr string
		var routingConfig []byte
		err := rows.Scan(
			&idStr, &config.Name, &config.Role, &config.Description, &config.SystemPrompt, &config.ModelName, &config.MaxTokens, &config.Temperature, &routingConfig, &config.IsDefault, &config.CreatedAt, &config.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent config: %w", err)
		}
		config.ID, _ = uuid.Parse(idStr)
		json.Unmarshal(routingConfig, &config.RoutingConfig)
		configs = append(configs, config)
	}
	return configs, nil
}

// Update 更新配置
func (r *AgentConfigRepository) Update(ctx context.Context, config *model.AgentConfig) error {
	query := `
		UPDATE agent_configs
		SET name = ?, role = ?, description = ?, system_prompt = ?, model_name = ?, max_tokens = ?, temperature = ?, routing_config = ?, is_default = ?, updated_at = ?
		WHERE id = ?
	`
	routingConfig, _ := json.Marshal(config.RoutingConfig)
	config.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, query,
		config.Name, config.Role, config.Description, config.SystemPrompt, config.ModelName, config.MaxTokens, config.Temperature, routingConfig, config.IsDefault, config.UpdatedAt, config.ID.String(),
	)
	return err
}

// Delete 删除配置
func (r *AgentConfigRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM agent_configs WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}