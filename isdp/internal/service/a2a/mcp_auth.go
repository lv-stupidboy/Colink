package a2a

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MCPAuthService MCP认证服务
type MCPAuthService struct {
	tokens map[string]*MCPToken
	mu     sync.RWMutex
	ttl    time.Duration
}

// MCPToken MCP令牌
type MCPToken struct {
	ID           string
	ThreadID     uuid.UUID
	InvocationID uuid.UUID
	CreatedAt    time.Time
	ExpiresAt    time.Time
	Used         bool
}

// NewMCPAuthService 创建MCP认证服务
func NewMCPAuthService(ttl time.Duration) *MCPAuthService {
	svc := &MCPAuthService{
		tokens: make(map[string]*MCPToken),
		ttl:    ttl,
	}

	// 启动清理协程
	go svc.cleanupExpiredTokens()

	return svc
}

// GenerateToken 生成MCP回调令牌
// 双UUID认证：ThreadID + InvocationID
func (s *MCPAuthService) GenerateToken(ctx context.Context, threadID, invocationID uuid.UUID) (string, error) {
	// 生成随机令牌
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	tokenID := hex.EncodeToString(tokenBytes)

	token := &MCPToken{
		ID:           tokenID,
		ThreadID:     threadID,
		InvocationID: invocationID,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(s.ttl),
		Used:         false,
	}

	s.mu.Lock()
	s.tokens[tokenID] = token
	s.mu.Unlock()

	return tokenID, nil
}

// ValidateToken 验证令牌
func (s *MCPAuthService) ValidateToken(ctx context.Context, tokenID string, threadID, invocationID uuid.UUID) error {
	s.mu.RLock()
	token, exists := s.tokens[tokenID]
	s.mu.RUnlock()

	if !exists {
		return ErrTokenNotFound
	}

	// 检查是否过期
	if time.Now().After(token.ExpiresAt) {
		return ErrTokenExpired
	}

	// 检查是否已使用（一次性令牌）
	if token.Used {
		return ErrTokenUsed
	}

	// 验证双UUID
	if token.ThreadID != threadID || token.InvocationID != invocationID {
		return ErrInvalidToken
	}

	return nil
}

// UseToken 使用令牌（标记为已用）
func (s *MCPAuthService) UseToken(ctx context.Context, tokenID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	token, exists := s.tokens[tokenID]
	if !exists {
		return ErrTokenNotFound
	}

	token.Used = true
	return nil
}

// ValidateAndUse 验证并使用令牌
func (s *MCPAuthService) ValidateAndUse(ctx context.Context, tokenID string, threadID, invocationID uuid.UUID) error {
	if err := s.ValidateToken(ctx, tokenID, threadID, invocationID); err != nil {
		return err
	}
	return s.UseToken(ctx, tokenID)
}

// GetTokenInfo 获取令牌信息
func (s *MCPAuthService) GetTokenInfo(tokenID string) (*MCPToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	token, exists := s.tokens[tokenID]
	if !exists {
		return nil, ErrTokenNotFound
	}
	return token, nil
}

// RevokeToken 撤销令牌
func (s *MCPAuthService) RevokeToken(ctx context.Context, tokenID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tokens[tokenID]; !exists {
		return ErrTokenNotFound
	}

	delete(s.tokens, tokenID)
	return nil
}

// cleanupExpiredTokens 清理过期令牌
func (s *MCPAuthService) cleanupExpiredTokens() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, token := range s.tokens {
			if now.After(token.ExpiresAt) || token.Used {
				delete(s.tokens, id)
			}
		}
		s.mu.Unlock()
	}
}

// MCPCallbackRequest MCP回调请求
type MCPCallbackRequest struct {
	TokenID      string          `json:"tokenId"`
	ThreadID     string          `json:"threadId"`
	InvocationID string          `json:"invocationId"`
	Action       string          `json:"action"`
	Data         interface{}     `json:"data"`
}

// MCPCallbackHandler MCP回调处理器
type MCPCallbackHandler struct {
	authSvc *MCPAuthService
	handlers map[string]CallbackFunc
}

// CallbackFunc 回调函数
type CallbackFunc func(ctx context.Context, req *MCPCallbackRequest) error

// NewMCPCallbackHandler 创建回调处理器
func NewMCPCallbackHandler(authSvc *MCPAuthService) *MCPCallbackHandler {
	return &MCPCallbackHandler{
		authSvc:  authSvc,
		handlers: make(map[string]CallbackFunc),
	}
}

// RegisterHandler 注册处理器
func (h *MCPCallbackHandler) RegisterHandler(action string, handler CallbackFunc) {
	h.handlers[action] = handler
}

// HandleCallback 处理回调
func (h *MCPCallbackHandler) HandleCallback(ctx context.Context, req *MCPCallbackRequest) error {
	// 验证令牌
	threadID, err := uuid.Parse(req.ThreadID)
	if err != nil {
		return err
	}
	invocationID, err := uuid.Parse(req.InvocationID)
	if err != nil {
		return err
	}

	if err := h.authSvc.ValidateAndUse(ctx, req.TokenID, threadID, invocationID); err != nil {
		return err
	}

	// 调用处理器
	handler, exists := h.handlers[req.Action]
	if !exists {
		return ErrUnknownAction
	}

	return handler(ctx, req)
}

var (
	ErrTokenNotFound = errors.New("token not found")
	ErrTokenExpired  = errors.New("token expired")
	ErrTokenUsed     = errors.New("token already used")
	ErrInvalidToken  = errors.New("invalid token")
	ErrUnknownAction = errors.New("unknown action")
)