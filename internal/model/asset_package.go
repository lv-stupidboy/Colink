// 文件路径: isdp/internal/model/asset_package.go
package model

// AssetPackageManifest 资产包 manifest.json 结构（简化版，无版本概念）
type AssetPackageManifest struct {
	ExportedAt string                 `json:"exportedAt"`
	Assets     AssetPackageAssetsList `json:"assets"`
}

// AssetPackageAssetsList 资产列表
type AssetPackageAssetsList struct {
	Skills    []AssetPackageSkillItem    `json:"skills,omitempty"`
	Commands  []AssetPackageCommandItem  `json:"commands,omitempty"`
	Subagents []AssetPackageSubagentItem `json:"subagents,omitempty"`
	Rules     []AssetPackageRuleItem     `json:"rules,omitempty"`
	Settings  []AssetPackageSettingsItem `json:"settings,omitempty"`
}

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

// ExportAssetPackageRequest 导出资产包请求（简化版）
type ExportAssetPackageRequest struct {
	SkillIDs    []string `json:"skillIds"`
	CommandIDs  []string `json:"commandIds"`
	SubagentIDs []string `json:"subagentIds"`
	RuleIDs     []string `json:"ruleIds"`
	SettingsIDs []string `json:"settingsIds"`
}

// ImportResult 导入结果
type ImportResult struct {
	Success int            `json:"success"`
	Skipped int            `json:"skipped"`
	Failed  int            `json:"failed"`
	Details []ImportDetail `json:"details"`
}

// ImportDetail 导入详情
type ImportDetail struct {
	AssetType string `json:"assetType"`
	Name      string `json:"name"`
	Status    string `json:"status"` // success, skipped, failed
	Message   string `json:"message,omitempty"`
}

// AssetPackagePreview 资产包导入预览
type AssetPackagePreview struct {
	Assets AssetPackagePreviewAssets `json:"assets"`
}

// AssetPackagePreviewAssets 资产预览
type AssetPackagePreviewAssets struct {
	Skills    []AssetPackagePreviewAsset `json:"skills"`
	Commands  []AssetPackagePreviewAsset `json:"commands"`
	Subagents []AssetPackagePreviewAsset `json:"subagents"`
	Rules     []AssetPackagePreviewAsset `json:"rules"`
	Settings  []AssetPackagePreviewAsset `json:"settings"`
}

// AssetPackagePreviewAsset 单个资产预览
type AssetPackagePreviewAsset struct {
	Name   string `json:"name"`
	Exists bool   `json:"exists"`
}

// AssetPackageImportConfirm 资产包导入确认请求
type AssetPackageImportConfirm struct {
	Mode         string                    `json:"mode"` // overwrite | skip | selective
	AssetActions []AssetPackageAssetAction `json:"assetActions"`
}

// AssetPackageAssetAction 资产操作
type AssetPackageAssetAction struct {
	AssetType string `json:"assetType"`
	Name      string `json:"name"`
	Action    string `json:"action"` // overwrite | skip
}