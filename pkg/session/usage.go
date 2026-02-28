package session

import "sync"

// Usage tracks token consumption for a session.
type Usage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	Requests     int64 `json:"requests"`
}

// UsageTracker stores per-session token usage. Thread-safe.
type UsageTracker struct {
	mu    sync.RWMutex
	usage map[string]*Usage // keyed by session token
}

// NewUsageTracker creates a new usage tracker.
func NewUsageTracker() *UsageTracker {
	return &UsageTracker{
		usage: make(map[string]*Usage),
	}
}

// Record adds token counts to a session's usage.
func (u *UsageTracker) Record(token string, input, output int64) {
	u.mu.Lock()
	defer u.mu.Unlock()

	usage, ok := u.usage[token]
	if !ok {
		usage = &Usage{}
		u.usage[token] = usage
	}

	usage.InputTokens += input
	usage.OutputTokens += output
	usage.Requests++
}

// Get returns the current usage for a session token.
func (u *UsageTracker) Get(token string) *Usage {
	u.mu.RLock()
	defer u.mu.RUnlock()

	usage, ok := u.usage[token]
	if !ok {
		return &Usage{}
	}
	// Return a copy.
	return &Usage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		Requests:     usage.Requests,
	}
}

// Clear removes usage data for a session token.
func (u *UsageTracker) Clear(token string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.usage, token)
}
