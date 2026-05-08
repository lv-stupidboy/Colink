package reporter

import (
	"os/exec"
	"strings"
)

// GitUserInfo Git 用户信息
type GitUserInfo struct {
	Name  string `json:"gitName"`
	Email string `json:"gitEmail"`
}

// GetGitUserInfo 获取 Git 用户信息
// 执行 git config user.name 和 git config user.email
// 获取失败时返回空字符串
func GetGitUserInfo() GitUserInfo {
	info := GitUserInfo{}

	// 获取 git config user.name
	name, err := exec.Command("git", "config", "user.name").Output()
	if err == nil {
		info.Name = strings.TrimSpace(string(name))
	}

	// 获取 git config user.email
	email, err := exec.Command("git", "config", "user.email").Output()
	if err == nil {
		info.Email = strings.TrimSpace(string(email))
	}

	return info
}