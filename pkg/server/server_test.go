package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"llm-proxy/pkg/session"
)

func newTestServer(t *testing.T, adminToken string) *Server {
	t.Helper()
	logger := log.New(io.Discard, "", 0)
	store := session.NewMemoryStore()
	return New(store, logger, adminToken)
}

func TestSessionEndpointsRequireAdminAuth(t *testing.T) {
	srv := newTestServer(t, "secret-admin-token")
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAdminEndpointsDisabledWithoutToken(t *testing.T) {
	srv := newTestServer(t, "")
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestListSessionsOmitsTokenAndAPIKey(t *testing.T) {
	srv := newTestServer(t, "secret-admin-token")

	registerBody := map[string]string{
		"token":      "session-abc123",
		"provider":   "anthropic",
		"api_key":    "sk-ant-real",
		"sandbox_id": "sandbox-a",
	}
	data, _ := json.Marshal(registerBody)
	registerReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewReader(data))
	registerReq.Header.Set("Content-Type", "application/json")
	registerReq.Header.Set("Authorization", "Bearer secret-admin-token")
	registerRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d", registerRec.Code, http.StatusCreated)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	listReq.Header.Set("Authorization", "Bearer secret-admin-token")
	listRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRec.Code, http.StatusOK)
	}

	body := listRec.Body.String()
	if bytes.Contains([]byte(body), []byte("session-abc123")) {
		t.Fatalf("list response leaked session token: %s", body)
	}
	if bytes.Contains([]byte(body), []byte("sk-ant-real")) {
		t.Fatalf("list response leaked api key: %s", body)
	}
}

func TestRevokeSandboxSessionsEndpoint(t *testing.T) {
	srv := newTestServer(t, "secret-admin-token")
	store := srv.store
	_ = store.Register(&session.Session{Token: "session-a", Provider: "anthropic", APIKey: "key-a", SandboxID: "sandbox-a"})
	_ = store.Register(&session.Session{Token: "session-b", Provider: "openai", APIKey: "key-b", SandboxID: "sandbox-a"})
	_ = store.Register(&session.Session{Token: "session-c", Provider: "openai", APIKey: "key-c", SandboxID: "sandbox-b"})

	req := httptest.NewRequest(http.MethodDelete, "/v1/sandboxes/sandbox-a/sessions", nil)
	req.Header.Set("Authorization", "Bearer secret-admin-token")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if _, err := store.Lookup("session-a"); err == nil {
		t.Fatal("expected session-a to be revoked")
	}
	if _, err := store.Lookup("session-b"); err == nil {
		t.Fatal("expected session-b to be revoked")
	}
	if _, err := store.Lookup("session-c"); err != nil {
		t.Fatalf("expected session-c to remain, got err: %v", err)
	}
}
