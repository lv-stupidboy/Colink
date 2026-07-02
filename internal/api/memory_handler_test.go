package api

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropic/isdp/internal/service/memory"
	"github.com/gin-gonic/gin"
)

func TestMemoryHandlerRaw(t *testing.T) {
	teamRoot := t.TempDir()
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(teamRoot, "team-a"), 0755); err != nil {
		t.Fatalf("mkdir team memory: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, ".colink", "project-memory"), 0755); err != nil {
		t.Fatalf("mkdir project memory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(teamRoot, "team-a", "MEMORY.md"), []byte("# Team\n\n- team fact\n"), 0644); err != nil {
		t.Fatalf("write team memory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".colink", "project-memory", "MEMORY.md"), []byte("# Project\n\n- project fact\n"), 0644); err != nil {
		t.Fatalf("write project memory: %v", err)
	}

	manager := memory.NewMemoryManagerWithTeamPath(nil, teamRoot)
	router := setupAPILightRouter(func(group *gin.RouterGroup) {
		NewMemoryHandler(manager).RegisterRoutes(group)
	})

	all := performAPILightJSON(router, http.MethodGet, "/api/v1/memory/raw?teamId=team-a&teamName=Team+A&projectName=Project+A&workspacePath="+workspace, nil)
	if all.Code != http.StatusOK || !bytes.Contains(all.Body.Bytes(), []byte("team fact")) || !bytes.Contains(all.Body.Bytes(), []byte("project fact")) {
		t.Fatalf("raw all code=%d body=%s", all.Code, all.Body.String())
	}

	teamOnly := performAPILightJSON(router, http.MethodGet, "/api/v1/memory/raw?type=team&teamId=team-a&workspacePath="+workspace, nil)
	if teamOnly.Code != http.StatusOK || !bytes.Contains(teamOnly.Body.Bytes(), []byte("team fact")) || bytes.Contains(teamOnly.Body.Bytes(), []byte("project fact")) {
		t.Fatalf("raw team code=%d body=%s", teamOnly.Code, teamOnly.Body.String())
	}

	projectOnly := performAPILightJSON(router, http.MethodGet, "/api/v1/memory/raw?type=project&workspacePath="+workspace, nil)
	if projectOnly.Code != http.StatusOK || !bytes.Contains(projectOnly.Body.Bytes(), []byte("project fact")) || bytes.Contains(projectOnly.Body.Bytes(), []byte("team fact")) {
		t.Fatalf("raw project code=%d body=%s", projectOnly.Code, projectOnly.Body.String())
	}

	badType := performAPILightJSON(router, http.MethodGet, "/api/v1/memory/raw?type=bad", nil)
	if badType.Code != http.StatusBadRequest {
		t.Fatalf("bad type code=%d body=%s", badType.Code, badType.Body.String())
	}
}

func TestMemoryHandlerRawUnavailable(t *testing.T) {
	router := setupAPILightRouter(func(group *gin.RouterGroup) {
		NewMemoryHandler(nil).RegisterRoutes(group)
	})

	w := performAPILightJSON(router, http.MethodGet, "/api/v1/memory/raw", nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil memory manager code=%d body=%s", w.Code, w.Body.String())
	}
}
