package skill

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// SkillDefinition Skill 定义
type SkillDefinition struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description"`
	Triggers    []TriggerConfig   `yaml:"triggers"`
	Bindings    []BindingConfig   `yaml:"bindings"`
	Priority    int               `yaml:"priority"`
	Validators  []ValidatorConfig `yaml:"validators"`
	TokenBudget TokenBudgetConfig `yaml:"token_budget"`

	// 加载后的内容
	Template string `yaml:"-"` // 从 SKILL.md 加载
	Path     string `yaml:"-"` // Skill 目录路径
}

// TriggerConfig 触发条件配置
type TriggerConfig struct {
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
}

// BindingConfig 绑定规则配置
type BindingConfig struct {
	Type   string                 `yaml:"type"`
	Scope  string                 `yaml:"scope"`
	Filter map[string]interface{} `yaml:"filter"`
}

// ValidatorConfig 校验规则配置
type ValidatorConfig struct {
	Name      string `yaml:"name"`
	Condition string `yaml:"condition"`
	Check     string `yaml:"check"`
	OnFail    string `yaml:"on_fail"`
}

// TokenBudgetConfig Token 约束配置
type TokenBudgetConfig struct {
	MaxHandoffTokens   int    `yaml:"max_handoff_tokens"`
	TruncationStrategy string `yaml:"truncation_strategy"`
}

// SkillRegistry Skill 注册表
// 参考 clowder-ai cat-cafe-skills 目录结构
type SkillRegistry struct {
	skillsPath string               // skills 目录路径
	skills     map[string]*SkillDefinition // skill name -> definition
	mu         sync.RWMutex

	// 缓存刷新
	lastRefresh time.Time
	ttl         time.Duration
}

// NewSkillRegistry 创建 Skill 注册表
func NewSkillRegistry(skillsPath string) *SkillRegistry {
	return &SkillRegistry{
		skillsPath: skillsPath,
		skills:     make(map[string]*SkillDefinition),
		ttl:        5 * time.Minute, // 缓存 5 分钟
	}
}

// Refresh 从文件系统刷新 Skill 定义
func (r *SkillRegistry) Refresh(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 清空旧数据
	r.skills = make(map[string]*SkillDefinition)

	// 检查 skills 目录是否存在
	if _, err := os.Stat(r.skillsPath); os.IsNotExist(err) {
		// 目录不存在，创建空注册表
		r.lastRefresh = time.Now()
		return nil
	}

	// 遍历 skills 目录下的子目录
	entries, err := os.ReadDir(r.skillsPath)
	if err != nil {
		return fmt.Errorf("failed to read skills directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		skillPath := filepath.Join(r.skillsPath, skillName)

		// 加载 manifest.yaml
		manifestPath := filepath.Join(skillPath, "manifest.yaml")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			continue // 没有 manifest.yaml，跳过
		}

		manifestContent, err := os.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("failed to read manifest for skill %s: %w", skillName, err)
		}

		skill := &SkillDefinition{}
		if err := yaml.Unmarshal(manifestContent, skill); err != nil {
			return fmt.Errorf("failed to parse manifest for skill %s: %w", skillName, err)
		}

		// 加载 SKILL.md
		skillMdPath := filepath.Join(skillPath, "SKILL.md")
		if _, err := os.Stat(skillMdPath); err == nil {
			templateContent, err := os.ReadFile(skillMdPath)
			if err != nil {
				return fmt.Errorf("failed to read SKILL.md for skill %s: %w", skillName, err)
			}
			skill.Template = string(templateContent)
		}

		skill.Path = skillPath
		r.skills[skill.Name] = skill
	}

	r.lastRefresh = time.Now()
	return nil
}

// GetSkill 获取指定 Skill 定义
func (r *SkillRegistry) GetSkill(ctx context.Context, name string) (*SkillDefinition, error) {
	// 检查是否需要刷新
	r.mu.RLock()
	needRefresh := time.Since(r.lastRefresh) > r.ttl || len(r.skills) == 0
	r.mu.RUnlock()

	if needRefresh {
		if err := r.Refresh(ctx); err != nil {
			return nil, err
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, exists := r.skills[name]
	if !exists {
		return nil, nil
	}

	return skill, nil
}

// GetAllSkills 获取所有 Skill 定义
func (r *SkillRegistry) GetAllSkills(ctx context.Context) ([]*SkillDefinition, error) {
	// 检查是否需要刷新
	r.mu.RLock()
	needRefresh := time.Since(r.lastRefresh) > r.ttl || len(r.skills) == 0
	r.mu.RUnlock()

	if needRefresh {
		if err := r.Refresh(ctx); err != nil {
			return nil, err
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*SkillDefinition, 0, len(r.skills))
	for _, skill := range r.skills {
		result = append(result, skill)
	}

	// 按优先级排序（数值越小优先级越高）
	// TODO: 实现排序

	return result, nil
}

// GetTemplate 获取 Skill 模板内容（用于注入 prompt）
func (r *SkillRegistry) GetTemplate(ctx context.Context, name string) (string, error) {
	skill, err := r.GetSkill(ctx, name)
	if err != nil {
		return "", err
	}

	if skill == nil {
		return "", nil
	}

	return skill.Template, nil
}

// ExtractHandoffTemplate 从 SKILL.md 中提取 handoff 模板部分
// 用于注入到 Agent prompt 中
func ExtractHandoffTemplate(template string) string {
	// 查找 Template 部分的代码块
	templateMarker := "## Template (强制输出)"
	idx := strings.Index(template, templateMarker)
	if idx == -1 {
		return template // 没有 Template 部分，返回完整内容
	}

	// 从 Template 开始截取
	start := idx + len(templateMarker)

	// 查找下一个 ## 标记作为结束
	nextSection := strings.Index(template[start:], "## ")
	if nextSection != -1 {
		return strings.TrimSpace(template[start : start+nextSection])
	}

	return strings.TrimSpace(template[start:])
}

// GetValidators 获取 Skill 的校验规则
func (r *SkillRegistry) GetValidators(ctx context.Context, name string) ([]ValidatorConfig, error) {
	skill, err := r.GetSkill(ctx, name)
	if err != nil {
		return nil, err
	}

	if skill == nil {
		return nil, nil
	}

	return skill.Validators, nil
}

// GetTokenBudget 获取 Skill 的 Token 约束配置
func (r *SkillRegistry) GetTokenBudget(ctx context.Context, name string) (*TokenBudgetConfig, error) {
	skill, err := r.GetSkill(ctx, name)
	if err != nil {
		return nil, err
	}

	if skill == nil {
		return nil, nil
	}

	return &skill.TokenBudget, nil
}