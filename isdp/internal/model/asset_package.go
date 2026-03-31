package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== AssetPackage Models ==========

// AssetPackage 资产包模型
type AssetPackage struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"` // v1.0.0-20240331-143052
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (a *AssetPackage) TableName() string {
	return "asset_packages"
}

// AssetPackageManifest 资产包 manifest.json 结构
type AssetPackageManifest struct {
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	ExportedAt  string                 `json:"exportedAt"`
	Description string                 `json:"description"`
	Assets      AssetPackageAssetsList `json:"assets"`
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
	Name    string `json:"name"`
	Version string `json:"version"`
}

// AssetPackageCommandItem 命令项
type AssetPackageCommandItem struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	BoundSkills []string `json:"boundSkills,omitempty"`
}

// AssetPackageSubagentItem 子代理项
type AssetPackageSubagentItem struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	BoundSkills []string `json:"boundSkills,omitempty"`
}

// AssetPackageRuleItem 规则项
type AssetPackageRuleItem struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// AssetPackageSettingsItem 配置项
type AssetPackageSettingsItem struct {
	Name string `json:"name"`
}

// AssetPackageListQuery 资产包列表查询参数
type AssetPackageListQuery struct {
	Search   string `form:"search"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// ExportAssetPackageRequest 导出资产包请求
type ExportAssetPackageRequest struct {
	Name        string      `json:"name" binding:"required"`
	Version     string      `json:"version" binding:"required"` // 语义化版本如 "1.0.0"
	Description string      `json:"description"`
	SkillIDs    []uuid.UUID `json:"skillIds"`
	CommandIDs  []uuid.UUID `json:"commandIds"`
	SubagentIDs []uuid.UUID `json:"subagentIds"`
	RuleIDs     []uuid.UUID `json:"ruleIds"`
	SettingsIDs []uuid.UUID `json:"settingsIds"`
}

// ImportResult 导入结果
type ImportResult struct {
	PackageName string         `json:"packageName"`
	PackageID   uuid.UUID      `json:"packageId"`
	Success     int            `json:"success"`
	Skipped     int            `json:"skipped"`
	Failed      int            `json:"failed"`
	Details     []ImportDetail `json:"details"`
}

// ImportDetail 导入详情
type ImportDetail struct {
	AssetType string `json:"assetType"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Status    string `json:"status"` // success, skipped, failed
	Message   string `json:"message,omitempty"`
}