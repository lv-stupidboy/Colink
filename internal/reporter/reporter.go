// internal/reporter/reporter.go
package reporter

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

// Reporter handles periodic usage statistics reporting.
type Reporter struct {
	collector  *Collector
	config     Config
	logger     *zap.Logger
	version    string
	stopChan   chan struct{}
	httpClient *http.Client
}

// NewReporter creates a new Reporter instance.
func NewReporter(db *sql.DB, config Config, version string) *Reporter {
	return &Reporter{
		collector: NewCollector(db),
		config:    config,
		logger:    zap.NewNop(),
		version:   version,
		stopChan:  make(chan struct{}),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetLogger sets the logger for the Reporter.
func (r *Reporter) SetLogger(logger *zap.Logger) {
	if logger != nil {
		r.logger = logger
	}
}

// Start begins the periodic reporting in a background goroutine.
func (r *Reporter) Start() {
	r.logger.Info("[Reporter] 已启动", zap.Duration("interval", r.config.Interval))

	go func() {
		// Initial delay to avoid startup contention (参考 UseCountUpdater 模式)
		time.Sleep(10 * time.Second)

		ticker := time.NewTicker(r.config.Interval)
		defer ticker.Stop()

		// First report immediately after delay
		r.doReport()

		for {
			select {
			case <-ticker.C:
				r.doReport()
			case <-r.stopChan:
				r.logger.Info("[Reporter] 已停止")
				return
			}
		}
	}()
}

// Stop stops the reporter goroutine.
func (r *Reporter) Stop() {
	close(r.stopChan)
}

// doReport performs a single reporting cycle.
func (r *Reporter) doReport() {
	defer func() {
		if err := recover(); err != nil {
			r.logger.Error("[Reporter] panic recovered", zap.Any("error", err))
		}
	}()

	ctx := context.Background()
	stats, err := r.collector.CollectStats(ctx)
	if err != nil {
		r.logger.Error("[Reporter] 收集统计数据失败", zap.Error(err))
		return
	}

	data := ReportData{
		Username:   r.getUsername(),
		Version:    r.version,
		ReportTime: time.Now().Format(time.RFC3339),
		Stats:      stats,
	}

	if err := r.sendWithRetry(ctx, data); err != nil {
		r.logger.Error("[Reporter] 上报失败: 连续重试均失败",
			zap.Int("retries", r.config.RetryTimes),
			zap.Error(err))
	} else {
		r.logger.Info("[Reporter] 上报成功", zap.String("endpoint", r.config.Endpoint))
	}
}

// sendWithRetry sends data to endpoint with retry logic.
func (r *Reporter) sendWithRetry(ctx context.Context, data ReportData) error {
	var lastErr error

	for attempt := 0; attempt <= r.config.RetryTimes; attempt++ {
		if attempt > 0 {
			r.logger.Warn("[Reporter] 准备重试",
				zap.Int("attempt", attempt),
				zap.Int("max", r.config.RetryTimes))
			time.Sleep(r.config.RetryInterval)
		}

		err := r.send(ctx, data)
		if err == nil {
			return nil
		}
		lastErr = err
		r.logger.Warn("[Reporter] 发送失败",
			zap.Int("attempt", attempt),
			zap.Error(err))
	}

	return lastErr
}

// send performs a single HTTP POST to the endpoint.
func (r *Reporter) send(ctx context.Context, data ReportData) error {
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

// getUsername returns system username from environment.
func (r *Reporter) getUsername() string {
	// Try USER environment variable (Unix)
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	// Try USERNAME environment variable (Windows)
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}
	return "unknown"
}