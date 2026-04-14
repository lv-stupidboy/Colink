package im

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrEmptyMessage   = errors.New("message cannot be empty")
	ErrMessageTooLong = errors.New("message exceeds maximum length")
	mentionPattern    = regexp.MustCompile(`@(\w+)`)
)

// ValidateInboundMessage validates an inbound message for basic constraints.
// It checks for empty text, maximum length, and extracts mentions for A2A routing.
func ValidateInboundMessage(text string) error {
	if strings.TrimSpace(text) == "" {
		return ErrEmptyMessage
	}
	return nil
}

// ExtractMentions extracts @mention references from text for A2A routing.
// Returns a list of mentioned agent names.
func ExtractMentions(text string) []string {
	matches := mentionPattern.FindAllStringSubmatch(text, -1)
	mentions := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			mentions = append(mentions, match[1])
		}
	}
	return mentions
}

// ValidateOutboundMessage validates an outbound message against platform constraints.
// It checks for empty text and maximum length for the target platform.
func ValidateOutboundMessage(text string, maxLen int) error {
	if strings.TrimSpace(text) == "" {
		return ErrEmptyMessage
	}
	if len(text) > maxLen {
		return ErrMessageTooLong
	}
	return nil
}
