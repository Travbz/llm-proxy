package session

import (
	"testing"
)

func TestMemoryStore_RegisterAndLookup(t *testing.T) {
	store := NewMemoryStore()

	sess := &Session{
		Token:     "test-token-123",
		Provider:  "anthropic",
		APIKey:    "sk-ant-real-key",
		SandboxID: "sandbox-1",
	}

	if err := store.Register(sess); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got, err := store.Lookup("test-token-123")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}

	if got.Token != sess.Token {
		t.Errorf("Token = %q, want %q", got.Token, sess.Token)
	}
	if got.Provider != sess.Provider {
		t.Errorf("Provider = %q, want %q", got.Provider, sess.Provider)
	}
	if got.APIKey != sess.APIKey {
		t.Errorf("APIKey = %q, want %q", got.APIKey, sess.APIKey)
	}
}

func TestMemoryStore_LookupNotFound(t *testing.T) {
	store := NewMemoryStore()

	_, err := store.Lookup("nonexistent")
	if err == nil {
		t.Fatal("Lookup() expected error for nonexistent token")
	}
}

func TestMemoryStore_Revoke(t *testing.T) {
	store := NewMemoryStore()

	sess := &Session{
		Token:    "revoke-me",
		Provider: "openai",
		APIKey:   "sk-openai-key",
	}

	_ = store.Register(sess)
	_ = store.Revoke("revoke-me")

	_, err := store.Lookup("revoke-me")
	if err == nil {
		t.Fatal("Lookup() expected error after Revoke()")
	}
}

func TestMemoryStore_RegisterEmptyToken(t *testing.T) {
	store := NewMemoryStore()

	err := store.Register(&Session{Token: ""})
	if err == nil {
		t.Fatal("Register() expected error for empty token")
	}
}

func TestMemoryStore_List(t *testing.T) {
	store := NewMemoryStore()

	_ = store.Register(&Session{Token: "a", Provider: "anthropic", APIKey: "k1"})
	_ = store.Register(&Session{Token: "b", Provider: "openai", APIKey: "k2"})

	sessions := store.List()
	if len(sessions) != 2 {
		t.Fatalf("List() returned %d sessions, want 2", len(sessions))
	}
}

func TestMemoryStore_RegisterOverwrite(t *testing.T) {
	store := NewMemoryStore()

	_ = store.Register(&Session{Token: "t1", Provider: "anthropic", APIKey: "old-key"})
	_ = store.Register(&Session{Token: "t1", Provider: "anthropic", APIKey: "new-key"})

	got, _ := store.Lookup("t1")
	if got.APIKey != "new-key" {
		t.Errorf("APIKey = %q, want %q after overwrite", got.APIKey, "new-key")
	}
}
