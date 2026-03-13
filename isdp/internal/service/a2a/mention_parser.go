package a2a

import (
	"regexp"
	"strings"

	"github.com/anthropic/isdp/internal/model"
)

// Mention @mention解析器
type MentionParser struct {
	mentionRegex *regexp.Regexp
}

// NewMentionParser 创建解析器
func NewMentionParser() *MentionParser {
	return &MentionParser{
		mentionRegex: regexp.MustCompile(`^@(\w+)`),
	}
}

// ParsedMention 解析结果
type ParsedMention struct {
	Role    model.AgentRole
	Content string
}

// ParseMentions 解析消息中的@mentions
// 规则：行首检测，最多2个@mention
func (p *MentionParser) ParseMentions(content string) []ParsedMention {
	var mentions []ParsedMention
	lines := strings.Split(content, "\n")

	mentionCount := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if mentionCount >= 2 {
			break
		}

		matches := p.mentionRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			role := ParseAgentRole(matches[1])
			if role != "" {
				// 提取剩余内容
				remaining := strings.TrimSpace(p.mentionRegex.ReplaceAllString(line, ""))
				mentions = append(mentions, ParsedMention{
					Role:    role,
					Content: remaining,
				})
				mentionCount++
			}
		}
	}

	return mentions
}

// ParseAgentRole 解析Agent角色
func ParseAgentRole(s string) model.AgentRole {
	switch strings.ToLower(s) {
	case "requirement", "req", "需求":
		return model.AgentRoleRequirement
	case "architect", "arch", "架构":
		return model.AgentRoleArchitect
	case "developer", "dev", "开发":
		return model.AgentRoleDeveloper
	case "reviewer", "review", "评审":
		return model.AgentRoleReviewer
	case "testengineer", "test", "测试":
		return model.AgentRoleTestEngineer
	case "devops", "ops", "运维":
		return model.AgentRoleDevOps
	default:
		return ""
	}
}

// ExtractRouting 提取路由信息
func (p *MentionParser) ExtractRouting(content string) (*RoutingInfo, error) {
	mentions := p.ParseMentions(content)

	if len(mentions) == 0 {
		return nil, nil
	}

	info := &RoutingInfo{
		Targets:  make([]model.AgentRole, 0, len(mentions)),
		Messages: make(map[model.AgentRole]string),
	}

	for _, m := range mentions {
		info.Targets = append(info.Targets, m.Role)
		info.Messages[m.Role] = m.Content
	}

	return info, nil
}

// ValidateRouting 验证路由是否有效
func (p *MentionParser) ValidateRouting(fromRole model.AgentRole, toRole model.AgentRole) bool {
	// 获取允许的路由
	allowedRoutes := getAllowedRoutes(fromRole)
	for _, allowed := range allowedRoutes {
		if allowed == toRole {
			return true
		}
	}
	return false
}

// getAllowedRoutes 获取允许的路由
func getAllowedRoutes(role model.AgentRole) []model.AgentRole {
	switch role {
	case model.AgentRoleRequirement:
		return []model.AgentRole{model.AgentRoleArchitect}
	case model.AgentRoleArchitect:
		return []model.AgentRole{model.AgentRoleDeveloper, model.AgentRoleReviewer}
	case model.AgentRoleDeveloper:
		return []model.AgentRole{model.AgentRoleReviewer, model.AgentRoleTestEngineer}
	case model.AgentRoleReviewer:
		return []model.AgentRole{model.AgentRoleDeveloper, model.AgentRoleDevOps}
	case model.AgentRoleTestEngineer:
		return []model.AgentRole{model.AgentRoleDeveloper, model.AgentRoleDevOps}
	case model.AgentRoleDevOps:
		return []model.AgentRole{}
	default:
		return []model.AgentRole{}
	}
}

// RoutingInfo 路由信息
type RoutingInfo struct {
	Targets  []model.AgentRole
	Messages map[model.AgentRole]string
}

// FormatMention 格式化@mention
func FormatMention(role model.AgentRole, message string) string {
	roleStr := roleToString(role)
	if message != "" {
		return "@" + roleStr + " " + message
	}
	return "@" + roleStr
}

// roleToString 角色转字符串
func roleToString(role model.AgentRole) string {
	switch role {
	case model.AgentRoleRequirement:
		return "requirement"
	case model.AgentRoleArchitect:
		return "architect"
	case model.AgentRoleDeveloper:
		return "developer"
	case model.AgentRoleReviewer:
		return "reviewer"
	case model.AgentRoleTestEngineer:
		return "testengineer"
	case model.AgentRoleDevOps:
		return "devops"
	default:
		return ""
	}
}