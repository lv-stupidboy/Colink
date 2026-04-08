// 文件路径: isdp/internal/service/command/service_test.go
package command

import (
	"testing"
)

func TestIsValidName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// 有效名称
		{"valid simple name", "my-command", true},
		{"valid name with numbers", "command-123", true},
		{"valid single letter", "a", true},
		{"valid name with multiple hyphens", "my-long-command-name", true},
		{"valid name ending with number", "command1", true},
		{"valid name with numbers in middle", "cmd123test", true},

		// 无效名称
		{"empty string", "", false},
		{"starts with number", "123-command", false},
		{"starts with hyphen", "-command", false},
		{"contains uppercase", "MyCommand", false},
		{"contains underscore", "my_command", false},
		{"contains space", "my command", false},
		{"contains special char", "my@command", false},
		{"contains dot", "my.command", false},
		{"uppercase only", "COMMAND", false},
		{"camelCase", "myCommand", false},
		{"PascalCase", "MyCommand", false},
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