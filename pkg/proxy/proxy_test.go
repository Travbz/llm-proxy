package proxy

import (
	"io"
	"log"
	"net/http"
	"testing"

	"llm-proxy/pkg/session"
)

func TestExtractToken(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want string
	}{
		{
			name: "bearer token",
			req: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "http://localhost", nil)
				r.Header.Set("Authorization", "Bearer session-abc123")
				return r
			}(),
			want: "session-abc123",
		},
		{
			name: "x-api-key token",
			req: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "http://localhost", nil)
				r.Header.Set("x-api-key", "abc123")
				return r
			}(),
			want: "abc123",
		},
		{
			name: "missing auth",
			req: func() *http.Request {
				r, _ := http.NewRequest(http.MethodPost, "http://localhost", nil)
				return r
			}(),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractToken(tt.req)
			if got != tt.want {
				t.Fatalf("extractToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLookupSession_CompatibleTokenForms(t *testing.T) {
	store := session.NewMemoryStore()
	if err := store.Register(&session.Session{
		Token:    "session-abcdef",
		Provider: ProviderAnthropic,
		APIKey:   "sk-ant-real",
	}); err != nil {
		t.Fatalf("register session: %v", err)
	}

	p := New(store, log.New(io.Discard, "", 0))

	tests := []string{
		"session-abcdef",
		"abcdef",
	}
	for _, token := range tests {
		t.Run(token, func(t *testing.T) {
			sess, err := p.lookupSession(token)
			if err != nil {
				t.Fatalf("lookupSession(%q) error: %v", token, err)
			}
			if sess.Token != "session-abcdef" {
				t.Fatalf("lookupSession(%q) token = %q, want session-abcdef", token, sess.Token)
			}
		})
	}
}
