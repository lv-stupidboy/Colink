package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-54
func TestCommandAndSubagentHandler_SkillBindingLifecycle(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	skillW := performJSON(f.router, http.MethodPost, "/api/v1/skills", map[string]any{
		"name":            "binding-helper",
		"description":     "Shared helper skill",
		"tags":            []string{"binding"},
		"sourceType":      "personal",
		"supportedAgents": []string{"claude_code"},
	})
	require.Equal(t, http.StatusCreated, skillW.Code)

	var skill model.Skill
	require.NoError(t, json.Unmarshal(skillW.Body.Bytes(), &skill))

	commandW := performJSON(f.router, http.MethodPost, "/api/v1/commands", map[string]any{
		"name":            "bind-skill-command",
		"description":     "Command skill binding",
		"content":         "run shared helper",
		"supportedAgents": []string{"claude_code"},
	})
	require.Equal(t, http.StatusCreated, commandW.Code)

	var command model.Command
	require.NoError(t, json.Unmarshal(commandW.Body.Bytes(), &command))

	bindCommandW := performJSON(f.router, http.MethodPost, "/api/v1/commands/"+command.ID.String()+"/skills", map[string]any{
		"skillIds": []string{skill.ID.String()},
	})
	require.Equal(t, http.StatusNoContent, bindCommandW.Code)

	commandSkillsW := performJSON(f.router, http.MethodGet, "/api/v1/commands/"+command.ID.String()+"/skills", nil)
	require.Equal(t, http.StatusOK, commandSkillsW.Code)
	assert.Contains(t, commandSkillsW.Body.String(), `"count":1`)
	assert.Contains(t, commandSkillsW.Body.String(), "binding-helper")

	unbindCommandW := performJSON(f.router, http.MethodDelete, "/api/v1/commands/"+command.ID.String()+"/skills/"+skill.ID.String(), nil)
	require.Equal(t, http.StatusNoContent, unbindCommandW.Code)

	commandSkillsAfterUnbindW := performJSON(f.router, http.MethodGet, "/api/v1/commands/"+command.ID.String()+"/skills", nil)
	require.Equal(t, http.StatusOK, commandSkillsAfterUnbindW.Code)
	assert.Contains(t, commandSkillsAfterUnbindW.Body.String(), `"count":0`)

	subagentW := performJSON(f.router, http.MethodPost, "/api/v1/subagents", map[string]any{
		"name":            "bind-skill-subagent",
		"description":     "Subagent skill binding",
		"content":         "use shared helper",
		"supportedAgents": []string{"claude_code"},
	})
	require.Equal(t, http.StatusCreated, subagentW.Code)

	var subagent model.Subagent
	require.NoError(t, json.Unmarshal(subagentW.Body.Bytes(), &subagent))

	bindSubagentW := performJSON(f.router, http.MethodPost, "/api/v1/subagents/"+subagent.ID.String()+"/skills", map[string]any{
		"skillIds": []string{skill.ID.String()},
	})
	require.Equal(t, http.StatusNoContent, bindSubagentW.Code)

	subagentSkillsW := performJSON(f.router, http.MethodGet, "/api/v1/subagents/"+subagent.ID.String()+"/skills", nil)
	require.Equal(t, http.StatusOK, subagentSkillsW.Code)
	assert.Contains(t, subagentSkillsW.Body.String(), `"count":1`)
	assert.Contains(t, subagentSkillsW.Body.String(), "binding-helper")

	unbindSubagentW := performJSON(f.router, http.MethodDelete, "/api/v1/subagents/"+subagent.ID.String()+"/skills/"+skill.ID.String(), nil)
	require.Equal(t, http.StatusNoContent, unbindSubagentW.Code)

	subagentSkillsAfterUnbindW := performJSON(f.router, http.MethodGet, "/api/v1/subagents/"+subagent.ID.String()+"/skills", nil)
	require.Equal(t, http.StatusOK, subagentSkillsAfterUnbindW.Code)
	assert.Contains(t, subagentSkillsAfterUnbindW.Body.String(), `"count":0`)
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-55
func TestBindingHandlers_BoundAgentValidationBranches(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	invalidCommandW := performJSON(f.router, http.MethodGet, "/api/v1/commands/not-a-uuid/agents", nil)
	assert.Equal(t, http.StatusBadRequest, invalidCommandW.Code)

	missingGeneratorCommandW := performJSON(f.router, http.MethodGet, "/api/v1/commands/"+uuid.New().String()+"/agents", nil)
	assert.Equal(t, http.StatusInternalServerError, missingGeneratorCommandW.Code)
	assert.Contains(t, missingGeneratorCommandW.Body.String(), "autoGenerator not initialized")

	invalidSubagentW := performJSON(f.router, http.MethodGet, "/api/v1/subagents/not-a-uuid/agents", nil)
	assert.Equal(t, http.StatusBadRequest, invalidSubagentW.Code)

	missingGeneratorSubagentW := performJSON(f.router, http.MethodGet, "/api/v1/subagents/"+uuid.New().String()+"/agents", nil)
	assert.Equal(t, http.StatusInternalServerError, missingGeneratorSubagentW.Code)
	assert.Contains(t, missingGeneratorSubagentW.Body.String(), "autoGenerator not initialized")

	invalidRuleW := performJSON(f.router, http.MethodGet, "/api/v1/rules/not-a-uuid/agents", nil)
	assert.Equal(t, http.StatusBadRequest, invalidRuleW.Code)

	missingGeneratorRuleW := performJSON(f.router, http.MethodGet, "/api/v1/rules/"+uuid.New().String()+"/agents", nil)
	assert.Equal(t, http.StatusInternalServerError, missingGeneratorRuleW.Code)
	assert.Contains(t, missingGeneratorRuleW.Body.String(), "autoGenerator not initialized")
}
