package api_test

import (
	"archive/zip"
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(w, content)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func performMultipart(router *gin.Engine, path string, fields map[string]string, fileField, filename string, fileBytes []byte) *httptest.ResponseRecorder {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		_ = writer.WriteField(key, value)
	}
	if fileField != "" {
		part, _ := writer.CreateFormFile(fileField, filename)
		_, _ = part.Write(fileBytes)
	}
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-51
func TestSkillHandler_UploadZipLifecycle(t *testing.T) {
	f := setupAPISurfaceFixture(t)
	zipBytes := makeZip(t, map[string]string{
		"SKILL.md": "# Uploaded Skill\n\n## Description\nUseful uploaded skill.",
	})

	w := performMultipart(f.router, "/api/v1/skills/upload", map[string]string{
		"directory_name": "uploaded-skill",
		"description":    "Useful uploaded skill",
		"source_type":    "personal",
	}, "file", "skill.zip", zipBytes)
	require.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "uploaded-skill")

	badExtW := performMultipart(f.router, "/api/v1/skills/upload", map[string]string{
		"directory_name": "bad-skill",
	}, "file", "skill.txt", []byte("not a zip"))
	assert.Equal(t, http.StatusBadRequest, badExtW.Code)
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-52
func TestSettingsHandler_CreateFromZipLifecycle(t *testing.T) {
	f := setupAPISurfaceFixture(t)
	zipBytes := makeZip(t, map[string]string{
		"config/settings.json": `{"theme":"dark"}`,
		"README.md":            "# Settings",
	})

	w := performMultipart(f.router, "/api/v1/settings", map[string]string{
		"name":        "uploaded-settings",
		"description": "Uploaded settings",
	}, "file", "settings.zip", zipBytes)
	require.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "uploaded-settings")

	badExtW := performMultipart(f.router, "/api/v1/settings", map[string]string{
		"name": "bad-settings",
	}, "file", "settings.txt", []byte("not a zip"))
	assert.Equal(t, http.StatusBadRequest, badExtW.Code)
}
