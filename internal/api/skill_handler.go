package api

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/configgen"
	"github.com/anthropic/isdp/internal/service/skill"
	"github.com/anthropic/isdp/pkg/config"
	pkgexec "github.com/anthropic/isdp/pkg/exec"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SkillHandler Skill API处理器
type SkillHandler struct {
	skillSvc      *skill.Service
	scanner       *skill.SkillScanner
	storagePath   string
	uploadMax     int64
	autoGenerator *configgen.AutoGenerator // 自动配置生成器
	cfg           *config.GitURLConversionConfig
}

// NewSkillHandler 创建SkillHandler
func NewSkillHandler(skillSvc *skill.Service, scanner *skill.SkillScanner, storagePath string, uploadMax int64, autoGenerator *configgen.AutoGenerator, cfg *config.GitURLConversionConfig) *SkillHandler {
	return &SkillHandler{
		skillSvc:      skillSvc,
		scanner:       scanner,
		storagePath:   storagePath,
		uploadMax:     uploadMax,
		autoGenerator: autoGenerator,
		cfg:           cfg,
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

	// 获取受影响的Agent角色列表
	var affectedAgents []map[string]string
	var affectedCount int
	if h.autoGenerator != nil {
		affectedIDs, err := h.autoGenerator.GetAffectedAgentsBySkill(c.Request.Context(), id)
		if err == nil && len(affectedIDs) > 0 {
			affectedCount = len(affectedIDs)
			// 获取角色名称列表
			agents, err := h.skillSvc.GetBoundAgents(c.Request.Context(), id)
			if err == nil {
				for _, agent := range agents {
					affectedAgents = append(affectedAgents, map[string]string{
						"id":   agent.ID.String(),
						"name": agent.Name,
					})
				}
			}
			// 批量生成配置（后台执行）
			go func() {
				ctx := context.Background()
				errors := h.autoGenerator.GenerateMultiple(ctx, affectedIDs)
				// 只记录错误，不影响响应
				for _, e := range errors {
					c.Error(e)
				}
			}()
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":               skill.ID,
		"name":             skill.Name,
		"description":      skill.Description,
		"tags":             skill.Tags,
		"sourceType":       skill.SourceType,
		"supportedAgents":  skill.SupportedAgents,
		"isPublic":         skill.IsPublic,
		"updatedAt":        skill.UpdatedAt,
		"affectedAgents":   affectedAgents,
		"affectedCount":    affectedCount,
	})
}

// Delete 删除Skill
func (h *SkillHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	// 删除技能（Service层会检查绑定关系并删除文件）
	if err := h.skillSvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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

	// 转换为简化格式
	result := make([]gin.H, 0)
	for _, agent := range agents {
		result = append(result, gin.H{
			"id":   agent.ID.String(),
			"name": agent.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"agents": result,
		"count":  len(result),
	})
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

	// 不在此处自动生成，前端批量操作后统一调用 generateConfig

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

	// 不在此处自动生成，前端批量操作后统一调用 generateConfig

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

	c.JSON(http.StatusOK, gin.H{
		"skills": skills,
		"count":  len(skills),
	})
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
		// 记录详细错误日志
		h.scanner.GetLogger().Error("扫描联邦源技能失败",
			zap.String("registryId", req.RegistryID),
			zap.Error(err),
			zap.String("errorDetail", fmt.Sprintf("%+v", err)),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
			"detail": fmt.Sprintf("扫描失败: %v", err),
		})
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

// Upload 上传技能包
func (h *SkillHandler) Upload(c *gin.Context) {
	// 获取配置
	maxSize := h.uploadMax
	storagePath := h.storagePath

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

	// 获取 directory_name 参数（目录名作为技能名称）
	directoryName := c.PostForm("directory_name")

	// 获取 description 参数（前端解析的描述，优先使用）
	frontendDescription := c.PostForm("description")

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

	// 查找 SKILL.md 或 skill.md 文件
	var skillMDContent string
	var rootDir string // 检测根目录层级

	// 首先查找 SKILL.md 文件，同时检测是否在根目录
	for _, f := range reader.File {
		// 跳过目录
		if f.FileInfo().IsDir() {
			continue
		}

		// 获取文件名
		parts := strings.Split(f.Name, "/")

		// 检查是否是 SKILL.md（大小写不敏感）
		fileName := parts[len(parts)-1]
		if strings.ToLower(fileName) == "skill.md" {
			rc, err := f.Open()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "读取 SKILL.md 失败"})
				return
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "读取 SKILL.md 内容失败"})
				return
			}
			skillMDContent = string(content)

			// 如果 SKILL.md 不在根目录（路径有多级），则记录根目录
			if len(parts) > 1 {
				rootDir = parts[0]
			}
			break
		}
	}

	if skillMDContent == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未找到 SKILL.md 文件"})
		return
	}

	// 确定技能名称：只能从目录名获取
	skillName := directoryName
	if skillName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "目录名不能为空"})
		return
	}

	// 创建技能记录（使用前端解析的描述）
	req := &model.CreateSkillRequest{
		Name:        skillName,
		Description: frontendDescription,
		SourceType:  sourceType,
		IsPublic:    sourceType != model.SkillSourcePersonal, // 个人类型私有，其他类型公开
	}

	skillRecord, err := h.skillSvc.Create(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 保存技能文件到本地（解压为目录）
	// 确保存储目录存在
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建存储目录失败"})
		return
	}

	// 使用技能 ID 作为目录名
	skillDir := filepath.Join(storagePath, skillRecord.ID.String())
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建技能目录失败"})
		return
	}

	// 解压 zip 文件到技能目录
	for _, f := range reader.File {
		// 跳过目录
		if f.FileInfo().IsDir() {
			continue
		}

		// 获取文件名（去掉可能的根目录前缀）
		fileName := f.Name
		if rootDir != "" {
			fileName = strings.TrimPrefix(fileName, rootDir+"/")
		}
		if fileName == "" {
			continue
		}

		// 创建目标文件路径
		destPath := filepath.Join(skillDir, fileName)

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建子目录失败"})
			return
		}

		// 解压文件
		rc, err := f.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "解压文件失败"})
			return
		}

		destFile, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建文件失败"})
			return
		}

		_, err = io.Copy(destFile, rc)
		destFile.Close()
		rc.Close()

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "写入文件失败"})
			return
		}
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
		RepoURL string `json:"repoUrl" binding:"required"`
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

	tempDir := filepath.Join(h.storagePath, ".temp", uuid.New().String())

	// 确保临时目录存在
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建临时目录失败"})
		return
	}
	defer os.RemoveAll(tempDir)

	// 克隆仓库
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	repoURL := h.cfg.ConvertHTTPToSSH(req.RepoURL)
	cmd := pkgexec.CommandContext(ctx, "git", "clone", "--depth", "1", repoURL, tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("克隆仓库失败: %s", string(output))})
		return
	}

	// 查找 SKILL.md 或 skill.md 文件
	var skillMDPath string
	skillMDPathLower := filepath.Join(tempDir, "skill.md")
	skillMDPathUpper := filepath.Join(tempDir, "SKILL.md")

	if _, err := os.Stat(skillMDPathUpper); err == nil {
		skillMDPath = skillMDPathUpper
	} else if _, err := os.Stat(skillMDPathLower); err == nil {
		skillMDPath = skillMDPathLower
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仓库根目录未找到 SKILL.md 文件"})
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
	if err := os.MkdirAll(h.storagePath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建存储目录失败"})
		return
	}

	// 复制整个目录到技能存储目录
	skillDir := filepath.Join(h.storagePath, skillRecord.ID.String())
	if err := copyDirectory(tempDir, skillDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("复制技能目录失败: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, skillRecord)
}

// ImportFromFederated 从联邦源导入技能
func (h *SkillHandler) ImportFromFederated(c *gin.Context) {
	var req struct {
		RegistryID string `json:"registryId" binding:"required"`
		SkillName  string `json:"skillName"` // 可选，不指定则列出可用技能
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

	// 如果没有指定技能名称，返回可用技能列表
	if req.SkillName == "" {
		result, err := h.scanner.ScanRegistry(c.Request.Context(), registryID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("扫描联邦技能列表失败: %v", err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"skills": result.Skills})
		return
	}

	// 先扫描获取技能信息（包含 path）
	scanResult, err := h.scanner.ScanRegistry(c.Request.Context(), registryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("扫描联邦源失败: %v", err)})
		return
	}

	// 从扫描结果中找到对应的技能
	var targetSkill *model.RemoteSkill
	for _, skill := range scanResult.Skills {
		if skill.Name == req.SkillName {
			targetSkill = skill
			break
		}
	}

	if targetSkill == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("未找到技能: %s", req.SkillName)})
		return
	}

	// 使用批量导入接口导入单个技能（复用认证注入逻辑）
	importReq := &model.BatchImportRequest{
		RegistryID: registryID,
		Skills: []model.SkillImportItem{
			{
				Name:            targetSkill.Name,
				Path:            targetSkill.Path,
				Description:     targetSkill.Description,
				Tags:            []string{},
				SupportedAgents: []string{"claude"}, // 默认支持 claude
			},
		},
	}

	result, err := h.scanner.ImportSkills(c.Request.Context(), importReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("导入技能失败: %v", err)})
		return
	}

	if len(result.Imported) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导入失败，请检查技能名称"})
		return
	}

	c.JSON(http.StatusCreated, result.Imported[0])
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

// copyDirectory 复制整个目录
func copyDirectory(src, dst string) error {
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

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

// extractZipToDirectory 将 zip 内容解压到目录
func extractZipToDirectory(zipContent []byte, destDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(zipContent), int64(len(zipContent)))
	if err != nil {
		return err
	}

	// 检测是否有根目录（嵌套结构）
	var rootDir string
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		parts := strings.Split(f.Name, "/")
		if len(parts) > 1 {
			rootDir = parts[0]
			break
		}
	}

	// 创建目标目录
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// 解压文件
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}

		// 获取文件名（去掉可能的根目录前缀）
		fileName := f.Name
		if rootDir != "" {
			fileName = strings.TrimPrefix(fileName, rootDir+"/")
		}
		if fileName == "" {
			continue
		}

		destPath := filepath.Join(destDir, fileName)

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		destFile, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(destFile, rc)
		destFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
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
		skills.POST("/import/federated/scan", h.ScanFederatedSkills)
		skills.POST("/import/federated/batch", h.BatchImportFederated)
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