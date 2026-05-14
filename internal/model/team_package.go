// 文件路径: isdp/internal/model/team_package.go
package model

// AssetPackageSkillItem 技能项
type AssetPackageSkillItem struct {
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	Tags            []string        `json:"tags,omitempty"`
	SupportedAgents []string        `json:"supportedAgents,omitempty"`
	IsPublic        bool            `json:"isPublic"`
	SourceType      SkillSourceType `json:"sourceType"`
}

// AssetPackageCommandItem 命令项
type AssetPackageCommandItem struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	BoundSkills []string `json:"boundSkills,omitempty"`
}

// AssetPackageSubagentItem 子代理项
type AssetPackageSubagentItem struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	BoundSkills []string `json:"boundSkills,omitempty"`
}

// AssetPackageRuleItem 规则项
type AssetPackageRuleItem struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// AssetPackageSettingsItem 配置项
type AssetPackageSettingsItem struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ImportResult 导入结果
type ImportResult struct {
	Success          int              `json:"success"`
	Skipped          int              `json:"skipped"`
	Failed           int              `json:"failed"`
	Details          []ImportDetail   `json:"details"`
	ConfigGenResults []ConfigGenResult `json:"configGenResults,omitempty"` // 配置生成结果
}

// ImportDetail 导入详情
type ImportDetail struct {
	AssetType string `json:"assetType"`
	Name      string `json:"name"`
	ID        string `json:"id,omitempty"`  // 成功导入时的ID
	Status    string `json:"status"`        // success, skipped, failed
	Message   string `json:"message,omitempty"`
}

// ConfigGenResult 配置生成结果
type ConfigGenResult struct {
	AgentID   string `json:"agentId"`
	AgentName string `json:"agentName"`
	Status    string `json:"status"` // success, failed, skipped
	Message   string `json:"message,omitempty"`
}

// TeamPackageManifest 团队包 manifest.json 结构
type TeamPackageManifest struct {
	ExportedAt string              `json:"exportedAt"`
	Workflow   TeamPackageWorkflow `json:"workflow"`
	Roles      []TeamPackageRole   `json:"roles"`
	Assets     TeamPackageAssets   `json:"assets"`
}

// TeamPackageWorkflow 工作流信息
type TeamPackageWorkflow struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	AgentIDs      []string     `json:"agentIds"`
	Transitions   []Transition `json:"transitions"`
	Checkpoints   []string     `json:"checkpoints"`
	EstimatedTime string       `json:"estimatedTime"`
	IsSystem      bool         `json:"isSystem"`
	IsDefault     bool         `json:"isDefault"`
}

// TeamPackageRole 角色信息
type TeamPackageRole struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Role            string              `json:"role"`
	Description     string              `json:"description"`
	SystemPrompt    string              `json:"systemPrompt"`
	MaxTokens       int                 `json:"maxTokens"`
	Temperature     float64             `json:"temperature"`
	RequiresHuman   bool                `json:"requiresHuman"` // 是否需要人工参与
	MentionPatterns []string            `json:"mentionPatterns"`
	Bindings        TeamPackageBindings `json:"bindings"`
}

// TeamPackageBindings 角色绑定信息
type TeamPackageBindings struct {
	Skills    []string `json:"skills,omitempty"`
	Commands  []string `json:"commands,omitempty"`
	Subagents []string `json:"subagents,omitempty"`
	Rules     []string `json:"rules,omitempty"`
	Settings  []string `json:"settings,omitempty"`
}

// TeamPackageAssets 团队包资产
type TeamPackageAssets struct {
	Skills    []AssetPackageSkillItem    `json:"skills,omitempty"`
	Commands  []AssetPackageCommandItem  `json:"commands,omitempty"`
	Subagents []AssetPackageSubagentItem `json:"subagents,omitempty"`
	Rules     []AssetPackageRuleItem     `json:"rules,omitempty"`
	Settings  []AssetPackageSettingsItem `json:"settings,omitempty"`
}

// TeamPackagePreview 团队包导入预览
type TeamPackagePreview struct {
	Workflow TeamPackagePreviewWorkflow `json:"workflow"`
	Roles    []TeamPackagePreviewRole   `json:"roles"`
	Assets   TeamPackagePreviewAssets   `json:"assets"`
}

// TeamPackagePreviewWorkflow 工作流预览
type TeamPackagePreviewWorkflow struct {
	Name   string `json:"name"`
	Exists bool   `json:"exists"`
}

// TeamPackagePreviewRole 角色预览
type TeamPackagePreviewRole struct {
	Name    string `json:"name"`
	Exists  bool   `json:"exists"`
	LocalID string `json:"localId,omitempty"`
}

// TeamPackagePreviewAssets 资产预览
type TeamPackagePreviewAssets struct {
	Skills    []TeamPackagePreviewAsset `json:"skills"`
	Commands  []TeamPackagePreviewAsset `json:"commands"`
	Subagents []TeamPackagePreviewAsset `json:"subagents"`
	Rules     []TeamPackagePreviewAsset `json:"rules"`
	Settings  []TeamPackagePreviewAsset `json:"settings"`
}

// TeamPackagePreviewAsset 单个资产预览
type TeamPackagePreviewAsset struct {
	Name   string `json:"name"`
	Exists bool   `json:"exists"`
}

// TeamPackageImportConfirm 团队包导入确认请求
type TeamPackageImportConfirm struct {
	Mode           string                   `json:"mode"` // overwrite | skip | selective
	WorkflowAction string                   `json:"workflowAction"`
	RoleActions    []TeamPackageRoleAction  `json:"roleActions"`
	AssetActions   []TeamPackageAssetAction `json:"assetActions"`
}

// TeamPackageRoleAction 角色操作
type TeamPackageRoleAction struct {
	Name   string `json:"name"`
	Action string `json:"action"` // overwrite | skip
}

// TeamPackageAssetAction 资产操作
type TeamPackageAssetAction struct {
	AssetType string `json:"assetType"`
	Name      string `json:"name"`
	Action    string `json:"action"` // overwrite | skip
}

// ExportTeamPackageRequest 导出团队包请求
type ExportTeamPackageRequest struct {
	WorkflowID string `json:"workflowId" binding:"required"`
}