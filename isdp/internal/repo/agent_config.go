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
func (r *AgentConfigRepository) Create(ctx context.Context, config *model.AgentRoleConfig) error {
	query := `
		INSERT INTO agent_configs (id, name, role, description, system_prompt, max_tokens, temperature, base_agent_id, is_default, is_system, mention_patterns, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	mentionPatterns, _ := json.Marshal(config.MentionPatterns)

	var baseAgentID interface{}
	if config.BaseAgentID != uuid.Nil {
		baseAgentID = config.BaseAgentID.String()
	}

	_, err := r.db.ExecContext(ctx, query,
		config.ID.String(), config.Name, config.Role, config.Description, config.SystemPrompt, config.MaxTokens, config.Temperature, baseAgentID, config.IsDefault, config.IsSystem, mentionPatterns, config.CreatedAt, config.UpdatedAt,
	)
	return err
}

// FindByID 根据ID查找
func (r *AgentConfigRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.AgentRoleConfig, error) {
	query := `
		SELECT id, name, role, description, system_prompt, max_tokens, temperature, base_agent_id, is_default, is_system, mention_patterns, config_generated_at, config_path, created_at, updated_at
		FROM agent_configs WHERE id = ?
	`
	config := &model.AgentRoleConfig{}
	var idStr string
	var mentionPatterns []byte
	var baseAgentID, description, systemPrompt, configPath sql.NullString
	var maxTokens sql.NullInt64
	var temperature sql.NullFloat64
	var configGeneratedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &config.Name, &config.Role, &description, &systemPrompt, &maxTokens, &temperature, &baseAgentID, &config.IsDefault, &config.IsSystem, &mentionPatterns, &configGeneratedAt, &configPath, &config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find agent config: %w", err)
	}
	config.ID, _ = uuid.Parse(idStr)
	if baseAgentID.Valid {
		config.BaseAgentID, _ = uuid.Parse(baseAgentID.String)
	}
	if description.Valid {
		config.Description = description.String
	}
	if systemPrompt.Valid {
		config.SystemPrompt = systemPrompt.String
	}
	if maxTokens.Valid {
		config.MaxTokens = int(maxTokens.Int64)
	}
	if temperature.Valid {
		config.Temperature = temperature.Float64
	}
	if configGeneratedAt.Valid {
		config.ConfigGeneratedAt = &configGeneratedAt.Time
	}
	if configPath.Valid {
		config.ConfigPath = configPath.String
	}
	json.Unmarshal(mentionPatterns, &config.MentionPatterns)
	return config, nil
}

// FindByRole 根据角色查找
func (r *AgentConfigRepository) FindByRole(ctx context.Context, role model.AgentRole) ([]*model.AgentRoleConfig, error) {
	query := `
		SELECT id, name, role, description, system_prompt, max_tokens, temperature, base_agent_id, is_default, is_system, mention_patterns, config_generated_at, config_path, created_at, updated_at
		FROM agent_configs WHERE role = ? ORDER BY is_default DESC, name
	`
	rows, err := r.db.QueryContext(ctx, query, role)
	if err != nil {
		return nil, fmt.Errorf("failed to find agent configs: %w", err)
	}
	defer rows.Close()

	configs := make([]*model.AgentRoleConfig, 0)
	for rows.Next() {
		config := &model.AgentRoleConfig{}
		var idStr string
		var mentionPatterns []byte
		var baseAgentID, description, systemPrompt, configPath sql.NullString
		var maxTokens sql.NullInt64
		var temperature sql.NullFloat64
		var configGeneratedAt sql.NullTime
		err := rows.Scan(
			&idStr, &config.Name, &config.Role, &description, &systemPrompt, &maxTokens, &temperature, &baseAgentID, &config.IsDefault, &config.IsSystem, &mentionPatterns, &configGeneratedAt, &configPath, &config.CreatedAt, &config.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent config: %w", err)
		}
		config.ID, _ = uuid.Parse(idStr)
		if baseAgentID.Valid {
			config.BaseAgentID, _ = uuid.Parse(baseAgentID.String)
		}
		if description.Valid {
			config.Description = description.String
		}
		if systemPrompt.Valid {
			config.SystemPrompt = systemPrompt.String
		}
		if maxTokens.Valid {
			config.MaxTokens = int(maxTokens.Int64)
		}
		if temperature.Valid {
			config.Temperature = temperature.Float64
		}
		if configGeneratedAt.Valid {
			config.ConfigGeneratedAt = &configGeneratedAt.Time
		}
		if configPath.Valid {
			config.ConfigPath = configPath.String
		}
		json.Unmarshal(mentionPatterns, &config.MentionPatterns)
		configs = append(configs, config)
	}
	return configs, nil
}

// List 列出所有配置
func (r *AgentConfigRepository) List(ctx context.Context) ([]*model.AgentRoleConfig, error) {
	query := `
		SELECT id, name, role, description, system_prompt, max_tokens, temperature, base_agent_id, is_default, is_system, mention_patterns, config_generated_at, config_path, created_at, updated_at
		FROM agent_configs ORDER BY is_system DESC, role, name
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent configs: %w", err)
	}
	defer rows.Close()

	configs := make([]*model.AgentRoleConfig, 0)
	for rows.Next() {
		config := &model.AgentRoleConfig{}
		var idStr string
		var mentionPatterns []byte
		var baseAgentID, description, systemPrompt, configPath sql.NullString
		var maxTokens sql.NullInt64
		var temperature sql.NullFloat64
		var configGeneratedAt sql.NullTime
		err := rows.Scan(
			&idStr, &config.Name, &config.Role, &description, &systemPrompt, &maxTokens, &temperature, &baseAgentID, &config.IsDefault, &config.IsSystem, &mentionPatterns, &configGeneratedAt, &configPath, &config.CreatedAt, &config.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent config: %w", err)
		}
		config.ID, _ = uuid.Parse(idStr)
		if baseAgentID.Valid {
			config.BaseAgentID, _ = uuid.Parse(baseAgentID.String)
		}
		if description.Valid {
			config.Description = description.String
		}
		if systemPrompt.Valid {
			config.SystemPrompt = systemPrompt.String
		}
		if maxTokens.Valid {
			config.MaxTokens = int(maxTokens.Int64)
		}
		if temperature.Valid {
			config.Temperature = temperature.Float64
		}
		if configGeneratedAt.Valid {
			config.ConfigGeneratedAt = &configGeneratedAt.Time
		}
		if configPath.Valid {
			config.ConfigPath = configPath.String
		}
		json.Unmarshal(mentionPatterns, &config.MentionPatterns)
		configs = append(configs, config)
	}
	return configs, nil
}

// Update 更新配置
func (r *AgentConfigRepository) Update(ctx context.Context, config *model.AgentRoleConfig) error {
	query := `
		UPDATE agent_configs
		SET name = ?, role = ?, description = ?, system_prompt = ?, max_tokens = ?, temperature = ?, base_agent_id = ?, is_default = ?, is_system = ?, mention_patterns = ?, updated_at = ?
		WHERE id = ?
	`
	mentionPatterns, _ := json.Marshal(config.MentionPatterns)
	config.UpdatedAt = time.Now()

	var baseAgentID interface{}
	if config.BaseAgentID != uuid.Nil {
		baseAgentID = config.BaseAgentID.String()
	}

	_, err := r.db.ExecContext(ctx, query,
		config.Name, config.Role, config.Description, config.SystemPrompt, config.MaxTokens, config.Temperature, baseAgentID, config.IsDefault, config.IsSystem, mentionPatterns, config.UpdatedAt, config.ID.String(),
	)
	return err
}

// Delete 删除配置
func (r *AgentConfigRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM agent_configs WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}

// UpdateConfigGeneratedAt 更新配置生成时间和路径
func (r *AgentConfigRepository) UpdateConfigGeneratedAt(ctx context.Context, id uuid.UUID, configPath string) error {
	query := `
		UPDATE agent_configs
		SET config_generated_at = NOW(), config_path = ?, updated_at = NOW()
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query, configPath, id.String())
	return err
}