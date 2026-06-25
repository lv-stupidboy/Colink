package mcp

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var ErrMCPServerNameExists = errors.New("mcp server name already exists")

// Service MCP asset service.
type Service struct {
	serverRepo  *repo.MCPServerRepository
	bindingRepo *repo.AgentMCPBindingRepository
	agentRepo   *repo.AgentConfigRepository
	logger      *zap.Logger
}

func NewService(
	serverRepo *repo.MCPServerRepository,
	bindingRepo *repo.AgentMCPBindingRepository,
	agentRepo *repo.AgentConfigRepository,
	logger *zap.Logger,
) *Service {
	return &Service{
		serverRepo:  serverRepo,
		bindingRepo: bindingRepo,
		agentRepo:   agentRepo,
		logger:      logger,
	}
}

func (s *Service) Create(ctx context.Context, req *model.CreateMCPServerRequest) (*model.MCPServer, error) {
	if !isValidName(req.Name) {
		return nil, errors.New("名称只能包含小写字母、数字和中划线，且必须以字母开头")
	}
	if err := validateTransport(req.Transport, req.Command, req.URL); err != nil {
		return nil, err
	}
	if existing, err := s.serverRepo.FindByName(ctx, req.Name); err == nil && existing != nil {
		return nil, ErrMCPServerNameExists
	}

	sourceType := req.SourceType
	if sourceType == "" {
		sourceType = model.MCPSourcePersonal
	}
	status := req.Status
	if status == "" {
		status = model.MCPStatusActive
	}

	now := time.Now()
	server := &model.MCPServer{
		ID:              uuid.New(),
		Name:            req.Name,
		DisplayName:     req.DisplayName,
		Description:     req.Description,
		Transport:       req.Transport,
		Command:         req.Command,
		Args:            req.Args,
		Env:             req.Env,
		URL:             req.URL,
		Headers:         req.Headers,
		SourceType:      sourceType,
		Status:          status,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if server.Env == nil {
		server.Env = map[string]string{}
	}
	if server.Headers == nil {
		server.Headers = map[string]string{}
	}

	if err := s.serverRepo.Create(ctx, server); err != nil {
		return nil, fmt.Errorf("创建 MCP Server 失败: %w", err)
	}
	s.logger.Info("创建 MCP Server 成功", zap.String("id", server.ID.String()), zap.String("name", server.Name))
	return server, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.MCPServer, error) {
	return s.serverRepo.FindByID(ctx, id)
}

func (s *Service) List(ctx context.Context, query *model.MCPServerListQuery) ([]*model.MCPServer, int64, error) {
	return s.serverRepo.List(ctx, query)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateMCPServerRequest) (*model.MCPServer, error) {
	server, err := s.serverRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.DisplayName != nil {
		server.DisplayName = *req.DisplayName
	}
	if req.Description != nil {
		server.Description = *req.Description
	}
	if req.Transport != nil {
		server.Transport = *req.Transport
	}
	if req.Command != nil {
		server.Command = *req.Command
	}
	if req.Args != nil {
		server.Args = req.Args
	}
	if req.Env != nil {
		server.Env = req.Env
	}
	if req.URL != nil {
		server.URL = *req.URL
	}
	if req.Headers != nil {
		server.Headers = req.Headers
	}
	if req.SourceType != nil {
		server.SourceType = *req.SourceType
	}
	if req.Status != nil {
		server.Status = *req.Status
	}
	if err := validateTransport(server.Transport, server.Command, server.URL); err != nil {
		return nil, err
	}
	server.UpdatedAt = time.Now()

	if err := s.serverRepo.Update(ctx, server); err != nil {
		return nil, fmt.Errorf("更新 MCP Server 失败: %w", err)
	}
	return server, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.serverRepo.Delete(ctx, id)
}

func (s *Service) ReplaceAgentBindings(ctx context.Context, agentRoleID uuid.UUID, serverIDs []uuid.UUID) error {
	if _, err := s.agentRepo.FindByID(ctx, agentRoleID); err != nil {
		return fmt.Errorf("Agent角色不存在: %w", err)
	}
	for _, serverID := range serverIDs {
		if _, err := s.serverRepo.FindByID(ctx, serverID); err != nil {
			return fmt.Errorf("MCP Server 不存在 %s: %w", serverID.String(), err)
		}
	}
	return s.bindingRepo.ReplaceBindings(ctx, agentRoleID, serverIDs)
}

func (s *Service) GetAgentBindings(ctx context.Context, agentRoleID uuid.UUID) ([]*model.MCPServer, error) {
	return s.bindingRepo.FindServersByAgentRoleID(ctx, agentRoleID)
}

func validateTransport(transport model.MCPTransport, command, url string) error {
	switch transport {
	case model.MCPTransportStdio:
		if command == "" {
			return errors.New("stdio MCP Server 必须配置 command")
		}
	case model.MCPTransportHTTP, model.MCPTransportSSE:
		if url == "" {
			return errors.New("http/sse MCP Server 必须配置 url")
		}
	default:
		return errors.New("不支持的 MCP transport")
	}
	return nil
}

func isValidName(name string) bool {
	if len(name) == 0 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-z][a-z0-9-]*$`, name)
	return matched
}
