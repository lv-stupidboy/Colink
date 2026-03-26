package agent

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// ConfigService Agent配置服务
type ConfigService struct {
	repo    *repo.AgentConfigRepository
	cache   map[uuid.UUID]*model.AgentRoleConfig
	cacheMu sync.RWMutex
}

// NewConfigService 创建配置服务
func NewConfigService(repo *repo.AgentConfigRepository) *ConfigService {
	return &ConfigService{
		repo:  repo,
		cache: make(map[uuid.UUID]*model.AgentRoleConfig),
	}
}

// GetByID 根据ID获取配置
func (s *ConfigService) GetByID(ctx context.Context, id uuid.UUID) (*model.AgentRoleConfig, error) {
	s.cacheMu.RLock()
	if config, ok := s.cache[id]; ok {
		s.cacheMu.RUnlock()
		return config, nil
	}
	s.cacheMu.RUnlock()

	config, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cache[id] = config
	s.cacheMu.Unlock()

	return config, nil
}

// GetByRole 根据角色获取配置
func (s *ConfigService) GetByRole(ctx context.Context, role model.AgentRole) ([]*model.AgentRoleConfig, error) {
	return s.repo.FindByRole(ctx, role)
}

// GetDefaultByRole 获取角色的默认配置
func (s *ConfigService) GetDefaultByRole(ctx context.Context, role model.AgentRole) (*model.AgentRoleConfig, error) {
	configs, err := s.repo.FindByRole(ctx, role)
	if err != nil {
		return nil, err
	}
	for _, c := range configs {
		if c.IsDefault {
			return c, nil
		}
	}
	if len(configs) > 0 {
		return configs[0], nil
	}
	return nil, ErrConfigNotFound
}

// Create 创建配置
func (s *ConfigService) Create(ctx context.Context, req *model.CreateAgentRequest) (*model.AgentRoleConfig, error) {
	// 角色必须由调用方指定
	role := req.Role
	if role == "" {
		role = model.AgentRole("custom")
	}

	config := &model.AgentRoleConfig{
		ID:             uuid.New(),
		Name:           req.Name,
		Role:           role,
		BaseAgentID:    req.BaseAgentID,
		Description:    req.Description,
		SystemPrompt:   req.SystemPrompt,
		MaxTokens:      req.MaxTokens,
		Temperature:    req.Temperature,
		IsDefault:      req.IsDefault,
		MentionPatterns: req.MentionPatterns,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.repo.Create(ctx, config); err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cache[config.ID] = config
	s.cacheMu.Unlock()

	return config, nil
}

// Update 更新配置
func (s *ConfigService) Update(ctx context.Context, id uuid.UUID, req *model.CreateAgentRequest) (*model.AgentRoleConfig, error) {
	config, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 角色必须由调用方指定
	role := req.Role
	if role == "" {
		role = model.AgentRole("custom")
	}

	config.Name = req.Name
	config.Role = role
	config.BaseAgentID = req.BaseAgentID
	config.Description = req.Description
	config.SystemPrompt = req.SystemPrompt
	config.MaxTokens = req.MaxTokens
	config.Temperature = req.Temperature
	config.IsDefault = req.IsDefault
	config.MentionPatterns = req.MentionPatterns
	config.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, config); err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cache[id] = config
	s.cacheMu.Unlock()

	return config, nil
}

// Delete 删除配置
func (s *ConfigService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	s.cacheMu.Lock()
	delete(s.cache, id)
	s.cacheMu.Unlock()

	return nil
}

// List 列出所有配置
func (s *ConfigService) List(ctx context.Context) ([]*model.AgentRoleConfig, error) {
	return s.repo.List(ctx)
}

var (
	ErrConfigNotFound = errors.New("agent config not found")
)

// InitSystemAgents 初始化系统预置角色
func (s *ConfigService) InitSystemAgents(ctx context.Context) error {
	// 检查全栈工程师角色是否已存在
	configs, err := s.repo.FindByRole(ctx, model.AgentRole("fullstack_engineer"))
	if err != nil {
		return err
	}

	// 如果已存在，跳过
	if len(configs) > 0 {
		return nil
	}

	// 创建全栈工程师角色
	fullstackEngineer := &model.AgentRoleConfig{
		ID:           uuid.New(),
		Name:         "全栈工程师",
		Role:         model.AgentRole("fullstack_engineer"),
		Description:  "全栈开发工程师，能独立完成从需求分析、架构设计、前后端开发到测试部署的完整项目开发流程",
		SystemPrompt: fullstackEngineerPrompt,
		MaxTokens:    4096,
		Temperature:  0.7,
		IsDefault:    false,
		IsSystem:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	return s.repo.Create(ctx, fullstackEngineer)
}

// 全栈工程师系统提示词
const fullstackEngineerPrompt = `你是一位资深的全栈工程师，具备完整的项目开发能力，能够独立完成从需求到上线的全部工作。

## 核心能力

### 1. 需求分析
- 理解业务需求，转化为技术方案
- 识别核心功能和边界条件
- 定义功能优先级和验收标准

### 2. 架构设计
- 系统架构设计：整体架构、模块划分、接口设计
- 技术选型：框架、中间件、数据库选择
- 数据建模：ER图设计、表结构设计

### 3. 前端开发
- React/Vue/Next.js 等现代前端框架开发
- 响应式布局、组件封装
- 状态管理、API 对接
- Tailwind CSS、Ant Design 等 UI 库使用

### 4. 后端开发
- Go/Node.js/Python 等后端语言开发
- RESTful/GraphQL API 设计与实现
- 数据库操作、缓存策略
- 消息队列、定时任务

### 5. 测试与质量
- 单元测试、集成测试编写
- 测试用例设计
- 代码审查、性能优化

### 6. 部署运维
- Docker 容器化
- CI/CD 流程配置
- 日志监控、故障排查

## 工作流程

### 接收任务时
1. 分析需求，确认理解无误
2. 制定开发计划，拆分任务
3. 设计技术方案

### 开发过程中
1. 按照最佳实践编写代码
2. 保持代码整洁、注释清晰
3. 编写必要的测试用例
4. 及时提交进度更新

### 完成任务后
1. 自测功能是否正常
2. 检查代码质量
3. 编写简要的完成说明

## 代码规范

### 通用规范
- 使用有意义的变量和函数命名
- 保持函数单一职责
- 添加必要的注释
- 遵循项目既有的代码风格

### 前端规范
- 组件化开发，保持组件可复用
- 合理使用状态管理
- 注意性能优化（懒加载、缓存等）

### 后端规范
- RESTful API 设计规范
- 统一的错误处理
- 合理的日志记录
- SQL 注入、XSS 等安全防护

## 输出标准

### 需求分析阶段
- 需求文档：功能描述、用户故事、验收标准
- 技术方案：架构图、技术选型、风险评估

### 开发阶段
- 代码实现：结构清晰、注释完整
- API 文档：接口说明、请求响应示例
- 数据库脚本：建表语句、索引设计

### 测试阶段
- 测试用例：覆盖主要场景
- 测试报告：通过情况、遗留问题

### 部署阶段
- 部署文档：部署步骤、配置说明
- 运维手册：监控配置、故障处理

## 完成标志
完成任务后，在输出末尾明确标注：【开发完成】`