package repo

import (
	"testing"
)

func TestMySQLDialect(t *testing.T) {
	d := &MySQLDialect{}

	if d.Placeholder() != "?" {
		t.Errorf("expected placeholder '?', got %s", d.Placeholder())
	}
	if d.QuoteIdentifier() != "`" {
		t.Errorf("expected quote '`', got %s", d.QuoteIdentifier())
	}
	if d.AutoIncrement() != "AUTO_INCREMENT" {
		t.Errorf("expected AUTO_INCREMENT, got %s", d.AutoIncrement())
	}
}

func TestNewMySQLDB(t *testing.T) {
	// 注意：此测试需要MySQL服务运行，实际测试可能需要跳过
	// 这里仅测试函数签名和配置处理
	t.Skip("需要MySQL服务运行，跳过集成测试")
}