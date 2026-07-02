package mention

import (
	"context"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

func TestPatternRegistryLookupAndFiltering(t *testing.T) {
	agentA := &model.AgentRoleConfig{ID: uuid.New(), Name: "Architect"}
	agentB := &model.AgentRoleConfig{ID: uuid.New(), Name: "Reviewer"}
	agentC := &model.AgentRoleConfig{ID: uuid.New(), Name: "Coder"}
	registry := NewPatternRegistry(nil)
	registry.patterns = map[string][]string{
		"@architect": {agentA.ID.String()},
		"@review":    {agentB.ID.String(), agentC.ID.String()},
	}
	registry.lastRefresh = time.Now()

	entries, err := registry.GetPatterns(context.Background())
	if err != nil {
		t.Fatalf("GetPatterns returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %#v", entries)
	}

	allowed, err := registry.GetPatternsForAgents(context.Background(), []*model.AgentRoleConfig{agentB})
	if err != nil {
		t.Fatalf("GetPatternsForAgents returned error: %v", err)
	}
	if len(allowed) != 1 || allowed[0].Pattern != "@review" || len(allowed[0].AgentIDs) != 1 || allowed[0].AgentIDs[0] != agentB.ID.String() {
		t.Fatalf("allowed entries = %#v", allowed)
	}

	found, err := registry.Lookup(context.Background(), "@REVIEW")
	if err != nil {
		t.Fatalf("Lookup returned error: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("lookup found = %#v", found)
	}
	missing, err := registry.Lookup(context.Background(), "@missing")
	if err != nil || missing != nil {
		t.Fatalf("missing lookup = %#v err=%v", missing, err)
	}
}

func TestParserParseVariants(t *testing.T) {
	agentA := &model.AgentRoleConfig{ID: uuid.New(), Name: "Architect"}
	agentB := &model.AgentRoleConfig{ID: uuid.New(), Name: "Reviewer"}
	currentID := uuid.New().String()
	registry := NewPatternRegistry(nil)
	registry.patterns = map[string][]string{
		"@architect": {agentA.ID.String()},
		"@review":    {agentB.ID.String(), currentID},
	}
	registry.lastRefresh = time.Now()
	parser := NewParser(registry)

	ids, err := parser.Parse(context.Background(), "@architect 请设计方案", currentID)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(ids) != 1 || ids[0] != agentA.ID.String() {
		t.Fatalf("parse ids = %#v", ids)
	}

	filtered, err := parser.ParseForAgents(context.Background(), "@review 请审查", currentID, []*model.AgentRoleConfig{agentB})
	if err != nil {
		t.Fatalf("ParseForAgents returned error: %v", err)
	}
	if len(filtered) != 1 || filtered[0] != agentB.ID.String() {
		t.Fatalf("filtered ids = %#v", filtered)
	}

	multi, err := parser.ParseMulti(context.Background(), "@review 请审查", currentID)
	if err != nil {
		t.Fatalf("ParseMulti returned error: %v", err)
	}
	if len(multi) != 1 || len(multi[0]) != 1 || multi[0][0] != agentB.ID.String() {
		t.Fatalf("multi ids = %#v", multi)
	}

	multiFiltered, err := parser.ParseMultiForAgents(context.Background(), "@review 请审查", currentID, []*model.AgentRoleConfig{agentB})
	if err != nil {
		t.Fatalf("ParseMultiForAgents returned error: %v", err)
	}
	if len(multiFiltered) != 1 || len(multiFiltered[0]) != 1 || multiFiltered[0][0] != agentB.ID.String() {
		t.Fatalf("multi filtered ids = %#v", multiFiltered)
	}

	none, err := parser.Parse(context.Background(), "正文里提到 @architect 不在行首", currentID)
	if err != nil || len(none) != 0 {
		t.Fatalf("non-leading parse = %#v err=%v", none, err)
	}
}

func TestPatternRegistryRequiresRefreshErrorsWhenEmpty(t *testing.T) {
	registry := NewPatternRegistry(nil)
	defer func() {
		if recover() == nil {
			t.Fatalf("nil repo refresh should panic with current implementation")
		}
	}()
	_, _ = registry.GetPatterns(context.Background())
}
