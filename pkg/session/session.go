// Package session manages sandbox session tokens and their associated
// credentials. The control plane registers sessions when sandboxes boot;
// the proxy validates tokens on every LLM request.
package session

// Session represents a registered sandbox session with its credentials.
type Session struct {
	// Token is the session-scoped token the sandbox uses to authenticate.
	Token string

	// Provider is the LLM provider name ("anthropic", "openai", "ollama").
	Provider string

	// APIKey is the real API key for the provider. Never sent to the sandbox.
	APIKey string

	// UpstreamURL is the provider API base URL. If empty, the default for
	// the provider is used.
	UpstreamURL string

	// SandboxID is the identifier of the sandbox this session belongs to.
	SandboxID string
}

// Store defines the interface for session management.
type Store interface {
	// Register adds or updates a session in the store.
	Register(s *Session) error

	// Lookup retrieves a session by token. Returns an error if not found.
	Lookup(token string) (*Session, error)

	// Revoke removes a session from the store.
	Revoke(token string) error

	// List returns all registered sessions.
	List() []*Session
}
