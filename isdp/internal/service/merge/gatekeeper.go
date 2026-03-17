package merge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// Gatekeeper 合入门禁服务
type Gatekeeper struct {
	reviewRepo   *repo.ReviewRepository
	artifactRepo *repo.ArtifactRepository
	threadRepo   *repo.ThreadRepository
}

// NewGatekeeper 创建门禁服务
func NewGatekeeper(reviewRepo *repo.ReviewRepository, artifactRepo *repo.ArtifactRepository, threadRepo *repo.ThreadRepository) *Gatekeeper {
	return &Gatekeeper{
		reviewRepo:   reviewRepo,
		artifactRepo: artifactRepo,
		threadRepo:   threadRepo,
	}
}

// ReviewGrade 评审等级
type ReviewGrade string

const (
	GradeP1 ReviewGrade = "P1" // 必须修复
	GradeP2 ReviewGrade = "P2" // 建议修复
	GradeP3 ReviewGrade = "P3" // 可选修复
)

// MergeDecision 合并决策
type MergeDecision string

const (
	DecisionAllow    MergeDecision = "allow"    // 允许合并
	DecisionBlock    MergeDecision = "block"    // 阻止合并
	DecisionConditional MergeDecision = "conditional" // 条件性允许
)

// CheckResult 检查结果
type CheckResult struct {
	Decision       MergeDecision `json:"decision"`
	Summary        string        `json:"summary"`
	P1Issues       int           `json:"p1Issues"`
	P2Issues       int           `json:"p2Issues"`
	P3Issues       int           `json:"p3Issues"`
	ResolvedP1     int           `json:"resolvedP1"`
	ResolvedP2     int           `json:"resolvedP2"`
	Unresolved      []Issue      `json:"unresolved"`
	Recommendations []string     `json:"recommendations"`
}

// Issue 问题
type Issue struct {
	ID          string      `json:"id"`
	Grade       ReviewGrade `json:"grade"`
	Description string      `json:"description"`
	File        string      `json:"file"`
	Line        int         `json:"line"`
	Status      string      `json:"status"`
}

// CheckMerge 检查是否可以合并
func (g *Gatekeeper) CheckMerge(ctx context.Context, threadID uuid.UUID) (*CheckResult, error) {
	// 获取所有评审
	reviews, err := g.reviewRepo.FindByThreadID(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviews: %w", err)
	}

	result := &CheckResult{
		Unresolved: make([]Issue, 0),
		Recommendations: make([]string, 0),
	}

	// 统计问题
	for _, review := range reviews {
		issues := g.parseReviewIssues(review.Content)
		for _, issue := range issues {
			switch issue.Grade {
			case GradeP1:
				result.P1Issues++
				if issue.Status != "resolved" {
					result.Unresolved = append(result.Unresolved, issue)
				} else {
					result.ResolvedP1++
				}
			case GradeP2:
				result.P2Issues++
				if issue.Status == "resolved" {
					result.ResolvedP2++
				}
			case GradeP3:
				result.P3Issues++
			}
		}
	}

	// 决策逻辑
	unresolvedP1 := result.P1Issues - result.ResolvedP1
	unresolvedP2 := result.P2Issues - result.ResolvedP2

	if unresolvedP1 > 0 {
		result.Decision = DecisionBlock
		result.Summary = fmt.Sprintf("存在 %d 个未解决的 P1 问题，阻止合并", unresolvedP1)
		result.Recommendations = append(result.Recommendations, "请优先修复所有 P1 级别的问题")
	} else if unresolvedP2 > 3 {
		result.Decision = DecisionConditional
		result.Summary = fmt.Sprintf("存在 %d 个未解决的 P2 问题，建议修复后合并", unresolvedP2)
		result.Recommendations = append(result.Recommendations, "建议修复主要的 P2 问题以提高代码质量")
	} else {
		result.Decision = DecisionAllow
		if result.P1Issues > 0 {
			result.Summary = fmt.Sprintf("所有 %d 个 P1 问题已解决，允许合并", result.P1Issues)
		} else {
			result.Summary = "评审通过，允许合并"
		}
	}

	return result, nil
}

// parseReviewIssues 解析评审问题
func (g *Gatekeeper) parseReviewIssues(content string) []Issue {
	var issues []Issue
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "P1:") || strings.HasPrefix(line, "P1：") {
			issues = append(issues, Issue{
				Grade:       GradeP1,
				Description: strings.TrimPrefix(strings.TrimPrefix(line, "P1:"), "P1："),
				Status:      "open",
			})
		} else if strings.HasPrefix(line, "P2:") || strings.HasPrefix(line, "P2：") {
			issues = append(issues, Issue{
				Grade:       GradeP2,
				Description: strings.TrimPrefix(strings.TrimPrefix(line, "P2:"), "P2："),
				Status:      "open",
			})
		} else if strings.HasPrefix(line, "P3:") || strings.HasPrefix(line, "P3：") {
			issues = append(issues, Issue{
				Grade:       GradeP3,
				Description: strings.TrimPrefix(strings.TrimPrefix(line, "P3:"), "P3："),
				Status:      "open",
			})
		}
	}

	return issues
}

// RecordReview 记录评审
func (g *Gatekeeper) RecordReview(ctx context.Context, req *ReviewRequest) (*model.Artifact, error) {
	artifact := &model.Artifact{
		ID:        uuid.New(),
		ThreadID:  req.ThreadID,
		Type:      model.ArtifactTypeReview,
		Name:      fmt.Sprintf("Review by %s", req.Reviewer),
		Content:   req.Content,
		Metadata:  req.Metadata,
		CreatedAt: time.Now(),
	}

	if err := g.artifactRepo.Create(ctx, artifact); err != nil {
		return nil, fmt.Errorf("failed to create review artifact: %w", err)
	}

	return artifact, nil
}

// ResolveIssue 解决问题
func (g *Gatekeeper) ResolveIssue(ctx context.Context, threadID uuid.UUID, issueID string, resolution string) error {
	// 更新问题状态
	// 实际实现中需要修改artifact内容
	return nil
}

// GenerateHandoverReport 生成交接报告
func (g *Gatekeeper) GenerateHandoverReport(ctx context.Context, threadID uuid.UUID) (*HandoverReport, error) {
	thread, err := g.threadRepo.FindByID(ctx, threadID)
	if err != nil {
		return nil, err
	}

	reviews, err := g.reviewRepo.FindByThreadID(ctx, threadID)
	if err != nil {
		return nil, err
	}

	artifacts, err := g.artifactRepo.FindByThreadID(ctx, threadID)
	if err != nil {
		return nil, err
	}

	report := &HandoverReport{
		ThreadID:   threadID,
		Phase:      thread.CurrentPhase,
		Status:     thread.Status,
		GeneratedAt: time.Now(),
	}

	// 汇总评审结果
	for _, review := range reviews {
		issues := g.parseReviewIssues(review.Content)
		for _, issue := range issues {
			report.TotalIssues++
			switch issue.Grade {
			case GradeP1:
				report.P1Count++
			case GradeP2:
				report.P2Count++
			case GradeP3:
				report.P3Count++
			}
		}
	}

	// 汇总工作产物
	for _, artifact := range artifacts {
		report.Artifacts = append(report.Artifacts, ArtifactSummary{
			ID:   artifact.ID,
			Type: string(artifact.Type),
			Name: artifact.Name,
		})
	}

	return report, nil
}

// ReviewRequest 评审请求
type ReviewRequest struct {
	ThreadID  uuid.UUID
	Reviewer  string
	Content   string
	Metadata  map[string]interface{}
}

// HandoverReport 交接报告
type HandoverReport struct {
	ThreadID    uuid.UUID
	Phase       model.Phase
	Status      model.ThreadStatus
	GeneratedAt time.Time
	TotalIssues int
	P1Count     int
	P2Count     int
	P3Count     int
	Artifacts   []ArtifactSummary
}

// ArtifactSummary 工作产物摘要
type ArtifactSummary struct {
	ID   uuid.UUID
	Type string
	Name string
}