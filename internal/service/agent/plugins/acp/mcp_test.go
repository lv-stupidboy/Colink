package acp

import (
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

func TestConvertManagedMCPToACPFormat(t *testing.T) {
	servers := []*model.MCPServer{
		{
			ID:        uuid.New(),
			Name:      "github-tools",
			Transport: model.MCPTransportStdio,
			Command:   "npx",
			Args:      []string{"-y", "@modelcontextprotocol/server-github"},
			Env:       map[string]string{"GITHUB_TOKEN": "token"},
			Status:    model.MCPStatusActive,
		},
		{
			ID:        uuid.New(),
			Name:      "docs-http",
			Transport: model.MCPTransportHTTP,
			URL:       "https://example.test/mcp",
			Headers:   map[string]string{"Authorization": "Bearer token"},
			Status:    model.MCPStatusActive,
		},
		{
			ID:        uuid.New(),
			Name:      "events",
			Transport: model.MCPTransportSSE,
			URL:       "https://example.test/sse",
			Status:    model.MCPStatusActive,
		},
		{
			ID:        uuid.New(),
			Name:      "disabled",
			Transport: model.MCPTransportStdio,
			Command:   "disabled",
			Status:    model.MCPStatusDisabled,
		},
	}

	got := convertManagedMCPToACPFormat(servers)
	if len(got) != 3 {
		t.Fatalf("expected 3 active ACP servers, got %d", len(got))
	}

	stdio := got[0].(map[string]interface{})
	if stdio["name"] != "github-tools" || stdio["command"] != "npx" {
		t.Fatalf("unexpected stdio ACP server: %#v", stdio)
	}
	if _, ok := stdio["type"]; ok {
		t.Fatalf("stdio ACP server should not include type: %#v", stdio)
	}

	http := got[1].(map[string]interface{})
	if http["name"] != "docs-http" || http["type"] != "http" || http["url"] != "https://example.test/mcp" {
		t.Fatalf("unexpected http ACP server: %#v", http)
	}

	sse := got[2].(map[string]interface{})
	if sse["name"] != "events" || sse["type"] != "sse" || sse["url"] != "https://example.test/sse" {
		t.Fatalf("unexpected sse ACP server: %#v", sse)
	}
}
