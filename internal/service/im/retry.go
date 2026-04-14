package im

import (
	"context"
	"math"
	"math/rand"
	"time"

	"go.uber.org/zap"
)

// RetryConfig controls retry behavior for IM send operations.
type RetryConfig struct {
	MaxAttempts   int           // Default: 3
	BaseDelay     time.Duration // Default: 1s
	MaxDelay      time.Duration // Default: 30s
	JitterMax     time.Duration // Default: 500ms
	InterMsgDelay time.Duration // Default: 300ms
}

const (
	defaultMaxAttempts   = 3
	defaultBaseDelay     = 1 * time.Second
	defaultMaxDelay      = 30 * time.Second
	defaultJitterMax     = 500 * time.Millisecond
	defaultInterMsgDelay = 300 * time.Millisecond
)

var (
	retrySleepFn  = sleepWithContext
	retryJitterFn = randomJitter
)

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   defaultMaxAttempts,
		BaseDelay:     defaultBaseDelay,
		MaxDelay:      defaultMaxDelay,
		JitterMax:     defaultJitterMax,
		InterMsgDelay: defaultInterMsgDelay,
	}
}

// RetryableSend wraps a send function with retry logic.
// Classifies errors and only retries retryable categories.
// Returns final SendResult after all attempts exhausted.
func RetryableSend(ctx context.Context, cfg RetryConfig, logger *zap.Logger, sendFn func() SendResult) SendResult {
	if logger == nil {
		logger = zap.NewNop()
	}

	cfg = normalizeRetryConfig(cfg)

	var last SendResult
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return SendResult{OK: false, Error: err.Error()}
		}

		result := sendFn()
		if result.OK {
			return result
		}

		last = result
		category := ClassifyError(result)
		if !category.ShouldRetry() {
			return result
		}

		if attempt == cfg.MaxAttempts-1 {
			logger.Error("retry exhausted",
				zap.Int("attempt", attempt+1),
				zap.String("category", category.String()),
				zap.String("error", result.Error),
				zap.Int("httpStatus", result.HTTPStatus),
			)
			return last
		}

		delay := retryDelay(cfg, attempt)
		logger.Warn("retrying send",
			zap.Int("attempt", attempt+1),
			zap.String("category", category.String()),
			zap.Duration("delay", delay),
			zap.String("error", result.Error),
			zap.Int("httpStatus", result.HTTPStatus),
		)

		if err := retrySleepFn(ctx, delay); err != nil {
			return SendResult{OK: false, Error: err.Error()}
		}
	}

	return last
}

func normalizeRetryConfig(cfg RetryConfig) RetryConfig {
	def := DefaultRetryConfig()

	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = def.MaxAttempts
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = def.BaseDelay
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = def.MaxDelay
	}
	if cfg.JitterMax < 0 {
		cfg.JitterMax = 0
	} else if cfg.JitterMax == 0 {
		cfg.JitterMax = def.JitterMax
	}
	if cfg.InterMsgDelay < 0 {
		cfg.InterMsgDelay = 0
	} else if cfg.InterMsgDelay == 0 {
		cfg.InterMsgDelay = def.InterMsgDelay
	}

	if cfg.MaxDelay < cfg.BaseDelay {
		cfg.MaxDelay = cfg.BaseDelay
	}

	return cfg
}

func retryDelay(cfg RetryConfig, attempt int) time.Duration {
	base := exponentialBaseDelay(cfg.BaseDelay, attempt)
	jitter := retryJitterFn(cfg.JitterMax)

	delay := base + jitter
	if delay > cfg.MaxDelay {
		return cfg.MaxDelay
	}
	return delay
}

func exponentialBaseDelay(base time.Duration, attempt int) time.Duration {
	if attempt <= 0 {
		return base
	}

	maxInt := int64(math.MaxInt64)
	value := int64(base)
	for range attempt {
		if value > maxInt/2 {
			return time.Duration(maxInt)
		}
		value *= 2
	}

	return time.Duration(value)
}

func randomJitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(max) + 1))
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
