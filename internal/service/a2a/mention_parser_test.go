package a2a

import (
	"testing"
)

func TestParseA2AMentions(t *testing.T) {
	// 构建测试用的 patterns（包含 CatIDs 支持博弈场景）
	tests := []struct {
		name         string
		text         string
		currentCatID string
		want         []string
	}{
		{
			name:         "single mention at line start",
			text:         "@backend 请实现这个功能",
			currentCatID: "architect",
			want:         []string{"backend_developer"},
		},
		{
			name:         "multiple mentions on same line - only first matches",
			text:         "@backend @architect 请协作完成", // only @backend at line start
			currentCatID: "code_reviewer",
			want:         []string{"backend_developer"},
		},
		{
			name:         "multiple mentions on separate lines",
			text:         "@backend 请实现\n@architect 请设计",
			currentCatID: "code_reviewer",
			want:         []string{"backend_developer", "architect"},
		},
		{
			name:         "max 2 targets",
			text:         "@backend 请实现\n@architect 请设计\n@code_reviewer 第三行", // only first 2
			currentCatID: "sre_engineer",
			want:         []string{"backend_developer", "architect"},
		},
		{
			name:         "filter self mention",
			text:         "@backend 请实现\n@architect 请设计", // backend filtered, architect kept
			currentCatID: "backend_developer",
			want:         []string{"architect"},
		},
		{
			name:         "mention inside code block ignored",
			text:         "```\n@backend\n```\n@architect this one counts",
			currentCatID: "backend_developer",
			want:         []string{"architect"},
		},
		{
			name:         "mention not at line start ignored",
			text:         "hello @backend not at start",
			currentCatID: "architect",
			want:         nil,
		},
		{
			name:         "token boundary check - hyphenated agent",
			text:         "@architect-extended should not match architect",
			currentCatID: "backend_developer",
			want:         nil, // @architect-extended is not a valid pattern
		},
		{
			name:         "token boundary check - valid pattern",
			text:         "@backend, please help", // comma after is valid boundary
			currentCatID: "architect",
			want:         []string{"backend_developer"},
		},
		{
			name:         "empty text",
			text:         "",
			currentCatID: "architect",
			want:         nil,
		},
		{
			name:         "no mentions",
			text:         "plain text without mentions",
			currentCatID: "architect",
			want:         nil,
		},
		{
			name:         "mention in middle of line ignored",
			text:         "some text\n@backend at start\nmore text @architect not at start",
			currentCatID: "code_reviewer",
			want:         []string{"backend_developer"},
		},
		{
			name:         "chinese mention pattern",
			text:         "@后端 请实现这个功能",
			currentCatID: "architect",
			want:         []string{"backend_developer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseA2AMentions(tt.text, tt.currentCatID)
			if len(got) != len(tt.want) {
				t.Errorf("ParseA2AMentions() = %v, want %v", got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("ParseA2AMentions()[%d] = %v, want %v", i, v, tt.want[i])
				}
			}
		})
	}
}