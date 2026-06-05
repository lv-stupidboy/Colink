package memory

import (
	"regexp"
)

var commandPattern = regexp.MustCompile(`(?i)\b(go test|npm run|pnpm|yarn|make|docker|kubectl|git)\b`)

type memoryTopic struct {
	Key      string
	Title    string
	Filename string
	Summary  string
}

func deriveMemoryTopic(entry MemoryEntry) memoryTopic {
	if key := slugForFilename(entry.Topic); key != "" {
		return memoryTopic{Key: key, Title: topicTitleFromKey(key), Filename: key + ".md", Summary: conciseMemorySummary(entry)}
	}
	if containsString(entry.Tags, "agent-role") {
		return memoryTopic{Key: "agent_roles", Title: "Agent 角色职责", Filename: "agent_roles.md", Summary: conciseMemorySummary(entry)}
	}
	if containsString(entry.Tags, "user-preference") || containsString(entry.Tags, "preference") {
		return memoryTopic{Key: "user_preferences", Title: "用户偏好", Filename: "user_preferences.md", Summary: conciseMemorySummary(entry)}
	}
	if containsString(entry.Tags, "port") || containsString(entry.Tags, "port-constraint") {
		return memoryTopic{Key: "port_constraints", Title: "端口约束", Filename: "port_constraints.md", Summary: conciseMemorySummary(entry)}
	}
	if containsString(entry.Tags, "collaboration") || containsString(entry.Tags, "test") || containsString(entry.Tags, "refactor") {
		key := collaborationTopicKey(entry)
		return memoryTopic{Key: key, Title: topicTitleFromKey(key), Filename: key + ".md", Summary: conciseMemorySummary(entry)}
	}
	if commandPattern.MatchString(entry.Memory) || containsAnyFold(entry.Memory, []string{"命令", "启动", "测试命令", "构建"}) {
		key := "project_commands"
		if entry.Type == MemoryTypeTeam {
			key = "team_commands"
		}
		return memoryTopic{Key: key, Title: topicTitleFromKey(key), Filename: key + ".md", Summary: conciseMemorySummary(entry)}
	}
	key := slugForFilename(memoryTitle(entry))
	if key == "" {
		key = "general_memory"
	}
	return memoryTopic{Key: key, Title: topicTitleFromKey(key), Filename: key + ".md", Summary: conciseMemorySummary(entry)}
}

func collaborationTopicKey(entry MemoryEntry) string {
	if containsString(entry.Tags, "test") {
		return "memory_test_rules"
	}
	if containsString(entry.Tags, "refactor") {
		return "refactor_collaboration_rules"
	}
	return "collaboration_rules"
}

func topicTitleFromKey(key string) string {
	switch key {
	case "agent_roles":
		return "Agent 角色职责"
	case "user_preferences":
		return "用户偏好"
	case "port_constraints":
		return "端口约束"
	case "memory_test_rules":
		return "记忆测试协作规则"
	case "refactor_collaboration_rules":
		return "重构协作规则"
	case "collaboration_rules":
		return "团队协作规则"
	case "project_commands":
		return "项目命令约定"
	case "team_commands":
		return "团队命令约定"
	default:
		return humanizeID(key)
	}
}
