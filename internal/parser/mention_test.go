package parser

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseA2AMentions(t *testing.T) {
	patterns := []MentionPattern{
		{Pattern: "@dev", AgentID: "dev"},
		{Pattern: "@developer", AgentID: "developer"},
		{Pattern: "@review", AgentIDs: []string{"qa", "lead"}},
		{Pattern: "@self", AgentID: "self"},
		{Pattern: "@ops", AgentID: "ops"},
	}

	tests := []struct {
		name    string
		text    string
		current string
		want    []string
	}{
		{name: "empty", text: "", want: []string{}},
		{name: "line start only", text: "hello @dev", want: []string{}},
		{name: "leading whitespace", text: "  @dev please help", want: []string{"dev"}},
		{name: "longest match wins", text: "@developer please help", want: []string{"developer"}},
		{name: "token boundary blocks partial handle", text: "@developerx no match", want: []string{}},
		{name: "multi agent pattern", text: "@review please check", want: []string{"qa", "lead"}},
		{name: "filters self", text: "@self do it", current: "self", want: []string{}},
		{name: "strips code block", text: "```\n@dev hidden\n```\n@ops visible", want: []string{"ops"}},
		{name: "limits max targets", text: "@review first\n@dev second\n@ops third", want: []string{"qa", "lead"}},
		{name: "chinese boundary", text: "@dev，请处理", want: []string{"dev"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseA2AMentions(tt.text, tt.current, append([]MentionPattern{}, patterns...))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseA2AMentions=%v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseA2AMentionsMultiDedupesPatterns(t *testing.T) {
	got := ParseA2AMentionsMulti("@dev one\n@dev two", "", []MentionPattern{
		{Pattern: "@dev", AgentIDs: []string{"a", "b"}},
	})
	want := [][]string{{"a", "b"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseA2AMentionsMulti=%v, want %v", got, want)
	}
}

func TestParseHandoffTo(t *testing.T) {
	patterns := []MentionPattern{
		{Pattern: "@dev", AgentID: "dev"},
		{Pattern: "@review", AgentIDs: []string{"qa", "lead"}},
		{Pattern: "@self", AgentID: "self"},
	}

	tests := []struct {
		name    string
		text    string
		current string
		want    []string
	}{
		{
			name: "recovers inline mention via To field",
			text: "我已完成，确认后将交给 @dev 继续\n<a2a-handoff>\n### To\n@dev\n### Goal\n目标内容\n</a2a-handoff>",
			want: []string{"dev"},
		},
		{
			name: "multiple handles in To field",
			text: "<a2a-handoff>\n### To\n@dev @review\n### Goal\n目标\n</a2a-handoff>",
			want: []string{"dev", "qa"},
		},
		{
			name: "stops at next field boundary",
			text: "<a2a-handoff>\n### To\n@dev\n### Goal\n这里有 @review 不应被当作路由\n</a2a-handoff>",
			want: []string{"dev"},
		},
		{name: "no To field returns nil", text: "<a2a-handoff>\n### Goal\n@dev\n</a2a-handoff>", want: []string{}},
		{name: "filters self", text: "### To\n@self", current: "self", want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseHandoffTo(tt.text, tt.current, append([]MentionPattern{}, patterns...))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseHandoffTo=%v, want %v", got, tt.want)
			}
		})
	}
}

func TestMentionHelpers(t *testing.T) {
	if got := countLeadingWhitespace(" \t hi"); got != 3 {
		t.Fatalf("countLeadingWhitespace=%d", got)
	}
	for _, text := range []string{"@user help", "hello @用户", "@CO-CREATOR", "@铲屎官"} {
		if !DetectUserMention(text) {
			t.Fatalf("expected user mention in %q", text)
		}
	}
	if DetectUserMention("@developer") {
		t.Fatal("developer mention should not be treated as user mention")
	}
	stripped := StripCodeBlocks("before\n```go\n@dev\n```\nafter")
	if strings.Contains(stripped, "@dev") || !strings.Contains(stripped, "before") || !strings.Contains(stripped, "after") {
		t.Fatalf("unexpected stripped text: %q", stripped)
	}
}
