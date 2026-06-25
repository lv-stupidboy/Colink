package model

import (
	"time"

	"github.com/google/uuid"
)

// MCPTransport MCP server transport type.
type MCPTransport string

const (
	MCPTransportStdio MCPTransport = "stdio"
	MCPTransportHTTP  MCPTransport = "http"
	MCPTransportSSE   MCPTransport = "sse"
)

// MCPSourceType MCP server source type.
type MCPSourceType string

const (
	MCPSourcePlatform  MCPSourceType = "platform"
	MCPSourcePersonal  MCPSourceType = "personal"
	MCPSourceTeam      MCPSourceType = "team_package"
	MCPSourceFederated MCPSourceType = "federated"
)

// MCPStatus MCP server status.
type MCPStatus string

const (
	MCPStatusActive   MCPStatus = "active"
	MCPStatusDisabled MCPStatus = "disabled"
)

// MCPServer MCP server asset.
type MCPServer struct {
	ID              uuid.UUID         `json:"id"`
	Name            string            `json:"name"`
	DisplayName     string            `json:"displayName,omitempty"`
	Description     string            `json:"description,omitempty"`
	Transport       MCPTransport      `json:"transport"`
	Command         string            `json:"command,omitempty"`
	Args            []string          `json:"args,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	URL             string            `json:"url,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	SourceType      MCPSourceType     `json:"sourceType"`
	Status          MCPStatus         `json:"status"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
}

func (m *MCPServer) TableName() string {
	return "mcp_servers"
}

// AgentMCPBinding Agent role to MCP server binding.
type AgentMCPBinding struct {
	ID          uuid.UUID `json:"id"`
	AgentRoleID uuid.UUID `json:"agentRoleId"`
	MCPServerID uuid.UUID `json:"mcpServerId"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (a *AgentMCPBinding) TableName() string {
	return "agent_mcp_bindings"
}

// CreateMCPServerRequest creates an MCP server asset.
type CreateMCPServerRequest struct {
	Name            string            `json:"name" binding:"required"`
	DisplayName     string            `json:"displayName"`
	Description     string            `json:"description"`
	Transport       MCPTransport      `json:"transport" binding:"required"`
	Command         string            `json:"command"`
	Args            []string          `json:"args"`
	Env             map[string]string `json:"env"`
	URL             string            `json:"url"`
	Headers         map[string]string `json:"headers"`
	SourceType      MCPSourceType     `json:"sourceType"`
	Status          MCPStatus         `json:"status"`
}

// UpdateMCPServerRequest updates an MCP server asset.
type UpdateMCPServerRequest struct {
	DisplayName     *string           `json:"displayName"`
	Description     *string           `json:"description"`
	Transport       *MCPTransport     `json:"transport"`
	Command         *string           `json:"command"`
	Args            []string          `json:"args"`
	Env             map[string]string `json:"env"`
	URL             *string           `json:"url"`
	Headers         map[string]string `json:"headers"`
	SourceType      *MCPSourceType    `json:"sourceType"`
	Status          *MCPStatus        `json:"status"`
}

// MCPServerListQuery MCP server list query.
type MCPServerListQuery struct {
	Search    string `form:"search"`
	Status    string `form:"status"`
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
}

// BindMCPServersRequest binds MCP servers to an Agent role.
type BindMCPServersRequest struct {
	MCPServerIDs []uuid.UUID `json:"mcpServerIds" binding:"required"`
}
