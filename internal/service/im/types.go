package im

type ChunkMessage struct {
	ChatID       string
	InvocationID string
	Type         string // "text", "card", "error", "status"
	Content      string
	CardJSON     string
	DedupKey     string
}

type DeliveryResult struct {
	OK         bool
	Attempts   int
	FinalError string
	Category   ErrorCategory
}
