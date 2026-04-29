package api

import (
	"context"
	"database/sql"
	"encoding/json"
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
	AgentID   string `json:"agentId"`
	AgentName string `json:"agentName"`
	Count     int    `json:"count"`
	IsDefault bool   `json:"isDefault"`
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
	UpdatedAt   string      `json:"updatedAt"`
}

// AgentInfo 角色信息（包含资产统计和名称列表）
type AgentInfo struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Role           string   `json:"role"`
	SkillsCount    int      `json:"skillsCount"`
	CommandsCount  int      `json:"commandsCount"`
	SubagentsCount int      `json:"subagentsCount"`
	RulesCount     int      `json:"rulesCount"`
	Skills         []string `json:"skills"`
	Commands       []string `json:"commands"`
	Subagents      []string `json:"subagents"`
	Rules          []string `json:"rules"`
}

// workflowRawData 用于临时存储解析结果
type workflowRawData struct {
	w        WorkflowWithAssets
	agentIds []string
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
func (h *DashboardHandler) GetActiveThreads(c *gin.Context) {
	ctx, cancel := contextWithTimeout(c)
	defer cancel()

	threadIDs := h.queryActiveThreadIDs(ctx)
	if len(threadIDs) == 0 {
		c.JSON(http.StatusOK, []ActiveThreadInfo{})
		return
	}

	threads := h.queryActiveThreadDetails(ctx, threadIDs)
	if threads == nil {
		threads = []ActiveThreadInfo{}
	}
	c.JSON(http.StatusOK, threads)
}

// GetRecentThreads 获取最近更新的任务列表
func (h *DashboardHandler) GetRecentThreads(c *gin.Context) {
	ctx, cancel := contextWithTimeout(c)
	defer cancel()

	threads := h.queryRecentThreads(ctx)
	if threads == nil {
		threads = []RecentThreadInfo{}
	}
	c.JSON(http.StatusOK, threads)
}

// ActiveThreadInfo 活跃线程信息
type ActiveThreadInfo struct {
	ID                 string   `json:"id"`
	ProjectID          string   `json:"projectId"`
	Name               string   `json:"name"`
	Status             string   `json:"status"`
	CurrentPhase       string   `json:"currentPhase"`
	CurrentAgentNames  []string `json:"currentAgentNames"`
	WorkflowTemplateID string   `json:"workflowTemplateId"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt"`
	ProjectName        string   `json:"projectName"`
	WorkflowName       string   `json:"workflowName"`
}

// RecentThreadInfo 最近任务信息
type RecentThreadInfo struct {
	ID                 string `json:"id"`
	ProjectID          string `json:"projectId"`
	Name               string `json:"name"`
	Status             string `json:"status"`
	CurrentPhase       string `json:"currentPhase"`
	WorkflowTemplateID string `json:"workflowTemplateId"`
	UpdatedAt          string `json:"updatedAt"`
	ProjectName        string `json:"projectName"`
	TeamName           string `json:"teamName"` // 所属团队名称（Agent团队，来自workflow_templates.name）
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

// queryWorkflowsWithAssets 查询团队及其资产数量（优化版 - 批量查询）
func (h *DashboardHandler) queryWorkflowsWithAssets(ctx context.Context) []WorkflowWithAssets {
	// Step 1: 查询所有 workflow 基本信息
	query := `
		SELECT wt.id, wt.name, wt.description, wt.is_system, wt.agent_ids, wt.updated_at
		FROM workflow_templates wt
		ORDER BY wt.is_system DESC, wt.updated_at DESC
	`

	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return []WorkflowWithAssets{}
	}
	defer rows.Close()

	var rawWorkflows []workflowRawData
	var allAgentIds []string
	seenAgentIds := make(map[string]bool)

	for rows.Next() {
		var w WorkflowWithAssets
		var agentIdsBytes []byte
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.IsSystem, &agentIdsBytes, &w.UpdatedAt); err != nil {
			continue
		}

		// 解析 agent_ids JSON 数组
		var agentIds []string
		if len(agentIdsBytes) > 0 {
			if err := json.Unmarshal(agentIdsBytes, &agentIds); err != nil {
				agentIds = parseJSONArray(string(agentIdsBytes))
			}
		}
		w.AgentCount = len(agentIds)

		// 收集所有唯一的 agentIds 用于批量查询
		for _, agentId := range agentIds {
			if !seenAgentIds[agentId] {
				seenAgentIds[agentId] = true
				allAgentIds = append(allAgentIds, agentId)
			}
		}

		rawWorkflows = append(rawWorkflows, workflowRawData{w: w, agentIds: agentIds})
	}

	if len(rawWorkflows) == 0 {
		return []WorkflowWithAssets{}
	}

	// Step 2: 批量查询所有 agent 基本信息
	agentInfoMap := h.batchQueryAgentInfo(ctx, allAgentIds)

	// Step 3: 批量查询所有 agent 资产数量
	assetCountMap := h.batchQueryAgentAssetCounts(ctx, allAgentIds)

	// Step 3b: 批量查询所有 agent 资产名称列表
	assetNamesMap := h.batchQueryAgentAssetNames(ctx, allAgentIds)

	// Step 3a: 批量查询活跃 workflow IDs
	activeWorkflowMap := h.batchQueryActiveWorkflowIDs(ctx)

	// Step 4: 组装数据
	var results []WorkflowWithAssets
	for _, raw := range rawWorkflows {
		w := raw.w
		// 构建每个 workflow 的 agents 列表
		for _, agentId := range raw.agentIds {
			agent := AgentInfo{ID: agentId}
			if info, ok := agentInfoMap[agentId]; ok {
				agent.Name = info.Name
				agent.Role = info.Role
			} else {
				agent.Name = "未知 Agent"
				agent.Role = "unknown"
			}
			if counts, ok := assetCountMap[agentId]; ok {
				agent.SkillsCount = counts.Skills
				agent.CommandsCount = counts.Commands
				agent.SubagentsCount = counts.Subagents
				agent.RulesCount = counts.Rules
			}
			if names, ok := assetNamesMap[agentId]; ok {
				agent.Skills = names.Skills
				agent.Commands = names.Commands
				agent.Subagents = names.Subagents
				agent.Rules = names.Rules
			}
			w.Agents = append(w.Agents, agent)
		}

		// 初始化 agents 数组（避免 JSON null）
		if w.Agents == nil {
			w.Agents = []AgentInfo{}
		}

		// 计算团队总资产
		for _, agent := range w.Agents {
			w.Skills += agent.SkillsCount
			w.Commands += agent.CommandsCount
			w.Subagents += agent.SubagentsCount
			w.Rules += agent.RulesCount
		}
		w.TotalAssets = w.Skills + w.Commands + w.Subagents + w.Rules

		// 使用批量查询结果判断是否活跃（替代循环内单独查询）
		w.IsActive = activeWorkflowMap[w.ID]

		results = append(results, w)
	}

	if results == nil {
		results = []WorkflowWithAssets{}
	}
	return results
}

// agentBasicInfo Agent基本信息
type agentBasicInfo struct {
	Name string
	Role string
}

// agentAssetCounts Agent资产数量
type agentAssetCounts struct {
	Skills    int
	Commands  int
	Subagents int
	Rules     int
}

// agentAssetNames Agent资产名称列表
type agentAssetNames struct {
	Skills    []string
	Commands  []string
	Subagents []string
	Rules     []string
}

// batchQueryAgentInfo 批量查询Agent基本信息
func (h *DashboardHandler) batchQueryAgentInfo(ctx context.Context, agentIds []string) map[string]agentBasicInfo {
	if len(agentIds) == 0 {
		return map[string]agentBasicInfo{}
	}

	// 构建 IN 查询参数
	placeholders := make([]string, len(agentIds))
	args := make([]interface{}, len(agentIds))
	for i, id := range agentIds {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `SELECT id, name, role FROM agent_configs WHERE id IN (` + strings.Join(placeholders, ",") + `)`

	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		return map[string]agentBasicInfo{}
	}
	defer rows.Close()

	result := make(map[string]agentBasicInfo)
	for rows.Next() {
		var id, name, role string
		if err := rows.Scan(&id, &name, &role); err != nil {
			continue
		}
		result[id] = agentBasicInfo{Name: name, Role: role}
	}

	return result
}

// batchQueryAgentAssetCounts 批量查询Agent资产数量（UNION ALL 合并查询）
func (h *DashboardHandler) batchQueryAgentAssetCounts(ctx context.Context, agentIds []string) map[string]agentAssetCounts {
	if len(agentIds) == 0 {
		return map[string]agentAssetCounts{}
	}

	placeholders := make([]string, len(agentIds))
	for i := range agentIds {
		placeholders[i] = "?"
	}

	// 构建 UNION ALL 查询，一次性获取所有资产类型计数
	inClause := strings.Join(placeholders, ",")
	query := `
		SELECT 'skills' as type, agent_role_id, COUNT(*) as count FROM agent_skill_bindings WHERE agent_role_id IN (` + inClause + `) GROUP BY agent_role_id
		UNION ALL
		SELECT 'commands' as type, agent_role_id, COUNT(*) as count FROM agent_command_bindings WHERE agent_role_id IN (` + inClause + `) GROUP BY agent_role_id
		UNION ALL
		SELECT 'subagents' as type, agent_role_id, COUNT(*) as count FROM agent_subagent_bindings WHERE agent_role_id IN (` + inClause + `) GROUP BY agent_role_id
		UNION ALL
		SELECT 'rules' as type, agent_role_id, COUNT(*) as count FROM agent_rule_bindings WHERE agent_role_id IN (` + inClause + `) GROUP BY agent_role_id
	`

	// 为每个 UNION 子查询提供完整的参数列表
	unionArgs := make([]interface{}, 0, len(agentIds)*4)
	for range 4 {
		for _, id := range agentIds {
			unionArgs = append(unionArgs, id)
		}
	}

	rows, err := h.db.QueryContext(ctx, query, unionArgs...)
	if err != nil {
		return map[string]agentAssetCounts{}
	}
	defer rows.Close()

	result := make(map[string]agentAssetCounts)
	for rows.Next() {
		var assetType, agentId string
		var count int
		if err := rows.Scan(&assetType, &agentId, &count); err != nil {
			continue
		}
		entry := result[agentId]
		switch assetType {
		case "skills":
			entry.Skills = count
		case "commands":
			entry.Commands = count
		case "subagents":
			entry.Subagents = count
		case "rules":
			entry.Rules = count
		}
		result[agentId] = entry
	}

	return result
}

// batchQueryAgentAssetNames 批量查询Agent资产名称列表
func (h *DashboardHandler) batchQueryAgentAssetNames(ctx context.Context, agentIds []string) map[string]agentAssetNames {
	if len(agentIds) == 0 {
		return map[string]agentAssetNames{}
	}

	placeholders := make([]string, len(agentIds))
	for i := range agentIds {
		placeholders[i] = "?"
	}
	inClause := strings.Join(placeholders, ",")

	// 查询 skills 名称
	skillQuery := `
		SELECT asb.agent_role_id, s.name
		FROM agent_skill_bindings asb
		JOIN skills s ON asb.skill_id = s.id
		WHERE asb.agent_role_id IN (` + inClause + `)
	`
	skillArgs := make([]interface{}, len(agentIds))
	for i, id := range agentIds {
		skillArgs[i] = id
	}

	result := make(map[string]agentAssetNames)
	rows, err := h.db.QueryContext(ctx, skillQuery, skillArgs...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var agentId, name string
			if err := rows.Scan(&agentId, &name); err == nil {
				entry := result[agentId]
				entry.Skills = append(entry.Skills, name)
				result[agentId] = entry
			}
		}
	}

	// 查询 commands 名称
	cmdQuery := `
		SELECT acb.agent_role_id, c.name
		FROM agent_command_bindings acb
		JOIN commands c ON acb.command_id = c.id
		WHERE acb.agent_role_id IN (` + inClause + `)
	`
	rows, err = h.db.QueryContext(ctx, cmdQuery, skillArgs...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var agentId, name string
			if err := rows.Scan(&agentId, &name); err == nil {
				entry := result[agentId]
				entry.Commands = append(entry.Commands, name)
				result[agentId] = entry
			}
		}
	}

	// 查询 subagents 名称
	subQuery := `
		SELECT asb.agent_role_id, s.name
		FROM agent_subagent_bindings asb
		JOIN subagents s ON asb.subagent_id = s.id
		WHERE asb.agent_role_id IN (` + inClause + `)
	`
	rows, err = h.db.QueryContext(ctx, subQuery, skillArgs...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var agentId, name string
			if err := rows.Scan(&agentId, &name); err == nil {
				entry := result[agentId]
				entry.Subagents = append(entry.Subagents, name)
				result[agentId] = entry
			}
		}
	}

	// 查询 rules 名称
	ruleQuery := `
		SELECT arb.agent_role_id, r.name
		FROM agent_rule_bindings arb
		JOIN rules r ON arb.rule_id = r.id
		WHERE arb.agent_role_id IN (` + inClause + `)
	`
	rows, err = h.db.QueryContext(ctx, ruleQuery, skillArgs...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var agentId, name string
			if err := rows.Scan(&agentId, &name); err == nil {
				entry := result[agentId]
				entry.Rules = append(entry.Rules, name)
				result[agentId] = entry
			}
		}
	}

	return result
}

// batchQueryActiveWorkflowIDs 批量查询有活跃任务的workflow IDs
func (h *DashboardHandler) batchQueryActiveWorkflowIDs(ctx context.Context) map[string]bool {
	query := `
		SELECT DISTINCT t.workflow_template_id
		FROM threads t
		JOIN agent_invocations ai ON ai.thread_id = t.id
		WHERE ai.status = 'running' AND t.workflow_template_id IS NOT NULL
	`
	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return map[string]bool{}
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		result[id] = true
	}
	return result
}

// queryActiveThreadIDs 查询活跃的 thread ID 列表
func (h *DashboardHandler) queryActiveThreadIDs(ctx context.Context) []string {
	query := `
		SELECT DISTINCT ai.thread_id
		FROM agent_invocations ai
		WHERE ai.status = 'running'
	`
	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var threadIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		threadIDs = append(threadIDs, id)
	}
	return threadIDs
}

// queryActiveThreadDetails 批量查询活跃线程详情
func (h *DashboardHandler) queryActiveThreadDetails(ctx context.Context, threadIDs []string) []ActiveThreadInfo {
	if len(threadIDs) == 0 {
		return nil
	}

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

	return threads
}

// queryRecentThreads 查询最近更新的任务列表
func (h *DashboardHandler) queryRecentThreads(ctx context.Context) []RecentThreadInfo {
	query := `
		SELECT t.id, t.project_id, t.name, t.status, t.current_phase,
			   t.workflow_template_id, t.updated_at,
			   p.name as project_name, wt.name as team_name
		FROM threads t
		LEFT JOIN projects p ON t.project_id = p.id
		LEFT JOIN workflow_templates wt ON t.workflow_template_id = wt.id
		ORDER BY t.updated_at DESC
		LIMIT 3
	`

	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return []RecentThreadInfo{}
	}
	defer rows.Close()

	var results []RecentThreadInfo
	for rows.Next() {
		var t RecentThreadInfo
		var projectName, teamName sql.NullString
		if err := rows.Scan(
			&t.ID, &t.ProjectID, &t.Name, &t.Status, &t.CurrentPhase,
			&t.WorkflowTemplateID, &t.UpdatedAt, &projectName, &teamName); err != nil {
			continue
		}
		t.ProjectName = projectName.String
		t.TeamName = teamName.String
		results = append(results, t)
	}

	if results == nil {
		results = []RecentThreadInfo{}
	}
	return results
}

// parseJSONArray 解析 JSON 数组字符串为字符串列表
func parseJSONArray(jsonStr string) []string {
	if jsonStr == "" || jsonStr == "[]" || jsonStr == "null" {
		return nil
	}

	// 去除空白
	jsonStr = strings.TrimSpace(jsonStr)
	if len(jsonStr) < 2 || jsonStr[0] != '[' || jsonStr[len(jsonStr)-1] != ']' {
		return nil
	}

	// 使用标准 JSON 解析
	var result []string
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// 如果标准解析失败，尝试手动解析
		content := jsonStr[1 : len(jsonStr)-1]
		if content == "" {
			return nil
		}
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
		dashboard.GET("/recent-threads", h.GetRecentThreads)
	}
}