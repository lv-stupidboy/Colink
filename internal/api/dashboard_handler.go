package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// DashboardHandler 首页统计API处理器
type DashboardHandler struct {
	db *sql.DB
}

// NewDashboardHandler 创建处理器
func NewDashboardHandler(db *sql.DB) *DashboardHandler {
	return &DashboardHandler{db: db}
}

// DashboardStats 首页统计数据响应
type DashboardStats struct {
	// 基础统计
	TotalProjects   int `json:"totalProjects"`
	ActiveThreads   int `json:"activeThreads"`
	WorkflowTeams   int `json:"workflowTeams"`
	AgentRoles      int `json:"agentRoles"`

	// 今日使用数据
	TodayAgentInteractions []AgentInteractionStats `json:"todayAgentInteractions"`
	TodayWorkflowUsage     []WorkflowUsageStats    `json:"todayWorkflowUsage"`

	// 资产统计
	TotalSkills    int `json:"totalSkills"`
	TotalCommands  int `json:"totalCommands"`
	TotalSubagents int `json:"totalSubagents"`
	TotalRules     int `json:"totalRules"`
}

// AgentInteractionStats Agent交互统计
type AgentInteractionStats struct {
	AgentID    string `json:"agentId"`
	AgentName  string `json:"agentName"`
	Count      int    `json:"count"`
	IsDefault  bool   `json:"isDefault"`
}

// WorkflowUsageStats 团队使用统计
type WorkflowUsageStats struct {
	WorkflowID   string `json:"workflowId"`
	WorkflowName string `json:"workflowName"`
	Count        int    `json:"count"`
	IsSystem     bool   `json:"isSystem"`
}

// WorkflowWithAssets 团队资产信息
type WorkflowWithAssets struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	IsSystem    bool        `json:"isSystem"`
	AgentCount  int         `json:"agentCount"`
	Agents      []AgentInfo `json:"agents"`
	Skills      int         `json:"skills"`
	Commands    int         `json:"commands"`
	Subagents   int         `json:"subagents"`
	Rules       int         `json:"rules"`
	TotalAssets int         `json:"totalAssets"`
	IsActive    bool        `json:"isActive"`
}

// AgentInfo 角色信息（包含资产统计）
type AgentInfo struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Role           string `json:"role"`
	SkillsCount    int    `json:"skillsCount"`
	CommandsCount  int    `json:"commandsCount"`
	SubagentsCount int    `json:"subagentsCount"`
	RulesCount     int    `json:"rulesCount"`
}

// GetStats 获取首页统计数据
func (h *DashboardHandler) GetStats(c *gin.Context) {
	ctx, cancel := contextWithTimeout(c)
	defer cancel()

	stats := DashboardStats{}

	// 基础统计
	stats.TotalProjects = h.queryCount(ctx, "SELECT COUNT(*) FROM projects")
	// 活跃任务：从 agent_invocations 表查询 status='running' 的 thread_id 去重数量
	stats.ActiveThreads = h.queryCount(ctx, "SELECT COUNT(DISTINCT thread_id) FROM agent_invocations WHERE status = 'running'")
	stats.WorkflowTeams = h.queryCount(ctx, "SELECT COUNT(*) FROM workflow_templates")
	stats.AgentRoles = h.queryCount(ctx, "SELECT COUNT(*) FROM agent_configs")

	// 资产统计
	stats.TotalSkills = h.queryCount(ctx, "SELECT COUNT(*) FROM skills")
	stats.TotalCommands = h.queryCount(ctx, "SELECT COUNT(*) FROM commands")
	stats.TotalSubagents = h.queryCount(ctx, "SELECT COUNT(*) FROM subagents")
	stats.TotalRules = h.queryCount(ctx, "SELECT COUNT(*) FROM rules")

	// 今日Agent交互统计
	stats.TodayAgentInteractions = h.queryTodayAgentInteractions(ctx)

	// 今日团队使用统计
	stats.TodayWorkflowUsage = h.queryTodayWorkflowUsage(ctx)

	c.JSON(http.StatusOK, stats)
}

// GetWorkflowsWithAssets 获取团队列表及其资产数量
func (h *DashboardHandler) GetWorkflowsWithAssets(c *gin.Context) {
	ctx, cancel := contextWithTimeout(c)
	defer cancel()

	workflows := h.queryWorkflowsWithAssets(ctx)
	c.JSON(http.StatusOK, workflows)
}

// GetActiveThreads 获取活跃线程列表（用于轮询）
// 从 agent_invocations 表查询 status='running' 的 thread_id，关联获取所有正在运行的 agent_names
func (h *DashboardHandler) GetActiveThreads(c *gin.Context) {
	ctx, cancel := contextWithTimeout(c)
	defer cancel()

	// 先查询活跃的 thread_id 列表
	threadQuery := `
		SELECT DISTINCT ai.thread_id
		FROM agent_invocations ai
		WHERE ai.status = 'running'
	`
	threadRows, err := h.db.QueryContext(ctx, threadQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer threadRows.Close()

	var threadIDs []string
	for threadRows.Next() {
		var id string
		if err := threadRows.Scan(&id); err != nil {
			continue
		}
		threadIDs = append(threadIDs, id)
	}

	if len(threadIDs) == 0 {
		c.JSON(http.StatusOK, []ActiveThreadInfo{})
		return
	}

	// 对每个 thread 查询详情和 agent_names
	var threads []ActiveThreadInfo
	for _, threadID := range threadIDs {
		// 查询 thread 详情
		detailQuery := `
			SELECT t.id, t.project_id, t.name, t.status, t.current_phase,
				   t.workflow_template_id, t.created_at, t.updated_at,
				   p.name as project_name, wt.name as workflow_name
			FROM threads t
			LEFT JOIN projects p ON t.project_id = p.id
			LEFT JOIN workflow_templates wt ON t.workflow_template_id = wt.id
			WHERE t.id = ?
		`
		var t ActiveThreadInfo
		var projectName, workflowName sql.NullString
		err := h.db.QueryRowContext(ctx, detailQuery, threadID).Scan(
			&t.ID, &t.ProjectID, &t.Name, &t.Status, &t.CurrentPhase,
			&t.WorkflowTemplateID, &t.CreatedAt, &t.UpdatedAt, &projectName, &workflowName)
		if err != nil {
			continue
		}
		t.ProjectName = projectName.String
		t.WorkflowName = workflowName.String

		// 查询该 thread 的所有 running agent_names
		agentQuery := `
			SELECT agent_name
			FROM agent_invocations
			WHERE thread_id = ? AND status = 'running'
		`
		agentRows, err := h.db.QueryContext(ctx, agentQuery, threadID)
		if err != nil {
			continue
		}
		var agentNames []string
		for agentRows.Next() {
			var name string
			if err := agentRows.Scan(&name); err != nil {
				continue
			}
			agentNames = append(agentNames, name)
		}
		agentRows.Close()
		t.CurrentAgentNames = agentNames

		threads = append(threads, t)
	}

	if threads == nil {
		threads = []ActiveThreadInfo{}
	}
	c.JSON(http.StatusOK, threads)
}

// ActiveThreadInfo 活跃线程信息
type ActiveThreadInfo struct {
	ID                string   `json:"id"`
	ProjectID         string   `json:"projectId"`
	Name              string   `json:"name"`
	Status            string   `json:"status"`
	CurrentPhase      string   `json:"currentPhase"`
	CurrentAgentNames []string `json:"currentAgentNames"`
	WorkflowTemplateID string   `json:"workflowTemplateId"`
	CreatedAt         string   `json:"createdAt"`
	UpdatedAt         string   `json:"updatedAt"`
	ProjectName       string   `json:"projectName"`
	WorkflowName      string   `json:"workflowName"`
}

// queryCount 执行COUNT查询
func (h *DashboardHandler) queryCount(ctx context.Context, query string) int {
	var count int
	row := h.db.QueryRowContext(ctx, query)
	if err := row.Scan(&count); err != nil {
		return 0
	}
	return count
}

// queryTodayAgentInteractions 查询今日Agent交互次数
func (h *DashboardHandler) queryTodayAgentInteractions(ctx context.Context) []AgentInteractionStats {
	today := time.Now().Format("2006-01-02")
	query := `
		SELECT ac.id, ac.name, ac.is_default, COUNT(ai.id) as count
		FROM agent_configs ac
		LEFT JOIN agent_invocations ai ON ai.agent_config_id = ac.id AND DATE(ai.created_at) = ?
		WHERE ai.id IS NOT NULL
		GROUP BY ac.id, ac.name, ac.is_default
		ORDER BY count DESC
		LIMIT 5
	`

	rows, err := h.db.QueryContext(ctx, query, today)
	if err != nil {
		return []AgentInteractionStats{}
	}
	defer rows.Close()

	var results []AgentInteractionStats
	for rows.Next() {
		var stat AgentInteractionStats
		if err := rows.Scan(&stat.AgentID, &stat.AgentName, &stat.IsDefault, &stat.Count); err != nil {
			continue
		}
		results = append(results, stat)
	}

	if results == nil {
		results = []AgentInteractionStats{}
	}
	return results
}

// queryTodayWorkflowUsage 查询今日团队使用次数
func (h *DashboardHandler) queryTodayWorkflowUsage(ctx context.Context) []WorkflowUsageStats {
	today := time.Now().Format("2006-01-02")
	query := `
		SELECT wt.id, wt.name, wt.is_system, COUNT(t.id) as count
		FROM workflow_templates wt
		LEFT JOIN threads t ON t.workflow_template_id = wt.id AND DATE(t.created_at) = ?
		WHERE t.id IS NOT NULL
		GROUP BY wt.id, wt.name, wt.is_system
		ORDER BY count DESC
		LIMIT 5
	`

	rows, err := h.db.QueryContext(ctx, query, today)
	if err != nil {
		return []WorkflowUsageStats{}
	}
	defer rows.Close()

	var results []WorkflowUsageStats
	for rows.Next() {
		var stat WorkflowUsageStats
		if err := rows.Scan(&stat.WorkflowID, &stat.WorkflowName, &stat.IsSystem, &stat.Count); err != nil {
			continue
		}
		results = append(results, stat)
	}

	if results == nil {
		results = []WorkflowUsageStats{}
	}
	return results
}

// queryWorkflowsWithAssets 查询团队及其资产数量（扩展版）
func (h *DashboardHandler) queryWorkflowsWithAssets(ctx context.Context) []WorkflowWithAssets {
	query := `
		SELECT wt.id, wt.name, wt.description, wt.is_system, wt.agent_ids
		FROM workflow_templates wt
		ORDER BY wt.is_system DESC, wt.updated_at DESC
	`

	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return []WorkflowWithAssets{}
	}
	defer rows.Close()

	var results []WorkflowWithAssets
	for rows.Next() {
		var w WorkflowWithAssets
		var agentIdsJSON string
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.IsSystem, &agentIdsJSON); err != nil {
			continue
		}

		// 解析 agent_ids JSON 数组
		agentIds := parseJSONArray(agentIdsJSON)
		w.AgentCount = len(agentIds)

		// 查询每个 Agent 的信息和资产统计
		w.Agents = h.queryAgentsWithAssets(ctx, agentIds)

		// 计算团队总资产（汇总所有 Agent 的资产）
		for _, agent := range w.Agents {
			w.Skills += agent.SkillsCount
			w.Commands += agent.CommandsCount
			w.Subagents += agent.SubagentsCount
			w.Rules += agent.RulesCount
		}
		w.TotalAssets = w.Skills + w.Commands + w.Subagents + w.Rules

		// 检查是否有活跃任务
		w.IsActive = h.checkWorkflowActive(ctx, w.ID)

		results = append(results, w)
	}

	if results == nil {
		results = []WorkflowWithAssets{}
	}
	return results
}

// queryAgentsWithAssets 查询 Agent 列表及其资产数量
func (h *DashboardHandler) queryAgentsWithAssets(ctx context.Context, agentIds []string) []AgentInfo {
	if len(agentIds) == 0 {
		return []AgentInfo{}
	}

	var results []AgentInfo
	for _, agentId := range agentIds {
		// 查询 Agent 基本信息
		var agent AgentInfo
		agent.ID = agentId

		// 查询名称和角色
		infoQuery := `SELECT name, role FROM agent_configs WHERE id = ?`
		row := h.db.QueryRowContext(ctx, infoQuery, agentId)
		var name, role sql.NullString
		if err := row.Scan(&name, &role); err != nil {
			// Agent 不存在，跳过
			continue
		}
		agent.Name = name.String
		agent.Role = role.String

		// 查询各类资产数量
		agent.SkillsCount = h.queryAgentAssetCount(ctx, agentId, "agent_skill_bindings")
		agent.CommandsCount = h.queryAgentAssetCount(ctx, agentId, "agent_command_bindings")
		agent.SubagentsCount = h.queryAgentAssetCount(ctx, agentId, "agent_subagent_bindings")
		agent.RulesCount = h.queryAgentAssetCount(ctx, agentId, "agent_rule_bindings")

		results = append(results, agent)
	}

	if results == nil {
		results = []AgentInfo{}
	}
	return results
}

// queryAgentAssetCount 查询 Agent 的某种资产数量
func (h *DashboardHandler) queryAgentAssetCount(ctx context.Context, agentId, bindingTable string) int {
	query := `SELECT COUNT(*) FROM ` + bindingTable + ` WHERE agent_role_id = ?`
	var count int
	row := h.db.QueryRowContext(ctx, query, agentId)
	if err := row.Scan(&count); err != nil {
		return 0
	}
	return count
}

// checkWorkflowActive 检查团队是否有活跃任务
func (h *DashboardHandler) checkWorkflowActive(ctx context.Context, workflowId string) bool {
	query := `
		SELECT COUNT(*) FROM threads t
		JOIN agent_invocations ai ON ai.thread_id = t.id
		WHERE t.workflow_template_id = ? AND ai.status = 'running'
	`
	var count int
	row := h.db.QueryRowContext(ctx, query, workflowId)
	if err := row.Scan(&count); err != nil {
		return false
	}
	return count > 0
}

// parseJSONArray 解析 JSON 数组字符串为字符串列表
func parseJSONArray(jsonStr string) []string {
	if jsonStr == "" || jsonStr == "[]" || jsonStr == "null" {
		return nil
	}

	// 去除空白和方括号
	jsonStr = strings.TrimSpace(jsonStr)
	if len(jsonStr) < 2 || jsonStr[0] != '[' || jsonStr[len(jsonStr)-1] != ']' {
		return nil
	}

	// 提取数组内容
	content := jsonStr[1:len(jsonStr)-1]
	if content == "" {
		return nil
	}

	// 分割元素（简单处理，假设格式正确）
	var result []string
	inString := false
	current := ""
	for i := 0; i < len(content); i++ {
		c := content[i]
		if c == '"' {
			inString = !inString
			if !inString && current != "" {
				result = append(result, strings.TrimSpace(current))
				current = ""
			}
		} else if inString {
			current += string(c)
		} else if c == ',' {
			if current != "" {
				result = append(result, strings.TrimSpace(current))
				current = ""
			}
		}
	}
	if current != "" {
		result = append(result, strings.TrimSpace(current))
	}

	return result
}

// contextWithTimeout 创建带超时的上下文
func contextWithTimeout(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), 10*time.Second)
}

// RegisterRoutes 注册路由
func (h *DashboardHandler) RegisterRoutes(r *gin.RouterGroup) {
	dashboard := r.Group("/dashboard")
	{
		dashboard.GET("/stats", h.GetStats)
		dashboard.GET("/workflows-with-assets", h.GetWorkflowsWithAssets)
		dashboard.GET("/active-threads", h.GetActiveThreads)
	}
}