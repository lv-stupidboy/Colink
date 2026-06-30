package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestKnowledgeServiceCRUDAndValidation(t *testing.T) {
	ctx := context.Background()
	db := openKnowledgeTestDB(t)
	service := NewService(repo.NewKnowledgeBaseRepository(db, repo.DBTypeSQLite))

	created, err := service.Create(ctx, &model.CreateKnowledgeBaseRequest{
		Name:          "code",
		DisplayName:   "Code KB",
		Description:   "code search",
		Type:          model.KnowledgeBaseTypeAPI,
		Config:        map[string]string{"token": "secret"},
		QueryEndpoint: "http://example.invalid/query",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID == uuid.Nil || created.Status != model.KnowledgeBaseStatusActive || created.QueryCount != 0 {
		t.Fatalf("created knowledge base = %#v", created)
	}
	if _, err := service.Create(ctx, &model.CreateKnowledgeBaseRequest{Name: "code", Type: model.KnowledgeBaseTypeAPI}); err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("duplicate create error = %v", err)
	}
	if got, err := service.GetByID(ctx, created.ID); err != nil || got.Name != "code" {
		t.Fatalf("GetByID = %#v err=%v", got, err)
	}
	if got, err := service.GetByName(ctx, "code"); err != nil || got.ID != created.ID {
		t.Fatalf("GetByName = %#v err=%v", got, err)
	}

	updated, err := service.Update(ctx, created.ID, &model.UpdateKnowledgeBaseRequest{
		DisplayName:   "Updated KB",
		Description:   "updated",
		Config:        map[string]string{"token": "new"},
		QueryEndpoint: "http://example.invalid/new",
		Status:        model.KnowledgeBaseStatusInactive,
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.DisplayName != "Updated KB" || updated.Config["token"] != "new" || updated.Status != model.KnowledgeBaseStatusInactive {
		t.Fatalf("updated knowledge base = %#v", updated)
	}
	list, total, err := service.List(ctx, &model.KnowledgeBaseListQuery{Type: string(model.KnowledgeBaseTypeAPI), Status: string(model.KnowledgeBaseStatusInactive), Search: "Updated", Page: -1, Size: 200})
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("List = %#v total=%d err=%v", list, total, err)
	}
	if err := service.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := service.GetByID(ctx, created.ID); err == nil {
		t.Fatalf("deleted knowledge base should not be found")
	}
}

func TestKnowledgeServiceQueryAPIAndMCP(t *testing.T) {
	ctx := context.Background()
	db := openKnowledgeTestDB(t)
	service := NewService(repo.NewKnowledgeBaseRepository(db, repo.DBTypeSQLite))

	restoreTransport := stubKnowledgeHTTP(t, func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Authorization"); got != "Bearer api-token" {
			t.Fatalf("Authorization = %q", got)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode api request: %v", err)
		}
		if req["query"] != "panic" || req["limit"].(float64) != 2 {
			t.Fatalf("api request = %#v", req)
		}
		return jsonKnowledgeResponse(http.StatusOK, map[string]any{
			"total": 1,
			"results": []map[string]any{{
				"title": "Runbook", "content": "restart service", "source": "api", "relevance": 0.9,
			}},
		}), nil
	})
	defer restoreTransport()
	apiKB := insertKnowledgeBase(t, db, "api", model.KnowledgeBaseTypeAPI, model.KnowledgeBaseStatusActive, "https://kb.test/api", map[string]string{"token": "api-token"})

	result, err := service.Query(ctx, apiKB, &model.KnowledgeQueryRequest{Query: "panic", Limit: 2})
	if err != nil || result.Total != 1 || result.Results[0].Title != "Runbook" || result.Source != "api" {
		t.Fatalf("API Query = %#v err=%v", result, err)
	}
	refreshed, err := service.GetByID(ctx, apiKB)
	if err != nil || refreshed.QueryCount != 1 || refreshed.LastQueryAt == nil {
		t.Fatalf("query stats = %#v err=%v", refreshed, err)
	}

	restoreTransport()
	restoreTransport = stubKnowledgeHTTP(t, func(r *http.Request) (*http.Response, error) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode mcp request: %v", err)
		}
		params := req["params"].(map[string]any)
		args := params["arguments"].(map[string]any)
		if params["name"] != "query" || args["query"] != "panic" || args["limit"].(float64) != 1 {
			t.Fatalf("mcp request = %#v", req)
		}
		return jsonKnowledgeResponse(http.StatusOK, map[string]any{
			"result": map[string]any{
				"content": []map[string]any{{
					"type": "text",
					"text": `[{"title":"Doc","content":"read logs","source":"mcp","relevance":0.8}]`,
				}},
			},
		}), nil
	})
	mcpKB := insertKnowledgeBase(t, db, "mcp", model.KnowledgeBaseTypeMCP, model.KnowledgeBaseStatusActive, "", map[string]string{"endpoint": "https://kb.test/mcp"})

	mcpResult, err := service.Query(ctx, mcpKB, &model.KnowledgeQueryRequest{Query: "panic", Limit: 1})
	if err != nil || mcpResult.Total != 1 || mcpResult.Results[0].Title != "Doc" || mcpResult.Source != "mcp" {
		t.Fatalf("MCP Query = %#v err=%v", mcpResult, err)
	}
}

func TestKnowledgeServiceQueryErrorsAndQueryAll(t *testing.T) {
	ctx := context.Background()
	db := openKnowledgeTestDB(t)
	service := NewService(repo.NewKnowledgeBaseRepository(db, repo.DBTypeSQLite))

	inactive := insertKnowledgeBase(t, db, "inactive", model.KnowledgeBaseTypeAPI, model.KnowledgeBaseStatusInactive, "http://example.invalid", nil)
	if _, err := service.Query(ctx, inactive, &model.KnowledgeQueryRequest{Query: "q"}); err == nil || !strings.Contains(err.Error(), "未激活") {
		t.Fatalf("inactive query error = %v", err)
	}
	gitKB := insertKnowledgeBase(t, db, "git", model.KnowledgeBaseTypeGit, model.KnowledgeBaseStatusActive, "", nil)
	if _, err := service.Query(ctx, gitKB, &model.KnowledgeQueryRequest{Query: "q"}); err == nil || !strings.Contains(err.Error(), "尚未实现") {
		t.Fatalf("git query error = %v", err)
	}

	restoreTransport := stubKnowledgeHTTP(t, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Host, "bad") {
			return jsonKnowledgeResponse(http.StatusBadGateway, map[string]any{"error": "boom"}), nil
		}
		return jsonKnowledgeResponse(http.StatusOK, map[string]any{"total": 0, "results": []any{}}), nil
	})
	defer restoreTransport()
	insertKnowledgeBase(t, db, "ok-api", model.KnowledgeBaseTypeAPI, model.KnowledgeBaseStatusActive, "https://ok.test/api", nil)
	insertKnowledgeBase(t, db, "bad-api", model.KnowledgeBaseTypeAPI, model.KnowledgeBaseStatusActive, "https://bad.test/api", nil)

	results, err := service.QueryAll(ctx, &model.KnowledgeQueryRequest{Query: "q"})
	if err != nil || len(results) != 3 {
		t.Fatalf("QueryAll = %#v err=%v", results, err)
	}
	var sawError bool
	for _, result := range results {
		if result.Source == "bad-api" && result.Error != "" {
			sawError = true
		}
	}
	if !sawError {
		t.Fatalf("QueryAll should include bad-api error result: %#v", results)
	}
}

func openKnowledgeTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE knowledge_bases (id TEXT PRIMARY KEY, name TEXT, display_name TEXT, description TEXT, type TEXT, config BLOB, query_endpoint TEXT, status TEXT, last_query_at TIMESTAMP, query_count INTEGER DEFAULT 0, created_at TIMESTAMP, updated_at TIMESTAMP)`)
	if err != nil {
		t.Fatalf("exec schema: %v", err)
	}
	return db
}

func insertKnowledgeBase(t *testing.T, db *sql.DB, name string, kbType model.KnowledgeBaseType, status model.KnowledgeBaseStatus, endpoint string, config map[string]string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	configBytes, _ := json.Marshal(config)
	_, err := db.Exec(`INSERT INTO knowledge_bases (id, name, display_name, description, type, config, query_endpoint, status, query_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, name, name+" desc", kbType, configBytes, endpoint, status, 0, now, now)
	if err != nil {
		t.Fatalf("insert knowledge base: %v", err)
	}
	return id
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func stubKnowledgeHTTP(t *testing.T, handler func(*http.Request) (*http.Response, error)) func() {
	t.Helper()
	original := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(handler)
	return func() {
		http.DefaultTransport = original
	}
}

func jsonKnowledgeResponse(status int, payload map[string]any) *http.Response {
	body, _ := json.Marshal(payload)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Header:     make(http.Header),
	}
}
