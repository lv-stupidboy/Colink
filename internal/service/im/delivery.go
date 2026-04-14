package im

import (
	"context"
	"strings"

	"go.uber.org/zap"
)

var deliverySleepFn = sleepWithContext

// DeliveryService orchestrates deduplication, rate limiting, retry, and chunked IM delivery.
type DeliveryService struct {
	adapter     IMAdapter
	retryCfg    RetryConfig
	rateLimiter *RateLimiter
	dedupCache  *DedupCache
	logger      *zap.Logger
}

// NewDeliveryService creates a new DeliveryService.
func NewDeliveryService(adapter IMAdapter, retryCfg RetryConfig, rateLimiter *RateLimiter, dedupCache *DedupCache, logger *zap.Logger) *DeliveryService {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &DeliveryService{
		adapter:     adapter,
		retryCfg:    retryCfg,
		rateLimiter: rateLimiter,
		dedupCache:  dedupCache,
		logger:      logger,
	}
}

// DeliverText sends text to a chat with dedup, rate limit, chunking, and retry.
func (d *DeliveryService) DeliverText(ctx context.Context, chatID, text, dedupKey string) DeliveryResult {
	if d.isDuplicate(dedupKey) {
		return DeliveryResult{OK: true}
	}

	if d.isRateLimited(chatID) {
		return DeliveryResult{OK: false, FinalError: "rate limited", Category: ErrCategoryRateLimit}
	}

	cfg := normalizeRetryConfig(d.retryCfg)
	chunks := chunkText(text, d.adapter.MaxMessageLength())

	result := DeliveryResult{OK: true, Category: ErrCategoryNetwork}
	for i, chunk := range chunks {
		sendResult, attempts := d.sendWithRetry(ctx, func() SendResult {
			return d.adapter.SendText(ctx, chatID, chunk)
		})
		result.Attempts += attempts

		if !sendResult.OK {
			result.OK = false
			result.FinalError = sendResult.Error
			result.Category = ClassifyError(sendResult)
			return result
		}

		if i < len(chunks)-1 {
			if err := deliverySleepFn(ctx, cfg.InterMsgDelay); err != nil {
				return DeliveryResult{OK: false, Attempts: result.Attempts, FinalError: err.Error(), Category: ErrCategoryNetwork}
			}
		}
	}

	return result
}

// DeliverCard sends a card payload with dedup, rate limit, and retry.
func (d *DeliveryService) DeliverCard(ctx context.Context, chatID, cardJSON, dedupKey string) DeliveryResult {
	if d.isDuplicate(dedupKey) {
		return DeliveryResult{OK: true}
	}

	if d.isRateLimited(chatID) {
		return DeliveryResult{OK: false, FinalError: "rate limited", Category: ErrCategoryRateLimit}
	}

	sendResult, attempts := d.sendWithRetry(ctx, func() SendResult {
		return d.adapter.SendCard(ctx, chatID, cardJSON)
	})

	if sendResult.OK {
		return DeliveryResult{OK: true, Attempts: attempts}
	}

	return DeliveryResult{
		OK:         false,
		Attempts:   attempts,
		FinalError: sendResult.Error,
		Category:   ClassifyError(sendResult),
	}
}

func (d *DeliveryService) sendWithRetry(ctx context.Context, sendFn func() SendResult) (SendResult, int) {
	attempts := 0
	result := RetryableSend(ctx, d.retryCfg, d.logger, func() SendResult {
		attempts++
		return sendFn()
	})
	return result, attempts
}

func (d *DeliveryService) isDuplicate(dedupKey string) bool {
	if d.dedupCache == nil || dedupKey == "" {
		return false
	}
	return d.dedupCache.IsDuplicate(dedupKey)
}

func (d *DeliveryService) isRateLimited(chatID string) bool {
	if d.rateLimiter == nil {
		return false
	}
	return !d.rateLimiter.TryAcquire(chatID)
}

func chunkText(text string, maxLen int) []string {
	if maxLen <= 0 || len(text) <= maxLen {
		return []string{text}
	}

	chunks := make([]string, 0, len(text)/maxLen+1)
	remainder := text
	for len(remainder) > maxLen {
		head := remainder[:maxLen]
		split := strings.LastIndex(head, "\n")
		if split == -1 || split < maxLen/2 {
			split = maxLen
		}

		chunks = append(chunks, remainder[:split])
		remainder = strings.TrimLeft(remainder[split:], "\n")
	}

	if remainder != "" {
		chunks = append(chunks, remainder)
	}

	return chunks
}
