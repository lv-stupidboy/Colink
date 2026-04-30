// auto-test/internal/setup_test.go
package internal_test

import (
	"os"
	"testing"
)

// TestMain 初始化测试环境
func TestMain(m *testing.M) {
	// 设置测试环境变量
	os.Setenv("ISDP_TEST_MODE", "true")

	// 运行测试
	code := m.Run()

	// 清理
	os.Unsetenv("ISDP_TEST_MODE")

	os.Exit(code)
}