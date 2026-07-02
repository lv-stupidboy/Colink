package model

import "testing"

func TestTableNames(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "market", got: (&Market{}).TableName(), want: "markets"},
		{name: "human task", got: (&HumanTask{}).TableName(), want: "human_tasks"},
		{name: "local repo", got: (&LocalRepo{}).TableName(), want: "local_repos"},
		{name: "team package version", got: (&TeamPackageVersion{}).TableName(), want: "team_package_versions"},
		{name: "artifact", got: (&Artifact{}).TableName(), want: "artifacts"},
		{name: "base agent", got: (&BaseAgent{}).TableName(), want: "base_agents"},
		{name: "message", got: (&Message{}).TableName(), want: "messages"},
		{name: "agent invocation", got: (&AgentInvocation{}).TableName(), want: "agent_invocations"},
		{name: "thread", got: (&Thread{}).TableName(), want: "threads"},
		{name: "skill", got: (&Skill{}).TableName(), want: "skills"},
		{name: "agent skill binding", got: (&AgentSkillBinding{}).TableName(), want: "agent_skill_bindings"},
		{name: "skill registry", got: (&SkillRegistry{}).TableName(), want: "skill_registries"},
		{name: "agent role config", got: (&AgentRoleConfig{}).TableName(), want: "agent_configs"},
		{name: "command", got: (&Command{}).TableName(), want: "commands"},
		{name: "agent command binding", got: (&AgentCommandBinding{}).TableName(), want: "agent_command_bindings"},
		{name: "command skill binding", got: (&CommandSkillBinding{}).TableName(), want: "command_skill_bindings"},
		{name: "settings", got: (&Settings{}).TableName(), want: "settings"},
		{name: "agent settings binding", got: (&AgentSettingsBinding{}).TableName(), want: "agent_settings_bindings"},
		{name: "project", got: (&Project{}).TableName(), want: "projects"},
		{name: "rule", got: (&Rule{}).TableName(), want: "rules"},
		{name: "agent rule binding", got: (&AgentRuleBinding{}).TableName(), want: "agent_rule_bindings"},
		{name: "mcp server", got: (&MCPServer{}).TableName(), want: "mcp_servers"},
		{name: "agent mcp binding", got: (&AgentMCPBinding{}).TableName(), want: "agent_mcp_bindings"},
		{name: "subagent", got: (&Subagent{}).TableName(), want: "subagents"},
		{name: "agent subagent binding", got: (&AgentSubagentBinding{}).TableName(), want: "agent_subagent_bindings"},
		{name: "subagent skill binding", got: (&SubagentSkillBinding{}).TableName(), want: "subagent_skill_bindings"},
		{name: "sandbox", got: (&Sandbox{}).TableName(), want: "sandboxes"},
		{name: "session record", got: (SessionRecord{}).TableName(), want: "session_records"},
		{name: "content block", got: (InvocationContentBlock{}).TableName(), want: "invocation_content_blocks"},
		{name: "knowledge", got: (&KnowledgeBase{}).TableName(), want: "knowledge_bases"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("TableName=%q, want %q", tt.got, tt.want)
			}
		})
	}
}
