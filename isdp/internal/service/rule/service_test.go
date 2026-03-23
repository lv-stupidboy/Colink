// 文件路径: isdp/internal/service/rule/service_test.go
package rule

import (
	"testing"

	"github.com/anthropic/isdp/internal/model"
)

func TestIsValidName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// 有效名称
		{"valid simple name", "my-rule", true},
		{"valid name with numbers", "rule-123", true},
		{"valid single letter", "a", true},
		{"valid name with multiple hyphens", "my-long-rule-name", true},
		{"valid name ending with number", "rule1", true},
		{"valid name with numbers in middle", "rule123test", true},

		// 无效名称
		{"empty string", "", false},
		{"starts with number", "123-rule", false},
		{"starts with hyphen", "-rule", false},
		{"contains uppercase", "MyRule", false},
		{"contains underscore", "my_rule", false},
		{"contains space", "my rule", false},
		{"contains special char", "my@rule", false},
		{"contains dot", "my.rule", false},
		{"uppercase only", "RULE", false},
		{"camelCase", "myRule", false},
		{"PascalCase", "MyRule", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidName(tt.input)
			if result != tt.expected {
				t.Errorf("isValidName(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRuleScope(t *testing.T) {
	tests := []struct {
		name     string
		scope    model.RuleScope
		expected string
	}{
		{"public scope", model.RuleScopePublic, "public"},
		{"instance scope", model.RuleScopeInstance, "instance"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.scope) != tt.expected {
				t.Errorf("RuleScope %s = %q, expected %q", tt.name, tt.scope, tt.expected)
			}
		})
	}
}

func TestRuleScopeValues(t *testing.T) {
	// 验证 scope 常量值符合预期
	if model.RuleScopePublic != "public" {
		t.Errorf("RuleScopePublic = %q, expected %q", model.RuleScopePublic, "public")
	}
	if model.RuleScopeInstance != "instance" {
		t.Errorf("RuleScopeInstance = %q, expected %q", model.RuleScopeInstance, "instance")
	}
}