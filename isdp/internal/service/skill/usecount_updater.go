package skill

import (
	"context"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// UseCountUpdater 技能使用次数更新器
type UseCountUpdater struct {
	skillRepo   *repo.SkillRepository
	projectRepo *repo.ProjectRepository
	bindingRepo *repo.AgentSkillBindingRepository
	workflowSvc interface {
		GetAgentIDs(ctx context.Context, templateID uuid.UUID) ([]uuid.UUID, error)
	}
	stopChan chan struct{}
	logger   *zap.Logger
}

// NewUseCountUpdater 创建使用次数更新器
func NewUseCountUpdater(
	skillRepo *repo.SkillRepository,
	projectRepo *repo.ProjectRepository,
	bindingRepo *repo.AgentSkillBindingRepository,
) *UseCountUpdater {
	return &UseCountUpdater{
		skillRepo:   skillRepo,
		projectRepo: projectRepo,
		bindingRepo: bindingRepo,
		stopChan:    make(chan struct{}),
		logger:      zap.NewNop(),
	}
}

// SetLogger 设置日志记录器
func (u *UseCountUpdater) SetLogger(logger *zap.Logger) {
	if logger != nil {
		u.logger = logger
	}
}

// SetWorkflowService 设置工作流服务
func (u *UseCountUpdater) SetWorkflowService(svc interface {
	GetAgentIDs(ctx context.Context, templateID uuid.UUID) ([]uuid.UUID, error)
}) {
	u.workflowSvc = svc
}

// Start 启动定时更新（默认每小时更新一次）
func (u *UseCountUpdater) Start(interval time.Duration) {
	if interval == 0 {
		interval = time.Hour // 默认每小时更新一次
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// 启动后等待 10 秒再执行第一次，让服务完全启动
		time.Sleep(10 * time.Second)
		u.UpdateAll(context.Background())

		for {
			select {
			case <-ticker.C:
				u.UpdateAll(context.Background())
			case <-u.stopChan:
				return
			}
		}
	}()

	u.logger.Info("[SkillUseCount] 定时更新器已启动", zap.Duration("interval", interval))
}

// Stop 停止定时更新
func (u *UseCountUpdater) Stop() {
	close(u.stopChan)
	u.logger.Info("[SkillUseCount] 定时更新器已停止")
}

// UpdateAll 更新所有技能的使用次数
func (u *UseCountUpdater) UpdateAll(ctx context.Context) {
	startTime := time.Now()
	u.logger.Info("[SkillUseCount] 开始更新技能使用次数...")

	// 1. 获取所有项目
	projects, err := u.projectRepo.ListAll(ctx)
	if err != nil {
		u.logger.Error("[SkillUseCount] 获取项目列表失败", zap.Error(err))
		return
	}

	// 2. 统计每个技能被多少个项目使用
	skillUseCount := make(map[string]int)

	for _, project := range projects {
		if project.WorkflowTemplateID == nil {
			continue
		}

		// 获取工作流模板中的 Agent ID 列表
		agentIDs, err := u.getAgentIDsFromWorkflow(ctx, *project.WorkflowTemplateID)
		if err != nil {
			u.logger.Warn("[SkillUseCount] 获取工作流的 Agent 列表失败",
				zap.String("workflowId", project.WorkflowTemplateID.String()),
				zap.Error(err))
			continue
		}

		// 对每个 Agent，获取其绑定的技能
		projectSkills := make(map[string]bool) // 去重：一个项目中同一技能只算一次
		for _, agentID := range agentIDs {
			skillIDs, err := u.bindingRepo.FindByAgentRoleID(ctx, agentID)
			if err != nil {
				continue
			}
			for _, skillID := range skillIDs {
				projectSkills[skillID.String()] = true
			}
		}

		// 累加到统计中
		for skillID := range projectSkills {
			skillUseCount[skillID]++
		}
	}

	// 3. 更新数据库
	updatedCount := 0
	for skillID, count := range skillUseCount {
		if err := u.skillRepo.UpdateUseCount(ctx, skillID, count); err != nil {
			u.logger.Error("[SkillUseCount] 更新技能使用次数失败",
				zap.String("skillId", skillID),
				zap.Error(err))
		} else {
			updatedCount++
		}
	}

	// 4. 对于没有被任何项目使用的技能，设置为 0
	allSkills, _, err := u.skillRepo.List(ctx, &model.SkillListQuery{Page: 1, PageSize: 10000})
	if err == nil {
		for _, skill := range allSkills {
			if _, exists := skillUseCount[skill.ID.String()]; !exists {
				u.skillRepo.UpdateUseCount(ctx, skill.ID.String(), 0)
			}
		}
	}

	u.logger.Info("[SkillUseCount] 更新完成",
		zap.Duration("duration", time.Since(startTime)),
		zap.Int("projectCount", len(projects)),
		zap.Int("updatedCount", updatedCount))
}

// getAgentIDsFromWorkflow 从工作流模板获取 Agent ID 列表
func (u *UseCountUpdater) getAgentIDsFromWorkflow(ctx context.Context, templateID uuid.UUID) ([]uuid.UUID, error) {
	if u.workflowSvc == nil {
		return []uuid.UUID{}, nil
	}
	return u.workflowSvc.GetAgentIDs(ctx, templateID)
}