package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// Service 知识库服务
type Service struct {
	kbRepo *repo.KnowledgeBaseRepository
}

// NewService 创建知识库服务
func NewService(kbRepo *repo.KnowledgeBaseRepository) *Service {
	return &Service{
		kbRepo: kbRepo,
	}
}

// Create 创建知识库
func (s *Service) Create(ctx context.Context, req *model.CreateKnowledgeBaseRequest) (*model.KnowledgeBase, error) {
	// 检查名称是否重复
	existing, err := s.kbRepo.FindByName(ctx, req.Name)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("检查知识库名称失败: %w", err)
		}
	} else if existing != nil {
		return nil, errors.New("知识库名称已存在")
	}

	kb := &model.KnowledgeBase{
		ID:            uuid.New(),
		Name:          req.Name,
		DisplayName:   req.DisplayName,
		Description:   req.Description,
		Type:          req.Type,
		Config:        req.Config,
		QueryEndpoint: req.QueryEndpoint,
		Status:        model.KnowledgeBaseStatusActive,
		QueryCount:    0,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.kbRepo.Create(ctx, kb); err != nil {
		return nil, fmt.Errorf("创建知识库失败: %w", err)
	}

	return kb, nil
}

// GetByID 根据 ID 获取知识库
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*model.KnowledgeBase, error) {
	return s.kbRepo.FindByID(ctx, id)
}

// GetByName 根据名称获取知识库
func (s *Service) GetByName(ctx context.Context, name string) (*model.KnowledgeBase, error) {
	return s.kbRepo.FindByName(ctx, name)
}

// List 列出知识库
func (s *Service) List(ctx context.Context, query *model.KnowledgeBaseListQuery) ([]*model.KnowledgeBase, int64, error) {
	return s.kbRepo.List(ctx, query)
}

// Update 更新知识库
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateKnowledgeBaseRequest) (*model.KnowledgeBase, error) {
	kb, err := s.kbRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("知识库不存在: %w", err)
	}

	// 更新字段
	if req.DisplayName != "" {
		kb.DisplayName = req.DisplayName
	}
	if req.Description != "" {
		kb.Description = req.Description
	}
	if req.Config != nil {
		kb.Config = req.Config
	}
	if req.QueryEndpoint != "" {
		kb.QueryEndpoint = req.QueryEndpoint
	}
	if req.Status != "" {
		kb.Status = req.Status
	}
	kb.UpdatedAt = time.Now()

	if err := s.kbRepo.Update(ctx, kb); err != nil {
		return nil, fmt.Errorf("更新知识库失败: %w", err)
	}

	return kb, nil
}

// Delete 删除知识库
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.kbRepo.Delete(ctx, id)
}

// Query 查询知识库
func (s *Service) Query(ctx context.Context, kbID uuid.UUID, req *model.KnowledgeQueryRequest) (*model.KnowledgeQueryResult, error) {
	kb, err := s.kbRepo.FindByID(ctx, kbID)
	if err != nil {
		return nil, fmt.Errorf("知识库不存在: %w", err)
	}

	if kb.Status != model.KnowledgeBaseStatusActive {
		return nil, errors.New("知识库未激活")
	}

	// 根据类型选择查询策略
	var result *model.KnowledgeQueryResult
	switch kb.Type {
	case model.KnowledgeBaseTypeMCP:
		result, err = s.queryMCP(ctx, kb, req)
	case model.KnowledgeBaseTypeAPI:
		result, err = s.queryAPI(ctx, kb, req)
	case model.KnowledgeBaseTypeGit:
		result, err = s.queryGit(ctx, kb, req)
	default:
		err = fmt.Errorf("不支持的知识库类型: %s", kb.Type)
	}

	if err != nil {
		return nil, err
	}

	// 更新查询统计
	s.kbRepo.UpdateQueryStats(ctx, kbID)

	return result, nil
}

// QueryAll 查询所有活跃知识库
func (s *Service) QueryAll(ctx context.Context, req *model.KnowledgeQueryRequest) ([]*model.KnowledgeQueryResult, error) {
	kbs, err := s.kbRepo.FindByStatus(ctx, model.KnowledgeBaseStatusActive)
	if err != nil {
		return nil, fmt.Errorf("获取知识库列表失败: %w", err)
	}

	results := make([]*model.KnowledgeQueryResult, 0)
	for _, kb := range kbs {
		result, err := s.Query(ctx, kb.ID, req)
		if err != nil {
			// 记录错误但继续查询其他知识库
			results = append(results, &model.KnowledgeQueryResult{
				Query:  req.Query,
				Source: kb.Name,
				Error:  err.Error(),
			})
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// queryMCP 通过 MCP 协议查询
func (s *Service) queryMCP(ctx context.Context, kb *model.KnowledgeBase, req *model.KnowledgeQueryRequest) (*model.KnowledgeQueryResult, error) {
	// MCP 查询实现
	// 通过 query_endpoint 或 config 中的 MCP 服务器配置进行查询

	endpoint := kb.QueryEndpoint
	if endpoint == "" {
		endpoint = kb.Config["endpoint"]
	}
	if endpoint == "" {
		return nil, errors.New("MCP 知识库缺少查询端点配置")
	}

	// 构建 MCP 请求
	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "query",
			"arguments": map[string]interface{}{
				"query": req.Query,
			},
		},
	}

	if req.Limit > 0 {
		mcpRequest["params"].(map[string]interface{})["arguments"].(map[string]interface{})["limit"] = req.Limit
	}

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	body, _ := json.Marshal(mcpRequest)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// 添加认证
	if token, ok := kb.Config["token"]; ok && token != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 MCP 服务失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MCP 服务返回错误: %d", resp.StatusCode)
	}

	// 解析响应
	var mcpResponse struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if err := json.Unmarshal(respBody, &mcpResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if mcpResponse.Error != nil {
		return nil, fmt.Errorf("MCP 错误: %s", mcpResponse.Error.Message)
	}

	// 转换结果
	result := &model.KnowledgeQueryResult{
		Query:  req.Query,
		Source: kb.Name,
	}

	for _, content := range mcpResponse.Result.Content {
		if content.Type == "text" {
			// 尝试解析为结构化数据
			var snippets []*model.KnowledgeSnippet
			if err := json.Unmarshal([]byte(content.Text), &snippets); err == nil {
				result.Results = append(result.Results, snippets...)
			} else {
				// 作为单个文本片段处理
				result.Results = append(result.Results, &model.KnowledgeSnippet{
					Content: content.Text,
					Source:  kb.Name,
				})
			}
		}
	}

	result.Total = len(result.Results)
	return result, nil
}

// queryAPI 通过 API 查询
func (s *Service) queryAPI(ctx context.Context, kb *model.KnowledgeBase, req *model.KnowledgeQueryRequest) (*model.KnowledgeQueryResult, error) {
	endpoint := kb.QueryEndpoint
	if endpoint == "" {
		return nil, errors.New("API 知识库缺少查询端点配置")
	}

	// 构建请求
	apiReq := map[string]interface{}{
		"query": req.Query,
	}
	if req.Limit > 0 {
		apiReq["limit"] = req.Limit
	}

	client := &http.Client{Timeout: 30 * time.Second}
	body, _ := json.Marshal(apiReq)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if token, ok := kb.Config["token"]; ok && token != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 返回错误: %d", resp.StatusCode)
	}

	// 解析响应
	var apiResponse struct {
		Results []*model.KnowledgeSnippet `json:"results"`
		Total   int                       `json:"total"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &model.KnowledgeQueryResult{
		Query:   req.Query,
		Results: apiResponse.Results,
		Total:   apiResponse.Total,
		Source:  kb.Name,
	}, nil
}

// queryGit 通过 Git 仓库查询
func (s *Service) queryGit(ctx context.Context, kb *model.KnowledgeBase, req *model.KnowledgeQueryRequest) (*model.KnowledgeQueryResult, error) {
	// Git 知识库查询实现
	// 可以通过 GitHub API 或本地 Git 仓库进行搜索
	// TODO: 实现具体的 Git 查询逻辑
	return nil, errors.New("Git 知识库查询尚未实现")
}