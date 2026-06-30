package agent

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestBuildPromptFromRequestAndLayers(t *testing.T) {
	if got := BuildPromptFromRequest(nil); got != "" {
		t.Fatalf("BuildPromptFromRequest(nil) = %q", got)
	}

	req := &ExecutionRequest{
		Input: "请实现功能",
		Context: &ContextLayers{
			Layer0:        "你是评审 Agent",
			Layer1:        "历史不应注入",
			Layer2:        "report.md",
			Layer3:        "Thread ID: 1",
			ChainHistory:  "上游输出",
			MemoryContext: "团队偏好",
		},
	}
	got := BuildPromptFromRequest(req)
	for _, want := range []string{
		"<system>\n你是评审 Agent\n</system>",
		"<a2a-context>\n上游输出\n</a2a-context>",
		"<artifacts>\nreport.md\n</artifacts>",
		"<environment>\nThread ID: 1\n</environment>",
		"<memory>\n团队偏好\n</memory>",
		"<user>\n请实现功能\n</user>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "历史不应注入") {
		t.Fatalf("Layer1 should not be injected:\n%s", got)
	}
}

func TestBuildPromptUsesA2AInputTag(t *testing.T) {
	cases := []string{
		"## 协作规则\n请处理下游任务",
		"Direct message from reviewer",
	}
	for _, input := range cases {
		got := BuildPrompt(nil, input)
		if !strings.Contains(got, "<a2a_input>\n"+input+"\n</a2a_input>") {
			t.Fatalf("BuildPrompt(%q) = %q", input, got)
		}
	}

	got := BuildPrompt(&ContextLayers{Layer0: "system"}, "")
	if !strings.Contains(got, "<user>\n\n</user>") {
		t.Fatalf("empty input should still be wrapped as user input: %q", got)
	}
}

func TestExecutionServicePromptFormattingFacade(t *testing.T) {
	es := NewExecutionService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, nil)
	got := es.formatFullPrompt(&ContextLayers{Layer0: "system"}, "hello")
	if !strings.Contains(got, "<system>\nsystem\n</system>") || !strings.Contains(got, "<user>\nhello\n</user>") {
		t.Fatalf("formatFullPrompt = %q", got)
	}
	es.broadcastFullPrompt(uuidMustParseForPromptTest("00000000-0000-0000-0000-000000000001"), uuidMustParseForPromptTest("00000000-0000-0000-0000-000000000002"), got)
}

func uuidMustParseForPromptTest(value string) uuid.UUID {
	id, err := uuid.Parse(value)
	if err != nil {
		panic(err)
	}
	return id
}
