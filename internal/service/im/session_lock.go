package im

import "sync"

// SessionLock provides per-session serialization for message processing.
// Multiple goroutines can call Acquire concurrently; they will be serialized
// per sessionID. Different sessions run in parallel.
type SessionLock struct {
	mu     sync.Mutex
	chains map[string]chan struct{} // sessionID → done channel
}

// NewSessionLock creates a new SessionLock.
func NewSessionLock() *SessionLock {
	return &SessionLock{
		chains: make(map[string]chan struct{}),
	}
}

// Acquire blocks until the session is available, then returns a release function.
// The release function must be called (typically via defer) to unblock the next waiter.
//
// Usage:
//
//	release := lock.Acquire(sessionID)
//	defer release()
//	// process message
func (sl *SessionLock) Acquire(sessionID string) func() {
	sl.mu.Lock()

	// Check if there's an existing chain for this session
	oldDone, exists := sl.chains[sessionID]

	// Create a new done channel for this acquisition
	newDone := make(chan struct{})
	sl.chains[sessionID] = newDone

	sl.mu.Unlock()

	// If there was a previous waiter, wait for it to finish
	if exists {
		<-oldDone
	}

	// Return release function
	return func() {
		sl.mu.Lock()
		// Only clean up if this is still the current chain
		if sl.chains[sessionID] == newDone {
			delete(sl.chains, sessionID)
		}
		sl.mu.Unlock()
		close(newDone)
	}
}
