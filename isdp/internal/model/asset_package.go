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
	Name string `json:"name"`
	File string `json:"file"`
}

// AssetPackageCommandItem 命令项
type AssetPackageCommandItem struct {
	Name        string   `json:"name"`
	File        string   `json:"file"`
	BoundSkills []string `json:"boundSkills,omitempty"`
}

// AssetPackageSubagentItem 子代理项
type AssetPackageSubagentItem struct {
	Name        string   `json:"name"`
	File        string   `json:"file"`
	BoundSkills []string `json:"boundSkills,omitempty"`
}

// AssetPackageRuleItem 规则项
type AssetPackageRuleItem struct {
	Name string `json:"name"`
	File string `json:"file"`
}

// AssetPackageSettingsItem 配置项
type AssetPackageSettingsItem struct {
	Name string `json:"name"`
	Dir  string `json:"dir"`
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