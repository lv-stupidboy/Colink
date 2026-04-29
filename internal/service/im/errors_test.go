package im

import (
	"testing"
)

func TestErrorCategoryString(t *testing.T) {
	tests := []struct {
		category ErrorCategory
		expected string
	}{
		{ErrCategoryRateLimit, "RateLimit"},
		{ErrCategoryServerError, "ServerError"},
		{ErrCategoryClientError, "ClientError"},
		{ErrCategoryParseError, "ParseError"},
		{ErrCategoryNetwork, "Network"},
		{ErrorCategory(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.category.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestErrorCategoryShouldRetry(t *testing.T) {
	tests := []struct {
		category    ErrorCategory
		shouldRetry bool
	}{
		{ErrCategoryRateLimit, true},
		{ErrCategoryServerError, true},
		{ErrCategoryNetwork, true},
		{ErrCategoryClientError, false},
		{ErrCategoryParseError, false},
		{ErrorCategory(999), false},
	}

	for _, tt := range tests {
		t.Run(tt.category.String(), func(t *testing.T) {
			if got := tt.category.ShouldRetry(); got != tt.shouldRetry {
				t.Errorf("ShouldRetry() = %v, want %v", got, tt.shouldRetry)
			}
		})
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		result   SendResult
		expected ErrorCategory
	}{
		// Rate limit tests
		{
			name:     "HTTP 429 rate limit",
			result:   SendResult{OK: false, Error: "", HTTPStatus: 429},
			expected: ErrCategoryRateLimit,
		},
		{
			name:     "HTTP 429 with error message",
			result:   SendResult{OK: false, Error: "too many requests", HTTPStatus: 429},
			expected: ErrCategoryRateLimit,
		},

		// Server error tests
		{
			name:     "HTTP 500 internal server error",
			result:   SendResult{OK: false, Error: "", HTTPStatus: 500},
			expected: ErrCategoryServerError,
		},
		{
			name:     "HTTP 503 service unavailable",
			result:   SendResult{OK: false, Error: "service unavailable", HTTPStatus: 503},
			expected: ErrCategoryServerError,
		},
		{
			name:     "HTTP 502 bad gateway",
			result:   SendResult{OK: false, Error: "", HTTPStatus: 502},
			expected: ErrCategoryServerError,
		},

		// Client error tests
		{
			name:     "HTTP 400 bad request",
			result:   SendResult{OK: false, Error: "bad request", HTTPStatus: 400},
			expected: ErrCategoryClientError,
		},
		{
			name:     "HTTP 401 unauthorized",
			result:   SendResult{OK: false, Error: "unauthorized", HTTPStatus: 401},
			expected: ErrCategoryClientError,
		},
		{
			name:     "HTTP 403 forbidden",
			result:   SendResult{OK: false, Error: "forbidden", HTTPStatus: 403},
			expected: ErrCategoryClientError,
		},
		{
			name:     "HTTP 404 not found",
			result:   SendResult{OK: false, Error: "not found", HTTPStatus: 404},
			expected: ErrCategoryClientError,
		},

		// Parse error tests
		{
			name:     "JSON parse error",
			result:   SendResult{OK: false, Error: "failed to parse JSON", HTTPStatus: 0},
			expected: ErrCategoryParseError,
		},
		{
			name:     "JSON unmarshal error",
			result:   SendResult{OK: false, Error: "json: cannot unmarshal", HTTPStatus: 0},
			expected: ErrCategoryParseError,
		},
		{
			name:     "Parse error uppercase",
			result:   SendResult{OK: false, Error: "Parse error in response", HTTPStatus: 0},
			expected: ErrCategoryParseError,
		},

		// Network error tests
		{
			name:     "Timeout error",
			result:   SendResult{OK: false, Error: "context deadline exceeded (timeout)", HTTPStatus: 0},
			expected: ErrCategoryNetwork,
		},
		{
			name:     "Connection refused",
			result:   SendResult{OK: false, Error: "connection refused", HTTPStatus: 0},
			expected: ErrCategoryNetwork,
		},
		{
			name:     "No such host",
			result:   SendResult{OK: false, Error: "no such host", HTTPStatus: 0},
			expected: ErrCategoryNetwork,
		},
		{
			name:     "DNS resolution failure",
			result:   SendResult{OK: false, Error: "lookup example.com: no such host", HTTPStatus: 0},
			expected: ErrCategoryNetwork,
		},

		// Default to network
		{
			name:     "Unknown error defaults to network",
			result:   SendResult{OK: false, Error: "some unknown error", HTTPStatus: 0},
			expected: ErrCategoryNetwork,
		},
		{
			name:     "Empty error defaults to network",
			result:   SendResult{OK: false, Error: "", HTTPStatus: 0},
			expected: ErrCategoryNetwork,
		},

		// Edge cases
		{
			name:     "HTTP 429 takes precedence over parse error message",
			result:   SendResult{OK: false, Error: "json parse error", HTTPStatus: 429},
			expected: ErrCategoryRateLimit,
		},
		{
			name:     "HTTP 500 takes precedence over network error message",
			result:   SendResult{OK: false, Error: "timeout", HTTPStatus: 500},
			expected: ErrCategoryServerError,
		},
		{
			name:     "HTTP 400 takes precedence over network error message",
			result:   SendResult{OK: false, Error: "connection refused", HTTPStatus: 400},
			expected: ErrCategoryClientError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyError(tt.result)
			if got != tt.expected {
				t.Errorf("ClassifyError() = %v (%s), want %v (%s)",
					got, got.String(), tt.expected, tt.expected.String())
			}
		})
	}
}

func TestClassifyErrorWithShouldRetry(t *testing.T) {
	tests := []struct {
		name        string
		result      SendResult
		shouldRetry bool
	}{
		{
			name:        "Rate limit should retry",
			result:      SendResult{OK: false, Error: "", HTTPStatus: 429},
			shouldRetry: true,
		},
		{
			name:        "Server error should retry",
			result:      SendResult{OK: false, Error: "", HTTPStatus: 500},
			shouldRetry: true,
		},
		{
			name:        "Network error should retry",
			result:      SendResult{OK: false, Error: "timeout", HTTPStatus: 0},
			shouldRetry: true,
		},
		{
			name:        "Client error should not retry",
			result:      SendResult{OK: false, Error: "bad request", HTTPStatus: 400},
			shouldRetry: false,
		},
		{
			name:        "Parse error should not retry",
			result:      SendResult{OK: false, Error: "json parse error", HTTPStatus: 0},
			shouldRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category := ClassifyError(tt.result)
			if got := category.ShouldRetry(); got != tt.shouldRetry {
				t.Errorf("ClassifyError().ShouldRetry() = %v, want %v", got, tt.shouldRetry)
			}
		})
	}
}
