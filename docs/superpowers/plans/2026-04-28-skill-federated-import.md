# Skill 联邦源导入功能实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 支持从外部联邦源（代码仓库）拉取 Skill，扫描仓库中的 Skill 列表，批量导入到本地。

**Architecture:** Git Clone + 本地并发扫描（限制 10 并发），前端弹窗选择 + 表格批量编辑模式。

**Tech Stack:** Go (Gin) 后端 + React (Ant Design) 前端

---

## 文件结构

### 后端文件

| 文件 | 职责 | 状态 |
|------|------|------|
| `internal/model/skill.go` | 新增 RemoteSkill、BatchImportRequest 结构体 | 修改 |
| `internal/service/skill/skill_scanner.go` | Git Clone + 并发扫描 SKILL.md | 新建 |
| `internal/api/skill_handler.go` | 新增 ScanFederatedSkills、BatchImportFederated API | 修改 |
| `internal/service/skill/registry_service.go` | 扩展 syncFromGitLab 方法 | 修改 |

### 前端文件

| 文件 | 职责 | 状态 |
|------|------|------|
| `web/src/types/index.ts` | 新增 RemoteSkill、ScanResult 类型 | 修改 |
| `web/src/api/client.ts` | 新增 scanFederatedSkills、batchImportFederated 方法 | 修改 |
| `web/src/pages/SkillLibrary/index.tsx` | 新增弹窗逻辑、表格批量编辑模式 | 修改 |

---

## Task 1: 后端数据模型扩展

**Files:**
- Modify: `internal/model/skill.go` (新增 RemoteSkill 和 BatchImportRequest 结构体)

- [ ] **Step 1: 在 skill.go 中添加 RemoteSkill 结构体**

打开 `internal/model/skill.go`，在 `SyncResult` 结构体后面添加：

```go
// RemoteSkill 远程 Skill 信息（扫描结果）
type RemoteSkill struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Path          string `json:"path"`          // Skill 在仓库中的相对路径
	ExistsLocally bool   `json:"existsLocally"` // 是否已存在本地同名 Skill
}

// SkillImportItem 单个 Skill 导入项
type SkillImportItem struct {
	Name            string   `json:"name" binding:"required"`
	Path            string   `json:"path" binding:"required"`
	Description     string   `json:"description"`
	Tags            []string `json:"tags"`
	SupportedAgents []string `json:"supportedAgents" binding:"required,min=1"`
}

// BatchImportRequest 批量导入请求
type BatchImportRequest struct {
	RegistryID string            `json:"registryId" binding:"required"`
	Skills     []SkillImportItem `json:"skills" binding:"required,min=1"`
}

// BatchImportResult 批量导入结果
type BatchImportResult struct {
	Imported []*Skill             `json:"imported"`
	Skipped  []SkippedSkillInfo   `json:"skipped"`
}

// SkippedSkillInfo 跳过的 Skill 信息
type SkippedSkillInfo struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// ScanResult 扫描结果
type ScanResult struct {
	RegistryID   string         `json:"registryId"`
	RegistryName string         `json:"registryName"`
	RegistryURL  string         `json:"registryUrl"`
	Skills       []*RemoteSkill `json:"skills"`
}
```

- [ ] **Step 2: 运行 go build 验证语法**

Run: `cd D:/workspace/isdp && go build ./internal/model`
Expected: 无错误输出

- [ ] **Step 3: Commit**

```bash
git add internal/model/skill.go
git commit -m "feat: add RemoteSkill and BatchImportRequest models for federated import"
```

---

## Task 2: 创建 SkillScanner 服务

**Files:**
- Create: `internal/service/skill/skill_scanner.go` (并发扫描逻辑)

- [ ] **Step 1: 创建 skill_scanner.go 文件**

创建文件 `internal/service/skill/skill_scanner.go`，内容：

```go
package skill

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// SkillScanner 联邦源 Skill 扫描器
type SkillScanner struct {
	registryRepo *repo.SkillRegistryRepository
	skillRepo    *repo.SkillRepository
	storagePath  string
}

// NewSkillScanner 创建 SkillScanner
func NewSkillScanner(
	registryRepo *repo.SkillRegistryRepository,
	skillRepo *repo.SkillRepository,
	storagePath string,
) *SkillScanner {
	return &SkillScanner{
		registryRepo: registryRepo,
		skillRepo:    skillRepo,
		storagePath:  storagePath,
	}
}

// ScanRegistry 扫描联邦源仓库
func (s *SkillScanner) ScanRegistry(ctx context.Context, registryID uuid.UUID) (*model.ScanResult, error) {
	// 1. 获取 Registry 信息
	registry, err := s.registryRepo.FindByID(ctx, registryID)
	if err != nil {
		return nil, fmt.Errorf("获取联邦源失败: %w", err)
	}

	// 2. 创建临时目录
	tempDir := filepath.Join(s.storagePath, ".temp", uuid.New().String())
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 3. 构建带认证的 git clone URL
	cloneURL := s.buildCloneURL(registry)

	// 4. 执行 git clone（使用 --depth 1 减少下载量）
	cloneCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cloneCtx, "git", "clone", "--depth", "1", cloneURL, tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("克隆仓库失败: %s", string(output))
	}

	// 5. 并发扫描 SKILL.md
	skills := s.scanSkillsConcurrent(tempDir)

	// 6. 检查本地是否存在同名 Skill
	for _, skill := range skills {
		existing, err := s.skillRepo.FindByName(ctx, skill.Name)
		if err == nil && existing != nil {
			skill.ExistsLocally = true
		}
	}

	// 7. 异步删除临时目录
	go os.RemoveAll(tempDir)

	// 8. 返回结果
	return &model.ScanResult{
		RegistryID:   registry.ID.String(),
		RegistryName: registry.DisplayName != "" ? registry.DisplayName : registry.Name,
		RegistryURL:  registry.URL,
		Skills:       skills,
	}, nil
}

// buildCloneURL 构建带认证的 clone URL
func (s *SkillScanner) buildCloneURL(registry *model.SkillRegistry) string {
	url := registry.URL

	// 如果有 token，根据类型注入
	if registry.AuthConfig != nil {
		token := registry.AuthConfig["token"]
		if token != "" {
			switch registry.Type {
			case model.RegistryTypeGitHub:
				// GitHub: https://{token}@github.com/owner/repo.git
				if strings.HasPrefix(url, "https://github.com/") {
					return "https://" + token + "@" + strings.TrimPrefix(url, "https://")
				}
			case model.RegistryTypeGitLab:
				// GitLab: https://oauth2:{token}@gitlab.com/owner/repo.git
				if strings.HasPrefix(url, "https://gitlab.com/") || strings.HasPrefix(url, "https://") && strings.Contains(url, "gitlab") {
					return "https://oauth2:" + token + "@" + strings.TrimPrefix(url, "https://")
				}
			}
		}
	}

	return url
}

// scanSkillsConcurrent 并发扫描 SKILL.md（限制 10 并发）
func (s *SkillScanner) scanSkillsConcurrent(repoDir string) []*model.RemoteSkill {
	// 1. 快速遍历找出所有包含 SKILL.md 的目录
	skillDirs := s.findSkillDirectories(repoDir)

	if len(skillDirs) == 0 {
		return []*model.RemoteSkill{}
	}

	// 2. 创建并发池（限制 10 个并发）
	pool := make(chan struct{}, 10)
	results := make(chan *model.RemoteSkill, len(skillDirs))

	var wg sync.WaitGroup

	// 3. 并发解析每个 SKILL.md
	for _, dir := range skillDirs {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			
			// 获取槽位
			pool <- struct{}{}
			defer func() { <-pool }() // 释放槽位

			// 解析 Skill
			skill := s.parseSkillFromDir(d, repoDir)
			if skill != nil {
				results <- skill
			}
		}(dir)
	}

	// 4. 等待所有任务完成
	wg.Wait()
	close(results)

	// 5. 收集结果
	skills := make([]*model.RemoteSkill, 0, len(skillDirs))
	for skill := range results {
		skills = append(skills, skill)
	}

	return skills
}

// findSkillDirectories 遍历仓库找出所有包含 SKILL.md 的目录
func (s *SkillScanner) findSkillDirectories(repoDir string) []string {
	skillDirs := []string{}

	// 首先检查根目录是否有 SKILL.md
	rootSkillMD := filepath.Join(repoDir, "SKILL.md")
	if _, err := os.Stat(rootSkillMD); err == nil {
		skillDirs = append(skillDirs, repoDir)
	}

	// 遍历子目录查找 SKILL.md
	err := filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误继续遍历
		}

		// 跳过根目录（已检查）
		if path == repoDir {
			return nil
		}

		// 跳过 .git 目录
		if strings.Contains(path, ".git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 检查是否是 SKILL.md 文件
		if !info.IsDir() && strings.ToLower(info.Name()) == "skill.md" {
			dir := filepath.Dir(path)
			// 避免重复添加
			for _, existing := range skillDirs {
				if existing == dir {
					return nil
				}
			}
			skillDirs = append(skillDirs, dir)
		}

		return nil
	})

	if err != nil {
		return skillDirs // 返回已找到的结果
	}

	return skillDirs
}

// parseSkillFromDir 解析目录中的 SKILL.md
func (s *SkillScanner) parseSkillFromDir(skillDir, repoDir string) *model.RemoteSkill {
	skillMDPath := filepath.Join(skillDir, "SKILL.md")

	// 检查文件是否存在（大小写不敏感）
	if _, err := os.Stat(skillMDPath); os.IsNotExist(err) {
		// 尝试小写
		skillMDPath = filepath.Join(skillDir, "skill.md")
		if _, err := os.Stat(skillMDPath); os.IsNotExist(err) {
			return nil
		}
	}

	content, err := os.ReadFile(skillMDPath)
	if err != nil {
		return nil
	}

	// 解析 SKILL.md
	metadata := parseSkillMDContent(string(content))

	// 计算相对路径
	relPath, err := filepath.Rel(repoDir, skillDir)
	if err != nil {
		relPath = ""
	}

	return &model.RemoteSkill{
		Name:          metadata.Name,
		Description:   metadata.Description,
		Path:          relPath,
		ExistsLocally: false,
	}
}

// SkillMetadata SKILL.md 元数据
type SkillMetadata struct {
	Name        string
	Description string
}

// parseSkillMDContent 解析 SKILL.md 内容提取元数据
func parseSkillMDContent(content string) SkillMetadata {
	metadata := SkillMetadata{}

	// 1. 尝试从 YAML front matter 提取
	frontMatterRegex := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---`)
	matches := frontMatterRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		frontMatter := matches[1]
		
		// 提取 name
		nameRegex := regexp.MustCompile(`(?m)^name:\s*(.+)$`)
		nameMatches := nameRegex.FindStringSubmatch(frontMatter)
		if len(nameMatches) > 1 {
			metadata.Name = strings.TrimSpace(strings.Trim(nameMatches[1], `"'`))
		}

		// 提取 description
		descRegex := regexp.MustCompile(`(?m)^description:\s*(.+)$`)
		descMatches := descRegex.FindStringSubmatch(frontMatter)
		if len(descMatches) > 1 {
			metadata.Description = strings.TrimSpace(strings.Trim(descMatches[1], `"'`))
		}
	}

	// 2. 如果没有从 front matter 获取到 name，从标题提取
	if metadata.Name == "" {
		titleRegex := regexp.MustCompile(`(?m)^#\s+(.+)$`)
		titleMatches := titleRegex.FindStringSubmatch(content)
		if len(titleMatches) > 1 {
			metadata.Name = strings.TrimSpace(titleMatches[1])
		}
	}

	// 3. 如果没有从 front matter 获取到 description，从 ## Description 提取
	if metadata.Description == "" {
		descRegex := regexp.MustCompile(`(?s)##\s*(?:Description|描述)\s*\n+(.+?)(?:\n##|$)`)
		descMatches := descRegex.FindStringSubmatch(content)
		if len(descMatches) > 1 {
			metadata.Description = strings.TrimSpace(descMatches[1])
		}
	}

	// 4. 清理名称格式（只保留小写字母、数字、中划线）
	if metadata.Name != "" {
		metadata.Name = cleanSkillName(metadata.Name)
	}

	return metadata
}

// cleanSkillName 清理 Skill 名称格式
func cleanSkillName(name string) string {
	// 转小写
	name = strings.ToLower(name)
	// 只保留字母、数字、中划线
	re := regexp.MustCompile(`[^a-z0-9-]`)
	name = re.ReplaceAllString(name, "-")
	// 移除连续的中划线
	name = regexp.MustCompile(`-+`).ReplaceAllString(name, "-")
	// 移除首尾中划线
	name = strings.Trim(name, "-")
	// 确保以字母开头
	if name != "" && !regexp.MustCompile(`^[a-z]`).MatchString(name) {
		name = "s-" + name
	}
	return name
}

// ImportSkills 批量导入 Skill
func (s *SkillScanner) ImportSkills(ctx context.Context, req *model.BatchImportRequest) (*model.BatchImportResult, error) {
	// 1. 获取 Registry 信息
	registryID, err := uuid.Parse(req.RegistryID)
	if err != nil {
		return nil, fmt.Errorf("无效的联邦源 ID: %w", err)
	}

	registry, err := s.registryRepo.FindByID(ctx, registryID)
	if err != nil {
		return nil, fmt.Errorf("获取联邦源失败: %w", err)
	}

	// 2. 创建临时目录
	tempDir := filepath.Join(s.storagePath, ".temp", uuid.New().String())
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 3. 克隆仓库
	cloneURL := s.buildCloneURL(registry)
	cloneCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cloneCtx, "git", "clone", "--depth", "1", cloneURL, tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("克隆仓库失败: %s", string(output))
	}

	result := &model.BatchImportResult{
		Imported: make([]*model.Skill, 0),
		Skipped:  make([]model.SkippedSkillInfo, 0),
	}

	// 4. 并发导入每个 Skill
	pool := make(chan struct{}, 5) // 导入并发限制为 5
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, item := range req.Skills {
		wg.Add(1)
		go func(skillItem model.SkillImportItem) {
			defer wg.Done()

			pool <- struct{}{}
			defer func() { <-pool }()

			// 检查是否已存在
			existing, err := s.skillRepo.FindByName(ctx, skillItem.Name)
			if err == nil && existing != nil {
				mu.Lock()
				result.Skipped = append(result.Skipped, model.SkippedSkillInfo{
					Name:   skillItem.Name,
					Reason: "已存在同名 Skill",
				})
				mu.Unlock()
				return
			}

			// 复制 Skill 文件到本地
			skillDir := filepath.Join(tempDir, skillItem.Path)
			destDir := filepath.Join(s.storagePath, skillItem.Name)

			if err := copySkillDirectory(skillDir, destDir); err != nil {
				mu.Lock()
				result.Skipped = append(result.Skipped, model.SkippedSkillInfo{
					Name:   skillItem.Name,
					Reason: fmt.Sprintf("复制文件失败: %v", err),
				})
				mu.Unlock()
				return
			}

			// 创建 Skill 记录
			skill := &model.Skill{
				ID:               uuid.New(),
				Name:             skillItem.Name,
				Description:      skillItem.Description,
				Tags:             skillItem.Tags,
				SourceType:       model.SkillSourceFederated,
				SourceRegistryID: registryID,
				SupportedAgents:  skillItem.SupportedAgents,
				IsPublic:         true,
				Status:           model.SkillStatusActive,
				UseCount:         0,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}

			if err := s.skillRepo.Create(ctx, skill); err != nil {
				mu.Lock()
				result.Skipped = append(result.Skipped, model.SkippedSkillInfo{
					Name:   skillItem.Name,
					Reason: fmt.Sprintf("创建记录失败: %v", err),
				})
				mu.Unlock()
				os.RemoveAll(destDir)
				return
			}

			mu.Lock()
			result.Imported = append(result.Imported, skill)
			mu.Unlock()
		}(item)
	}

	wg.Wait()

	// 5. 异步删除临时目录
	go os.RemoveAll(tempDir)

	return result, nil
}

// copySkillDirectory 复制 Skill 目录
func copySkillDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过 .git 目录
		if strings.Contains(path, ".git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// 复制文件
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = ioCopy(srcFile, dstFile)
		return err
	})
}

// ioCopy 复制 io（避免导入 io 包冲突）
func ioCopy(dst *os.File, src *os.File) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		nr, err := src.Read(buf)
		if nr > 0 {
			nw, err := dst.Write(buf[0:nr])
			if nw < 0 || nr != nw {
				return written, fmt.Errorf("写入失败")
			}
			written += int64(nw)
		}
		if err != nil {
			break
		}
	}
	return written, nil
}
```

- [ ] **Step 2: 运行 go build 验证语法**

Run: `cd D:/workspace/isdp && go build ./internal/service/skill`
Expected: 无错误输出

- [ ] **Step 3: Commit**

```bash
git add internal/service/skill/skill_scanner.go
git commit -m "feat: add SkillScanner service for federated skill scanning"
```

---

## Task 3: 扩展 SkillHandler API

**Files:**
- Modify: `internal/api/skill_handler.go` (新增 ScanFederatedSkills、BatchImportFederated API)

- [ ] **Step 1: 在 SkillHandler 中添加 Scanner 依赖**

修改 `internal/api/skill_handler.go`，更新 SkillHandler 结构体和构造函数：

```go
// SkillHandler Skill API处理器
type SkillHandler struct {
	skillSvc    *skill.Service
	scanner     *skill.SkillScanner
	storagePath string
	uploadMax   int64
}

// NewSkillHandler 创建SkillHandler
func NewSkillHandler(
	skillSvc *skill.Service,
	scanner *skill.SkillScanner,
	storagePath string,
	uploadMax int64,
) *SkillHandler {
	return &SkillHandler{
		skillSvc:    skillSvc,
		scanner:     scanner,
		storagePath: storagePath,
		uploadMax:   uploadMax,
	}
}
```

- [ ] **Step 2: 添加 ScanFederatedSkills API**

在 `skill_handler.go` 的 `GetBuiltInTags` 方法后添加：

```go
// ScanFederatedSkills 扫描联邦源中的 Skill 列表
func (h *SkillHandler) ScanFederatedSkills(c *gin.Context) {
	var req struct {
		RegistryID string `json:"registryId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择联邦源"})
		return
	}

	registryID, err := uuid.Parse(req.RegistryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的联邦源 ID"})
		return
	}

	result, err := h.scanner.ScanRegistry(c.Request.Context(), registryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// BatchImportFederated 批量导入联邦源 Skill
func (h *SkillHandler) BatchImportFederated(c *gin.Context) {
	var req model.BatchImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.scanner.ImportSkills(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
```

- [ ] **Step 3: 注册新路由**

在 `RegisterRoutes` 方法中添加新路由：

```go
// RegisterRoutes 注册路由
func (h *SkillHandler) RegisterRoutes(r *gin.RouterGroup) {
	skills := r.Group("/skills")
	{
		skills.GET("", h.List)
		skills.GET("/tags", h.GetTags)
		skills.GET("/tags/builtin", h.GetBuiltInTags)
		skills.POST("", h.Create)
		skills.POST("/upload", h.Upload)
		skills.POST("/import/repo", h.ImportFromRepo)
		skills.POST("/import/federated/scan", h.ScanFederatedSkills)      // 新增
		skills.POST("/import/federated/batch", h.BatchImportFederated)    // 新增
		skills.GET("/:id", h.Get)
		skills.PUT("/:id", h.Update)
		skills.DELETE("/:id", h.Delete)
		skills.GET("/:id/agents", h.GetBoundAgents)
	}

	// Agent-Skill 绑定路由（使用独立路径避免与 /agents/:id 冲突）
	agentSkills := r.Group("/agent-skills")
	{
		agentSkills.GET("/:agentId", h.GetAgentSkills)
		agentSkills.POST("/:agentId", h.BindSkills)
		agentSkills.DELETE("/:agentId/:skillId", h.UnbindSkill)
	}
}
```

- [ ] **Step 4: 运行 go build 验证语法**

Run: `cd D:/workspace/isdp && go build ./internal/api`
Expected: 无错误输出

- [ ] **Step 5: Commit**

```bash
git add internal/api/skill_handler.go
git commit -m "feat: add ScanFederatedSkills and BatchImportFederated API endpoints"
```

---

## Task 4: 更新主程序依赖注入

**Files:**
- Modify: `cmd/server/main.go` (更新 SkillHandler 初始化)

- [ ] **Step 1: 查找 SkillHandler 初始化位置**

Run: `grep -n "NewSkillHandler" D:/workspace/isdp/cmd/server/main.go`

- [ ] **Step 2: 更新 SkillHandler 初始化**

找到 SkillHandler 初始化代码，更新为：

```go
// 创建 SkillScanner
skillScanner := skill.NewSkillScanner(skillRegistryRepo, skillRepo, cfg.Skill.StoragePath)

// 创建 SkillHandler
skillHandler := api.NewSkillHandler(skillSvc, skillScanner, cfg.Skill.StoragePath, cfg.Skill.UploadMax)
```

（注：具体行号需要根据实际 main.go 内容调整）

- [ ] **Step 3: 运行 go build 验证**

Run: `cd D:/workspace/isdp && go build ./cmd/server`
Expected: 无错误输出

- [ ] **Step 4: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: update main.go to inject SkillScanner dependency"
```

---

## Task 5: 扩展 GitLab Token 支持

**Files:**
- Modify: `internal/service/skill/registry_service.go` (实现 syncFromGitLab)

- [ ] **Step 1: 实现 syncFromGitLab 方法**

在 `registry_service.go` 的 `syncFromGitLab` 方法中替换 TODO 为实现：

```go
// syncFromGitLab 从 GitLab 同步
func (s *RegistryService) syncFromGitLab(ctx context.Context, registry *model.SkillRegistry) ([]*RemoteSkill, error) {
	// GitLab API 调用
	client := &http.Client{Timeout: 30 * time.Second}

	// 从 URL 提取项目路径
	// URL 格式: https://gitlab.com/owner/repo
	projectPath := strings.TrimPrefix(registry.URL, "https://gitlab.com/")
	projectPath = strings.TrimSuffix(projectPath, ".git")
	projectPath = strings.TrimSuffix(projectPath, "/")

	// GitLab API: 编码项目路径
	encodedPath := urlEncodePath(projectPath)

	// 获取项目 ID
	projectAPI := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s", encodedPath)
	req, err := http.NewRequestWithContext(ctx, "GET", projectAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 添加认证头
	if token, ok := registry.AuthConfig["token"]; ok && token != "" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 GitLab API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitLab API 返回错误: %d", resp.StatusCode)
	}

	var projectInfo struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&projectInfo); err != nil {
		return nil, fmt.Errorf("解析项目信息失败: %w", err)
	}

	// 获取仓库目录树
	treeAPI := fmt.Sprintf("https://gitlab.com/api/v4/projects/%d/repository/tree?recursive=true", projectInfo.ID)
	req, err = http.NewRequestWithContext(ctx, "GET", treeAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	if token, ok := registry.AuthConfig["token"]; ok && token != "" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}

	resp, err = client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 GitLab API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitLab API 返回错误: %d", resp.StatusCode)
	}

	var treeNodes []struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&treeNodes); err != nil {
		return nil, fmt.Errorf("解析目录树失败: %w", err)
	}

	skills := make([]*RemoteSkill, 0)
	for _, node := range treeNodes {
		if node.Type == "blob" && strings.ToLower(node.Name) == "skill.md" {
			// 下载文件内容
			skill, err := s.downloadGitLabSkill(ctx, client, projectInfo.ID, node.Path, registry.AuthConfig["token"])
			if err != nil {
				continue
			}
			skills = append(skills, skill)
		}
	}

	return skills, nil
}

// downloadGitLabSkill 从 GitLab 下载 Skill 文件
func (s *RegistryService) downloadGitLabSkill(ctx context.Context, client *http.Client, projectID int, filePath string, token string) (*RemoteSkill, error) {
	encodedPath := urlEncodePath(filePath)
	fileAPI := fmt.Sprintf("https://gitlab.com/api/v4/projects/%d/repository/files/%s/raw?ref=HEAD", projectID, encodedPath)

	req, err := http.NewRequestWithContext(ctx, "GET", fileAPI, nil)
	if err != nil {
		return nil, err
	}

	if token != "" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败: %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析 Skill 名称（从文件路径提取）
	dirPath := filepath.Dir(filePath)
	skillName := filepath.Base(dirPath)
	if skillName == "" || skillName == "." {
		skillName = strings.TrimSuffix(filePath, "/SKILL.md")
		skillName = filepath.Base(skillName)
	}

	return &RemoteSkill{
		Name:        skillName,
		Description: string(content),
	}, nil
}

// urlEncodePath URL 编码路径（GitLab 要求）
func urlEncodePath(path string) string {
	return strings.ReplaceAll(path, "/", "%2F")
}
```

- [ ] **Step 2: 添加必要的导入**

在 `registry_service.go` 顶部确认导入包含：

```go
import (
	// ... 其他导入
	"net/url"
	"path/filepath"
)
```

- [ ] **Step 3: 运行 go build 验证**

Run: `cd D:/workspace/isdp && go build ./internal/service/skill`
Expected: 无错误输出

- [ ] **Step 4: Commit**

```bash
git add internal/service/skill/registry_service.go
git commit -m "feat: implement syncFromGitLab for GitLab token support"
```

---

## Task 6: 前端类型定义扩展

**Files:**
- Modify: `web/src/types/index.ts` (新增 RemoteSkill、ScanResult 类型)

- [ ] **Step 1: 在 types/index.ts 中添加类型定义**

在 SkillRegistry 类型定义后添加：

```typescript
// ========== 联邦源导入相关类型 ==========

// 远程 Skill 信息（扫描结果）
export interface RemoteSkill {
  name: string;
  description: string;
  path: string;          // Skill 在仓库中的相对路径
  existsLocally: boolean; // 是否已存在本地同名 Skill
}

// 扫描结果
export interface ScanResult {
  registryId: string;
  registryName: string;
  registryUrl: string;
  skills: RemoteSkill[];
}

// Skill 导入项
export interface SkillImportItem {
  name: string;
  path: string;
  description: string;
  tags: string[];
  supportedAgents: string[];
}

// 批量导入请求
export interface BatchImportRequest {
  registryId: string;
  skills: SkillImportItem[];
}

// 批量导入结果
export interface BatchImportResult {
  imported: Skill[];
  skipped: SkippedSkillInfo[];
}

// 跳过的 Skill 信息
export interface SkippedSkillInfo {
  name: string;
  reason: string;
}
```

- [ ] **Step 2: 运行 TypeScript 检查**

Run: `cd D:/workspace/isdp/web && npm run typecheck`
Expected: 无错误输出

- [ ] **Step 3: Commit**

```bash
git add web/src/types/index.ts
git commit -m "feat: add RemoteSkill and ScanResult types for federated import"
```

---

## Task 7: 前端 API Client 扩展

**Files:**
- Modify: `web/src/api/client.ts` (新增 scanFederatedSkills、batchImportFederated 方法)

- [ ] **Step 1: 在 skills 对象中添加新方法**

在 `web/src/api/client.ts` 的 `skills` 对象中，在 `importFederated` 方法后添加：

```typescript
// 扫描联邦源 Skill 列表
scanFederatedSkills: (registryId: string): Promise<ScanResult> =>
  this.request('/skills/import/federated/scan', 'POST', { registryId }),
// 批量导入联邦源 Skill
batchImportFederated: (req: BatchImportRequest): Promise<BatchImportResult> =>
  this.request('/skills/import/federated/batch', 'POST', req),
```

- [ ] **Step 2: 更新类型导入**

确认 `client.ts` 顶部导入包含新类型：

```typescript
import type {
  // ... 其他类型
  ScanResult,
  BatchImportRequest,
  BatchImportResult,
} from '@/types';
```

- [ ] **Step 3: 运行 TypeScript 检查**

Run: `cd D:/workspace/isdp/web && npm run typecheck`
Expected: 无错误输出

- [ ] **Step 4: Commit**

```bash
git add web/src/api/client.ts
git commit -m "feat: add scanFederatedSkills and batchImportFederated API methods"
```

---

## Task 8: 前端弹窗组件实现

**Files:**
- Modify: `web/src/pages/SkillLibrary/index.tsx` (新增 SkillSelectModal 逻辑)

- [ ] **Step 1: 添加状态变量**

在 `SkillLibrary` 组件的状态变量区域添加：

```typescript
// 联邦源导入状态
const [scanModalVisible, setScanModalVisible] = useState(false);
const [scanLoading, setScanLoading] = useState(false);
const [scanResult, setScanResult] = useState<ScanResult | null>(null);
const [selectedRemoteSkills, setSelectedRemoteSkills] = useState<RemoteSkill[]>([]);
const [batchEditMode, setBatchEditMode] = useState(false);
const [batchSkills, setBatchSkills] = useState<SkillImportItem[]>([]);
const [batchImporting, setBatchImporting] = useState(false);
```

- [ ] **Step 2: 添加扫描联邦源函数**

在组件内添加扫描函数：

```typescript
// 扫描联邦源
const handleScanFederated = async () => {
  if (!selectedRegistryId) {
    message.error('请选择联邦源');
    return;
  }
  setScanLoading(true);
  try {
    const result = await api.skills.scanFederatedSkills(selectedRegistryId);
    setScanResult(result);
    setSelectedRemoteSkills([]);
    setScanModalVisible(true);
  } catch (error: any) {
    message.error(error.response?.data?.error || '扫描联邦源失败');
  } finally {
    setScanLoading(false);
  }
};
```

- [ ] **Step 3: 添加确认导入函数**

```typescript
// 确认导入选中的 Skill
const handleConfirmImport = () => {
  if (selectedRemoteSkills.length === 0) {
    message.error('请选择至少一个 Skill');
    return;
  }
  
  setScanModalVisible(false);
  
  // 单选：填入表单
  if (selectedRemoteSkills.length === 1) {
    const skill = selectedRemoteSkills[0];
    setEditingSkill({
      id: '',
      name: skill.name,
      description: skill.description,
      sourceType: 'federated',
      sourceRegistryId: selectedRegistryId,
      isPublic: true,
    } as any);
    setIsAfterUpload(false);
    form.setFieldsValue({
      name: skill.name,
      description: skill.description || '',
      tags: [],
      sourceType: 'federated',
      supportedAgents: [],
      isPublic: true,
    });
    setSourceType('federated');
    setModalVisible(true);
  } else {
    // 多选：切换为批量编辑模式
    setBatchSkills(selectedRemoteSkills.map(s => ({
      name: s.name,
      path: s.path,
      description: s.description,
      tags: [],
      supportedAgents: [],
    })));
    setBatchEditMode(true);
    setModalVisible(true);
  }
};
```

- [ ] **Step 4: 添加批量保存函数**

```typescript
// 批量保存 Skill
const handleBatchSave = async (skills: SkillImportItem[], unifiedTags?: string[], unifiedAgents?: string[]) => {
  setBatchImporting(true);
  try {
    // 应用统一设置
    const finalSkills = skills.map(s => ({
      ...s,
      tags: unifiedTags ? [...s.tags, ...unifiedTags] : s.tags,
      supportedAgents: unifiedAgents || s.supportedAgents,
    }));
    
    const result = await api.skills.batchImportFederated({
      registryId: selectedRegistryId,
      skills: finalSkills,
    });
    
    message.success(`成功导入 ${result.imported.length} 个 Skill`);
    if (result.skipped.length > 0) {
      message.warning(`跳过 ${result.skipped.length} 个：${result.skipped.map(s => s.name).join(', ')}`);
    }
    
    setModalVisible(false);
    setBatchEditMode(false);
    setBatchSkills([]);
    loadSkills();
  } catch (error: any) {
    message.error(error.response?.data?.error || '批量导入失败');
  } finally {
    setBatchImporting(false);
  }
};
```

- [ ] **Step 5: Commit 中间进度**

```bash
git add web/src/pages/SkillLibrary/index.tsx
git commit -m "feat: add federated scan and batch import state/logic to SkillLibrary"
```

---

## Task 9: 前端弹窗 UI 实现

**Files:**
- Modify: `web/src/pages/SkillLibrary/index.tsx` (新增弹窗组件渲染)

- [ ] **Step 1: 添加扫描弹窗渲染**

在组件末尾 Modal 之后添加扫描弹窗：

```typescript
{/* 联邦源扫描弹窗 */}
<Modal
  title="从联邦源导入 Skill"
  open={scanModalVisible}
  onCancel={() => setScanModalVisible(false)}
  width={600}
  footer={[
    <Button key="cancel" onClick={() => setScanModalVisible(false)}>取消</Button>,
    <Button key="confirm" type="primary" onClick={handleConfirmImport} disabled={selectedRemoteSkills.length === 0}>
      确认导入（已选择 {selectedRemoteSkills.length} 个）
    </Button>,
  ]}
>
  {scanResult && (
    <>
      <div style={{ marginBottom: 12 }}>
        <Text strong>联邦源：</Text>{scanResult.registryName}
        <br />
        <Text type="secondary">{scanResult.registryUrl}</Text>
      </div>
      <List
        dataSource={scanResult.skills}
        renderItem={(skill) => (
          <List.Item
            style={{ 
              opacity: skill.existsLocally ? 0.5 : 1,
              background: skill.existsLocally ? 'var(--ant-color-bg-container-disabled)' : undefined,
            }}
          >
            <Checkbox
              disabled={skill.existsLocally}
              checked={selectedRemoteSkills.some(s => s.name === skill.name)}
              onChange={(e) => {
                if (e.target.checked) {
                  setSelectedRemoteSkills([...selectedRemoteSkills, skill]);
                } else {
                  setSelectedRemoteSkills(selectedRemoteSkills.filter(s => s.name !== skill.name));
                }
              }}
            >
              <div>
                <Text strong>{skill.name}</Text>
                {skill.existsLocally && <Tag color="default" style={{ marginLeft: 8 }}>已存在本地</Tag>}
                <br />
                <Text type="secondary" style={{ fontSize: 12 }}>{skill.description || '暂无描述'}</Text>
              </div>
            </Checkbox>
          </List.Item>
        )}
      />
    </>
  )}
</Modal>
```

- [ ] **Step 2: 添加批量编辑表格渲染**

修改主 Modal 的内容，在 `renderCreateMethodSelector` 后添加批量编辑模式：

```typescript
{batchEditMode && batchSkills.length > 0 && (
  <div style={{ marginBottom: 16 }}>
    <Text strong>已选择 {batchSkills.length} 个 Skill，请补充属性信息：</Text>
    
    {/* 统一设置 */}
    <div style={{ marginTop: 12, padding: 12, background: 'var(--ant-color-bg-container)', borderRadius: 8 }}>
      <Checkbox
        checked={unifySettings}
        onChange={(e) => setUnifySettings(e.target.checked)}
      >
        统一设置：为所有 Skill 应用相同的标签和 Agent
      </Checkbox>
      {unifySettings && (
        <Space style={{ marginTop: 8, width: '100%' }} direction="vertical">
          <Select
            mode="tags"
            placeholder="统一标签"
            style={{ width: '100%' }}
            value={unifiedTags}
            onChange={setUnifiedTags}
          />
          <Select
            mode="multiple"
            placeholder="统一 Agent"
            style={{ width: '100%' }}
            options={agentTypeOptions}
            value={unifiedAgents}
            onChange={setUnifiedAgents}
          />
        </Space>
      )}
    </div>
    
    {/* Skill 表格 */}
    <Table
      dataSource={batchSkills}
      columns={[
        {
          title: '名称',
          dataIndex: 'name',
          key: 'name',
          width: 150,
          render: (name: string, record, index) => (
            <Input
              value={name}
              onChange={(e) => {
                const updated = [...batchSkills];
                updated[index].name = e.target.value;
                setBatchSkills(updated);
              }}
            />
          ),
        },
        {
          title: '描述',
          dataIndex: 'description',
          key: 'description',
          render: (desc: string, record, index) => (
            <Input
              value={desc}
              onChange={(e) => {
                const updated = [...batchSkills];
                updated[index].description = e.target.value;
                setBatchSkills(updated);
              }}
            />
          ),
        },
        {
          title: '标签',
          dataIndex: 'tags',
          key: 'tags',
          width: 150,
          render: (_, record, index) => (
            unifySettings ? (
              <Text type="secondary">使用统一设置</Text>
            ) : (
              <Select
                mode="tags"
                placeholder="标签"
                style={{ width: '100%' }}
                value={batchSkills[index].tags}
                onChange={(val) => {
                  const updated = [...batchSkills];
                  updated[index].tags = val;
                  setBatchSkills(updated);
                }}
              />
            )
          ),
        },
        {
          title: 'Agent',
          dataIndex: 'supportedAgents',
          key: 'supportedAgents',
          width: 150,
          render: (_, record, index) => (
            unifySettings ? (
              <Text type="secondary">使用统一设置</Text>
            ) : (
              <Select
                mode="multiple"
                placeholder="Agent"
                style={{ width: '100%' }}
                options={agentTypeOptions}
                value={batchSkills[index].supportedAgents}
                onChange={(val) => {
                  const updated = [...batchSkills];
                  updated[index].supportedAgents = val;
                  setBatchSkills(updated);
                }}
              />
            )
          ),
        },
      ]}
      rowKey="name"
      pagination={false}
      size="small"
    />
  </div>
)}
```

- [ ] **Step 3: 添加统一设置状态**

```typescript
const [unifySettings, setUnifySettings] = useState(false);
const [unifiedTags, setUnifiedTags] = useState<string[]>([]);
const [unifiedAgents, setUnifiedAgents] = useState<string[]>([]);
```

- [ ] **Step 4: 修改 Modal footer 支持批量保存**

```typescript
footer={batchEditMode ? [
  <Button key="cancel" onClick={() => { setBatchEditMode(false); setModalVisible(false); }}>取消</Button>,
  <Button key="save" type="primary" loading={batchImporting} onClick={() => handleBatchSave(batchSkills, unifySettings ? unifiedTags : undefined, unifySettings ? unifiedAgents : undefined)}>
    保存全部
  </Button>,
] : undefined}
```

- [ ] **Step 5: 运行前端构建验证**

Run: `cd D:/workspace/isdp/web && npm run typecheck`
Expected: 无错误输出

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/SkillLibrary/index.tsx
git commit -m "feat: add scan modal and batch edit table UI for federated import"
```

---

## Task 10: 修改联邦源导入按钮交互

**Files:**
- Modify: `web/src/pages/SkillLibrary/index.tsx` (修改"导入"按钮触发扫描)

- [ ] **Step 1: 修改 handleFederatedImport 函数**

将原有的 `handleFederatedImport` 替换为触发扫描：

```typescript
// 联邦源导入（触发扫描）
const handleFederatedImport = async () => {
  if (!selectedRegistryId) {
    message.error('请选择联邦源');
    return;
  }
  await handleScanFederated();
};
```

- [ ] **Step 2: 运行前端构建验证**

Run: `cd D:/workspace/isdp/web && npm run build`
Expected: 构建成功

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/SkillLibrary/index.tsx
git commit -m "feat: change federated import button to trigger scan"
```

---

## Task 11: 集成测试

**Files:**
- 无文件变更，纯测试验证

- [ ] **Step 1: 启动后端服务**

Run: `cd D:/workspace/isdp && go run ./cmd/server`

- [ ] **Step 2: 启动前端开发服务器**

Run: `cd D:/workspace/isdp/web && npm run dev`

- [ ] **Step 3: 手动测试完整流程**

测试步骤：
1. 打开浏览器访问前端
2. 进入 Skills 管理页面
3. 点击"新建 Skill"
4. 选择来源"联邦"
5. 选择一个联邦源
6. 点击"导入"按钮
7. 等待扫描完成，观察弹窗列表
8. 勾选多个 Skill
9. 点击"确认导入"
10. 观察表格批量编辑界面
11. 设置统一 Agent
12. 点击"保存全部"
13. 验证 Skill 列表刷新，新 Skill 出现

- [ ] **Step 4: 最终 Commit**

```bash
git add -A
git commit -m "feat: complete skill federated import feature with batch edit mode"
```

---

## Self-Review

**1. Spec coverage:**
- ✅ API 设计（扫描 API、批量导入 API）→ Task 3
- ✅ Skill 识别规则（混合模式）→ Task 2 scanSkillsConcurrent
- ✅ 并发扫描优化（10 并发）→ Task 2
- ✅ Git Clone + Token 注入 → Task 2 buildCloneURL
- ✅ GitLab Token 支持 → Task 5
- ✅ 前端弹窗 UI → Task 9
- ✅ 表格批量编辑 → Task 9
- ✅ 单选/多选区分 → Task 8 handleConfirmImport
- ✅ 统一设置功能 → Task 9

**2. Placeholder scan:**
- 无 TBD/TODO
- 无 "implement later"
- 无 "add validation" 空泛描述
- 所有代码步骤都有完整代码块

**3. Type consistency:**
- RemoteSkill 结构体 → Go: internal/model/skill.go, TS: web/src/types/index.ts
- ScanResult 结构体 → Go: internal/model/skill.go, TS: web/src/types/index.ts
- BatchImportRequest → Go: internal/model/skill.go, TS: web/src/types/index.ts
- SkillImportItem → Go: SkillImportItem, TS: SkillImportItem
- 所有类型名称一致

---

**Plan complete and saved to `docs/superpowers/plans/2026-04-28-skill-federated-import.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**