package skill

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestSkillScannerBasicHelpers(t *testing.T) {
	storage := t.TempDir()
	scanner := NewSkillScanner(nil, nil, nil, nil, storage, filepath.Join(storage, "agent-configs"), zap.NewNop())

	if scanner.GetLogger() == nil {
		t.Fatalf("logger should be set")
	}
	if scanner.GetStoragePath() != storage {
		t.Fatalf("storage path = %q", scanner.GetStoragePath())
	}
	if scanner.cloneTimeout == 0 || scanner.scanPoolSize == 0 || scanner.importPoolSize == 0 {
		t.Fatalf("scanner defaults not initialized: %#v", scanner)
	}

	cases := map[string]string{
		"Review Buddy":  "review-buddy",
		"123 Analyze!":  "skill-123-analyze",
		"Already-clean": "already-clean",
		"中文技能":          "skill-中文技能",
	}
	for input, want := range cases {
		if got := scanner.cleanSkillName(input); got != want {
			t.Fatalf("cleanSkillName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSkillScannerBuildCloneURL(t *testing.T) {
	scanner := NewSkillScanner(nil, nil, nil, nil, "", "", zap.NewNop())

	github := scanner.buildCloneURL(&model.SkillRegistry{
		Type:       model.RegistryTypeGitHub,
		URL:        "https://github.com/acme/skills",
		AuthConfig: map[string]string{"token": "tok@en:1"},
	})
	encodedToken := url.QueryEscape("tok@en:1")
	if github != "https://"+encodedToken+"@github.com/acme/skills.git" {
		t.Fatalf("github clone url = %q", github)
	}
	if got := scanner.buildCloneURL(&model.SkillRegistry{Type: model.RegistryTypeGitHub, URL: "https://github.com/acme/skills.git"}); got != "https://github.com/acme/skills.git" {
		t.Fatalf("github no token url = %q", got)
	}

	gitlab := scanner.buildCloneURL(&model.SkillRegistry{
		Type:       model.RegistryTypeGitLab,
		URL:        "https://gitlab.com/acme/skills",
		AuthConfig: map[string]string{"token": "gl token"},
	})
	if !strings.HasPrefix(gitlab, "https://oauth2:gl+token@gitlab.com/") || !strings.HasSuffix(gitlab, ".git") {
		t.Fatalf("gitlab clone url = %q", gitlab)
	}

	ssh := "git@codehub-g.example.com:team/skills.git"
	if got := scanner.buildCloneURL(&model.SkillRegistry{Type: model.RegistryTypeCodeHub, URL: ssh}); got != ssh {
		t.Fatalf("codehub ssh url = %q", got)
	}
	codehub := scanner.buildCloneURL(&model.SkillRegistry{
		Type:       model.RegistryTypeCodeHub,
		URL:        "https://codehub.example.com/team/skills.git",
		AuthConfig: map[string]string{"username": "u@corp", "password": "p:a/s"},
	})
	if !strings.HasPrefix(codehub, "https://u%40corp:p%3Aa%2Fs@codehub.example.com/") {
		t.Fatalf("codehub https url = %q", codehub)
	}

	custom := scanner.buildCloneURL(&model.SkillRegistry{Type: model.RegistryTypeCustom, URL: "ssh://example/skills"})
	if custom != "ssh://example/skills" {
		t.Fatalf("custom url = %q", custom)
	}
}

func TestSkillScannerFindAndParseSkills(t *testing.T) {
	root := t.TempDir()
	scanner := NewSkillScanner(nil, nil, nil, nil, "", "", zap.NewNop())

	writeFile(t, filepath.Join(root, "SKILL.md"), `---
name: Root Skill
description: Root description
---
# Ignored`)
	writeFile(t, filepath.Join(root, "nested", "skill.md"), `# Nested Skill

## Description
First line.
Second line.

## Usage
Other section`)
	writeFile(t, filepath.Join(root, ".git", "ignored", "SKILL.md"), `# Should Not Scan`)
	writeFile(t, filepath.Join(root, "bad", "SKILL.md"), `plain text without title`)

	dirs, err := scanner.findSkillDirectories(root)
	if err != nil {
		t.Fatalf("findSkillDirectories returned error: %v", err)
	}
	if len(dirs) != 3 {
		t.Fatalf("skill dirs = %#v", dirs)
	}

	rootSkill, err := scanner.parseSkillFromDir(root, root)
	if err != nil {
		t.Fatalf("parse root skill: %v", err)
	}
	if rootSkill.Name != "root-skill" || rootSkill.Description != "Root description" || rootSkill.Path != "." {
		t.Fatalf("root skill = %#v", rootSkill)
	}

	nested, err := scanner.parseSkillFromDir(filepath.Join(root, "nested"), root)
	if err != nil {
		t.Fatalf("parse nested skill: %v", err)
	}
	if nested.Name != "nested-skill" || nested.Description != "First line. Second line." || nested.Path != "nested" {
		t.Fatalf("nested skill = %#v", nested)
	}

	if _, err := scanner.parseSkillFromDir(filepath.Join(root, "bad"), root); err == nil {
		t.Fatalf("bad skill should fail")
	}
	if _, err := scanner.parseSkillFromDir(filepath.Join(root, "missing"), root); err == nil {
		t.Fatalf("missing skill should fail")
	}

	skills, err := scanner.scanSkillsConcurrent(context.Background(), root)
	if err != nil {
		t.Fatalf("scanSkillsConcurrent returned error: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("concurrent skills = %#v", skills)
	}

	name, desc := scanner.parseSkillMDContent(`---
description: Only description
---
# Fallback Name
## Description
Fallback description`)
	if name != "Fallback Name" || desc != "Fallback description" {
		t.Fatalf("fallback parsed name=%q desc=%q", name, desc)
	}
}

func TestSkillRegistryRefreshAndAccessors(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "handoff")
	writeFile(t, filepath.Join(skillDir, "manifest.yaml"), `name: handoff
version: 1.0.0
description: A2A handoff helper
priority: 5
triggers:
  - type: mention
    description: routes to next agent
validators:
  - name: handoff
    condition: mention
    check: five_parts
    on_fail: reject
token_budget:
  max_handoff_tokens: 1200
  truncation_strategy: summarize
`)
	template := "# Handoff\n\n## Template (强制输出)\n<a2a-handoff>\nbody\n</a2a-handoff>\n\n## Other\nignored"
	writeFile(t, filepath.Join(skillDir, "SKILL.md"), template)
	writeFile(t, filepath.Join(root, "not-a-dir"), "ignored")
	writeFile(t, filepath.Join(root, "no-manifest", "SKILL.md"), "# skipped")

	registry := NewSkillRegistry(root)
	if err := registry.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	got, err := registry.GetSkill(context.Background(), "handoff")
	if err != nil {
		t.Fatalf("GetSkill returned error: %v", err)
	}
	if got == nil || got.Name != "handoff" || got.Template != template || got.Path != skillDir {
		t.Fatalf("skill = %#v", got)
	}

	all, err := registry.GetAllSkills(context.Background())
	if err != nil || len(all) != 1 {
		t.Fatalf("GetAllSkills = %#v err=%v", all, err)
	}
	if tpl, err := registry.GetTemplate(context.Background(), "handoff"); err != nil || tpl != template {
		t.Fatalf("GetTemplate = %q err=%v", tpl, err)
	}
	if tpl, err := registry.GetTemplate(context.Background(), "missing"); err != nil || tpl != "" {
		t.Fatalf("missing template = %q err=%v", tpl, err)
	}
	validators, err := registry.GetValidators(context.Background(), "handoff")
	if err != nil || len(validators) != 1 || validators[0].Name != "handoff" {
		t.Fatalf("validators = %#v err=%v", validators, err)
	}
	budget, err := registry.GetTokenBudget(context.Background(), "handoff")
	if err != nil || budget == nil || budget.MaxHandoffTokens != 1200 || budget.TruncationStrategy != "summarize" {
		t.Fatalf("budget = %#v err=%v", budget, err)
	}

	extracted := ExtractHandoffTemplate(template)
	if !strings.Contains(extracted, "<a2a-handoff>") || strings.Contains(extracted, "## Other") {
		t.Fatalf("extracted template = %q", extracted)
	}
	if got := ExtractHandoffTemplate("plain template"); got != "plain template" {
		t.Fatalf("plain template = %q", got)
	}

	empty := NewSkillRegistry(filepath.Join(root, "missing"))
	if err := empty.Refresh(context.Background()); err != nil {
		t.Fatalf("missing directory refresh should not fail: %v", err)
	}
	if all, err := empty.GetAllSkills(context.Background()); err != nil || len(all) != 0 {
		t.Fatalf("empty registry = %#v err=%v", all, err)
	}
}

func TestSkillValidatorHandoffRules(t *testing.T) {
	validator := NewSkillValidator(nil)

	noMention := validator.ValidateHandoff("普通回复，不触发下游")
	if !noMention.Valid || len(noMention.Errors) != 0 {
		t.Fatalf("no mention result = %#v", noMention)
	}
	if !validator.detectMention("@代码工程师 请继续") || !validator.detectMention("  @reviewer please review") {
		t.Fatalf("detectMention should match line-leading mentions")
	}
	if validator.detectMention("正文里提到 @reviewer 但不是行首") {
		t.Fatalf("inline mention should not route")
	}

	missing := validator.ValidateHandoff("@测试工程师 请继续")
	if missing.Valid || !strings.Contains(strings.Join(missing.Errors, "\n"), "缺少 <a2a-handoff>") {
		t.Fatalf("missing handoff result = %#v", missing)
	}

	validOutput := `@测试工程师
<a2a-handoff>
### To
@测试工程师
### Goal
请验证本次登录流程是否满足验收目标。
### Context
当前实现已经完成主要业务逻辑和错误提示。
### Done
已修改 internal/api/login.go 并补充 docs/login.md。
### Constraints
不要修改数据库迁移，只验证现有行为。
### Task
请运行相关测试并给出审查结论。
</a2a-handoff>`
	valid := validator.ValidateHandoff(validOutput)
	if !valid.Valid {
		t.Fatalf("valid handoff result = %#v", valid)
	}
	if reject, msg := validator.ShouldRejectRouting(validOutput); reject || msg != "" {
		t.Fatalf("valid should not reject: %v %q", reject, msg)
	}
	if prompt := validator.GetRejectionPrompt(validOutput); prompt != "" {
		t.Fatalf("valid rejection prompt = %q", prompt)
	}

	invalid := `@测试工程师
<a2a-handoff>
### Goal
短
### Context
上下文描述足够长
</a2a-handoff>`
	result := validator.ValidateHandoff(invalid)
	if result.Valid || len(result.Errors) == 0 || len(result.Suggestions) == 0 {
		t.Fatalf("invalid result = %#v", result)
	}
	reject, msg := validator.ShouldRejectRouting(invalid)
	if !reject || !strings.Contains(msg, "A2A") {
		t.Fatalf("reject=%v msg=%q", reject, msg)
	}
	if prompt := validator.GetRejectionPrompt(invalid); !strings.Contains(prompt, "请补充交接信息") {
		t.Fatalf("prompt = %q", prompt)
	}

	block, ok := validator.extractHandoffBlock(validOutput)
	if !ok || !strings.Contains(block, "### Goal") {
		t.Fatalf("block=%q ok=%v", block, ok)
	}
	if got := validator.extractPartContent(block, "### Done"); !strings.Contains(got, "internal/api/login.go") {
		t.Fatalf("done content = %q", got)
	}
	result = ValidationResult{Valid: true}
	validator.validateFilePaths("### Done\n没有路径信息", &result)
	if len(result.Suggestions) == 0 {
		t.Fatalf("validateFilePaths should add suggestion")
	}

	if !validator.DetectMentionWithLog("@中文角色 继续") {
		t.Fatalf("DetectMentionWithLog should detect mention")
	}
}

func TestRegistryRefreshErrors(t *testing.T) {
	root := t.TempDir()
	badManifestDir := filepath.Join(root, "bad")
	writeFile(t, filepath.Join(badManifestDir, "manifest.yaml"), "name: [")
	registry := NewSkillRegistry(root)
	if err := registry.Refresh(context.Background()); err == nil {
		t.Fatalf("bad manifest should fail")
	}

	root = t.TempDir()
	missingTemplateDir := filepath.Join(root, "skill")
	writeFile(t, filepath.Join(missingTemplateDir, "manifest.yaml"), "name: broken\n")
	if err := os.Mkdir(filepath.Join(missingTemplateDir, "SKILL.md"), 0755); err != nil {
		t.Fatalf("mkdir skill.md dir: %v", err)
	}
	registry = NewSkillRegistry(root)
	if err := registry.Refresh(context.Background()); err == nil {
		t.Fatalf("directory SKILL.md should fail to read")
	}
}

func TestScanSkillsConcurrentContextCanceled(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "one", "SKILL.md"), "# One\n")
	scanner := NewSkillScanner(nil, nil, nil, nil, "", "", zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := scanner.scanSkillsConcurrent(ctx, root)
	if err != nil {
		t.Fatalf("scan with canceled context returned error: %v", err)
	}
}

func TestNewSkillValidatorAndRegistryAreUsable(t *testing.T) {
	registry := NewSkillRegistry(t.TempDir())
	validator := NewSkillValidator(registry)
	if validator.registry != registry {
		t.Fatalf("validator registry not set")
	}

	scanner := NewSkillScanner(nil, nil, nil, nil, "", "", nil)
	if _, err := scanner.parseSkillFromDir(t.TempDir(), t.TempDir()); err == nil {
		t.Fatalf("empty dir should not parse")
	}

	_ = uuid.New()
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
