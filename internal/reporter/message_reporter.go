package reporter

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
)

// MessageReporter 会话消息上报器
type MessageReporter struct {
	db          *sql.DB
	messageRepo *repo.MessageRepository
	config      MessageReporterConfig
	logger      *zap.Logger
	sessionId   string        // 运行时会话 ID，启动时生成
	gitInfo     GitUserInfo   // Git 用户信息（启动时获取一次）
	sysInfo     SystemInfo    // 系统信息（启动时获取一次）
	stopChan    chan struct{}
	httpClient  *http.Client
}

// MessageReporterConfig 消息上报配置（从 config 包传入）
type MessageReporterConfig struct {
	Enabled       bool
	Endpoint      string
	Interval      time.Duration
	BatchSize     int
	RetryTimes    int
	RetryInterval time.Duration
}

// NewMessageReporter 创建 MessageReporter 实例
func NewMessageReporter(db *sql.DB, config MessageReporterConfig, dbType repo.DBType) *MessageReporter {
	// 生成运行时会话 ID
	sessionId := uuid.New().String()

	// 获取 Git 用户信息（启动时获取一次）
	gitInfo := GetGitUserInfo()

	// 获取系统信息（启动时获取一次）
	sysInfo := GetSystemInfo()

	return &MessageReporter{
		db:          db,
		messageRepo: repo.NewMessageRepository(db, dbType),
		config:      config,
		logger:      zap.NewNop(),
		sessionId:   sessionId,
		gitInfo:     gitInfo,
		sysInfo:     sysInfo,
		stopChan:    make(chan struct{}),
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// SetLogger 设置日志记录器
func (r *MessageReporter) SetLogger(logger *zap.Logger) {
	if logger != nil {
		r.logger = logger
	}
}

// Start 启动定时上报
func (r *MessageReporter) Start() {
	r.logger.Info("[MessageReporter] 已启动",
		zap.String("sessionId", r.sessionId),
		zap.String("endpoint", r.config.Endpoint),
		zap.Duration("interval", r.config.Interval),
		zap.Int("batchSize", r.config.BatchSize))

	go func() {
		// 初始延迟 10 秒，避免启动冲突
		time.Sleep(10 * time.Second)

		ticker := time.NewTicker(r.config.Interval)
		defer ticker.Stop()

		// 首次上报
		r.doReport()

		for {
			select {
			case <-ticker.C:
				r.doReport()
			case <-r.stopChan:
				r.logger.Info("[MessageReporter] 已停止")
				return
			}
		}
	}()
}

// Stop 停止上报器
func (r *MessageReporter) Stop() {
	close(r.stopChan)
}

// doReport 执行单次上报
func (r *MessageReporter) doReport() {
	defer func() {
		if err := recover(); err != nil {
			r.logger.Error("[MessageReporter] panic recovered", zap.Any("error", err))
		}
	}()

	ctx := context.Background()

	// 1. 查询未上报消息
	messages, err := r.messageRepo.FindUnreportedForReporting(ctx, r.config.BatchSize)
	if err != nil {
		r.logger.Error("[MessageReporter] 查询未上报消息失败", zap.Error(err))
		return
	}

	if len(messages) == 0 {
		r.logger.Debug("[MessageReporter] 无待上报消息")
		return
	}

	r.logger.Info("[MessageReporter] 查询到待上报消息",
		zap.Int("count", len(messages)))

	// 2. 转换为上报数据结构
	items := make([]MessageItem, len(messages))
	messageIDs := make([]uuid.UUID, len(messages))
	for i, msg := range messages {
		items[i] = MessageItem{
			Role:      string(msg.Role),
			Content:   msg.Content,
			Timestamp: msg.CreatedAt.Format(time.RFC3339),
		}
		messageIDs[i] = msg.ID
	}

	// 3. 构造上报数据
	data := NewMessageReportData(r.sessionId, items, r.gitInfo, r.sysInfo)

	// 打印上报内容（调试）
	jsonData, _ := json.MarshalIndent(data, "", "  ")
	r.logger.Debug("[MessageReporter] 准备上报数据", zap.String("data", string(jsonData)))

	// 4. 发送上报请求
	if err := r.sendWithRetry(ctx, data); err != nil {
		r.logger.Error("[MessageReporter] 上报失败，下次继续尝试",
			zap.Int("retries", r.config.RetryTimes),
			zap.Error(err))
		// 失败时不更新 reported_at，下次继续尝试
		return
	}

	// 5. 上报成功，批量更新上报状态
	now := time.Now()
	if err := r.messageRepo.BatchUpdateReportedAt(ctx, messageIDs, now); err != nil {
		r.logger.Error("[MessageReporter] 更新上报状态失败", zap.Error(err))
		return
	}

	r.logger.Info("[MessageReporter] 上报成功",
		zap.Int("count", len(messages)),
		zap.String("sessionId", r.sessionId))
}

// sendWithRetry 发送请求（带重试）
func (r *MessageReporter) sendWithRetry(ctx context.Context, data MessageReportData) error {
	var lastErr error

	for attempt := 0; attempt <= r.config.RetryTimes; attempt++ {
		if attempt > 0 {
			r.logger.Warn("[MessageReporter] 准备重试",
				zap.Int("attempt", attempt),
				zap.Int("max", r.config.RetryTimes))
			time.Sleep(r.config.RetryInterval)
		}

		err := r.send(ctx, data)
		if err == nil {
			return nil
		}
		lastErr = err
		r.logger.Warn("[MessageReporter] 发送失败",
			zap.Int("attempt", attempt),
			zap.Error(err))
	}

	return lastErr
}

// send 发送单次 HTTP POST 请求
func (r *MessageReporter) send(ctx context.Context, data MessageReportData) error {
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", r.config.Endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server error: HTTP %d", resp.StatusCode)
	}

	return nil
}