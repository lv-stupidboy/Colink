package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/skill"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SkillHandler Skill API处理器
type SkillHandler struct {
	skillSvc *skill.Service
	config   *config.SkillConfig
}

// NewSkillHandler 创建SkillHandler
func NewSkillHandler(skillSvc *skill.Service, cfg *config.SkillConfig) *SkillHandler {
	return &SkillHandler{
		skillSvc: skillSvc,
		config:   cfg,
	}
}

// List 列出所有Skills
func (h *SkillHandler) List(c *gin.Context) {
	var query model.SkillListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 设置默认分页
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}

	skills, total, err := h.skillSvc.List(c.Request.Context(), &query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  skills,
		"total": total,
		"page":  query.Page,
		"page_size": query.PageSize,
	})
}

// Get 获取单个Skill
func (h *SkillHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	skill, err := h.skillSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}

	c.JSON(http.StatusOK, skill)
}

// Create 创建Skill
func (h *SkillHandler) Create(c *gin.Context) {
	var req model.CreateSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	skill, err := h.skillSvc.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, skill)
}

// Update 更新Skill
func (h *SkillHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req model.UpdateSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	skill, err := h.skillSvc.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, skill)
}

// Delete 删除Skill
func (h *SkillHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	// 先获取技能信息，用于删除对应的文件
	skillRecord, err := h.skillSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}

	// 删除数据库记录
	if err := h.skillSvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 删除对应的zip文件
	storagePath := h.config.GetStoragePath()
	zipPath := filepath.Join(storagePath, skillRecord.Name+".zip")
	if _, err := os.Stat(zipPath); err == nil {
		if err := os.Remove(zipPath); err != nil {
			// 文件删除失败只记录日志，不影响删除操作结果
			fmt.Printf("删除技能文件失败: %v\n", err)
		}
	}

	c.Status(http.StatusNoContent)
}

// GetBoundAgents 获取Skill绑定的所有Agents
func (h *SkillHandler) GetBoundAgents(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	agents, err := h.skillSvc.GetBoundAgents(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, agents)
}

// BindSkills 绑定Skills到Agent
func (h *SkillHandler) BindSkills(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("agentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	var req model.BindSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.skillSvc.BindSkills(c.Request.Context(), agentRoleID, req.SkillIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// UnbindSkill 解除Skill绑定
func (h *SkillHandler) UnbindSkill(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("agentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	skillID, err := uuid.Parse(c.Param("skillId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid skill id"})
		return
	}

	if err := h.skillSvc.UnbindSkill(c.Request.Context(), agentRoleID, skillID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetAgentSkills 获取Agent绑定的所有Skills
func (h *SkillHandler) GetAgentSkills(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("agentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	skills, err := h.skillSvc.GetBoundSkills(c.Request.Context(), agentRoleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, skills)
}

// GetTags 获取所有标签
func (h *SkillHandler) GetTags(c *gin.Context) {
	tags, err := h.skillSvc.GetAllTags(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tags)
}

// GetBuiltInTags 获取内置标签分类
func (h *SkillHandler) GetBuiltInTags(c *gin.Context) {
	categories := h.skillSvc.GetBuiltInTagCategories()
	c.JSON(http.StatusOK, categories)
}

// Upload 上传技能包
func (h *SkillHandler) Upload(c *gin.Context) {
	// 获取配置
	maxSize := h.config.GetUploadMaxSize()
	storagePath := h.config.GetStoragePath()

	// 获取 source_type 参数（默认为 personal）
	sourceTypeStr := c.PostForm("source_type")
	sourceType := model.SkillSourcePersonal
	if sourceTypeStr != "" {
		sourceType = model.SkillSourceType(sourceTypeStr)
		// 验证 source_type
		if sourceType != model.SkillSourcePlatform && sourceType != model.SkillSourcePersonal && sourceType != model.SkillSourceFederated {
			sourceType = model.SkillSourcePersonal
		}
	}

	// 检查文件大小（在读取前检查）
	if c.Request.ContentLength > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("文件大小超过限制，最大允许 %dMB", maxSize/1024/1024)})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的文件"})
		return
	}
	defer file.Close()

	// 再次检查文件大小（以实际大小为准）
	if header.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("文件大小超过限制，最大允许 %dMB", maxSize/1024/1024)})
		return
	}

	// 检查文件扩展名 - 只支持 zip
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".zip" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持 .zip 格式的文件"})
		return
	}

	// 读取文件内容
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	// 解析 zip 文件
	reader, err := zip.NewReader(bytes.NewReader(fileBytes), int64(len(fileBytes)))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "解压文件失败: " + err.Error()})
		return
	}

	// 查找 skill.md 文件
	var skillMDContent string
	for _, f := range reader.File {
		if strings.HasSuffix(f.Name, "skill.md") {
			rc, err := f.Open()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "读取 skill.md 失败"})
				return
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "读取 skill.md 内容失败"})
				return
			}
			skillMDContent = string(content)
			break
		}
	}

	if skillMDContent == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未找到 skill.md 文件"})
		return
	}

	// 解析 skill.md 提取元数据
	metadata := parseSkillMD(skillMDContent)

	// 创建技能记录（仅填充从 skill.md 提取的名称和描述）
	req := &model.CreateSkillRequest{
		Name:        metadata.Name,
		Description: metadata.Description,
		SourceType:  sourceType,
		IsPublic:    sourceType == model.SkillSourcePersonal, // 只有个人类型默认私有
	}

	skillRecord, err := h.skillSvc.Create(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 保存技能包文件到本地
	// 确保存储目录存在
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建存储目录失败"})
		return
	}

	// 使用技能名称作为文件名
	savePath := filepath.Join(storagePath, skillRecord.Name+".zip")
	if err := os.WriteFile(savePath, fileBytes, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}

	c.JSON(http.StatusCreated, skillRecord)
}

// SkillMetadata 技能元数据
type SkillMetadata struct {
	Name        string
	Description string
}

// parseSkillMD 解析 skill.md 文件提取元数据（仅提取名称和描述）
func parseSkillMD(content string) SkillMetadata {
	metadata := SkillMetadata{}

	// 提取标题 (第一个 # 标题)
	titleRegex := regexp.MustCompile(`(?m)^#\s+(.+)$`)
	if matches := titleRegex.FindStringSubmatch(content); len(matches) > 1 {
		metadata.Name = strings.TrimSpace(matches[1])
	}

	// 提取描述 (## Description 下的内容，直到下一个 ## 标题)
	// 使用 (?s) 让 . 匹配换行符
	descRegex := regexp.MustCompile(`(?s)##\s*Description\s*\n+(.+?)(?:\n##|$)`)
	if matches := descRegex.FindStringSubmatch(content); len(matches) > 1 {
		desc := strings.TrimSpace(matches[1])
		// 移除末尾的空行
		desc = strings.TrimRight(desc, "\n")
		metadata.Description = desc
	}

	return metadata
}

// ImportFromRepo 从 Git 仓库导入技能
func (h *SkillHandler) ImportFromRepo(c *gin.Context) {
	var req struct {
		RepoURL string `json:"repo_url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入仓库地址"})
		return
	}

	// 验证仓库地址（仅支持 GitHub 和 Gitee）
	if !isValidRepoURL(req.RepoURL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 GitHub 和 Gitee 仓库地址"})
		return
	}

	storagePath := h.config.GetStoragePath()
	tempDir := filepath.Join(storagePath, ".temp", uuid.New().String())

	// 确保临时目录存在
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建临时目录失败"})
		return
	}
	defer os.RemoveAll(tempDir)

	// 克隆仓库
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", req.RepoURL, tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("克隆仓库失败: %s", string(output))})
		return
	}

	// 查找 skill.md 文件
	skillMDPath := filepath.Join(tempDir, "skill.md")
	if _, err := os.Stat(skillMDPath); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仓库根目录未找到 skill.md 文件"})
		return
	}

	// 读取 skill.md 内容
	skillMDContent, err := os.ReadFile(skillMDPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取 skill.md 失败"})
		return
	}

	// 解析元数据
	metadata := parseSkillMD(string(skillMDContent))

	// 创建技能记录
	createReq := &model.CreateSkillRequest{
		Name:        metadata.Name,
		Description: metadata.Description,
		SourceType:  model.SkillSourcePersonal,
		IsPublic:    true,
	}

	skillRecord, err := h.skillSvc.Create(c.Request.Context(), createReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 创建存储目录
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建存储目录失败"})
		return
	}

	// 打包为 zip 文件，使用技能名称作为文件名
	zipPath := filepath.Join(storagePath, skillRecord.Name+".zip")
	if err := zipDirectory(tempDir, zipPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("打包技能失败: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, skillRecord)
}

// ImportFromFederated 从联邦源导入技能
func (h *SkillHandler) ImportFromFederated(c *gin.Context) {
	var req struct {
		RegistryID string `json:"registry_id" binding:"required"`
		SkillName  string `json:"skill_name"` // 可选，不指定则列出可用技能
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择联邦源"})
		return
	}

	// TODO: 从数据库查询 Registry 信息
	// 这里先硬编码支持 skills.sh
	federatedURL := "https://skills.sh"

	// 如果没有指定技能名称，返回可用技能列表
	if req.SkillName == "" {
		skills, err := h.listFederatedSkills(federatedURL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取联邦技能列表失败: %v", err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"skills": skills})
		return
	}

	// 下载指定技能
	skillData, err := h.downloadFederatedSkill(federatedURL, req.SkillName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("下载技能失败: %v", err)})
		return
	}

	// 创建技能记录
	createReq := &model.CreateSkillRequest{
		Name:        skillData.Name,
		Description: skillData.Description,
		SourceType:  model.SkillSourceFederated,
		IsPublic:    true,
	}

	skillRecord, err := h.skillSvc.Create(c.Request.Context(), createReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 保存技能包
	storagePath := h.config.GetStoragePath()
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建存储目录失败"})
		return
	}

	zipPath := filepath.Join(storagePath, skillRecord.Name+".zip")
	if err := os.WriteFile(zipPath, skillData.ZipContent, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存技能包失败"})
		return
	}

	c.JSON(http.StatusCreated, skillRecord)
}

// isValidRepoURL 验证仓库地址
func isValidRepoURL(url string) bool {
	return strings.HasPrefix(url, "https://github.com/") ||
		strings.HasPrefix(url, "https://gitee.com/") ||
		strings.HasPrefix(url, "git@github.com:") ||
		strings.HasPrefix(url, "git@gitee.com:")
}

// zipDirectory 将目录打包为 zip 文件
func zipDirectory(sourceDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
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

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			_, err = zipWriter.Create(relPath + "/")
			return err
		}

		writer, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}

// FederatedSkillData 联邦技能数据
type FederatedSkillData struct {
	Name        string
	Description string
	ZipContent  []byte
}

// listFederatedSkills 列出联邦源可用技能
func (h *SkillHandler) listFederatedSkills(baseURL string) ([]map[string]interface{}, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(baseURL + "/api/v1/skills")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// downloadFederatedSkill 从联邦源下载技能
func (h *SkillHandler) downloadFederatedSkill(baseURL, skillName string) (*FederatedSkillData, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// 1. 获取技能元数据
	resp, err := client.Get(fmt.Sprintf("%s/api/v1/skills?search=%s", baseURL, skillName))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取技能元数据失败: HTTP %d", resp.StatusCode)
	}

	var listResult struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResult); err != nil {
		return nil, err
	}

	if len(listResult.Data) == 0 {
		return nil, fmt.Errorf("未找到技能: %s", skillName)
	}

	skill := listResult.Data[0]

	// 2. 下载技能包
	downloadResp, err := client.Get(fmt.Sprintf("%s/api/v1/skills/%s/download", baseURL, skill.ID))
	if err != nil {
		return nil, err
	}
	defer downloadResp.Body.Close()

	if downloadResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载技能包失败: HTTP %d", downloadResp.StatusCode)
	}

	zipContent, err := io.ReadAll(downloadResp.Body)
	if err != nil {
		return nil, err
	}

	return &FederatedSkillData{
		Name:        skill.Name,
		Description: skill.Description,
		ZipContent:  zipContent,
	}, nil
}

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
		skills.POST("/import/federated", h.ImportFromFederated)
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