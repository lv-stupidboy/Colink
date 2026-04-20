package teampackagesync

import "github.com/anthropic/isdp/internal/model"

type RemotePackage struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Repository  string `json:"repository"` // 包所在仓库URL（用于market场景）
	Source      string `json:"source"`     // 包在仓库中的相对路径（用于market场景）
}

type UpdateCheckResult struct {
	NeedUpdate  []PackageUpdateInfo `json:"needUpdate"`
	NewPackages []RemotePackage     `json:"newPackages"`
	Removed     []string            `json:"removed"`
}

type PackageUpdateInfo struct {
	Local  model.TeamPackageVersion `json:"local"`
	Remote RemotePackage            `json:"remote"`
}

type SyncPackageRequest struct {
	PackageName string                         `json:"packageName" binding:"required"`
	MarketId    string                         `json:"marketId"` // 可选：指定从哪个市场同步
	Confirm     *model.TeamPackageImportConfirm `json:"confirm"`
}

type PreviewPackageRequest struct {
	PackageName string `json:"packageName" binding:"required"`
	MarketId    string `json:"marketId"` // 可选：指定从哪个市场预览
}

// PreviewPackageResponse 包预览响应
type PreviewPackageResponse struct {
	PackageName string                      `json:"packageName"`
	Version     string                      `json:"version"`
	Description string                      `json:"description"`
	Workflow    PreviewWorkflowInfo         `json:"workflow"`
	Roles       []PreviewRoleInfo           `json:"roles"`
	Assets      PreviewAssetsInfo           `json:"assets"`
}

type PreviewWorkflowInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type PreviewRoleInfo struct {
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	Description string   `json:"description"`
	Assets      []string `json:"assets"` // 角色绑定的资产名称列表
}

type PreviewAssetsInfo struct {
	Skills    []PreviewAssetInfo `json:"skills"`
	Commands  []PreviewAssetInfo `json:"commands"`
	Subagents []PreviewAssetInfo `json:"subagents"`
	Rules     []PreviewAssetInfo `json:"rules"`
	Settings  []PreviewAssetInfo `json:"settings"`
}

type PreviewAssetInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}