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

// MCPServerRepository MCP server data access.
type MCPServerRepository struct {
	BaseRepository
}

func NewMCPServerRepository(db *sql.DB, dbType DBType) *MCPServerRepository {
	return &MCPServerRepository{BaseRepository: NewBaseRepository(db, dbType)}
}

func (r *MCPServerRepository) Create(ctx context.Context, server *model.MCPServer) error {
	query := `
		INSERT INTO mcp_servers (
			id, name, display_name, description, transport, command, args, env, url, headers,
			source_type, supported_agents, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	argsJSON, _ := json.Marshal(server.Args)
	envJSON, _ := json.Marshal(server.Env)
	headersJSON, _ := json.Marshal(server.Headers)
	supportedAgentsJSON, _ := json.Marshal(server.SupportedAgents)

	_, err := r.DB().ExecContext(ctx, query,
		server.ID.String(), server.Name, server.DisplayName, server.Description,
		string(server.Transport), server.Command, string(argsJSON), string(envJSON), server.URL, string(headersJSON),
		string(server.SourceType), string(supportedAgentsJSON), string(server.Status),
		server.CreatedAt, server.UpdatedAt,
	)
	return err
}

func scanMCPServer(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.MCPServer, error) {
	server := &model.MCPServer{}
	var idStr string
	var displayName, description, command, url sql.NullString
	var transport, sourceType, status string
	var argsJSON, envJSON, headersJSON, supportedAgentsJSON []byte
	var createdAt, updatedAt SQLiteTimeScanner

	err := scanner.Scan(
		&idStr, &server.Name, &displayName, &description, &transport, &command,
		&argsJSON, &envJSON, &url, &headersJSON, &sourceType, &supportedAgentsJSON,
		&status, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	server.ID, _ = uuid.Parse(idStr)
	if displayName.Valid {
		server.DisplayName = displayName.String
	}
	if description.Valid {
		server.Description = description.String
	}
	server.Transport = model.MCPTransport(transport)
	if command.Valid {
		server.Command = command.String
	}
	json.Unmarshal(argsJSON, &server.Args)
	json.Unmarshal(envJSON, &server.Env)
	if url.Valid {
		server.URL = url.String
	}
	json.Unmarshal(headersJSON, &server.Headers)
	server.SourceType = model.MCPSourceType(sourceType)
	json.Unmarshal(supportedAgentsJSON, &server.SupportedAgents)
	server.Status = model.MCPStatus(status)
	server.CreatedAt = createdAt.Time
	server.UpdatedAt = updatedAt.Time
	return server, nil
}

func (r *MCPServerRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.MCPServer, error) {
	query := `
		SELECT id, name, display_name, description, transport, command, args, env, url, headers,
		       source_type, supported_agents, status, created_at, updated_at
		FROM mcp_servers WHERE id = ?
	`
	server, err := scanMCPServer(r.DB().QueryRowContext(ctx, query, id.String()))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("mcp server not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find mcp server: %w", err)
	}
	return server, nil
}

func (r *MCPServerRepository) FindByName(ctx context.Context, name string) (*model.MCPServer, error) {
	query := `
		SELECT id, name, display_name, description, transport, command, args, env, url, headers,
		       source_type, supported_agents, status, created_at, updated_at
		FROM mcp_servers WHERE name = ?
	`
	server, err := scanMCPServer(r.DB().QueryRowContext(ctx, query, name))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("mcp server not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find mcp server: %w", err)
	}
	return server, nil
}

func (r *MCPServerRepository) List(ctx context.Context, query *model.MCPServerListQuery) ([]*model.MCPServer, int64, error) {
	var conditions []string
	var args []interface{}

	if query.Search != "" {
		conditions = append(conditions, "(name LIKE ? OR display_name LIKE ? OR description LIKE ?)")
		searchPattern := "%" + query.Search + "%"
		args = append(args, searchPattern, searchPattern, searchPattern)
	}
	if query.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, query.Status)
	}
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

	countQuery := "SELECT COUNT(*) FROM mcp_servers " + whereClause
	var total int64
	if err := r.DB().QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count mcp servers: %w", err)
	}

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

	listQuery := `
		SELECT id, name, display_name, description, transport, command, args, env, url, headers,
		       source_type, supported_agents, status, created_at, updated_at
		FROM mcp_servers ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	args = append(args, pageSize, offset)

	rows, err := r.DB().QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list mcp servers: %w", err)
	}
	defer rows.Close()

	servers := make([]*model.MCPServer, 0)
	for rows.Next() {
		server, err := scanMCPServer(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan mcp server: %w", err)
		}
		servers = append(servers, server)
	}
	return servers, total, nil
}

func (r *MCPServerRepository) Update(ctx context.Context, server *model.MCPServer) error {
	query := `
		UPDATE mcp_servers
		SET display_name = ?, description = ?, transport = ?, command = ?, args = ?, env = ?,
		    url = ?, headers = ?, source_type = ?, supported_agents = ?, status = ?, updated_at = ?
		WHERE id = ?
	`
	argsJSON, _ := json.Marshal(server.Args)
	envJSON, _ := json.Marshal(server.Env)
	headersJSON, _ := json.Marshal(server.Headers)
	supportedAgentsJSON, _ := json.Marshal(server.SupportedAgents)

	_, err := r.DB().ExecContext(ctx, query,
		server.DisplayName, server.Description, string(server.Transport), server.Command,
		string(argsJSON), string(envJSON), server.URL, string(headersJSON), string(server.SourceType),
		string(supportedAgentsJSON), string(server.Status), server.UpdatedAt, server.ID.String(),
	)
	return err
}

func (r *MCPServerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.DB().ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = ?`, id.String())
	return err
}

// AgentMCPBindingRepository manages AgentRole -> MCP bindings.
type AgentMCPBindingRepository struct {
	BaseRepository
}

func NewAgentMCPBindingRepository(db *sql.DB, dbType DBType) *AgentMCPBindingRepository {
	return &AgentMCPBindingRepository{BaseRepository: NewBaseRepository(db, dbType)}
}

func (r *AgentMCPBindingRepository) ReplaceBindings(ctx context.Context, agentRoleID uuid.UUID, serverIDs []uuid.UUID) error {
	tx, err := r.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM agent_mcp_bindings WHERE agent_role_id = ?`, agentRoleID.String()); err != nil {
		return err
	}
	for _, serverID := range serverIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO agent_mcp_bindings (id, agent_role_id, mcp_server_id, enabled, created_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, uuid.New().String(), agentRoleID.String(), serverID.String(), true); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *AgentMCPBindingRepository) FindServerIDsByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.DB().QueryContext(ctx, `
		SELECT mcp_server_id FROM agent_mcp_bindings
		WHERE agent_role_id = ? AND enabled = 1
	`, agentRoleID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var idStr string
		if err := rows.Scan(&idStr); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *AgentMCPBindingRepository) FindServersByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]*model.MCPServer, error) {
	rows, err := r.DB().QueryContext(ctx, `
		SELECT s.id, s.name, s.display_name, s.description, s.transport, s.command, s.args, s.env,
		       s.url, s.headers, s.source_type, s.supported_agents, s.status, s.created_at, s.updated_at
		FROM mcp_servers s
		INNER JOIN agent_mcp_bindings b ON b.mcp_server_id = s.id
		WHERE b.agent_role_id = ? AND b.enabled = 1 AND s.status = 'active'
		ORDER BY s.name ASC
	`, agentRoleID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	servers := make([]*model.MCPServer, 0)
	for rows.Next() {
		server, err := scanMCPServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, nil
}

func (r *AgentMCPBindingRepository) ExistsBinding(ctx context.Context, agentRoleID, serverID uuid.UUID) (bool, error) {
	var count int
	err := r.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM agent_mcp_bindings WHERE agent_role_id = ? AND mcp_server_id = ?
	`, agentRoleID.String(), serverID.String()).Scan(&count)
	return count > 0, err
}
