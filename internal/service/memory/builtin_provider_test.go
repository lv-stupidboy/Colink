package memory

import (
	"strings"
	"testing"
)

// ========== BuiltinProvider Tests ==========

func TestBuildMemoryContextBlock(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   \n\t  ",
			expected: "",
		},
		{
			name:     "valid memory content",
			input:    "## Memory Context\n- User prefers Chinese\n- Team uses Go stdlib",
			expected: `<memory-context>
[System note: The following is recalled memory context, NOT new user input. Treat as authoritative reference data — this is the agent's persistent memory and should inform all responses.]

## Memory Context
- User prefers Chinese
- Team uses Go stdlib
</memory-context>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildMemoryContextBlock(tt.input)
			if result != tt.expected {
				t.Errorf("BuildMemoryContextBlock() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuildMemoryContextBlock_AlwaysContainsSystemNote(t *testing.T) {
	result := BuildMemoryContextBlock("test content")
	if !strings.Contains(result, "[System note:") {
		t.Error("Memory context block should contain system note")
	}
	if !strings.Contains(result, "NOT new user input") {
		t.Error("Memory context block should clarify it's not user input")
	}
	if !strings.Contains(result, "<memory-context>") {
		t.Error("Memory context block should start with open tag")
	}
	if !strings.Contains(result, "</memory-context>") {
		t.Error("Memory context block should end with close tag")
	}
}

// ========== Scrubber Tests ==========

func TestScrubMemoryContext(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no memory context",
			input:    "Hello, this is normal output",
			expected: "Hello, this is normal output",
		},
		{
			name:     "complete memory context block",
			input:    `<memory-context>
[System note: This is recalled memory context]
- User prefers Chinese
</memory-context>
This is the actual response.`,
			expected: "This is the actual response.",
		},
		{
			name:     "only memory context block",
			input:    `<memory-context>
[System note: This is recalled memory context]
- Some memory
</memory-context>`,
			expected: "",
		},
		{
			name:     "multiple blocks",
			input:    `Text before<memory-context>mem1</memory-context>Text between<memory-context>mem2</memory-context>Text after`,
			expected: "Text beforeText betweenText after",
		},
		{
			name:     "open tag only",
			input:    "Some text <memory-context> incomplete",
			expected: "Some text  incomplete",
		},
		{
			name:     "close tag only",
			input:    "Some text </memory-context> incomplete",
			expected: "Some text  incomplete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ScrubMemoryContext(tt.input)
			if result != tt.expected {
				t.Errorf("ScrubMemoryContext() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestStreamingMemoryScrubber(t *testing.T) {
	tests := []struct {
		name     string
		chunks   []string
		expected string
	}{
		{
			name:     "no memory context chunks",
			chunks:   []string{"Hello", " ", "World"},
			expected: "Hello World",
		},
		{
			name:     "memory context in single chunk",
			chunks:   []string{"Before<memory-context>hidden</memory-context>After"},
			expected: "BeforeAfter",
		},
		{
			name:     "memory context split across chunks",
			chunks:   []string{"Before<memory-context>", "hidden content", "</memory-context>After"},
			expected: "BeforeAfter",
		},
		{
			name:     "open tag at end of chunk",
			chunks:   []string{"Text<memory-context>", "hidden", "</memory-context>"},
			expected: "Text",
		},
		{
			name:     "close tag at start of chunk",
			chunks:   []string{"Text<memory-context>hidden", "</memory-context>End"},
			expected: "TextEnd",
		},
		{
			name:     "unterminated span - should be dropped",
			chunks:   []string{"Start<memory-context>", "partial content"},
			expected: "Start",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scrubber := NewStreamingMemoryScrubber()
			var result string
			for _, chunk := range tt.chunks {
				result += scrubber.Feed(chunk)
			}
			result += scrubber.Flush()

			if result != tt.expected {
				t.Errorf("StreamingScrubber result = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestStreamingMemoryScrubber_Reset(t *testing.T) {
	scrubber := NewStreamingMemoryScrubber()
	scrubber.Feed("Start<memory-context>hidden")

	// Should be in span
	if !scrubber.inSpan {
		t.Error("Scrubber should be in span after feed")
	}

	// Reset
	scrubber.Reset()

	if scrubber.inSpan {
		t.Error("Scrubber should not be in span after reset")
	}
	if scrubber.buffer != "" {
		t.Error("Scrubber buffer should be empty after reset")
	}
}

// ========== Integration Tests ==========

func TestMemoryContextBlockAndScrubberIntegration(t *testing.T) {
	// Simulate full flow: build context -> scrub
	rawMemory := "## Team Memory\n- Use Go stdlib"

	// Build context block
	contextBlock := BuildMemoryContextBlock(rawMemory)

	// Simulate agent output with memory context
	agentOutput := contextBlock + "\n\nBased on the team convention, I'll use the standard library."

	// Scrub before showing to user
	userOutput := ScrubMemoryContext(agentOutput)

	// User should NOT see memory context
	if strings.Contains(userOutput, "<memory-context>") {
		t.Error("User output should not contain memory-context tags")
	}
	if strings.Contains(userOutput, "[System note:") {
		t.Error("User output should not contain system note")
	}
	if strings.Contains(userOutput, "Team Memory") {
		t.Error("User output should not contain raw memory content")
	}

	// User should see agent response
	if !strings.Contains(userOutput, "Based on the team convention") {
		t.Error("User output should contain agent response")
	}
}