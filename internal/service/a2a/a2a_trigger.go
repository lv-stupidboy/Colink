package a2a

import (
	"context"
	"fmt"
	"sync"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/service/humantask"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
)

// A2ATriggerDeps A2A 触发依赖
type A2ATriggerDeps struct {
	Registry        *InvocationRegistry
	Orchestrator    *agent.Orchestrator
	WSHub           *ws.Hub
	Queue           *InvocationQueue
	HumanTaskSvc    *humantask.Service      // 人角色任务服务
	AgentConfigRepo *repo.AgentConfigRepository // Agent配置仓库
}

// A2ATriggerOptions A2A 触发选项
type A2ATriggerOptions struct {
	TargetCats         []string      // 目标 Agent IDs
	Content            string        // 触发内容
	UserID             string        // 用户 ID
	ThreadID           uuid.UUID     // 线程 ID
	TriggerMessage     *model.Message // 触发消息
	CallerCatID        string        // 调用者 Agent ID
	ParentInvocationID *uuid.UUID    // 父调用 ID
}

// A2AResult A2A 触发结果
type A2AResult struct {
	Enqueued []string // 成功入队的 Agent IDs
	Fallback bool     // 是否使用了 fallback 模式
}

// handleHumanRoleTask 处理人类角色任务创建
// 当目标是人类角色时，创建人工任务而非触发 Agent
// 返回: (是否处理, 入队的ID)
func handleHumanRoleTask(ctx context.Context, deps *A2ATriggerDeps, opts *A2ATriggerOptions, roleConfigID uuid.UUID) (bool, string) {
	// 检查依赖是否可用
	if deps.AgentConfigRepo == nil || deps.HumanTaskSvc == nil {
		return false, ""
	}

	// 获取角色配置
	roleConfig, err := deps.AgentConfigRepo.FindByID(ctx, roleConfigID)
	if err != nil {
		return false, ""
	}

	// 检查是否是人类角色
	if !roleConfig.Role.IsHumanRole() {
		return false, ""
	}

	// 获取调用者名称
	callerName := opts.CallerCatID
	if opts.CallerCatID != "" {
		if callerID, err := uuid.Parse(opts.CallerCatID); err == nil {
			if callerConfig, err := deps.AgentConfigRepo.FindByID(ctx, callerID); err == nil {
				callerName = callerConfig.Name
			}
		}
	}

	// 创建人工任务
	var sourceInvocationID uuid.UUID
	if opts.ParentInvocationID != nil {
		sourceInvocationID = *opts.ParentInvocationID
	}

	_, createErr := deps.HumanTaskSvc.CreateTask(ctx, opts.ThreadID, roleConfigID, opts.Content, sourceInvocationID, callerName)
	if createErr != nil {
		fmt.Printf("[handleHumanRoleTask] Failed to create human task: %v\n", createErr)
		return false, ""
	}

	return true, roleConfigID.String()
}

// EnqueueA2ATargets 将 @mentioned 的 Agent 加入工作队列
//
// 流程：
// 1. 检查深度限制
// 2. 检查去重
// 3. 加入队列或直接触发
func EnqueueA2ATargets(ctx context.Context, deps *A2ATriggerDeps, opts *A2ATriggerOptions) (*A2AResult, error) {
	if deps == nil || opts == nil {
		return nil, fmt.Errorf("invalid parameters")
	}

	enqueued := make([]string, 0, len(opts.TargetCats))

	// 如果有队列，使用队列模式
	if deps.Queue != nil {
		for _, catID := range opts.TargetCats {
			// 解析 catID 为 UUID
			roleConfigID, err := uuid.Parse(catID)
			if err != nil {
				// 无效的 UUID，跳过
				continue
			}

			// 检查是否是人类角色，如果是则创建人工任务
			if handled, enqueuedID := handleHumanRoleTask(ctx, deps, opts, roleConfigID); handled {
				enqueued = append(enqueued, enqueuedID)
				continue
			}

			// 深度限制检查
			currentDepth := deps.Queue.CountAgentEntriesForThread(opts.ThreadID.String())
			if currentDepth >= MaxA2ADepth {
				break
			}

			// 去重检查
			if deps.Queue.HasQueuedAgent(opts.ThreadID.String(), catID) {
				continue
			}

			// 入队
			entry := &QueueEntry{
				ThreadID:      opts.ThreadID,
				UserID:        opts.UserID,
				Content:       opts.Content,
				Source:        "agent",
				TargetAgents:  []string{catID},
				Intent:        "execute",
				AutoExecute:   true,
				CallerAgentID: opts.CallerCatID,
			}

			if _, err := deps.Queue.Enqueue(entry); err != nil {
				continue
			}

			enqueued = append(enqueued, catID)
		}

		// 广播队列更新
		if deps.WSHub != nil && len(enqueued) > 0 {
			deps.WSHub.BroadcastToThread(opts.ThreadID.String(), ws.WSMessage{
				Type:      "queue_updated",
				ThreadID:  opts.ThreadID.String(),
				Timestamp: model.Now(),
				Payload: map[string]interface{}{
					"action":   "enqueued",
					"enqueued": enqueued,
				},
			})
		}

		return &A2AResult{Enqueued: enqueued, Fallback: false}, nil
	}

	// 无队列时直接触发
	for _, catID := range opts.TargetCats {
		// 解析 catID 为 UUID
		roleConfigID, err := uuid.Parse(catID)
		if err != nil {
			// 无效的 UUID，跳过
			continue
		}

		// 检查是否是人类角色，如果是则创建人工任务
		if handled, enqueuedID := handleHumanRoleTask(ctx, deps, opts, roleConfigID); handled {
			enqueued = append(enqueued, enqueuedID)
			continue
		}

		// 检查 slot 是否被占用
		if deps.Registry.HasActiveSlot(opts.ThreadID, catID) {
			continue
		}

		// 直接触发 Agent
		// catID 就是 role（如 "backend_developer"），直接使用
		if deps.Orchestrator != nil {
			go func(targetCatID string) {
				req := &agent.SpawnRequest{
					ThreadID: opts.ThreadID,
					Role:     model.AgentRole(targetCatID),
					Input:    opts.Content,
				}
				_, _ = deps.Orchestrator.SpawnAgent(context.Background(), req)
			}(catID)
		}

		enqueued = append(enqueued, catID)
	}

	return &A2AResult{Enqueued: enqueued, Fallback: true}, nil
}

// A2AHandoffEvent A2A 交接事件
type A2AHandoffEvent struct {
	FromCat  string    `json:"fromCat"`
	ToCat    string    `json:"toCat"`
	ThreadID uuid.UUID `json:"threadId"`
	Depth    int       `json:"depth"`
}

// A2AEventBus A2A 事件总线（用于调试和审计）
type A2AEventBus struct {
	subscribers []chan A2AHandoffEvent
	mu          sync.RWMutex
}

// NewA2AEventBus 创建事件总线
func NewA2AEventBus() *A2AEventBus {
	return &A2AEventBus{
		subscribers: make([]chan A2AHandoffEvent, 0),
	}
}

// Subscribe 订阅事件
func (b *A2AEventBus) Subscribe() chan A2AHandoffEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan A2AHandoffEvent, 100)
	b.subscribers = append(b.subscribers, ch)
	return ch
}

// Unsubscribe 取消订阅
func (b *A2AEventBus) Unsubscribe(ch chan A2AHandoffEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, sub := range b.subscribers {
		if sub == ch {
			b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
			close(ch)
			break
		}
	}
}

// Publish 发布事件
func (b *A2AEventBus) Publish(event A2AHandoffEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscribers {
		select {
		case sub <- event:
		default:
			// channel full, skip
		}
	}
}

// 全局事件总线
var globalA2AEventBus = NewA2AEventBus()

// GetA2AEventBus 获取全局事件总线
func GetA2AEventBus() *A2AEventBus {
	return globalA2AEventBus
}