package im

import (
	"strings"
)

// ErrorCategory represents the classification of an error.
type ErrorCategory int

const (
	ErrCategoryRateLimit ErrorCategory = iota
	ErrCategoryServerError
	ErrCategoryClientError
	ErrCategoryParseError
	ErrCategoryNetwork
)

// String returns the string representation of the error category.
func (c ErrorCategory) String() string {
	switch c {
	case ErrCategoryRateLimit:
		return "RateLimit"
	case ErrCategoryServerError:
		return "ServerError"
	case ErrCategoryClientError:
		return "ClientError"
	case ErrCategoryParseError:
		return "ParseError"
	case ErrCategoryNetwork:
		return "Network"
	default:
		return "Unknown"
	}
}

// ShouldRetry returns true if the error is retryable.
func (c ErrorCategory) ShouldRetry() bool {
	switch c {
	case ErrCategoryRateLimit, ErrCategoryServerError, ErrCategoryNetwork:
		return true
	case ErrCategoryClientError, ErrCategoryParseError:
		return false
	default:
		return false
	}
}

// SendResult represents the result of sending a message.
type SendResult struct {
	OK         bool
	Error      string
	HTTPStatus int
}

// ClassifyError classifies an error based on HTTP status and error message.
func ClassifyError(result SendResult) ErrorCategory {
	// Check HTTP status first
	if result.HTTPStatus == 429 {
		return ErrCategoryRateLimit
	}
	if result.HTTPStatus >= 500 {
		return ErrCategoryServerError
	}
	if result.HTTPStatus >= 400 && result.HTTPStatus < 500 {
		return ErrCategoryClientError
	}

	// Check error message for parse errors
	if result.Error != "" {
		lowerErr := strings.ToLower(result.Error)
		if strings.Contains(lowerErr, "parse") ||
			strings.Contains(lowerErr, "json") ||
			strings.Contains(lowerErr, "unmarshal") {
			return ErrCategoryParseError
		}

		// Check error message for network errors
		if strings.Contains(lowerErr, "timeout") ||
			strings.Contains(lowerErr, "connection refused") ||
			strings.Contains(lowerErr, "no such host") {
			return ErrCategoryNetwork
		}
	}

	// Default to network error
	return ErrCategoryNetwork
}
