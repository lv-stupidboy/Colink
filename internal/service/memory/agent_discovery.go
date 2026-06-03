package memory

import (
	"context"
	"sort"
	"strings"

	"github.com/anthropic/isdp/internal/model"
)

type AgentConfigLister interface {
	List(ctx context.Context) ([]*model.AgentRoleConfig, error)
}

func (m *MemoryManager) ListTeamAgents(ctx context.Context, workspacePath string) ([]AgentInfo, error) {
	if m.agentLister == nil {
		return []AgentInfo{}, nil
	}
	configs, err := m.agentLister.List(ctx)
	if err != nil {
		return nil, err
	}
	agents := make([]AgentInfo, 0, len(configs))
	for _, config := range configs {
		if config == nil || config.Role.IsHumanRole() {
			continue
		}
		agents = append(agents, AgentInfo{
			ID:           config.ID.String(),
			Name:         config.Name,
			Role:         string(config.Role),
			Description:  config.Description,
			Capabilities: inferAgentCapabilities(config),
			Source:       "agent-config",
		})
	}
	sort.SliceStable(agents, func(i, j int) bool {
		return agents[i].Name < agents[j].Name
	})
	return agents, nil
}

func inferAgentCapabilities(config *model.AgentRoleConfig) []string {
	text := strings.ToLower(config.Name + " " + config.Description + " " + config.SystemPrompt)
	capabilitySignals := map[string][]string{
		"ui":             {"ui", "界面", "前端", "组件", "浏览器"},
		"components":     {"组件", "component"},
		"router":         {"路由", "router"},
		"frontend-build": {"前端构建", "vite", "webpack", "frontend"},
		"api":            {"api", "接口", "后端"},
		"database":       {"数据库", "sql", "mysql", "postgres", "sqlite"},
		"service":        {"服务", "service", "业务逻辑"},
		"test":           {"测试", "test", "playwright", "vitest"},
		"architecture":   {"架构", "设计", "拆解"},
		"review":         {"审查", "review"},
	}
	var capabilities []string
	for capability, signals := range capabilitySignals {
		for _, signal := range signals {
			if strings.Contains(text, strings.ToLower(signal)) {
				capabilities = append(capabilities, capability)
				break
			}
		}
	}
	sort.Strings(capabilities)
	return capabilities
}
