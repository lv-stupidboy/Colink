package agent

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// GovernanceDigestConfigFile 配置文件路径（相对于 data 目录）
const GovernanceDigestConfigFile = "configs/governance_digest.yaml"

// GovernanceDigestManager 管理治理摘要的热更新
type GovernanceDigestManager struct {
	mu          sync.RWMutex
	content     string
	version     string
	configPath  string
	lastModTime time.Time
}

// 全局管理器实例
var governanceManager *GovernanceDigestManager

// InitGovernanceDigestManager 初始化治理摘要管理器
// configDir: data/configs 目录路径
func InitGovernanceDigestManager(configDir string) error {
	governanceManager = &GovernanceDigestManager{
		configPath: filepath.Join(configDir, "governance_digest.yaml"),
		version:    GovernanceDigestVersion,
		content:    GovernanceDigest, // 默认使用代码中的默认值
	}

	// 尝试从配置文件加载
	if err := governanceManager.loadFromFile(); err != nil {
		// 配置文件不存在或读取失败，使用默认值
		logInfo("GovernanceDigest: using default value", zap.String("reason", err.Error()))
		return nil // 不返回错误，使用默认值即可
	}

	logInfo("GovernanceDigest: loaded from config file", zap.String("path", governanceManager.configPath))
	return nil
}

// loadFromFile 从配置文件加载内容
func (m *GovernanceDigestManager) loadFromFile() error {
	content, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.content = string(content)
	m.lastModTime = time.Now()
	m.mu.Unlock()

	return nil
}

// GetContent 获取当前治理摘要内容
func GetGovernanceDigestContent() string {
	if governanceManager == nil {
		return GovernanceDigest // 未初始化时使用默认值
	}

	governanceManager.mu.RLock()
	defer governanceManager.mu.RUnlock()

	return governanceManager.content
}

// GetVersion 获取当前版本
func GetGovernanceDigestVersion() string {
	if governanceManager == nil {
		return GovernanceDigestVersion
	}

	governanceManager.mu.RLock()
	defer governanceManager.mu.RUnlock()

	return governanceManager.version
}

// UpdateContent 更新治理摘要内容（热更新）
// 同时保存到配置文件
func UpdateGovernanceDigestContent(newContent string) error {
	if governanceManager == nil {
		return nil // 未初始化时不保存
	}

	// 验证 Token 数量
	tokens := EstimateTokens(newContent)
	if tokens > 500 {
		return &GovernanceDigestError{Message: "content exceeds 500 tokens limit", Tokens: tokens}
	}

	// 更新内存中的内容
	governanceManager.mu.Lock()
	governanceManager.content = newContent
	governanceManager.lastModTime = time.Now()
	governanceManager.mu.Unlock()

	// 保存到配置文件
	if err := governanceManager.saveToFile(); err != nil {
		logError("GovernanceDigest: failed to save to file", zap.Error(err))
		return err
	}

	logInfo("GovernanceDigest: updated successfully", zap.Int("tokens", tokens))
	return nil
}

// saveToFile 保存内容到配置文件
func (m *GovernanceDigestManager) saveToFile() error {
	// 确保目录存在
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 写入文件
	return os.WriteFile(m.configPath, []byte(m.content), 0644)
}

// GetGovernanceDigestStatus 获取治理摘要状态信息
func GetGovernanceDigestStatus() map[string]interface{} {
	if governanceManager == nil {
		return map[string]interface{}{
			"initialized": false,
			"version":     GovernanceDigestVersion,
			"source":      "default",
			"tokens":      EstimateTokens(GovernanceDigest),
		}
	}

	governanceManager.mu.RLock()
	defer governanceManager.mu.RUnlock()

	return map[string]interface{}{
		"initialized":    true,
		"version":        governanceManager.version,
		"source":         "config_file",
		"configPath":     governanceManager.configPath,
		"lastModTime":    governanceManager.lastModTime.Format(time.RFC3339),
		"tokens":         EstimateTokens(governanceManager.content),
		"contentLength":  len(governanceManager.content),
	}
}

// GovernanceDigestError 治理摘要错误
type GovernanceDigestError struct {
	Message string
	Tokens  int
}

func (e *GovernanceDigestError) Error() string {
	return e.Message
}