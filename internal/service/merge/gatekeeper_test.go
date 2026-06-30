package merge

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestGatekeeperCheckMergeDecisions(t *testing.T) {
	ctx := context.Background()
	db := openMergeTestDB(t)
	gatekeeper := newMergeTestGatekeeper(db)
	threadID := uuid.New()

	insertMergeArtifact(t, db, threadID, model.ArtifactTypeReview, "Review 1", "P1: data loss\nP2: simplify\nP3: naming")
	result, err := gatekeeper.CheckMerge(ctx, threadID)
	if err != nil {
		t.Fatalf("CheckMerge returned error: %v", err)
	}
	if result.Decision != DecisionBlock || result.P1Issues != 1 || len(result.Unresolved) != 1 {
		t.Fatalf("block result = %#v", result)
	}

	conditionalThread := uuid.New()
	insertMergeArtifact(t, db, conditionalThread, model.ArtifactTypeReview, "Review 2", "P2: a\nP2：b\nP2: c\nP2: d")
	result, err = gatekeeper.CheckMerge(ctx, conditionalThread)
	if err != nil {
		t.Fatalf("CheckMerge conditional returned error: %v", err)
	}
	if result.Decision != DecisionConditional || result.P2Issues != 4 {
		t.Fatalf("conditional result = %#v", result)
	}

	cleanThread := uuid.New()
	insertMergeArtifact(t, db, cleanThread, model.ArtifactTypeReview, "Review 3", "P3: optional")
	result, err = gatekeeper.CheckMerge(ctx, cleanThread)
	if err != nil {
		t.Fatalf("CheckMerge allow returned error: %v", err)
	}
	if result.Decision != DecisionAllow || result.P3Issues != 1 {
		t.Fatalf("allow result = %#v", result)
	}
}

func TestGatekeeperRecordReviewAndHandoverReport(t *testing.T) {
	ctx := context.Background()
	db := openMergeTestDB(t)
	gatekeeper := newMergeTestGatekeeper(db)
	threadID := insertMergeThread(t, db)

	artifact, err := gatekeeper.RecordReview(ctx, &ReviewRequest{
		ThreadID: threadID,
		Reviewer: "reviewer",
		Content:  "P1: fix auth\nP2: add test",
		Metadata: map[string]interface{}{"source": "unit"},
	})
	if err != nil {
		t.Fatalf("RecordReview returned error: %v", err)
	}
	if artifact.Type != model.ArtifactTypeReview || artifact.Name != "Review by reviewer" {
		t.Fatalf("review artifact = %#v", artifact)
	}
	insertMergeArtifact(t, db, threadID, model.ArtifactTypeCode, "Patch", "diff")

	report, err := gatekeeper.GenerateHandoverReport(ctx, threadID)
	if err != nil {
		t.Fatalf("GenerateHandoverReport returned error: %v", err)
	}
	if report.ThreadID != threadID || report.Phase != model.PhaseDevelopment || report.Status != model.ThreadStatusRunning {
		t.Fatalf("report thread summary = %#v", report)
	}
	if report.TotalIssues != 2 || report.P1Count != 1 || report.P2Count != 1 || len(report.Artifacts) != 2 {
		t.Fatalf("report issue/artifact summary = %#v", report)
	}
	if err := gatekeeper.ResolveIssue(ctx, threadID, "issue-1", "fixed"); err != nil {
		t.Fatalf("ResolveIssue returned error: %v", err)
	}
}

func newMergeTestGatekeeper(db *sql.DB) *Gatekeeper {
	artifactRepo := repo.NewArtifactRepository(db, repo.DBTypeSQLite)
	return NewGatekeeper(repo.NewReviewRepository(artifactRepo), artifactRepo, repo.NewThreadRepository(db, repo.DBTypeSQLite))
}

func openMergeTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema := []string{
		`CREATE TABLE artifacts (id TEXT PRIMARY KEY, thread_id TEXT, type TEXT, name TEXT, path TEXT, content TEXT, metadata BLOB, created_at TIMESTAMP)`,
		`CREATE TABLE threads (id TEXT PRIMARY KEY, project_id TEXT, name TEXT, status TEXT, current_phase TEXT, current_agent TEXT, depth INTEGER, workflow_template_id TEXT, abort_token TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}
	return db
}

func insertMergeThread(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	_, err := db.Exec(`INSERT INTO threads (id, project_id, name, status, current_phase, current_agent, depth, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), uuid.New().String(), "Thread", model.ThreadStatusRunning, model.PhaseDevelopment, "coder", 0, now, now)
	if err != nil {
		t.Fatalf("insert thread: %v", err)
	}
	return id
}

func insertMergeArtifact(t *testing.T, db *sql.DB, threadID uuid.UUID, artifactType model.ArtifactType, name string, content string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.Exec(`INSERT INTO artifacts (id, thread_id, type, name, path, content, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), threadID.String(), artifactType, name, "", content, []byte(`{}`), time.Now())
	if err != nil {
		t.Fatalf("insert artifact: %v", err)
	}
	return id
}
