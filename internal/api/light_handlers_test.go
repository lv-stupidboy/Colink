package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	mergeservice "github.com/anthropic/isdp/internal/service/merge"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestArtifactHandlerCreateListGetAndRejectInvalidRequests(t *testing.T) {
	db := openAPILightTestDB(t)
	router := setupAPILightRouter(func(group *gin.RouterGroup) {
		NewArtifactHandler(repo.NewArtifactRepository(db, repo.DBTypeSQLite)).RegisterRoutes(group)
	})
	threadID := uuid.New()

	createW := performAPILightJSON(router, http.MethodPost, "/api/v1/threads/"+threadID.String()+"/artifacts", map[string]any{
		"type":    "review",
		"name":    "Review Notes",
		"path":    "reviews/notes.md",
		"content": "P1: fix auth",
		"metadata": map[string]any{
			"reviewer": "qa",
		},
	})
	if createW.Code != http.StatusCreated {
		t.Fatalf("Create code=%d body=%s", createW.Code, createW.Body.String())
	}
	var created model.Artifact
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created artifact: %v", err)
	}
	if created.ThreadID != threadID || created.Type != model.ArtifactTypeReview || created.Metadata["reviewer"] != "qa" {
		t.Fatalf("created artifact = %#v", created)
	}

	listW := performAPILightJSON(router, http.MethodGet, "/api/v1/threads/"+threadID.String()+"/artifacts", nil)
	if listW.Code != http.StatusOK || !bytes.Contains(listW.Body.Bytes(), []byte(created.ID.String())) {
		t.Fatalf("List code=%d body=%s", listW.Code, listW.Body.String())
	}
	getW := performAPILightJSON(router, http.MethodGet, "/api/v1/artifacts/"+created.ID.String(), nil)
	if getW.Code != http.StatusOK || !bytes.Contains(getW.Body.Bytes(), []byte("P1: fix auth")) {
		t.Fatalf("Get code=%d body=%s", getW.Code, getW.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/threads/not-a-uuid/artifacts", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid list code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/threads/not-a-uuid/artifacts", map[string]any{"type": "review"}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid create thread code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/threads/"+threadID.String()+"/artifacts", map[string]any{"name": "Missing type"}); w.Code != http.StatusBadRequest {
		t.Fatalf("missing type create code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/artifacts/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid get code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/artifacts/"+uuid.New().String(), nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing get code=%d", w.Code)
	}
}

func TestMergeHandlerCheckApproveAndHandover(t *testing.T) {
	db := openAPILightTestDB(t)
	artifactRepo := repo.NewArtifactRepository(db, repo.DBTypeSQLite)
	gatekeeper := mergeservice.NewGatekeeper(
		repo.NewReviewRepository(artifactRepo),
		artifactRepo,
		repo.NewThreadRepository(db, repo.DBTypeSQLite),
	)
	router := setupAPILightRouter(func(group *gin.RouterGroup) {
		NewMergeHandler(gatekeeper).RegisterRoutes(group)
	})
	threadID := uuid.New()
	insertAPILightArtifact(t, db, threadID, model.ArtifactTypeReview, "Review", "P1: dangerous change\nP2: missing test")

	checkW := performAPILightJSON(router, http.MethodGet, "/api/v1/threads/"+threadID.String()+"/merge/check", nil)
	if checkW.Code != http.StatusOK || !bytes.Contains(checkW.Body.Bytes(), []byte(`"decision":"block"`)) {
		t.Fatalf("Check code=%d body=%s", checkW.Code, checkW.Body.String())
	}
	approveW := performAPILightJSON(router, http.MethodPost, "/api/v1/threads/"+threadID.String()+"/merge/approve", nil)
	if approveW.Code != http.StatusOK || !bytes.Contains(approveW.Body.Bytes(), []byte(`"status":"approved"`)) {
		t.Fatalf("Approve code=%d body=%s", approveW.Code, approveW.Body.String())
	}
	handoverW := performAPILightJSON(router, http.MethodGet, "/api/v1/threads/"+threadID.String()+"/merge/handover", nil)
	if handoverW.Code != http.StatusOK || !bytes.Contains(handoverW.Body.Bytes(), []byte(`"status":"handover"`)) {
		t.Fatalf("Handover code=%d body=%s", handoverW.Code, handoverW.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/threads/not-a-uuid/merge/check", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid check code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/threads/not-a-uuid/merge/approve", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid approve code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/threads/not-a-uuid/merge/handover", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid handover code=%d", w.Code)
	}
}

func TestRuntimeConfigHandlerGet(t *testing.T) {
	router := setupAPILightRouter(func(group *gin.RouterGroup) {
		NewRuntimeConfigHandler(&config.Config{
			Deployment: config.DeploymentConfig{
				Type:          config.DeploymentTypeLinux,
				WorkspacePath: "/tmp/colink-workspace",
			},
		}).RegisterRoutes(group)
	})

	w := performAPILightJSON(router, http.MethodGet, "/api/v1/runtime/config", nil)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"deploymentType":"linux"`)) || !bytes.Contains(w.Body.Bytes(), []byte(`/tmp/colink-workspace`)) {
		t.Fatalf("Runtime config code=%d body=%s", w.Code, w.Body.String())
	}
}

func setupAPILightRouter(register func(*gin.RouterGroup)) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	register(router.Group("/api/v1"))
	return router
}

func performAPILightJSON(router *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, _ := json.Marshal(body)
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func openAPILightTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE artifacts (id TEXT PRIMARY KEY, thread_id TEXT, type TEXT, name TEXT, path TEXT, content TEXT, metadata BLOB, created_at TIMESTAMP)`)
	if err != nil {
		t.Fatalf("create artifacts table: %v", err)
	}
	return db
}

func insertAPILightArtifact(t *testing.T, db *sql.DB, threadID uuid.UUID, artifactType model.ArtifactType, name string, content string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO artifacts (id, thread_id, type, name, path, content, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), threadID.String(), artifactType, name, "", content, []byte(`{}`), time.Now())
	if err != nil {
		t.Fatalf("insert artifact: %v", err)
	}
}
