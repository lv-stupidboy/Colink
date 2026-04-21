package agent

import (
	"strings"
	"testing"

	"github.com/anthropic/isdp/internal/model"
)

func TestBuildGovernanceDigest(t *testing.T) {
	digest := BuildGovernanceDigest()

	// 验证非空
	if digest == "" {
		t.Error("GovernanceDigest should not be empty")
	}

	// 验证包含关键内容
	keyPhrases := []string{
		"协作守则",
		"出口检查",
		"@mention",
		"落盘记录",
		"阻塞流程",
		"工作成果",
	}

	for _, phrase := range keyPhrases {
		if !strings.Contains(digest, phrase) {
			t.Errorf("GovernanceDigest should contain '%s'", phrase)
		}
	}
}

func TestBuildGovernanceDigestWithVersion(t *testing.T) {
	digest := BuildGovernanceDigestWithVersion()

	// 验证包含版本标记
	if !strings.Contains(digest, "GOVERNANCE_DIGEST_VERSION") {
		t.Error("Digest with version should contain version marker")
	}

	if !strings.Contains(digest, GovernanceDigestVersion) {
		t.Errorf("Digest should contain version '%s'", GovernanceDigestVersion)
	}
}

func TestGovernanceDigestTokens(t *testing.T) {
	tokens := GovernanceDigestTokens()

	// 验证 Token 数在约束范围内（≤ 200）
	if tokens > 200 {
		t.Errorf("GovernanceDigest tokens (%d) exceeds limit (200)", tokens)
	}

	t.Logf("GovernanceDigest estimated tokens: %d", tokens)
}

func TestValidateGovernanceDigest(t *testing.T) {
	if !ValidateGovernanceDigest() {
		t.Error("GovernanceDigest should pass validation")
	}
}

func TestBuildStaticLayer0WithGovernance(t *testing.T) {
	config := &model.AgentConfig{
		Name:         "测试Agent",
		Description:  "测试描述",
		SystemPrompt: "测试系统提示",
	}

	layer0 := BuildStaticLayer0(config)

	// 验证包含角色定义
	if !strings.Contains(layer0, "测试Agent") {
		t.Error("Layer0 should contain agent name")
	}

	// 验证包含治理摘要
	if !strings.Contains(layer0, "协作守则") {
		t.Error("Layer0 should contain governance digest")
	}

	// 验证包含版本标记
	if !strings.Contains(layer0, "GOVERNANCE_DIGEST_VERSION") {
		t.Error("Layer0 should contain governance version marker")
	}
}

func TestBuildStaticLayer0Minimal(t *testing.T) {
	config := &model.AgentConfig{
		Name:         "测试Agent",
		Description:  "测试描述",
		SystemPrompt: "测试系统提示",
	}

	layer0Minimal := BuildStaticLayer0Minimal(config)

	// 验证包含角色定义
	if !strings.Contains(layer0Minimal, "测试Agent") {
		t.Error("Layer0Minimal should contain agent name")
	}

	// 验证不包含治理摘要（最小版本）
	if strings.Contains(layer0Minimal, "协作守则") {
		t.Error("Layer0Minimal should NOT contain governance digest")
	}
}