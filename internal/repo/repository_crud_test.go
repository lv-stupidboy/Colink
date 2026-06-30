package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestBaseAgentRepositoryCRUDAndDefaults(t *testing.T) {
	ctx := context.Background()
	db := openRepoCRUDTestDB(t)
	repository := NewBaseAgentRepository(db, DBTypeSQLite)
	now := time.Now()

	first := &model.BaseAgent{ID: uuid.New(), Name: "Hermes", Type: "hermes", ApiURL: "https://api", ApiToken: "tok", DefaultModel: "qwen", CliPath: "hermes", MaxTokens: 1024, TimeoutMinutes: 3, IsDefault: true, CreatedAt: now, UpdatedAt: now}
	second := &model.BaseAgent{ID: uuid.New(), Name: "OpenCode", Type: "open_code", CliPath: "opencode", CreatedAt: now, UpdatedAt: now}
	if err := repository.Create(ctx, first); err != nil {
		t.Fatalf("create first: %v", err)
	}
	if err := repository.Create(ctx, second); err != nil {
		t.Fatalf("create second: %v", err)
	}

	got, err := repository.FindByID(ctx, first.ID)
	if err != nil || got.Name != "Hermes" || got.ApiToken != "tok" {
		t.Fatalf("FindByID = %+v err=%v", got, err)
	}
	byType, err := repository.FindByType(ctx, "hermes")
	if err != nil || len(byType) != 1 || byType[0].ID != first.ID {
		t.Fatalf("FindByType = %+v err=%v", byType, err)
	}
	all, err := repository.ListActive(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("ListActive = %+v err=%v", all, err)
	}
	def, err := repository.FindDefault(ctx)
	if err != nil || def == nil || def.ID != first.ID {
		t.Fatalf("FindDefault = %+v err=%v", def, err)
	}

	if err := repository.SetDefault(ctx, second.ID); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	def, err = repository.FindDefault(ctx)
	if err != nil || def == nil || def.ID != second.ID {
		t.Fatalf("FindDefault after SetDefault = %+v err=%v", def, err)
	}
	second.Name = "OpenCode Updated"
	second.IsDefault = true
	if err := repository.Update(ctx, second); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err = repository.FindByID(ctx, second.ID)
	if err != nil || got.Name != "OpenCode Updated" {
		t.Fatalf("updated agent = %+v err=%v", got, err)
	}
	if err := repository.ClearDefault(ctx, second.ID); err != nil {
		t.Fatalf("ClearDefault: %v", err)
	}
	if def, err = repository.FindDefault(ctx); err != nil || def != nil {
		t.Fatalf("FindDefault after ClearDefault = %+v err=%v", def, err)
	}
	if err := repository.Delete(ctx, first.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestAssetRepositoriesAndBindings(t *testing.T) {
	ctx := context.Background()
	db := openRepoCRUDTestDB(t)
	now := time.Now()

	skillRepo := NewSkillRepository(db, DBTypeSQLite)
	commandRepo := NewCommandRepository(db, DBTypeSQLite)
	ruleRepo := NewRuleRepository(db, DBTypeSQLite)
	subagentRepo := NewSubagentRepository(db, DBTypeSQLite)
	settingsRepo := NewSettingsRepository(db, DBTypeSQLite)
	agentRepo := NewAgentConfigRepository(db, DBTypeSQLite)

	skill := &model.Skill{ID: uuid.New(), Name: "Review", Description: "reviews code", Tags: []string{"review", "go"}, SourceType: model.SkillSourcePersonal, Status: model.SkillStatusActive, IsPublic: true, CreatedAt: now, UpdatedAt: now}
	if err := skillRepo.Create(ctx, skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}
	gotSkill, err := skillRepo.FindByName(ctx, "Review")
	if err != nil || gotSkill.ID != skill.ID || len(gotSkill.Tags) != 2 {
		t.Fatalf("FindByName skill = %+v err=%v", gotSkill, err)
	}
	skills, total, err := skillRepo.List(ctx, &model.SkillListQuery{Search: "Review", Page: -1, PageSize: 1000})
	if err != nil || total != 1 || len(skills) != 1 {
		t.Fatalf("List skills = %+v total=%d err=%v", skills, total, err)
	}
	if err := skillRepo.IncrementUseCount(ctx, skill.ID); err != nil {
		t.Fatalf("IncrementUseCount: %v", err)
	}
	if err := skillRepo.UpdateUseCount(ctx, skill.ID.String(), 7); err != nil {
		t.Fatalf("UpdateUseCount: %v", err)
	}
	tags, err := skillRepo.GetAllTags(ctx)
	if err != nil || len(tags) != 2 {
		t.Fatalf("GetAllTags = %+v err=%v", tags, err)
	}
	skill.Description = "updated"
	if err := skillRepo.Update(ctx, skill); err != nil {
		t.Fatalf("Update skill: %v", err)
	}
	if _, err := skillRepo.FindBySourcePath(ctx, "Review", uuid.New(), "missing"); err == nil {
		t.Fatalf("FindBySourcePath should miss unrelated source")
	}

	command := &model.Command{ID: uuid.New(), Name: "Build", Description: "build command", CreatedAt: now, UpdatedAt: now}
	rule := &model.Rule{ID: uuid.New(), Name: "Secure", Description: "secure rule", CreatedAt: now, UpdatedAt: now}
	subagent := &model.Subagent{ID: uuid.New(), Name: "Reviewer", Description: "review agent", CreatedAt: now, UpdatedAt: now}
	settings := &model.Settings{ID: uuid.New(), Name: "Defaults", Description: "settings", DirectoryPath: "/tmp/settings", CreatedAt: now, UpdatedAt: now}
	if err := commandRepo.Create(ctx, command); err != nil {
		t.Fatalf("create command: %v", err)
	}
	if err := ruleRepo.Create(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}
	if err := subagentRepo.Create(ctx, subagent); err != nil {
		t.Fatalf("create subagent: %v", err)
	}
	if err := settingsRepo.Create(ctx, settings); err != nil {
		t.Fatalf("create settings: %v", err)
	}
	gotCommand, err := commandRepo.FindByID(ctx, command.ID)
	assertNamedRepoItem(t, gotCommand, err, "Build", "command")
	gotRule, err := ruleRepo.FindByName(ctx, "Secure")
	assertNamedRepoItem(t, gotRule, err, "Secure", "rule")
	gotSubagent, err := subagentRepo.FindByID(ctx, subagent.ID)
	assertNamedRepoItem(t, gotSubagent, err, "Reviewer", "subagent")
	gotSettings, err := settingsRepo.FindByName(ctx, "Defaults")
	assertNamedRepoItem(t, gotSettings, err, "Defaults", "settings")
	if commands, total, err := commandRepo.List(ctx, &model.CommandListQuery{Search: "Build", PageSize: 1000}); err != nil || total != 1 || len(commands) != 1 {
		t.Fatalf("List commands = %+v total=%d err=%v", commands, total, err)
	}
	if rules, total, err := ruleRepo.List(ctx, &model.RuleListQuery{Search: "Secure", PageSize: 1000}); err != nil || total != 1 || len(rules) != 1 {
		t.Fatalf("List rules = %+v total=%d err=%v", rules, total, err)
	}
	if subagents, total, err := subagentRepo.List(ctx, &model.SubagentListQuery{Search: "Reviewer", PageSize: 1000}); err != nil || total != 1 || len(subagents) != 1 {
		t.Fatalf("List subagents = %+v total=%d err=%v", subagents, total, err)
	}
	if settingsList, total, err := settingsRepo.List(ctx, &model.SettingsListQuery{Search: "Defaults", PageSize: 1000}); err != nil || total != 1 || len(settingsList) != 1 {
		t.Fatalf("List settings = %+v total=%d err=%v", settingsList, total, err)
	}
	command.Description = "updated"
	rule.Description = "updated"
	subagent.Description = "updated"
	settings.Description = "updated"
	if err := commandRepo.Update(ctx, command); err != nil {
		t.Fatalf("update command: %v", err)
	}
	if err := ruleRepo.Update(ctx, rule); err != nil {
		t.Fatalf("update rule: %v", err)
	}
	if err := subagentRepo.Update(ctx, subagent); err != nil {
		t.Fatalf("update subagent: %v", err)
	}
	if err := settingsRepo.Update(ctx, settings); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	agent := &model.AgentRoleConfig{ID: uuid.New(), Name: "Coder", Role: model.AgentRoleAgent, Description: "codes", SystemPrompt: "code", MentionPatterns: []string{"@coder"}, CreatedAt: now, UpdatedAt: now}
	if err := agentRepo.Create(ctx, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	gotAgent, err := agentRepo.FindByID(ctx, agent.ID)
	if err != nil || gotAgent.Name != "Coder" || len(gotAgent.MentionPatterns) != 1 {
		t.Fatalf("FindByID agent = %+v err=%v", gotAgent, err)
	}
	byRole, err := agentRepo.FindByRole(ctx, model.AgentRoleAgent)
	if err != nil || len(byRole) != 1 {
		t.Fatalf("FindByRole = %+v err=%v", byRole, err)
	}
	agent.Description = "updated"
	if err := agentRepo.Update(ctx, agent); err != nil {
		t.Fatalf("Update agent: %v", err)
	}
	if err := agentRepo.UpdateConfigGeneratedAt(ctx, agent.ID, "/tmp/config"); err != nil {
		t.Fatalf("UpdateConfigGeneratedAt: %v", err)
	}

	skillBindingRepo := NewAgentSkillBindingRepository(db, DBTypeSQLite)
	commandBindingRepo := NewAgentCommandBindingRepository(db, DBTypeSQLite)
	subagentBindingRepo := NewAgentSubagentBindingRepository(db, DBTypeSQLite)
	ruleBindingRepo := NewAgentRuleBindingRepository(db, DBTypeSQLite)
	settingsBindingRepo := NewAgentSettingsBindingRepository(db, DBTypeSQLite)
	commandSkillRepo := NewCommandSkillBindingRepository(db, DBTypeSQLite)
	subagentSkillRepo := NewSubagentSkillBindingRepository(db, DBTypeSQLite)

	mustRepo(t, skillBindingRepo.Create(ctx, &model.AgentSkillBinding{ID: uuid.New(), AgentRoleID: agent.ID, SkillID: skill.ID, CreatedAt: now}))
	mustRepo(t, commandBindingRepo.Create(ctx, &model.AgentCommandBinding{ID: uuid.New(), AgentRoleID: agent.ID, CommandID: command.ID, CreatedAt: now}))
	mustRepo(t, subagentBindingRepo.Create(ctx, &model.AgentSubagentBinding{ID: uuid.New(), AgentRoleID: agent.ID, SubagentID: subagent.ID, CreatedAt: now}))
	mustRepo(t, ruleBindingRepo.Create(ctx, &model.AgentRuleBinding{ID: uuid.New(), AgentRoleID: agent.ID, RuleID: rule.ID, CreatedAt: now}))
	mustRepo(t, settingsBindingRepo.Create(ctx, &model.AgentSettingsBinding{ID: uuid.New(), AgentRoleID: agent.ID, SettingsID: settings.ID, CreatedAt: now}))
	mustRepo(t, commandSkillRepo.Create(ctx, &model.CommandSkillBinding{ID: uuid.New(), CommandID: command.ID, SkillID: skill.ID, CreatedAt: now}))
	mustRepo(t, subagentSkillRepo.Create(ctx, &model.SubagentSkillBinding{ID: uuid.New(), SubagentID: subagent.ID, SkillID: skill.ID, CreatedAt: now}))

	ids, err := skillBindingRepo.FindByAgentRoleID(ctx, agent.ID)
	assertUUIDList(t, ids, err, skill.ID, "agent skills")
	ids, err = commandBindingRepo.FindByAgentRoleID(ctx, agent.ID)
	assertUUIDList(t, ids, err, command.ID, "agent commands")
	ids, err = subagentBindingRepo.FindByAgentRoleID(ctx, agent.ID)
	assertUUIDList(t, ids, err, subagent.ID, "agent subagents")
	ids, err = ruleBindingRepo.FindByAgentRoleID(ctx, agent.ID)
	assertUUIDList(t, ids, err, rule.ID, "agent rules")
	ids, err = settingsBindingRepo.FindByAgentRoleID(ctx, agent.ID)
	assertUUIDList(t, ids, err, settings.ID, "agent settings")
	ids, err = commandSkillRepo.FindByCommandID(ctx, command.ID)
	assertUUIDList(t, ids, err, skill.ID, "command skills")
	ids, err = subagentSkillRepo.FindBySubagentID(ctx, subagent.ID)
	assertUUIDList(t, ids, err, skill.ID, "subagent skills")
	ids, err = skillBindingRepo.FindBySkillID(ctx, skill.ID)
	assertUUIDList(t, ids, err, agent.ID, "skill agents")
	ids, err = commandBindingRepo.FindByCommandID(ctx, command.ID)
	assertUUIDList(t, ids, err, agent.ID, "command agents")
	ids, err = subagentBindingRepo.FindBySubagentID(ctx, subagent.ID)
	assertUUIDList(t, ids, err, agent.ID, "subagent agents")
	ids, err = ruleBindingRepo.FindByRuleID(ctx, rule.ID)
	assertUUIDList(t, ids, err, agent.ID, "rule agents")
	ids, err = settingsBindingRepo.FindBySettingsID(ctx, settings.ID)
	assertUUIDList(t, ids, err, agent.ID, "settings agents")

	exists, err := skillBindingRepo.ExistsBinding(ctx, agent.ID, skill.ID)
	assertExists(t, exists, err, "agent skill")
	exists, err = commandBindingRepo.ExistsBinding(ctx, agent.ID, command.ID)
	assertExists(t, exists, err, "agent command")
	exists, err = subagentBindingRepo.ExistsBinding(ctx, agent.ID, subagent.ID)
	assertExists(t, exists, err, "agent subagent")
	exists, err = ruleBindingRepo.ExistsBinding(ctx, agent.ID, rule.ID)
	assertExists(t, exists, err, "agent rule")
	exists, err = settingsBindingRepo.ExistsBinding(ctx, agent.ID, settings.ID)
	assertExists(t, exists, err, "agent settings")
	exists, err = commandSkillRepo.ExistsBinding(ctx, command.ID, skill.ID)
	assertExists(t, exists, err, "command skill")
	exists, err = subagentSkillRepo.ExistsBinding(ctx, subagent.ID, skill.ID)
	assertExists(t, exists, err, "subagent skill")

	mustRepo(t, skillBindingRepo.DeleteBinding(ctx, agent.ID, skill.ID))
	mustRepo(t, commandBindingRepo.DeleteBinding(ctx, agent.ID, command.ID))
	mustRepo(t, subagentBindingRepo.DeleteBinding(ctx, agent.ID, subagent.ID))
	mustRepo(t, ruleBindingRepo.DeleteBinding(ctx, agent.ID, rule.ID))
	mustRepo(t, settingsBindingRepo.DeleteBinding(ctx, agent.ID, settings.ID))
	mustRepo(t, commandSkillRepo.DeleteBinding(ctx, command.ID, skill.ID))
	mustRepo(t, subagentSkillRepo.DeleteBinding(ctx, subagent.ID, skill.ID))
}

func TestWorkflowTemplateRepositoryQueries(t *testing.T) {
	ctx := context.Background()
	db := openRepoCRUDTestDB(t)
	repository := NewWorkflowTemplateRepository(db, DBTypeSQLite)
	agentID := uuid.New()
	agentIDs, _ := json.Marshal([]string{agentID.String()})
	transitions, _ := json.Marshal([]model.Transition{{FromAgentID: agentID.String(), ToAgentID: agentID.String(), Type: model.TransitionTypeSequence}})
	checkpoints, _ := json.Marshal([]string{"review"})
	routable, _ := json.Marshal([]string{"ops"})
	template := &model.WorkflowTemplate{ID: uuid.New(), Name: "Delivery", Description: "deliver", AgentIDs: agentIDs, Transitions: transitions, Checkpoints: checkpoints, EstimatedTime: "2h", RoutableTeams: routable}
	if err := repository.Create(ctx, template); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	got, err := repository.FindByID(ctx, template.ID)
	if err != nil || got.Name != "Delivery" {
		t.Fatalf("FindByID workflow = %+v err=%v", got, err)
	}
	all, err := repository.FindAll(ctx)
	if err != nil || len(all) != 1 {
		t.Fatalf("FindAll workflow = %+v err=%v", all, err)
	}
	if err := repository.SetDefault(ctx, template.ID); err != nil {
		t.Fatalf("SetDefault workflow: %v", err)
	}
	def, err := repository.GetDefault(ctx)
	if err != nil || def.ID != template.ID {
		t.Fatalf("GetDefault workflow = %+v err=%v", def, err)
	}
	template.Description = "updated"
	if err := repository.Update(ctx, template); err != nil {
		t.Fatalf("Update workflow: %v", err)
	}
	byAgent, err := repository.FindByAgentID(ctx, agentID)
	if err != nil || len(byAgent) != 1 {
		t.Fatalf("FindByAgentID = %+v err=%v", byAgent, err)
	}
	if _, err := db.Exec(`INSERT INTO projects (id, workflow_template_id) VALUES (?, ?)`, uuid.NewString(), template.ID.String()); err != nil {
		t.Fatalf("insert project reference: %v", err)
	}
	if count, err := repository.CountProjectReferences(ctx, template.ID); err != nil || count != 1 {
		t.Fatalf("CountProjectReferences = %d err=%v", count, err)
	}
	if err := repository.Delete(ctx, template.ID); err != nil {
		t.Fatalf("Delete workflow: %v", err)
	}
	if _, err := repository.FindByID(ctx, template.ID); err == nil {
		t.Fatalf("deleted workflow should not be found")
	}
}

func TestTeamPackageVersionRepositoryLifecycle(t *testing.T) {
	db := openRepoCRUDTestDB(t)
	repository := NewTeamPackageVersionRepository(db, DBTypeSQLite)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	version := &model.TeamPackageVersion{
		WorkflowID:    uuid.New(),
		Name:          "delivery-team",
		Category:      "development",
		Version:       "1.0.0",
		Description:   "initial release",
		LastSyncedAt:  &now,
	}
	if err := repository.Create(ctx, version); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if version.ID == uuid.Nil || version.CreatedAt.IsZero() || version.UpdatedAt.IsZero() {
		t.Fatalf("created version missing generated fields: %+v", version)
	}

	found, err := repository.FindByName(ctx, "delivery-team")
	if err != nil {
		t.Fatalf("FindByName() error = %v", err)
	}
	if found == nil || found.ID != version.ID || found.LastSyncedAt == nil {
		t.Fatalf("FindByName() = %+v, want created version", found)
	}
	missing, err := repository.FindByName(ctx, "missing")
	if err != nil || missing != nil {
		t.Fatalf("FindByName(missing) = %+v err=%v, want nil nil", missing, err)
	}

	all, err := repository.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	if len(all) != 1 || all[0].Name != "delivery-team" {
		t.Fatalf("ListAll() = %+v", all)
	}

	updatedTime := now.Add(time.Hour)
	version.Version = "1.1.0"
	version.Category = "ops"
	version.Description = "updated release"
	version.LastSyncedAt = &updatedTime
	if err := repository.Update(ctx, version); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updated, err := repository.FindByName(ctx, "delivery-team")
	if err != nil {
		t.Fatalf("FindByName(updated) error = %v", err)
	}
	if updated.Version != "1.1.0" || updated.Category != "ops" || updated.Description != "updated release" {
		t.Fatalf("updated version = %+v", updated)
	}
}

func TestTeamPackageVersionRepositoryErrors(t *testing.T) {
	db := openRepoCRUDTestDB(t)
	repository := NewTeamPackageVersionRepository(db, DBTypeSQLite)
	ctx := context.Background()

	mustRepo(t, db.Close())
	if err := repository.Create(ctx, &model.TeamPackageVersion{Name: "broken"}); err == nil {
		t.Fatal("Create(closed db) error = nil, want error")
	}
	if _, err := repository.FindByName(ctx, "broken"); err == nil {
		t.Fatal("FindByName(closed db) error = nil, want error")
	}
	if _, err := repository.ListAll(ctx); err == nil {
		t.Fatal("ListAll(closed db) error = nil, want error")
	}
	if err := repository.Update(ctx, &model.TeamPackageVersion{ID: uuid.New()}); err == nil {
		t.Fatal("Update(closed db) error = nil, want error")
	}
}

func TestLocalRepoAndMarketRepositories(t *testing.T) {
	ctx := context.Background()
	db := openRepoCRUDTestDB(t)

	localRepo := NewLocalRepoRepository(db, DBTypeSQLite)
	branch := "main"
	commit := "abc123"
	errMsg := "offline"
	item := &model.LocalRepo{ID: uuid.New(), Name: "repo", GitUrl: "git@example.com:repo.git", LocalPath: "/tmp/repo", Branch: &branch, LastCommit: &commit, Status: model.RepoStatusError, ErrorMessage: &errMsg}
	if err := localRepo.Create(ctx, item); err != nil {
		t.Fatalf("create local repo: %v", err)
	}
	got, err := localRepo.FindByID(ctx, item.ID)
	if err != nil || got.Branch == nil || *got.Branch != branch || got.ErrorMessage == nil {
		t.Fatalf("FindByID local repo = %+v err=%v", got, err)
	}
	item.Status = model.RepoStatusReady
	item.ErrorMessage = nil
	if err := localRepo.Update(ctx, item); err != nil {
		t.Fatalf("update local repo: %v", err)
	}
	allRepos, err := localRepo.FindAll(ctx)
	if err != nil || len(allRepos) != 1 || allRepos[0].Status != model.RepoStatusReady {
		t.Fatalf("FindAll local repos = %+v err=%v", allRepos, err)
	}
	if err := localRepo.Delete(ctx, item.ID); err != nil {
		t.Fatalf("delete local repo: %v", err)
	}

	marketRepo := NewMarketRepository(db, DBTypeSQLite)
	syncedAt := time.Now().UTC().Truncate(time.Second)
	market := &model.Market{Name: "Colink", URL: "git@example.com:market.git", Branch: "main", Enabled: true, AutoUpdate: true, CheckInterval: "1h", LastSyncedAt: &syncedAt}
	if err := marketRepo.Create(ctx, market); err != nil {
		t.Fatalf("create market: %v", err)
	}
	gotMarket, err := marketRepo.FindByID(ctx, market.ID)
	if err != nil || gotMarket == nil || gotMarket.Name != "Colink" || gotMarket.LastSyncedAt == nil {
		t.Fatalf("FindByID market = %+v err=%v", gotMarket, err)
	}
	market.LastError = "failed"
	market.Enabled = false
	if err := marketRepo.Update(ctx, market); err != nil {
		t.Fatalf("update market: %v", err)
	}
	if err := marketRepo.UpdateSyncStatus(ctx, market.ID, nil, "ok"); err != nil {
		t.Fatalf("UpdateSyncStatus: %v", err)
	}
	markets, err := marketRepo.List(ctx)
	if err != nil || len(markets) != 1 || markets[0].LastError != "ok" {
		t.Fatalf("List markets = %+v err=%v", markets, err)
	}
	if missing, err := marketRepo.FindByID(ctx, uuid.New()); err != nil || missing != nil {
		t.Fatalf("missing market = %+v err=%v", missing, err)
	}
	if err := marketRepo.Delete(ctx, market.ID); err != nil {
		t.Fatalf("delete market: %v", err)
	}
}

func TestKnowledgeBaseRepositoryQueries(t *testing.T) {
	ctx := context.Background()
	db := openRepoCRUDTestDB(t)
	repository := NewKnowledgeBaseRepository(db, DBTypeSQLite)
	now := time.Now()
	kb := &model.KnowledgeBase{
		ID:            uuid.New(),
		Name:          "code-graph",
		DisplayName:   "Code Graph",
		Description:   "code knowledge",
		Type:          model.KnowledgeBaseTypeMCP,
		Config:        map[string]string{"url": "http://127.0.0.1/mcp"},
		QueryEndpoint: "/mcp",
		Status:        model.KnowledgeBaseStatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := repository.Create(ctx, kb); err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	got, err := repository.FindByID(ctx, kb.ID)
	if err != nil || got.Config["url"] == "" {
		t.Fatalf("FindByID knowledge = %+v err=%v", got, err)
	}
	got, err = repository.FindByName(ctx, "code-graph")
	if err != nil || got.DisplayName != "Code Graph" {
		t.Fatalf("FindByName knowledge = %+v err=%v", got, err)
	}
	list, total, err := repository.List(ctx, &model.KnowledgeBaseListQuery{Type: string(model.KnowledgeBaseTypeMCP), Status: string(model.KnowledgeBaseStatusActive), Search: "Graph", Page: -1, Size: 1000})
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("List knowledge = %+v total=%d err=%v", list, total, err)
	}
	if err := repository.UpdateQueryStats(ctx, kb.ID); err != nil {
		t.Fatalf("UpdateQueryStats: %v", err)
	}
	active, err := repository.FindByStatus(ctx, model.KnowledgeBaseStatusActive)
	if err != nil || len(active) != 1 || active[0].QueryCount != 1 || active[0].LastQueryAt == nil {
		t.Fatalf("FindByStatus = %+v err=%v", active, err)
	}
	kb.DisplayName = "Updated Graph"
	kb.Status = model.KnowledgeBaseStatusInactive
	if err := repository.Update(ctx, kb); err != nil {
		t.Fatalf("Update knowledge: %v", err)
	}
	inactive, err := repository.FindByStatus(ctx, model.KnowledgeBaseStatusInactive)
	if err != nil || len(inactive) != 1 || inactive[0].DisplayName != "Updated Graph" {
		t.Fatalf("inactive knowledge = %+v err=%v", inactive, err)
	}
	if err := repository.Delete(ctx, kb.ID); err != nil {
		t.Fatalf("Delete knowledge: %v", err)
	}
	if _, err := repository.FindByName(ctx, "code-graph"); err == nil {
		t.Fatalf("deleted knowledge base should not be found")
	}
}

func openRepoCRUDTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	schema := []string{
		`CREATE TABLE base_agents (id TEXT PRIMARY KEY, name TEXT, type TEXT, api_url TEXT, api_token TEXT, default_model TEXT, cli_path TEXT, git_bash_path TEXT, max_tokens INTEGER, timeout_minutes INTEGER, is_default INTEGER, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE local_repos (id TEXT PRIMARY KEY, name TEXT, git_url TEXT, local_path TEXT, branch TEXT, last_commit TEXT, status TEXT, error_message TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE markets (id TEXT PRIMARY KEY, name TEXT, url TEXT, branch TEXT, enabled INTEGER, auto_update INTEGER, check_interval TEXT, last_synced_at TEXT, last_error TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE knowledge_bases (id TEXT PRIMARY KEY, name TEXT, display_name TEXT, description TEXT, type TEXT, config BLOB, query_endpoint TEXT, status TEXT, last_query_at TIMESTAMP, query_count INTEGER, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE skills (id TEXT PRIMARY KEY, name TEXT, description TEXT, tags BLOB, source_type TEXT, source_registry_id TEXT, source_path TEXT, author_id TEXT, project_id TEXT, use_count INTEGER, status TEXT, is_public INTEGER, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE commands (id TEXT PRIMARY KEY, name TEXT, description TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE rules (id TEXT PRIMARY KEY, name TEXT, description TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE subagents (id TEXT PRIMARY KEY, name TEXT, description TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE settings (id TEXT PRIMARY KEY, name TEXT, description TEXT, directory_path TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE agent_configs (id TEXT PRIMARY KEY, name TEXT, role TEXT, description TEXT, system_prompt TEXT, max_tokens INTEGER, temperature REAL, base_agent_id TEXT, is_default INTEGER, is_system INTEGER, requires_human INTEGER, mention_patterns BLOB, config_generated_at TEXT, config_path TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE workflow_templates (id TEXT PRIMARY KEY, name TEXT, description TEXT, agent_ids BLOB, transitions BLOB, checkpoints BLOB, estimated_time TEXT, is_system INTEGER, is_default INTEGER, routable_teams BLOB, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE projects (id TEXT PRIMARY KEY, workflow_template_id TEXT)`,
		`CREATE TABLE team_package_versions (id TEXT PRIMARY KEY, workflow_id TEXT, name TEXT, category TEXT, version TEXT, description TEXT, last_synced_at TEXT, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE agent_skill_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, skill_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_command_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, command_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_subagent_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, subagent_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_rule_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, rule_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_settings_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, settings_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE command_skill_bindings (id TEXT PRIMARY KEY, command_id TEXT, skill_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE subagent_skill_bindings (id TEXT PRIMARY KEY, subagent_id TEXT, skill_id TEXT, created_at TIMESTAMP)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}

type namedRepoItem interface {
	GetNameForTest() string
}

func assertNamedRepoItem[T interface{ *model.Command | *model.Rule | *model.Subagent | *model.Settings }](t *testing.T, item T, err error, want string, label string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s lookup error: %v", label, err)
	}
	var got string
	switch v := any(item).(type) {
	case *model.Command:
		got = v.Name
	case *model.Rule:
		got = v.Name
	case *model.Subagent:
		got = v.Name
	case *model.Settings:
		got = v.Name
	}
	if got != want {
		t.Fatalf("%s name = %q, want %q", label, got, want)
	}
}

func assertUUIDList(t *testing.T, values []uuid.UUID, err error, want uuid.UUID, label string) {
	t.Helper()
	if err != nil || len(values) != 1 || values[0] != want {
		t.Fatalf("%s = %v err=%v, want [%s]", label, values, err, want)
	}
}

func assertExists(t *testing.T, exists bool, err error, label string) {
	t.Helper()
	if err != nil || !exists {
		t.Fatalf("%s exists=%v err=%v", label, exists, err)
	}
}

func mustRepo(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected repo error: %v", err)
	}
}
