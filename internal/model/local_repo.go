package model

import (
	"time"

	"github.com/google/uuid"
)

// RepoStatus 代码仓状态
type RepoStatus string

const (
	RepoStatusPending  RepoStatus = "pending"
	RepoStatusReady    RepoStatus = "ready"
	RepoStatusSyncing  RepoStatus = "syncing"
	RepoStatusError    RepoStatus = "error"
)

// LocalRepo 本地代码仓模型
type LocalRepo struct {
	ID           uuid.UUID      `json:"id"`
	Name         string         `json:"name"`
	GitUrl       string         `json:"gitUrl"`
	LocalPath    string         `json:"localPath"`
	Branch       *string        `json:"branch,omitempty"`
	LastCommit   *string        `json:"lastCommit,omitempty"`
	Status       RepoStatus     `json:"status"`
	ErrorMessage *string        `json:"errorMessage,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

func (r *LocalRepo) TableName() string { return "local_repos" }

// CreateRepoRequest 创建代码仓请求
type CreateRepoRequest struct {
	Name      string `json:"name" binding:"required"`
	LocalPath string `json:"localPath" binding:"required"`
	GitUrl    string `json:"gitUrl"`
	Branch    string `json:"branch"`
}

// UploadRepoRequest ZIP上传请求（multipart，不在JSON binding中）
type UploadRepoRequest struct {
	Name       string
	TargetPath string
}

// CloneRepoRequest 远程克隆请求
type CloneRepoRequest struct {
	GitUrl     string `json:"gitUrl" binding:"required"`
	Branch     string `json:"branch" binding:"required"`
	Name       string `json:"name"`
	TargetPath string `json:"targetPath" binding:"required"`
}

// RemoteBranchesRequest 获取远程分支请求
type RemoteBranchesRequest struct {
	GitUrl string `json:"gitUrl" binding:"required"`
}

// GitConfigRequest 配置GIT请求
type GitConfigRequest struct {
	GitUrl string `json:"gitUrl" binding:"required"`
	Branch string `json:"branch" binding:"required"`
}

// RemoteBranch 远程分支信息
type RemoteBranch struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// RepoBrowseRequest 目录浏览请求
type RepoBrowseRequest struct {
	Path string `form:"path"`
}
