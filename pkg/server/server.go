// Package server wires together the HTTP server, the session registry API,
// and the LLM proxy handler.
package server

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"llm-proxy/pkg/proxy"
	"llm-proxy/pkg/session"
)

// Server is the HTTP server for the LLM proxy.
type Server struct {
	store      session.Store
	proxy      *proxy.Proxy
	mux        *http.ServeMux
	logger     *log.Logger
	adminToken string
}

// New creates a new Server with the given session store and admin token.
func New(store session.Store, logger *log.Logger, adminToken string) *Server {
	s := &Server{
		store:      store,
		proxy:      proxy.New(store, logger),
		mux:        http.NewServeMux(),
		logger:     logger,
		adminToken: adminToken,
	}

	// Session registry API (called by the control plane).
	s.mux.HandleFunc("POST /v1/sessions", s.requireAdminAuth(s.handleRegisterSession))
	s.mux.HandleFunc("DELETE /v1/sessions/{token}", s.requireAdminAuth(s.handleRevokeSession))
	s.mux.HandleFunc("DELETE /v1/sandboxes/{id}/sessions", s.requireAdminAuth(s.handleRevokeSandboxSessions))
	s.mux.HandleFunc("GET /v1/sessions", s.requireAdminAuth(s.handleListSessions))

	// Health endpoint.
	s.mux.HandleFunc("GET /v1/health", s.handleHealth)

	// Everything else goes to the LLM proxy.
	s.mux.Handle("/", s.proxy)

	return s
}

// Run starts the server on the given address.
func (s *Server) Run(addr string) error {
	s.logger.Printf("llm-proxy listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

// RunWithListener starts the server using the provided listener.
func (s *Server) RunWithListener(l net.Listener) error {
	s.logger.Printf("llm-proxy listening on %s", l.Addr())
	return http.Serve(l, s.mux)
}

// Handler returns the underlying http.Handler for testing.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) requireAdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.adminToken == "" {
			http.Error(w, `{"error":"admin api disabled"}`, http.StatusServiceUnavailable)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) != 1 {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// registerRequest is the JSON body for POST /v1/sessions.
type registerRequest struct {
	Token       string `json:"token"`
	Provider    string `json:"provider"`
	APIKey      string `json:"api_key"`
	UpstreamURL string `json:"upstream_url,omitempty"`
	SandboxID   string `json:"sandbox_id,omitempty"`
}

func (s *Server) handleRegisterSession(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	if req.Token == "" || req.Provider == "" || req.APIKey == "" {
		http.Error(w, `{"error":"token, provider, and api_key are required"}`, http.StatusBadRequest)
		return
	}

	sess := &session.Session{
		Token:       req.Token,
		Provider:    req.Provider,
		APIKey:      req.APIKey,
		UpstreamURL: req.UpstreamURL,
		SandboxID:   req.SandboxID,
	}

	if err := s.store.Register(sess); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"register failed: %s"}`, err), http.StatusInternalServerError)
		return
	}

	s.logger.Printf("registered session for sandbox=%s provider=%s", req.SandboxID, req.Provider)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
}

func (s *Server) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		http.Error(w, `{"error":"token is required"}`, http.StatusBadRequest)
		return
	}

	if err := s.store.Revoke(token); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"revoke failed: %s"}`, err), http.StatusInternalServerError)
		return
	}

	s.logger.Printf("revoked session")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "revoked"})
}

func (s *Server) handleRevokeSandboxSessions(w http.ResponseWriter, r *http.Request) {
	sandboxID := r.PathValue("id")
	if sandboxID == "" {
		http.Error(w, `{"error":"sandbox id is required"}`, http.StatusBadRequest)
		return
	}
	revoked := s.store.RevokeBySandboxID(sandboxID)
	s.logger.Printf("revoked %d sessions for sandbox=%s", revoked, sandboxID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "revoked",
		"count":   revoked,
		"sandbox": sandboxID,
	})
}

// sessionInfo is the JSON representation of a session in list responses.
type sessionInfo struct {
	Provider    string `json:"provider"`
	SandboxID   string `json:"sandbox_id"`
	UpstreamURL string `json:"upstream_url,omitempty"`
}

func (s *Server) handleListSessions(w http.ResponseWriter, _ *http.Request) {
	sessions := s.store.List()

	infos := make([]sessionInfo, len(sessions))
	for i, sess := range sessions {
		infos[i] = sessionInfo{
			Provider:    sess.Provider,
			SandboxID:   sess.SandboxID,
			UpstreamURL: sess.UpstreamURL,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(infos)
}
