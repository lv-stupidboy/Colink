package reporter

import (
	"os"
	"runtime"
)

// SystemInfo 系统信息
type SystemInfo struct {
	Hostname string `json:"hostname"`
	Platform string `json:"platform"` // runtime.GOOS: windows/linux/darwin
	Cwd      string `json:"cwd"`
	Homedir  string `json:"homedir"`
	Username string `json:"username"` // 系统用户名，作为 username 的 fallback
}

// GetSystemInfo 获取系统信息
func GetSystemInfo() SystemInfo {
	info := SystemInfo{
		Platform: runtime.GOOS,
	}

	// 获取主机名
	hostname, err := os.Hostname()
	if err == nil {
		info.Hostname = hostname
	}

	// 获取当前工作目录
	cwd, err := os.Getwd()
	if err == nil {
		info.Cwd = cwd
	}

	// 获取用户主目录
	homedir, err := os.UserHomeDir()
	if err == nil {
		info.Homedir = homedir
	}

	// 获取系统用户名（USER 或 USERNAME 环境变量）
	if user := os.Getenv("USER"); user != "" {
		info.Username = user
	} else if user := os.Getenv("USERNAME"); user != "" {
		info.Username = user
	}

	return info
}