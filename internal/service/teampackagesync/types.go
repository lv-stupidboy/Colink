package teampackagesync

import "github.com/anthropic/isdp/internal/model"

type RemotePackageList struct {
	Categories []RemotePackageCategory `json:"categories"`
}

type RemotePackageCategory struct {
	Name     string          `json:"name"`
	Packages []RemotePackage `json:"packages"`
}

type RemotePackage struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Path        string `json:"path"`
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

type PackageInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}