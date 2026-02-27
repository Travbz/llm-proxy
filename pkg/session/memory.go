package session

import (
	"fmt"
	"sync"
)

// MemoryStore is a thread-safe in-memory session store.
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewMemoryStore creates a new empty in-memory session store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]*Session),
	}
}

// Register adds or updates a session in the store.
func (m *MemoryStore) Register(s *Session) error {
	if s.Token == "" {
		return fmt.Errorf("session token cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.Token] = s
	return nil
}

// Lookup retrieves a session by token.
func (m *MemoryStore) Lookup(token string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[token]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", token)
	}
	return s, nil
}

// Revoke removes a session from the store.
func (m *MemoryStore) Revoke(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, token)
	return nil
}

// List returns all registered sessions.
func (m *MemoryStore) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}
