package api

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestPrivateHelpers_ParseJSONArray(t *testing.T) {
	assert.Nil(t, parseJSONArray(""))
	assert.Nil(t, parseJSONArray("null"))
	assert.Nil(t, parseJSONArray("not-json"))
	assert.Equal(t, []string{"agent-a", "agent-b"}, parseJSONArray(`["agent-a","agent-b"]`))
	assert.Equal(t, []string{"agent-a"}, parseJSONArray(`["agent-a", agent-b]`))
}

func TestPrivateHelpers_CallbackUtilities(t *testing.T) {
	assert.Equal(t, []string{"agent-a", "agent-b", "agent-c"}, mergeMentions(
		[]string{"agent-a", "agent-b"},
		[]string{"agent-b", "agent-c"},
	))

	assert.True(t, shouldBroadcastMemoryUpdate("add", `{"success":true}`))
	assert.True(t, shouldBroadcastMemoryUpdate("", `{"success":true}`))
	assert.False(t, shouldBroadcastMemoryUpdate("search", `{"success":true}`))
	assert.False(t, shouldBroadcastMemoryUpdate("add", `{"success":false}`))
	assert.False(t, shouldBroadcastMemoryUpdate("add", `not-json`))

	assert.Equal(t, "agent-a", getFrom(&model.Message{AgentID: "agent-a"}))
	assert.Equal(t, "user", getFrom(&model.Message{}))
}

func TestPrivateHelpers_NameValidation(t *testing.T) {
	for _, valid := range []string{"agent", "agent-1", "a1-b2"} {
		assert.True(t, isValidCommandName(valid))
		assert.True(t, isValidRuleName(valid))
		assert.True(t, isValidSubagentName(valid))
	}

	for _, invalid := range []string{"", "1-agent", "Agent", "agent_name"} {
		assert.False(t, isValidCommandName(invalid))
		assert.False(t, isValidRuleName(invalid))
		assert.False(t, isValidSubagentName(invalid))
	}
}

func TestPrivateHelpers_LocalRepoAndProjectValidationErrors(t *testing.T) {
	assert.False(t, isLocalRepoValidationError(nil))
	assert.False(t, isLocalRepoValidationError(assert.AnError))
	assert.True(t, isLocalRepoValidationError(namedError("名称不能为空")))
	assert.True(t, isLocalRepoValidationError(namedError("路径必须位于工作空间内")))
	assert.False(t, isProjectValidationError(nil))
	assert.True(t, isProjectValidationError(namedError("项目路径不能为空")))
}

func TestPrivateHelpers_SkillMetadataAndFileUtilities(t *testing.T) {
	metadata := parseSkillMD(`# Review Skill

## Description
Reviews API contracts.
Keeps regression notes.

## Usage
Run before merge.
`)
	assert.Equal(t, "Review Skill", metadata.Name)
	assert.Equal(t, "Reviews API contracts.\nKeeps regression notes.", metadata.Description)
	assert.Empty(t, parseSkillMD("no title").Name)

	src := t.TempDir()
	requireNoError(t, os.MkdirAll(filepath.Join(src, "nested"), 0755))
	requireNoError(t, os.MkdirAll(filepath.Join(src, ".git"), 0755))
	requireNoError(t, os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Review Skill"), 0644))
	requireNoError(t, os.WriteFile(filepath.Join(src, "nested", "note.txt"), []byte("note"), 0644))
	requireNoError(t, os.WriteFile(filepath.Join(src, ".git", "config"), []byte("ignored"), 0644))

	dst := filepath.Join(t.TempDir(), "copy")
	requireNoError(t, copyDirectory(src, dst))
	assert.FileExists(t, filepath.Join(dst, "SKILL.md"))
	assert.FileExists(t, filepath.Join(dst, "nested", "note.txt"))
	assert.NoFileExists(t, filepath.Join(dst, ".git", "config"))

	zipPath := filepath.Join(t.TempDir(), "skill.zip")
	requireNoError(t, zipDirectory(src, zipPath))
	zr, err := zip.OpenReader(zipPath)
	requireNoError(t, err)
	defer zr.Close()
	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	assert.Contains(t, names, "SKILL.md")
	assert.NotContains(t, names, ".git/config")

	extracted := filepath.Join(t.TempDir(), "extracted")
	zipBytes, err := os.ReadFile(zipPath)
	requireNoError(t, err)
	requireNoError(t, extractZipToDirectory(zipBytes, extracted))
	assert.FileExists(t, filepath.Join(extracted, "SKILL.md"))
	assert.FileExists(t, filepath.Join(extracted, "note.txt"))

	assert.Error(t, extractZipToDirectory([]byte("not a zip"), t.TempDir()))
}

type namedError string

func (e namedError) Error() string {
	return string(e)
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
