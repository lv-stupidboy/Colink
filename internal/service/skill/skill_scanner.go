package skill

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	pkgexec "github.com/anthropic/isdp/pkg/exec"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Package-level regex patterns for skill name cleaning (compiled once)
var (
	nonAlphaNumRegex      = regexp.MustCompile(`[^a-z0-9-]`)
	consecutiveDashRegex  = regexp.MustCompile(`-+`)
	startsWithLetterRegex = regexp.MustCompile(`^[a-z]`)
)

// SkillScanner 联邦技能扫描服务
type SkillScanner struct {
	registryRepo    *repo.SkillRegistryRepository
	skillRepo       *repo.SkillRepository
	bindingRepo     *repo.AgentSkillBindingRepository // 角色-Skill 关联
	agentConfigRepo *repo.AgentConfigRepository       // 角色配置
	storagePath     string
	agentConfigPath string                            // agent-configs 目录路径
	logger          *zap.Logger
	cloneTimeout    time.Duration // git clone 超时时间
	scanPoolSize    int           // 扫描并发数
	importPoolSize  int           // 导入并发数
}

// NewSkillScanner 创建 SkillScanner
func NewSkillScanner(
	registryRepo *repo.SkillRegistryRepository,
	skillRepo *repo.SkillRepository,
	bindingRepo *repo.AgentSkillBindingRepository,
	agentConfigRepo *repo.AgentConfigRepository,
	storagePath string,
	agentConfigPath string,
	logger *zap.Logger,
) *SkillScanner {
	return &SkillScanner{
		registryRepo:    registryRepo,
		skillRepo:       skillRepo,
		bindingRepo:     bindingRepo,
		agentConfigRepo: agentConfigRepo,
		storagePath:     storagePath,
		agentConfigPath: agentConfigPath,
		logger:          logger,
		cloneTimeout:    120 * time.Second, // 默认120秒
		scanPoolSize:    10,                 // 默认扫描并发数
		importPoolSize:  5,                  // 默认导入并发数（导入较慢）
	}
}

// GetLogger 获取 logger 实例
func (s *SkillScanner) GetLogger() *zap.Logger {
	return s.logger
}

// GetStoragePath 获取 skill 存储路径
func (s *SkillScanner) GetStoragePath() string {
	return s.storagePath
}

// ScanRegistry 扫描注册表中的技能
func (s *SkillScanner) ScanRegistry(ctx context.Context, registryID uuid.UUID) (*model.ScanResult, error) {
	s.logger.Info("开始扫描联邦源技能",
		zap.String("registryId", registryID.String()),
		zap.Duration("cloneTimeout", s.cloneTimeout),
	)

	// 获取注册表信息
	registry, err := s.registryRepo.FindByID(ctx, registryID)
	if err != nil {
		s.logger.Error("获取注册表失败",
			zap.String("registryId", registryID.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取注册表失败: %w", err)
	}

	s.logger.Info("获取注册表成功",
		zap.String("registryId", registryID.String()),
		zap.String("registryName", registry.Name),
		zap.String("registryURL", registry.URL),
		zap.String("registryType", string(registry.Type)),
	)

	// 创建临时目录
	tempUUID := uuid.New()
	tempDir := filepath.Join(s.storagePath, ".temp", tempUUID.String())
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		s.logger.Error("创建临时目录失败",
			zap.String("tempDir", tempDir),
			zap.Error(err),
		)
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 构建克隆URL（带Token注入）
	cloneURL := s.buildCloneURL(registry)

	s.logger.Info("开始执行 git clone",
		zap.String("registryId", registryID.String()),
		zap.String("cloneURL", cloneURL),
		zap.String("tempDir", tempDir),
	)

	// 执行 git clone --depth 1
	cloneCtx, cancel := context.WithTimeout(ctx, s.cloneTimeout)
	defer cancel()

	cmd := pkgexec.GitCommandContext(cloneCtx, "git", "clone", "--depth", "1", cloneURL, tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 清理临时目录
		go os.RemoveAll(tempDir)

		// 记录详细错误信息
		s.logger.Error("git clone 失败",
			zap.String("registryId", registryID.String()),
			zap.String("cloneURL", cloneURL),
			zap.String("tempDir", tempDir),
			zap.Error(err),
			zap.String("gitOutput", string(output)),
			zap.Bool("isTimeout", cloneCtx.Err() == context.DeadlineExceeded),
		)

		if cloneCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("git clone 超时 (%v秒): 请检查仓库地址是否正确或网络是否通畅", s.cloneTimeout.Seconds())
		}
		return nil, fmt.Errorf("git clone 失败: %s", string(output))
	}

	s.logger.Info("git clone 成功",
		zap.String("registryId", registryID.String()),
		zap.String("tempDir", tempDir),
	)

	// 扫描技能目录
	skills, err := s.scanSkillsConcurrent(ctx, tempDir)
	if err != nil {
		// 清理临时目录
		go os.RemoveAll(tempDir)
		s.logger.Error("扫描技能目录失败",
			zap.String("registryId", registryID.String()),
			zap.String("tempDir", tempDir),
			zap.Error(err),
		)
		return nil, fmt.Errorf("扫描技能失败: %w", err)
	}

	s.logger.Info("扫描技能目录完成",
		zap.String("registryId", registryID.String()),
		zap.Int("skillCount", len(skills)),
	)

	// 检查每个技能是否本地已存在
	for _, skill := range skills {
		existing, err := s.skillRepo.FindByName(ctx, skill.Name)
		if err == nil && existing != nil {
			skill.ExistsLocally = true
				// 填充本地 Skill 详情
				skill.LocalSkill = &model.LocalSkillInfo{
					ID:          existing.ID,
					SourceType:  string(existing.SourceType),
					Description: existing.Description,
				}
				// 如果是 federated 类型，查询联邦源名称
				if existing.SourceType == model.SkillSourceFederated && existing.SourceRegistryID != uuid.Nil {
					registry, err := s.registryRepo.FindByID(ctx, existing.SourceRegistryID)
					if err == nil && registry != nil {
						skill.LocalSkill.SourceRegistryID = existing.SourceRegistryID
						skill.LocalSkill.SourceRegistryName = registry.Name
					}
				}
		} else {
			skill.ExistsLocally = false
		}
	}

	// 异步删除临时目录
	go os.RemoveAll(tempDir)

	s.logger.Info("扫描联邦源技能完成",
		zap.String("registryId", registryID.String()),
		zap.String("registryName", registry.Name),
		zap.Int("totalSkills", len(skills)),
	)

	// 返回扫描结果
	return &model.ScanResult{
		RegistryID:   registry.ID,
		RegistryName: registry.Name,
		RegistryURL:  registry.URL,
		Skills:       skills,
	}, nil
}

// buildCloneURL 构建克隆URL（带认证注入）
func (s *SkillScanner) buildCloneURL(registry *model.SkillRegistry) string {
	// 根据注册表类型构建URL
	switch registry.Type {
	case model.RegistryTypeGitHub:
		// GitHub: https://{token}@github.com/owner/repo.git
		token := ""
		if registry.AuthConfig != nil {
			token = registry.AuthConfig["token"]
		}
		if token == "" {
			return registry.URL
		}
		urlStr := registry.URL
		if !strings.HasSuffix(urlStr, ".git") {
			urlStr = urlStr + ".git"
		}
		// URL 编码 Token（防止特殊字符破坏 URL 结构）
		encodedToken := url.QueryEscape(token)
		return strings.Replace(urlStr, "https://", fmt.Sprintf("https://%s@", encodedToken), 1)

	case model.RegistryTypeGitLab:
		// GitLab: https://oauth2:{token}@gitlab.com/owner/repo.git
		token := ""
		if registry.AuthConfig != nil {
			token = registry.AuthConfig["token"]
		}
		if token == "" {
			return registry.URL
		}
		urlStr := registry.URL
		if !strings.HasSuffix(urlStr, ".git") {
			urlStr = urlStr + ".git"
		}
		// URL 编码 Token（防止特殊字符破坏 URL 结构）
		encodedToken := url.QueryEscape(token)
		return strings.Replace(urlStr, "https://", fmt.Sprintf("https://oauth2:%s@", encodedToken), 1)

	case model.RegistryTypeCodeHub:
		// 华为内网 CodeHub 支持 SSH 和 HTTPS 认证
		urlStr := registry.URL

		// SSH 格式：直接使用，依赖系统 SSH Key
		if strings.HasPrefix(urlStr, "git@") {
			return urlStr
		}

		// HTTPS 格式：注入账号密码
		if strings.HasPrefix(urlStr, "https://") {
			username := ""
			password := ""
			if registry.AuthConfig != nil {
				username = registry.AuthConfig["username"]
				password = registry.AuthConfig["password"]
			}

			if username != "" && password != "" {
				// URL 编码用户名和密码（防止特殊字符破坏 URL 结构）
				// 特殊字符如 @, :, #, %, / 等必须编码
				encodedUsername := url.QueryEscape(username)
				encodedPassword := url.QueryEscape(password)
				// https://{encoded_username}:{encoded_password}@codehub-g.huawei.com/xxx.git
				urlStr = strings.Replace(urlStr, "https://",
					fmt.Sprintf("https://%s:%s@", encodedUsername, encodedPassword), 1)
			}
			return urlStr
		}

		return urlStr

	default:
		// 其他类型使用原始URL
		return registry.URL
	}
}

// scanSkillsConcurrent 并发扫描技能目录
func (s *SkillScanner) scanSkillsConcurrent(ctx context.Context, repoDir string) ([]*model.RemoteSkill, error) {
	// 首先找到所有包含 SKILL.md 的目录
	skillDirs, err := s.findSkillDirectories(repoDir)
	if err != nil {
		return nil, fmt.Errorf("查找技能目录失败: %w", err)
	}

	if len(skillDirs) == 0 {
		return []*model.RemoteSkill{}, nil
	}

	// 使用 goroutine pool 进行并发解析
	results := make([]*model.RemoteSkill, 0, len(skillDirs))
	resultChan := make(chan *model.RemoteSkill, len(skillDirs))
	errChan := make(chan error, len(skillDirs))

	// 创建信号量控制并发数
	sem := make(chan struct{}, s.scanPoolSize)
	var wg sync.WaitGroup

	for _, skillDir := range skillDirs {
		wg.Add(1)
		go func(dir string) {
			defer wg.Done()
			// Check context cancellation before starting work
			select {
			case <-ctx.Done():
				return // skip work if context cancelled
			case sem <- struct{}{}:
			}
			defer func() { <-sem }() // 释放信号量

			skill, err := s.parseSkillFromDir(dir, repoDir)
			if err != nil {
				errChan <- err
				return
			}
			resultChan <- skill
		}(skillDir)
	}

	// 等待所有任务完成
	wg.Wait()
	close(resultChan)
	close(errChan)

	// 收集结果
	for skill := range resultChan {
		results = append(results, skill)
	}

	// 检查是否有错误（忽略单个文件的解析错误）
	for err := range errChan {
		s.logger.Warn("解析技能失败", zap.Error(err))
	}

	return results, nil
}

// findSkillDirectories 查找所有包含 SKILL.md 的目录
func (s *SkillScanner) findSkillDirectories(repoDir string) ([]string, error) {
	skillDirs := make([]string, 0)

	// 检查根目录是否有 SKILL.md
	rootSkillMD := filepath.Join(repoDir, "SKILL.md")
	rootSkillMDLower := filepath.Join(repoDir, "skill.md")
	if _, err := os.Stat(rootSkillMD); err == nil {
		skillDirs = append(skillDirs, repoDir)
	} else if _, err := os.Stat(rootSkillMDLower); err == nil {
		skillDirs = append(skillDirs, repoDir)
	}

	// 遍历子目录，跳过 .git
	err := filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过 .git 目录
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// 只检查目录
		if !info.IsDir() {
			return nil
		}

		// 跳过根目录（已检查）
		if path == repoDir {
			return nil
		}

		// 检查是否存在 SKILL.md 或 skill.md
		skillMD := filepath.Join(path, "SKILL.md")
		skillMDLower := filepath.Join(path, "skill.md")
		if _, err := os.Stat(skillMD); err == nil {
			skillDirs = append(skillDirs, path)
		} else if _, err := os.Stat(skillMDLower); err == nil {
			skillDirs = append(skillDirs, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("遍历目录失败: %w", err)
	}

	return skillDirs, nil
}

// parseSkillFromDir 从目录解析 SKILL.md
func (s *SkillScanner) parseSkillFromDir(skillDir, repoDir string) (*model.RemoteSkill, error) {
	// 处理大小写不敏感的文件名
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	skillMDPathLower := filepath.Join(skillDir, "skill.md")

	var filePath string
	if _, err := os.Stat(skillMDPath); err == nil {
		filePath = skillMDPath
	} else if _, err := os.Stat(skillMDPathLower); err == nil {
		filePath = skillMDPathLower
	} else {
		return nil, fmt.Errorf("目录 %s 中未找到 SKILL.md", skillDir)
	}

	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取 SKILL.md 失败: %w", err)
	}

	// 计算相对路径
	relPath, err := filepath.Rel(repoDir, skillDir)
	if err != nil {
		relPath = skillDir // 如果无法计算相对路径，使用绝对路径
	}

	// 解析技能内容
	name, description := s.parseSkillMDContent(string(content))

	// 清理名称
	name = s.cleanSkillName(name)
	if name == "" {
		return nil, fmt.Errorf("无法从 SKILL.md 提取有效的技能名称: %s", filePath)
	}

	return &model.RemoteSkill{
		Name:          name,
		Description:   description,
		Path:          relPath,
		ExistsLocally: false, // 由 ScanRegistry 方法设置
	}, nil
}

// parseSkillMDContent 解析 SKILL.md 内容，提取名称和描述
func (s *SkillScanner) parseSkillMDContent(content string) (name string, description string) {
	// 尝试解析 YAML front matter
	// 格式: ---\nname: xxx\ndescription: xxx\n---
	if strings.HasPrefix(content, "---") {
		// 找到结束的 ---
		endIndex := strings.Index(content[3:], "---")
		if endIndex != -1 {
			frontMatter := content[3:endIndex+3]
			// 简单解析 YAML（不使用 yaml 库）
			lines := strings.Split(frontMatter, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "name:") {
					name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
					// 去除可能的引号
					name = strings.Trim(name, "\"'")
				}
				if strings.HasPrefix(line, "description:") {
					description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
					// 去除可能的引号
					description = strings.Trim(description, "\"'")
				}
			}
			// 如果从 front matter 提取到名称，直接返回
			if name != "" {
				return name, description
			}
			// 移除 front matter，继续解析
			content = content[endIndex+6:]
		}
	}

	// Fallback: 从 # Title 提取名称
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 匹配标题行
		if strings.HasPrefix(line, "# ") && !strings.HasPrefix(line, "## ") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			break
		}
	}

	// Fallback: 从 ## Description 提取描述
	descFound := false
	descLines := make([]string, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## Description") || strings.HasPrefix(line, "## 描述") {
			descFound = true
			continue
		}
		if descFound {
			// 如果遇到下一个 ## 标题，停止
			if strings.HasPrefix(line, "## ") {
				break
			}
			// 收集描述内容（跳过空行）
			if line != "" {
				descLines = append(descLines, line)
			}
			// 只取前几行作为描述
			if len(descLines) >= 3 {
				break
			}
		}
	}
	if len(descLines) > 0 {
		description = strings.Join(descLines, " ")
	}

	return name, description
}

// cleanSkillName 清理技能名称
func (s *SkillScanner) cleanSkillName(name string) string {
	if name == "" {
		return ""
	}

	// 转小写
	name = strings.ToLower(name)

	// 只保留字母、数字、连字符
	name = nonAlphaNumRegex.ReplaceAllString(name, "-")

	// 移除连续的连字符
	name = consecutiveDashRegex.ReplaceAllString(name, "-")

	// 移除两端的连字符
	name = strings.Trim(name, "-")

	// 确保以字母开头
	if len(name) > 0 && !startsWithLetterRegex.MatchString(name) {
		// 如果第一个字符不是字母，添加 "skill-" 前缀
		name = "skill-" + name
	}

	return name
}

// ImportSkills 批量导入技能
func (s *SkillScanner) ImportSkills(ctx context.Context, req *model.BatchImportRequest) (*model.BatchImportResult, error) {
	// 获取注册表信息
	registry, err := s.registryRepo.FindByID(ctx, req.RegistryID)
	if err != nil {
		return nil, fmt.Errorf("获取注册表失败: %w", err)
	}

	// 创建临时目录
	tempUUID := uuid.New()
	tempDir := filepath.Join(s.storagePath, ".temp", tempUUID.String())
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 构建克隆URL（带Token注入）
	cloneURL := s.buildCloneURL(registry)

	// 执行 git clone --depth 1
	cloneCtx, cancel := context.WithTimeout(ctx, s.cloneTimeout)
	defer cancel()

	cmd := pkgexec.GitCommandContext(cloneCtx, "git", "clone", "--depth", "1", cloneURL, tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 清理临时目录
		go os.RemoveAll(tempDir)
		if cloneCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("git clone 超时: %w", err)
		}
		return nil, fmt.Errorf("git clone 失败: %s, %w", string(output), err)
	}

	// 使用 goroutine pool 进行并发导入
	imported := make([]*model.Skill, 0, len(req.Skills))
	updated := make([]*model.Skill, 0, len(req.Skills))  // 更新的 Skill 列表
	skipped := make([]model.SkippedSkillInfo, 0, len(req.Skills))

	// 冲突统计
	conflictSummary := &model.ConflictSummary{}

	// 创建通道收集结果
	importChan := make(chan *model.Skill, len(req.Skills))
	updateChan := make(chan *model.Skill, len(req.Skills))  // 更新结果通道
	skipChan := make(chan model.SkippedSkillInfo, len(req.Skills))
	errChan := make(chan error, len(req.Skills))
	// 冲突统计通道
	autoUpdateChan := make(chan struct{}, len(req.Skills))
	userCreateChan := make(chan struct{}, len(req.Skills))
	userUpdateChan := make(chan struct{}, len(req.Skills))
	// 刷新错误通道
	refreshErrChan := make(chan []model.RefreshError, len(req.Skills))

	// 创建信号量控制并发数
	sem := make(chan struct{}, s.importPoolSize)
	var wg sync.WaitGroup
	var nameMu sync.Mutex // protect directory copy operations within batch

	for _, skillItem := range req.Skills {
		wg.Add(1)
		go func(item model.SkillImportItem) {
			defer wg.Done()
			sem <- struct{}{}        // 获取信号量
			defer func() { <-sem }() // 释放信号量

			// 允许 skill 重名，不再检查名称唯一性
			// 加锁防止同批次目录复制冲突（仅用于目录操作）
			nameMu.Lock()

					// 确定导入模式（默认 create）
					importMode := item.ImportMode
					if importMode == "" {
						importMode = "create"
					}

					if importMode == "update" {
						// 更新模式：需要 targetSkillID
						if item.TargetSkillID == uuid.Nil {
							nameMu.Unlock()
							errChan <- fmt.Errorf("更新模式需要指定 targetSkillId: %s", item.Name)
							return
						}

						// 获取现有 Skill
						existing, err := s.skillRepo.FindByID(ctx, item.TargetSkillID)
						if err != nil {
							nameMu.Unlock()
							errChan <- fmt.Errorf("找不到目标 Skill %s: %w", item.Name, err)
							return
						}

						// 更新元数据（替换策略）
						existing.Description = item.Description
						existing.Tags = item.Tags
						existing.SupportedAgents = item.SupportedAgents
						existing.SourceType = model.SkillSourceFederated
						existing.SourceRegistryID = registry.ID
						existing.SourcePath = item.Path // 联邦源仓库相对路径
						existing.UpdatedAt = time.Now()

						// 更新文件目录
						srcDir := filepath.Join(tempDir, item.Path)
						dstDir := filepath.Join(s.storagePath, existing.ID.String())
						if err := s.updateSkillFiles(srcDir, dstDir); err != nil {
							nameMu.Unlock()
							errChan <- fmt.Errorf("更新 Skill 文件 %s 失败: %w", item.Name, err)
							return
						}

						// 保存更新
						if err := s.skillRepo.Update(ctx, existing); err != nil {
							nameMu.Unlock()
							errChan <- fmt.Errorf("更新 Skill 记录 %s 失败: %w", item.Name, err)
							return
						}

						// 刷新关联角色的配置目录
						refreshErrors := s.RefreshAgentConfigsForSkill(ctx, existing.ID)
						if len(refreshErrors) > 0 {
							refreshErrChan <- refreshErrors
						}

						nameMu.Unlock()
						updateChan <- existing
						userUpdateChan <- struct{}{}
						return
					}

					// 创建模式：创建 Skill 记录

			// 创建 Skill 记录
			skill := &model.Skill{
				ID:               uuid.New(),
				Name:             item.Name,
				Description:      item.Description,
				Tags:             item.Tags,
				SourceType:       model.SkillSourceFederated,
				SourceRegistryID: registry.ID,
				SourcePath:       item.Path, // 联邦源仓库相对路径
				SupportedAgents:  item.SupportedAgents,
				IsPublic:         true, // 联邦技能固定公开
				Status:           model.SkillStatusActive,
				UseCount:         0,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}

			// 复制技能目录（使用 skill.ID 作为目录名）
			srcDir := filepath.Join(tempDir, item.Path)
			dstDir := filepath.Join(s.storagePath, skill.ID.String())

			if err := s.copySkillDirectory(srcDir, dstDir); err != nil {
				nameMu.Unlock()
				errChan <- fmt.Errorf("复制技能目录 %s 失败: %w", item.Name, err)
				return
			}

			if err := s.skillRepo.Create(ctx, skill); err != nil {
				// 删除已复制的目录
				os.RemoveAll(dstDir)
				nameMu.Unlock()
				errChan <- fmt.Errorf("创建技能记录 %s 失败: %w", item.Name, err)
				return
			}

			nameMu.Unlock()
			importChan <- skill
		}(skillItem)
	}

	// 等待所有任务完成
	wg.Wait()
	close(importChan)
	close(updateChan)
	close(skipChan)
	close(errChan)
	close(autoUpdateChan)
	close(userCreateChan)
	close(userUpdateChan)
	close(refreshErrChan)

	// 收集结果
	for skill := range importChan {
		imported = append(imported, skill)
	}
	for skill := range updateChan {
		updated = append(updated, skill)
	}
	for skip := range skipChan {
		skipped = append(skipped, skip)
	}
	for err := range errChan {
		s.logger.Warn("导入技能失败", zap.Error(err))
	}

	// 收集冲突统计
	for range autoUpdateChan {
		conflictSummary.AutoUpdated++
	}
	for range userCreateChan {
		conflictSummary.UserCreated++
	}
	for range userUpdateChan {
		conflictSummary.UserUpdated++
	}

	// 收集刷新错误
	var configRefreshErrors []model.RefreshError
	for errs := range refreshErrChan {
		configRefreshErrors = append(configRefreshErrors, errs...)
	}

	// 异步删除临时目录
	go os.RemoveAll(tempDir)

	return &model.BatchImportResult{
		Imported:            imported,
		Updated:             updated,
		Skipped:             skipped,
		ConflictSummary:     conflictSummary,
		ConfigRefreshErrors: configRefreshErrors,
	}, nil
}

// copySkillDirectory 复制技能目录
func (s *SkillScanner) copySkillDirectory(src, dst string) error {
	// 检查源目录是否存在
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("源目录不存在: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("源路径不是目录: %s", src)
	}

	// 创建目标目录
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 遍历源目录复制文件
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过 .git 目录
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// 计算相对路径
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// 构建目标路径
		dstPath := filepath.Join(dst, relPath)

		// 如果是目录，创建目录
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// 如果是文件，复制文件
		return copyFile(path, dstPath, info.Mode())
	})

	return err
}

// copyFile 复制单个文件
func copyFile(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// updateSkillFiles 更新 Skill 文件目录（清空原目录，复制新文件）
func (s *SkillScanner) updateSkillFiles(srcDir, dstDir string) error {
	// 检查源目录是否存在
	srcInfo, err := os.Stat(srcDir)
	if err != nil {
		return fmt.Errorf("源目录不存在: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("源路径不是目录: %s", srcDir)
	}

	// 清空目标目录（保留目录本身）
	entries, err := os.ReadDir(dstDir)
	if err != nil {
		// 如果目录不存在，创建它
		if os.IsNotExist(err) {
			if err := os.MkdirAll(dstDir, 0755); err != nil {
				return fmt.Errorf("创建目标目录失败: %w", err)
			}
		} else {
			return fmt.Errorf("读取目标目录失败: %w", err)
		}
	} else {
		// 删除目录内的所有内容
		for _, entry := range entries {
			path := filepath.Join(dstDir, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("清理目标目录失败: %w", err)
			}
		}
	}

	// 复制新文件到目标目录
	return s.copySkillDirectory(srcDir, dstDir)
}

// RefreshAgentConfigsForSkill 刷新角色配置目录中的 skill 文件
// 参数：skillID - 被更新的 skill ID
// 返回：刷新错误列表（空表示全部成功）
func (s *SkillScanner) RefreshAgentConfigsForSkill(ctx context.Context, skillID uuid.UUID) []model.RefreshError {
	if s.agentConfigPath == "" || s.bindingRepo == nil {
		return nil // 未配置 agent-configs 目录，跳过刷新
	}

	// 1. 获取 skill 信息
	skill, err := s.skillRepo.FindByID(ctx, skillID)
	if err != nil {
		s.logger.Warn("获取 skill 信息失败，跳过刷新", zap.String("skillId", skillID.String()), zap.Error(err))
		return nil
	}

	// 2. 查询关联角色 ID 列表
	agentRoleIDs, err := s.bindingRepo.FindBySkillID(ctx, skillID)
	if err != nil {
		s.logger.Warn("查询角色关联失败，跳过刷新", zap.String("skillId", skillID.String()), zap.Error(err))
		return nil
	}
	if len(agentRoleIDs) == 0 {
		s.logger.Info("skill 未被任何角色关联，无需刷新", zap.String("skillId", skillID.String()))
		return nil
	}

	// 3. 遍历角色，过滤已生成配置的，执行刷新
	var refreshErrors []model.RefreshError
	srcDir := filepath.Join(s.storagePath, skill.ID.String())

	for _, agentRoleID := range agentRoleIDs {
		// 查询角色配置信息
		agentConfig, err := s.agentConfigRepo.FindByID(ctx, agentRoleID)
		if err != nil {
			s.logger.Warn("获取角色配置失败", zap.String("agentRoleId", agentRoleID.String()), zap.Error(err))
			continue
		}

		// 过滤：只刷新已生成配置的角色
		if agentConfig.ConfigGeneratedAt == nil {
			s.logger.Info("角色未生成配置，跳过刷新",
				zap.String("agentRoleId", agentRoleID.String()),
				zap.String("agentRoleName", agentConfig.Name))
			continue
		}

		// 刷新 skill 文件
		dstDir := filepath.Join(s.agentConfigPath, agentRoleID.String(), "skills", skill.Name)
		if err := s.updateSkillFiles(srcDir, dstDir); err != nil {
			refreshErrors = append(refreshErrors, model.RefreshError{
				AgentRoleID:   agentRoleID,
				AgentRoleName: agentConfig.Name,
				Error:         err.Error(),
			})
			s.logger.Warn("刷新角色配置 skill 文件失败",
				zap.String("agentRoleId", agentRoleID.String()),
				zap.String("skillName", skill.Name),
				zap.Error(err))
		} else {
			s.logger.Info("刷新角色配置 skill 文件成功",
				zap.String("agentRoleId", agentRoleID.String()),
				zap.String("agentRoleName", agentConfig.Name),
				zap.String("skillName", skill.Name))
		}
	}

	return refreshErrors
}