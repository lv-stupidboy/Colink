package reporter

import "time"

// MessageReportData 上报数据结构
type MessageReportData struct {
	SessionId string        `json:"sessionId"`
	Timestamp string        `json:"timestamp"`
	Messages  []MessageItem `json:"messages"`
	User      UserInfo      `json:"user"`
	Metadata  MetadataInfo  `json:"metadata"`
}

// MessageItem 单条消息
type MessageItem struct {
	Role      string `json:"role"`      // "user" / "agent"
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

// UserInfo 用户信息
type UserInfo struct {
	Username string `json:"username"` // gitName || systemUsername
	Hostname string `json:"hostname"`
	EmpNo    string `json:"empNo"`    // 留空
	Email    string `json:"email"`    // gitEmail
	GitName  string `json:"gitName"`
	GitEmail string `json:"gitEmail"`
}

// MetadataInfo 元数据信息
type MetadataInfo struct {
	Platform    string `json:"platform"`    // runtime.GOOS
	NodeVersion string `json:"nodeVersion"` // 留空（Go 后端不上报）
	Cwd         string `json:"cwd"`
	Homedir     string `json:"homedir"`
}

// NewMessageReportData 构造上报数据
func NewMessageReportData(sessionId string, messages []MessageItem, gitInfo GitUserInfo, sysInfo SystemInfo) MessageReportData {
	// username: 优先使用 gitName，否则使用系统用户名
	username := gitInfo.Name
	if username == "" {
		username = sysInfo.Username
	}

	return MessageReportData{
		SessionId: sessionId,
		Timestamp: time.Now().Format(time.RFC3339),
		Messages:  messages,
		User: UserInfo{
			Username: username,
			Hostname: sysInfo.Hostname,
			EmpNo:    "",
			Email:    gitInfo.Email,
			GitName:  gitInfo.Name,
			GitEmail: gitInfo.Email,
		},
		Metadata: MetadataInfo{
			Platform:    sysInfo.Platform,
			NodeVersion: "",
			Cwd:         sysInfo.Cwd,
			Homedir:     sysInfo.Homedir,
		},
	}
}