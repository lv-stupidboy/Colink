package agent

import (
	"errors"
	"testing"
)

// MockSession 实现 CircuitBreakerSession 接口用于测试
type MockSession struct {
	consecutiveFailures int
}

func (m *MockSession) GetConsecutiveRestoreFailures() int {
	return m.consecutiveFailures
}

func TestIsContextWindowOverflowError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"context too large", errors.New("context too large"), true},
		{"ran out of room", errors.New("ran out of room"), true},
		{"context window exceeded", errors.New("context window exceeded"), true},
		{"token limit exceeded", errors.New("token limit exceeded"), true},
		{"exceeds token limit", errors.New("exceeds token limit"), true},
		{"other error", errors.New("some other error"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsContextWindowOverflowError(tt.err)
			if result != tt.expected {
				t.Errorf("IsContextWindowOverflowError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestEstimateCJKTokens(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{"empty string", "", 0},
		{"ASCII text", "hello", 7}, // 5 runes * 1.5 = 7.5 -> 7
		{"CJK text", "你好世界", 6}, // 4 runes * 1.5 = 6
		{"mixed content", "hello你好", 10}, // 7 runes * 1.5 = 10.5 -> 10
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimateCJKTokens(tt.content)
			if result != tt.expected {
				t.Errorf("estimateCJKTokens() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestShouldSealOnOverflow(t *testing.T) {
	tests := []struct {
		name               string
		consecutiveFailures int
		expected           bool
	}{
		{"zero failures", 0, false},
		{"one failure", 1, false},
		{"two failures", 2, false},
		{"three failures", 3, true},
		{"four failures", 4, true},
		{"five failures", 5, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldSealOnOverflow(tt.consecutiveFailures)
			if result != tt.expected {
				t.Errorf("ShouldSealOnOverflow() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCheckCircuitBreaker(t *testing.T) {
	tests := []struct {
		name     string
		session  CircuitBreakerSession
		expected bool
	}{
		{
			name:     "nil session",
			session:  nil,
			expected: false,
		},
		{
			name:     "zero failures",
			session:  &MockSession{consecutiveFailures: 0},
			expected: false,
		},
		{
			name:     "one failure",
			session:  &MockSession{consecutiveFailures: 1},
			expected: false,
		},
		{
			name:     "two failures",
			session:  &MockSession{consecutiveFailures: 2},
			expected: false,
		},
		{
			name:     "three failures",
			session:  &MockSession{consecutiveFailures: 3},
			expected: true,
		},
		{
			name:     "four failures",
			session:  &MockSession{consecutiveFailures: 4},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckCircuitBreaker(tt.session)
			if result != tt.expected {
				t.Errorf("CheckCircuitBreaker() = %v, want %v", result, tt.expected)
			}
		})
	}
}