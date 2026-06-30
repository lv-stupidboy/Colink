package tools

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestToolMetadataSchemas(t *testing.T) {
	tools := []interface {
		Name() string
		Description() string
		InputSchema() map[string]interface{}
	}{
		&PostMessageTool{},
		&ThreadContextTool{},
		&TeamListAgentsTool{},
		&MemoryAddTool{},
		&MemorySearchTool{},
	}
	for _, tool := range tools {
		if tool.Name() == "" || tool.Description() == "" {
			t.Fatalf("tool metadata missing: %#v", tool)
		}
		if schema := tool.InputSchema(); schema["type"] != "object" {
			t.Fatalf("%s schema = %#v", tool.Name(), schema)
		}
	}
}

func TestPostMessageThreadContextAndTeamToolsExecute(t *testing.T) {
	restore := stubToolHTTP(t, func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("X-Invocation-ID") != "inv-1" || req.Header.Get("X-Callback-Token") != "token" {
			t.Fatalf("auth headers missing: %#v", req.Header)
		}
		body, _ := io.ReadAll(req.Body)
		switch req.URL.Path {
		case "/api/callbacks/post-message":
			if req.Method != http.MethodPost || !strings.Contains(string(body), "@Reviewer") {
				t.Fatalf("post-message request method=%s body=%s", req.Method, body)
			}
			return jsonToolResponse(http.StatusOK, map[string]interface{}{"triggered": true}), nil
		case "/api/callbacks/thread-context":
			if req.Method != http.MethodGet || !strings.Contains(string(body), `"limit":5`) {
				t.Fatalf("thread-context request method=%s body=%s", req.Method, body)
			}
			return jsonToolResponse(http.StatusOK, map[string]interface{}{"messages": []interface{}{map[string]interface{}{"content": "hi"}}}), nil
		case "/api/callbacks/team/list-agents":
			if req.Method != http.MethodPost || !strings.Contains(string(body), "/repo") {
				t.Fatalf("team list request method=%s body=%s", req.Method, body)
			}
			return jsonToolResponse(http.StatusOK, map[string]interface{}{"agents": []interface{}{map[string]interface{}{"name": "Reviewer"}}}), nil
		default:
			t.Fatalf("unexpected path %s", req.URL.Path)
			return nil, nil
		}
	})
	defer restore()

	postResult, err := (&PostMessageTool{APIURL: "https://colink.test", InvocationID: "inv-1", CallbackToken: "token"}).Execute(map[string]interface{}{"message": "@Reviewer please review"})
	if err != nil || postResult.(map[string]interface{})["triggered"] != true {
		t.Fatalf("post result=%#v err=%v", postResult, err)
	}
	contextResult, err := (&ThreadContextTool{APIURL: "https://colink.test", InvocationID: "inv-1", CallbackToken: "token"}).Execute(map[string]interface{}{"limit": float64(5)})
	if err != nil || len(contextResult.(map[string]interface{})["messages"].([]interface{})) != 1 {
		t.Fatalf("context result=%#v err=%v", contextResult, err)
	}
	teamResult, err := (&TeamListAgentsTool{APIURL: "https://colink.test", InvocationID: "inv-1", CallbackToken: "token"}).Execute(map[string]interface{}{"workspacePath": "/repo"})
	if err != nil || len(teamResult.(map[string]interface{})["agents"].([]interface{})) != 1 {
		t.Fatalf("team result=%#v err=%v", teamResult, err)
	}
}

func TestMemoryToolsExecuteAndValidation(t *testing.T) {
	restore := stubToolHTTP(t, func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		if req.URL.Path != "/api/callbacks/memory" || req.Method != http.MethodPost {
			t.Fatalf("memory request method=%s path=%s", req.Method, req.URL.Path)
		}
		if strings.Contains(string(body), `"action":"add"`) {
			if !strings.Contains(string(body), `"topic":"testing"`) || !strings.Contains(string(body), `"facts":["`) {
				t.Fatalf("memory add body=%s", body)
			}
			return jsonToolResponse(http.StatusOK, map[string]interface{}{"added": true}), nil
		}
		if strings.Contains(string(body), `"action":"search"`) {
			if !strings.Contains(string(body), `"query":"coverage"`) {
				t.Fatalf("memory search body=%s", body)
			}
			return jsonToolResponse(http.StatusOK, map[string]interface{}{"results": []interface{}{"hit"}}), nil
		}
		t.Fatalf("unexpected memory body=%s", body)
		return nil, nil
	})
	defer restore()

	add := &MemoryAddTool{APIURL: "https://colink.test", InvocationID: "inv-1", CallbackToken: "token"}
	if _, err := add.Execute(map[string]interface{}{}); err == nil || !strings.Contains(err.Error(), "content or facts") {
		t.Fatalf("missing memory content error=%v", err)
	}
	addResult, err := add.Execute(map[string]interface{}{
		"facts": []interface{}{"fact"},
		"topic": "testing",
	})
	if err != nil || addResult.(map[string]interface{})["added"] != true {
		t.Fatalf("add result=%#v err=%v", addResult, err)
	}
	searchResult, err := (&MemorySearchTool{APIURL: "https://colink.test", InvocationID: "inv-1", CallbackToken: "token"}).Execute(map[string]interface{}{"query": "coverage"})
	if err != nil || len(searchResult.(map[string]interface{})["results"].([]interface{})) != 1 {
		t.Fatalf("search result=%#v err=%v", searchResult, err)
	}
}

func TestToolsReturnErrorsForBadArgumentsAndResponses(t *testing.T) {
	if _, err := (&PostMessageTool{}).Execute(map[string]interface{}{"message": 123}); err == nil {
		t.Fatalf("PostMessageTool should reject non-string message")
	}

	restore := stubToolHTTP(t, func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("{not json")),
			Header:     make(http.Header),
		}, nil
	})
	defer restore()

	if _, err := (&PostMessageTool{APIURL: "https://colink.test"}).Execute(map[string]interface{}{"message": "hi"}); err == nil || !strings.Contains(err.Error(), "parse response") {
		t.Fatalf("bad json error=%v", err)
	}
}

func stubToolHTTP(t *testing.T, handler func(*http.Request) (*http.Response, error)) func() {
	t.Helper()
	original := http.DefaultTransport
	http.DefaultTransport = toolRoundTripFunc(handler)
	return func() {
		http.DefaultTransport = original
	}
}

type toolRoundTripFunc func(*http.Request) (*http.Response, error)

func (f toolRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonToolResponse(status int, payload map[string]interface{}) *http.Response {
	body, _ := json.Marshal(payload)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Header:     make(http.Header),
	}
}
