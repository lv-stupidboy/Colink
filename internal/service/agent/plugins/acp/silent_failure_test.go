package acp

import (
	"strings"
	"testing"
)

func TestDetectSilentFailure(t *testing.T) {
	cases := []struct {
		name       string
		output     string
		stderr     string
		stopReason string
		wantErr    bool
	}{
		{
			name:       "normal end_turn with output",
			output:     "some response text",
			stderr:     "",
			stopReason: "end_turn",
			wantErr:    false,
		},
		{
			name:       "empty output with end_turn — auth failure signature",
			output:     "",
			stderr:     "",
			stopReason: "end_turn",
			wantErr:    true,
		},
		{
			name:       "empty output with empty stopReason",
			output:     "",
			stderr:     "",
			stopReason: "",
			wantErr:    true,
		},
		{
			name:       "cancelled with empty output — user cancel",
			output:     "",
			stderr:     "",
			stopReason: "cancelled",
			wantErr:    true,
		},
		{
			name:       "refusal with empty output — model refusal",
			output:     "",
			stderr:     "",
			stopReason: "refusal",
			wantErr:    true,
		},
		{
			name:       "max_tokens with empty output — no silent failure (unusual stop)",
			output:     "",
			stderr:     "",
			stopReason: "max_tokens",
			wantErr:    false,
		},
		{
			name:       "empty output but stderr has content — CLI reported error path handles it",
			output:     "",
			stderr:     "some CLI error occurred",
			stopReason: "end_turn",
			wantErr:    false,
		},
		{
			name:       "non-empty output with cancelled — user cancelled after partial output",
			output:     "partial",
			stderr:     "",
			stopReason: "cancelled",
			wantErr:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := detectSilentFailure(tc.output, tc.stderr, tc.stopReason)
			gotErr := err != nil
			if gotErr != tc.wantErr {
				t.Fatalf("detectSilentFailure(%q, %q, %q) err=%v, wantErr=%v",
					tc.output, tc.stderr, tc.stopReason, err, tc.wantErr)
			}
			if tc.wantErr && err != nil {
				// 消息应包含关键 apiKey 提示 —— 用户能直接看到根因线索
				if !strings.Contains(err.Error(), "apiKey") {
					t.Errorf("error message should hint at apiKey issue, got: %v", err)
				}
				// 应包含 stopReason 信息以便定位
				if tc.stopReason != "" && !strings.Contains(err.Error(), tc.stopReason) {
					t.Errorf("error should include stopReason=%q, got: %v", tc.stopReason, err)
				}
			}
		})
	}
}
