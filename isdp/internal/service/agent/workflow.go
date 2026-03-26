package agent

import (
	"context"
	"errors"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// WorkflowEngine 工作流引擎
type WorkflowEngine struct {
	threadRepo *repo.ThreadRepository
	msgRepo    *repo.MessageRepository
	configSvc  *ConfigService
}

// NewWorkflowEngine 创建工作流引擎
func NewWorkflowEngine(threadRepo *repo.ThreadRepository, msgRepo *repo.MessageRepository, configSvc *ConfigService) *WorkflowEngine {
	return &WorkflowEngine{
		threadRepo: threadRepo,
		msgRepo:    msgRepo,
		configSvc:  configSvc,
	}
}

// PhaseTransition 阶段转换
func (e *WorkflowEngine) PhaseTransition(ctx context.Context, threadID uuid.UUID, toPhase model.Phase) error {
	thread, err := e.threadRepo.FindByID(ctx, threadID)
	if err != nil {
		return err
	}

	// 验证阶段转换是否有效
	if !isValidTransition(thread.CurrentPhase, toPhase) {
		return ErrInvalidPhaseTransition
	}

	thread.CurrentPhase = toPhase
	thread.UpdatedAt = time.Now()

	return e.threadRepo.Update(ctx, thread)
}

// GetNextPhase 获取下一阶段
func (e *WorkflowEngine) GetNextPhase(currentPhase model.Phase) model.Phase {
	switch currentPhase {
	case model.PhaseRequirement:
		return model.PhaseDesign
	case model.PhaseDesign:
		return model.PhaseDevelopment
	case model.PhaseDevelopment:
		return model.PhaseReview
	case model.PhaseReview:
		return model.PhaseMerge
	case model.PhaseMerge:
		return model.PhaseComplete
	default:
		return currentPhase
	}
}

// ValidatePhaseCompletion 验证阶段是否完成
func (e *WorkflowEngine) ValidatePhaseCompletion(ctx context.Context, threadID uuid.UUID, phase model.Phase) error {
	thread, err := e.threadRepo.FindByID(ctx, threadID)
	if err != nil {
		return err
	}

	if thread.CurrentPhase != phase {
		return ErrPhaseMismatch
	}

	// 检查阶段完成条件
	switch phase {
	case model.PhaseRequirement:
		// 需求阶段需要有需求文档
	case model.PhaseDesign:
		// 设计阶段需要有设计文档
	case model.PhaseDevelopment:
		// 开发阶段需要有代码提交
	case model.PhaseReview:
		// 评审阶段需要有评审通过
	case model.PhaseTest:
		// 测试阶段需要有测试通过
	}

	return nil
}

// isValidTransition 验证阶段转换是否有效
func isValidTransition(from, to model.Phase) bool {
	validTransitions := map[model.Phase][]model.Phase{
		model.PhaseRequirement:  {model.PhaseDesign},
		model.PhaseDesign:       {model.PhaseDevelopment, model.PhaseRequirement},
		model.PhaseDevelopment:  {model.PhaseReview, model.PhaseTest, model.PhaseDesign},
		model.PhaseReview:       {model.PhaseMerge, model.PhaseDevelopment},
		model.PhaseTest:         {model.PhaseDevelopment, model.PhaseMerge},
		model.PhaseMerge:        {model.PhaseComplete, model.PhaseDevelopment},
		model.PhaseComplete:     {},
	}

	allowed, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, phase := range allowed {
		if phase == to {
			return true
		}
	}
	return false
}

var (
	ErrInvalidPhaseTransition = errors.New("invalid phase transition")
	ErrPhaseMismatch          = errors.New("phase mismatch")
)