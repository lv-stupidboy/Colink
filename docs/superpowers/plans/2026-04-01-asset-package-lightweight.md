# 资产包轻量化与团队包实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将资产包简化为纯导入导出工具（移除版本和元数据表），并新增团队包功能支持工作流+角色的批量导入导出。

**Architecture:** 后端采用服务层模式，资产包和团队包各自独立的服务和API。前端采用React+Ant Design，两个独立页面，左右两卡片布局。

**Tech Stack:** Go (Gin) + React + Ant Design + TypeScript

---

## 文件结构

### 需要删除的文件
- `isdp/internal/model/asset_package.go` - 资产包元数据模型
- `isdp/internal/repo/asset_package.go` - 资产包仓库
- `isdp/sql-change/migrations/202603310001_add_asset_package_tables.sql` - 资产包表迁移

### 需要修改的文件
- `isdp/internal/service/assetpackage/service.go` - 简化为纯导入导出
- `isdp/internal/api/asset_package_handler.go` - 只保留 Import/Export
- `isdp/web/src/pages/AssetPackage/index.tsx` - 重写为两卡片布局
- `isdp/web/src/layouts/MainLayout.tsx` - 调整菜单结构
- `isdp/web/src/App.tsx` - 更新路由
- `isdp/web/src/api/client.ts` - 简化资产包API

### 需要新增的文件
- `isdp/internal/model/team_package.go` - 团队包数据结构
- `isdp/internal/service/teampackage/service.go` - 团队包服务
- `isdp/internal/api/team_package_handler.go` - 团队包API
- `isdp/web/src/pages/TeamPackage/index.tsx` - 团队包页面
- `isdp/web/src/api/teamPackage.ts` - 团队包API调用
- `isdp/sql-change/migrations/202604010001_drop_asset_packages_table.sql` - 删除表

---

## Task 1: 创建数据库迁移脚本（删除 asset_packages 表）

**Files:**
- Create: `isdp/sql-change/migrations/202604010001_drop_asset_packages_table.sql`

- [ ] **Step 1: 编写删除 asset_packages 表的迁移脚本**

```sql
-- 文件路径: isdp/sql-change/migrations/202604010001_drop_asset_packages_table.sql
-- 变更说明：删除 asset_packages 表，资产包不再存储元数据
-- 作者：AI Assistant
-- 日期：2026-04-01

SET NAMES utf8mb4;

-- 删除 asset_packages 表
DROP TABLE IF EXISTS asset_packages;

-- 回滚语句（如需回滚执行以下语句）
-- CREATE TABLE asset_packages (
--   id VARCHAR(36) PRIMARY KEY,
--   name VARCHAR(255) NOT NULL,
--   version VARCHAR(100),
--   description TEXT,
--   created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
--   updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
-- );
```

- [ ] **Step 2: 提交迁移脚本**

```bash
git add isdp/sql-change/migrations/202604010001_drop_asset_packages_table.sql
git commit -m "chore(db): 添加删除 asset_packages 表的迁移脚本"
```

---

## Task 2: 删除后端资产包元数据相关代码

**Files:**
- Delete: `isdp/internal/model/asset_package.go`
- Delete: `isdp/internal/repo/asset_package.go`
- Delete: `isdp/sql-change/migrations/202603310001_add_asset_package_tables.sql`

- [ ] **Step 1: 删除资产包模型文件**

```bash
rm isdp/internal/model/asset_package.go
```

- [ ] **Step 2: 删除资产包仓库文件**

```bash
rm isdp/internal/repo/asset_package.go
```

- [ ] **Step 3: 删除旧的迁移脚本**

```bash
rm isdp/sql-change/migrations/202603310001_add_asset_package_tables.sql
```

- [ ] **Step 4: 提交删除**

```bash
git add -A
git commit -m "refactor: 删除资产包元数据模型和仓库"
```

---

## Task 3: 创建简化的资产包模型（仅保留导入导出结构）

**Files:**
- Create: `isdp/internal/model/asset_package.go`

- [ ] **Step 1: 编写简化的资产包模型**

```go
// 文件路径: isdp/internal/model/asset_package.go
package model

import "time"

// AssetPackageManifest 资产包 manifest.json 结构（简化版，无版本概念）
type AssetPackageManifest struct {
	ExportedAt string                 `json:"exportedAt"`
	Assets     AssetPackageAssetsList `json:"assets"`
}

// AssetPackageAssetsList 资产列表
type AssetPackageAssetsList struct {
	Skills    []AssetPackageSkillItem    `json:"skills,omitempty"`
	Commands  []AssetPackageCommandItem  `json:"commands,omitempty"`
	Subagents []AssetPackageSubagentItem `json:"subagents,omitempty"`
	Rules     []AssetPackageRuleItem     `json:"rules,omitempty"`
	Settings  []AssetPackageSettingsItem `json:"settings,omitempty"`
}

// AssetPackageSkillItem 技能项
type AssetPackageSkillItem struct {
	Name string `json:"name"`
	File string `json:"file"`
}

// AssetPackageCommandItem 命令项
type AssetPackageCommandItem struct {
	Name        string   `json:"name"`
	File        string   `json:"file"`
	BoundSkills []string `json:"boundSkills,omitempty"`
}

// AssetPackageSubagentItem 子代理项
type AssetPackageSubagentItem struct {
	Name        string   `json:"name"`
	File        string   `json:"file"`
	BoundSkills []string `json:"boundSkills,omitempty"`
}

// AssetPackageRuleItem 规则项
type AssetPackageRuleItem struct {
	Name string `json:"name"`
	File string `json:"file"`
}

// AssetPackageSettingsItem 配置项
type AssetPackageSettingsItem struct {
	Name string `json:"name"`
	Dir  string `json:"dir"`
}

// ExportAssetPackageRequest 导出资产包请求（简化版）
type ExportAssetPackageRequest struct {
	SkillIDs    []string `json:"skillIds"`
	CommandIDs  []string `json:"commandIds"`
	SubagentIDs []string `json:"subagentIds"`
	RuleIDs     []string `json:"ruleIds"`
	SettingsIDs []string `json:"settingsIds"`
}

// ImportResult 导入结果
type ImportResult struct {
	Success int            `json:"success"`
	Skipped int            `json:"skipped"`
	Failed  int            `json:"failed"`
	Details []ImportDetail `json:"details"`
}

// ImportDetail 导入详情
type ImportDetail struct {
	AssetType string `json:"assetType"`
	Name      string `json:"name"`
	Status    string `json:"status"` // success, skipped, failed
	Message   string `json:"message,omitempty"`
}
```

- [ ] **Step 2: 提交模型文件**

```bash
git add isdp/internal/model/asset_package.go
git commit -m "refactor(model): 简化资产包模型，移除版本和元数据"
```

---

## Task 4: 重写资产包服务（移除数据库依赖）

**Files:**
- Modify: `isdp/internal/service/assetpackage/service.go`

- [ ] **Step 1: 重写资产包服务**

服务需要：
1. 移除 `packageRepo` 依赖
2. Export 方法：不再保存到数据库，直接生成 ZIP
3. Import 方法：不再创建 AssetPackage 记录
4. 移除 List/GetByID/Delete 方法
5. 简化 manifest 结构（移除 name/version/description）

关键修改点：
- 删除第26行 `packageRepo *repo.AssetPackageRepository`
- 删除第62行赋值
- 删除第81-93行 List/GetByID/Delete 方法
- 修改 Export 方法（第96-299行）：移除第272-283行数据库保存逻辑
- 修改 Import 方法（第302-438行）：移除第326-338行数据库保存逻辑
- 简化 manifest 生成（移除 Name/Version/Description）

- [ ] **Step 2: 提交服务修改**

```bash
git add isdp/internal/service/assetpackage/service.go
git commit -m "refactor(service): 简化资产包服务，移除数据库依赖"
```

---

## Task 5: 重写资产包 API Handler

**Files:**
- Modify: `isdp/internal/api/asset_package_handler.go`

- [ ] **Step 1: 重写 Handler**

只保留 Import 和 Export 两个 API：
- 删除 List 方法（第27-54行）
- 删除 GetByID 方法（第56-71行）
- 删除 Delete 方法（第137-150行）
- 保留 Import 和 Export 方法
- 更新 NewAssetPackageHandler 移除 packageRepo 参数

- [ ] **Step 2: 更新路由注册**

```go
// RegisterRoutes 注册路由
func (h *AssetPackageHandler) RegisterRoutes(r *gin.RouterGroup) {
	packages := r.Group("/asset-packages")
	{
		packages.POST("/import", h.Import)
		packages.POST("/export", h.Export)
	}
}
```

- [ ] **Step 3: 提交 Handler 修改**

```bash
git add isdp/internal/api/asset_package_handler.go
git commit -m "refactor(api): 简化资产包 API，只保留导入导出"
```

---

## Task 6: 创建团队包模型

**Files:**
- Create: `isdp/internal/model/team_package.go`

- [ ] **Step 1: 编写团队包模型**

```go
// 文件路径: isdp/internal/model/team_package.go
package model

// TeamPackageManifest 团队包 manifest.json 结构
type TeamPackageManifest struct {
	ExportedAt string             `json:"exportedAt"`
	Workflow   TeamPackageWorkflow `json:"workflow"`
	Roles      []TeamPackageRole   `json:"roles"`
	Assets     TeamPackageAssets   `json:"assets"`
}

// TeamPackageWorkflow 工作流信息
type TeamPackageWorkflow struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	AgentIDs      []string      `json:"agentIds"`
	Transitions   []Transition  `json:"transitions"`
	Checkpoints   []string      `json:"checkpoints"`
	EstimatedTime string        `json:"estimatedTime"`
	IsSystem      bool          `json:"isSystem"`
	IsDefault     bool          `json:"isDefault"`
}

// TeamPackageRole 角色信息
type TeamPackageRole struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Role           string              `json:"role"`
	Description    string              `json:"description"`
	SystemPrompt   string              `json:"systemPrompt"`
	MaxTokens      int                 `json:"maxTokens"`
	Temperature    float64             `json:"temperature"`
	MentionPatterns []string            `json:"mentionPatterns"`
	Bindings       TeamPackageBindings `json:"bindings"`
}

// TeamPackageBindings 角色绑定信息
type TeamPackageBindings struct {
	Skills    []string `json:"skills,omitempty"`
	Commands  []string `json:"commands,omitempty"`
	Subagents []string `json:"subagents,omitempty"`
	Rules     []string `json:"rules,omitempty"`
	Settings  []string `json:"settings,omitempty"`
}

// TeamPackageAssets 团队包资产
type TeamPackageAssets struct {
	Skills    []AssetPackageSkillItem    `json:"skills,omitempty"`
	Commands  []AssetPackageCommandItem  `json:"commands,omitempty"`
	Subagents []AssetPackageSubagentItem `json:"subagents,omitempty"`
	Rules     []AssetPackageRuleItem     `json:"rules,omitempty"`
	Settings  []AssetPackageSettingsItem `json:"settings,omitempty"`
}

// TeamPackagePreview 团队包导入预览
type TeamPackagePreview struct {
	Workflow TeamPackagePreviewWorkflow `json:"workflow"`
	Roles    []TeamPackagePreviewRole   `json:"roles"`
	Assets   TeamPackagePreviewAssets   `json:"assets"`
}

// TeamPackagePreviewWorkflow 工作流预览
type TeamPackagePreviewWorkflow struct {
	Name   string `json:"name"`
	Exists bool   `json:"exists"`
}

// TeamPackagePreviewRole 角色预览
type TeamPackagePreviewRole struct {
	Name    string `json:"name"`
	Exists  bool   `json:"exists"`
	LocalID string `json:"localId,omitempty"`
}

// TeamPackagePreviewAssets 资产预览
type TeamPackagePreviewAssets struct {
	Skills    []TeamPackagePreviewAsset `json:"skills"`
	Commands  []TeamPackagePreviewAsset `json:"commands"`
	Subagents []TeamPackagePreviewAsset `json:"subagents"`
	Rules     []TeamPackagePreviewAsset `json:"rules"`
	Settings  []TeamPackagePreviewAsset `json:"settings"`
}

// TeamPackagePreviewAsset 单个资产预览
type TeamPackagePreviewAsset struct {
	Name   string `json:"name"`
	Exists bool   `json:"exists"`
}

// TeamPackageImportConfirm 团队包导入确认请求
type TeamPackageImportConfirm struct {
	Mode          string                        `json:"mode"` // overwrite | skip | selective
	WorkflowAction string                        `json:"workflowAction"`
	RoleActions   []TeamPackageRoleAction       `json:"roleActions"`
	AssetActions  []TeamPackageAssetAction      `json:"assetActions"`
}

// TeamPackageRoleAction 角色操作
type TeamPackageRoleAction struct {
	Name   string `json:"name"`
	Action string `json:"action"` // overwrite | skip
}

// TeamPackageAssetAction 资产操作
type TeamPackageAssetAction struct {
	AssetType string `json:"assetType"`
	Name      string `json:"name"`
	Action    string `json:"action"` // overwrite | skip
}

// ExportTeamPackageRequest 导出团队包请求
type ExportTeamPackageRequest struct {
	WorkflowID string `json:"workflowId" binding:"required"`
}
```

- [ ] **Step 2: 提交模型文件**

```bash
git add isdp/internal/model/team_package.go
git commit -m "feat(model): 添加团队包数据结构"
```

---

## Task 7: 创建团队包服务

**Files:**
- Create: `isdp/internal/service/teampackage/service.go`

- [ ] **Step 1: 编写团队包服务**

服务需要实现：
1. `Export(ctx, workflowID)` - 导出团队包
   - 获取工作流详情
   - 获取所有角色及其绑定关系
   - 收集所有资产文件
   - 生成 manifest.json
   - 打包为 ZIP

2. `ImportPreview(ctx, zipData)` - 导入预览
   - 解压 ZIP
   - 解析 manifest
   - 检查工作流、角色、资产是否已存在
   - 返回预览信息

3. `ImportConfirm(ctx, zipData, confirm)` - 确认导入
   - 根据确认策略导入工作流、角色、资产
   - 恢复绑定关系

参考现有 `assetpackage/service.go` 的工具函数：
- `copyDir`, `copyFile`, `createZip`, `extractZip`

- [ ] **Step 2: 提交服务文件**

```bash
git add isdp/internal/service/teampackage/service.go
git commit -m "feat(service): 实现团队包导入导出服务"
```

---

## Task 8: 创建团队包 API Handler

**Files:**
- Create: `isdp/internal/api/team_package_handler.go`

- [ ] **Step 1: 编写团队包 Handler**

```go
// 文件路径: isdp/internal/api/team_package_handler.go
package api

import (
	"io"
	"net/http"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/teampackage"
	"github.com/gin-gonic/gin"
)

// TeamPackageHandler 团队包 API 处理器
type TeamPackageHandler struct {
	svc *teampackage.Service
}

// NewTeamPackageHandler 创建 TeamPackageHandler
func NewTeamPackageHandler(svc *teampackage.Service) *TeamPackageHandler {
	return &TeamPackageHandler{svc: svc}
}

// Import 预览导入的团队包
func (h *TeamPackageHandler) Import(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的文件"})
		return
	}
	defer file.Close()

	if len(header.Filename) < 4 || header.Filename[len(header.Filename)-4:] != ".zip" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持 .zip 格式的文件"})
		return
	}

	zipData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	preview, err := h.svc.ImportPreview(c.Request.Context(), zipData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, preview)
}

// ImportConfirm 确认导入团队包
func (h *TeamPackageHandler) ImportConfirm(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的文件"})
		return
	}
	defer file.Close()

	zipData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	var confirm model.TeamPackageImportConfirm
	if err := c.ShouldBindJSON(&confirm); err != nil {
		// 如果没有 JSON body，使用默认策略
		confirm = model.TeamPackageImportConfirm{
			Mode:           "overwrite",
			WorkflowAction: "overwrite",
		}
	}

	result, err := h.svc.ImportConfirm(c.Request.Context(), zipData, &confirm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Export 导出团队包
func (h *TeamPackageHandler) Export(c *gin.Context) {
	var req model.ExportTeamPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	zipData, filename, err := h.svc.Export(c.Request.Context(), req.WorkflowID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/zip", zipData)
}

// RegisterRoutes 注册路由
func (h *TeamPackageHandler) RegisterRoutes(r *gin.RouterGroup) {
	teamPackages := r.Group("/team-packages")
	{
		teamPackages.POST("/import", h.Import)
		teamPackages.POST("/import/confirm", h.ImportConfirm)
		teamPackages.POST("/export", h.Export)
	}
}
```

- [ ] **Step 2: 提交 Handler 文件**

```bash
git add isdp/internal/api/team_package_handler.go
git commit -m "feat(api): 添加团队包 API Handler"
```

---

## Task 9: 更新服务注册和依赖注入

**Files:**
- Modify: `isdp/cmd/server/main.go` 或相应的服务初始化文件

- [ ] **Step 1: 更新服务初始化**

需要：
1. 移除 AssetPackageRepository 的创建
2. 更新 AssetPackage Service 的创建（移除 packageRepo 参数）
3. 创建 TeamPackage Service
4. 注册 TeamPackage Handler

- [ ] **Step 2: 提交修改**

```bash
git add isdp/cmd/server/main.go
git commit -m "refactor: 更新服务注册，移除资产包仓库依赖"
```

---

## Task 10: 更新前端菜单结构

**Files:**
- Modify: `isdp/web/src/layouts/MainLayout.tsx`

- [ ] **Step 1: 修改菜单配置**

将当前的菜单结构：
```
角色资产
├── 资产包 ← 当前位置
├── Commands
...
```

改为：
```
角色资产
├── Commands
├── Subagents
├── Skills
├── Rules
├── Settings
└── Plugins
管理工具 ← 新增
├── 团队包 ← 第一个
└── 资产包 ← 第二个
```

修改 `menuItems` 配置（第67-136行），移除资产包从角色资产中，新增管理工具菜单。

- [ ] **Step 2: 更新 openKeys 处理**

修改 `useEffect` 中的路径处理逻辑，添加对 `/management-tools` 路径的处理。

- [ ] **Step 3: 更新 getSelectedKey**

添加对团队包和资产包新路由的处理。

- [ ] **Step 4: 提交菜单修改**

```bash
git add isdp/web/src/layouts/MainLayout.tsx
git commit -m "feat(ui): 新增管理工具菜单，调整资产包位置"
```

---

## Task 11: 创建团队包前端页面

**Files:**
- Create: `isdp/web/src/pages/TeamPackage/index.tsx`
- Create: `isdp/web/src/api/teamPackage.ts`

- [ ] **Step 1: 创建团队包 API 模块**

```typescript
// 文件路径: isdp/web/src/api/teamPackage.ts
import client from './client';

export interface TeamPackagePreview {
  workflow: { name: string; exists: boolean };
  roles: Array<{ name: string; exists: boolean; localId?: string }>;
  assets: {
    skills: Array<{ name: string; exists: boolean }>;
    commands: Array<{ name: string; exists: boolean }>;
    subagents: Array<{ name: string; exists: boolean }>;
    rules: Array<{ name: string; exists: boolean }>;
    settings: Array<{ name: string; exists: boolean }>;
  };
}

export interface ImportConfirm {
  mode: 'overwrite' | 'skip' | 'selective';
  workflowAction: 'overwrite' | 'skip';
  roleActions: Array<{ name: string; action: 'overwrite' | 'skip' }>;
  assetActions: Array<{ assetType: string; name: string; action: 'overwrite' | 'skip' }>;
}

export const teamPackageApi = {
  import: async (file: File): Promise<TeamPackagePreview> => {
    const formData = new FormData();
    formData.append('file', file);
    const response = await client.post('/api/team-packages/import', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    return response.data;
  },

  importConfirm: async (file: File, confirm: ImportConfirm): Promise<any> => {
    const formData = new FormData();
    formData.append('file', file);
    const blob = new Blob([JSON.stringify(confirm)], { type: 'application/json' });
    formData.append('confirm', blob);
    const response = await client.post('/api/team-packages/import/confirm', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    return response.data;
  },

  export: async (workflowId: string): Promise<Blob> => {
    const response = await client.post('/api/team-packages/export', { workflowId }, {
      responseType: 'blob',
    });
    return response.data;
  },
};

export default teamPackageApi;
```

- [ ] **Step 2: 创建团队包页面组件**

页面布局：
- 左侧：导入区域（拖拽上传 + 预览 + 确认导入）
- 右侧：导出区域（选择工作流 + 导出按钮）

组件需要：
1. 工作流下拉选择器
2. 文件上传区域
3. 预览表格（显示工作流、角色、资产的冲突情况）
4. 操作按钮（全部覆盖、全部跳过、确认导入）

- [ ] **Step 3: 提交团队包页面**

```bash
git add isdp/web/src/pages/TeamPackage/index.tsx
git add isdp/web/src/api/teamPackage.ts
git commit -m "feat(ui): 添加团队包页面"
```

---

## Task 12: 重写资产包前端页面

**Files:**
- Modify: `isdp/web/src/pages/AssetPackage/index.tsx`
- Modify: `isdp/web/src/api/client.ts`

- [ ] **Step 1: 简化资产包 API**

在 `client.ts` 中简化资产包 API，移除 list/get/delete 方法，只保留 import/export。

- [ ] **Step 2: 重写资产包页面**

改为两卡片布局：
- 左侧：导入区域（拖拽上传 + 结果展示）
- 右侧：导出区域（勾选资产 + 导出按钮）

移除：
- 列表展示
- 分页
- 搜索
- 详情模态框
- 删除功能

- [ ] **Step 3: 提交资产包页面重写**

```bash
git add isdp/web/src/pages/AssetPackage/index.tsx
git add isdp/web/src/api/client.ts
git commit -m "refactor(ui): 重写资产包页面为两卡片布局"
```

---

## Task 13: 更新前端路由配置

**Files:**
- Modify: `isdp/web/src/App.tsx`

- [ ] **Step 1: 更新路由配置**

```typescript
// 添加团队包和资产包的新路由
<Route path="management-tools" element={<Navigate to="/management-tools/team-package" replace />} />
<Route path="management-tools/team-package" element={<TeamPackagePage />} />
<Route path="management-tools/asset-package" element={<AssetPackageManagement />} />

// 更新旧路由重定向
<Route path="asset-packages" element={<Navigate to="/management-tools/asset-package" replace />} />
```

- [ ] **Step 2: 提交路由配置**

```bash
git add isdp/web/src/App.tsx
git commit -m "feat(router): 添加管理工具路由"
```

---

## Task 14: 执行数据库迁移

**Files:**
- 执行迁移脚本

- [ ] **Step 1: 连接数据库执行迁移**

使用项目的数据库连接信息执行：
```bash
mysqlsh --sql -h <host> -P 3306 -u <user> -p<password> -D <database> -f sql-change/migrations/202604010001_drop_asset_packages_table.sql
```

- [ ] **Step 2: 验证表已删除**

```sql
SHOW TABLES LIKE 'asset_packages';
```

---

## Task 15: 构建测试

**Files:**
- 无文件修改，仅测试

- [ ] **Step 1: 构建后端**

```bash
cd isdp && go build ./cmd/server
```

- [ ] **Step 2: 构建前端**

```bash
cd isdp/web && npm run build
```

- [ ] **Step 3: 启动服务进行功能测试**

```bash
cd isdp && go run ./cmd/server
```

测试内容：
1. 访问 `/management-tools/team-package` 页面
2. 访问 `/management-tools/asset-package` 页面
3. 测试资产包导入导出
4. 测试团队包导入导出

---

## Task 16: 提交最终变更

- [ ] **Step 1: 检查所有变更**

```bash
git status
```

- [ ] **Step 2: 提交所有未提交的变更**

```bash
git add -A
git commit -m "feat: 资产包轻量化与团队包功能实现"
```

- [ ] **Step 3: 更新 CHANGELOG**

在 `docs/CHANGELOG.md` 中记录本次变更。