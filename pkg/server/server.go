// Package server wires together the HTTP server, the session registry API,
// and the LLM proxy handler.
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"

	"llm-proxy/pkg/proxy"
	"llm-proxy/pkg/session"
)

// Server is the HTTP server for the LLM proxy.
type Server struct {
	store  session.Store
	proxy  *proxy.Proxy
	mux    *http.ServeMux
	logger *log.Logger
}

// New creates a new Server with the given session store.
func New(store session.Store, logger *log.Logger) *Server {
	s := &Server{
		store:  store,
		proxy:  proxy.New(store, logger),
		mux:    http.NewServeMux(),
		logger: logger,
	}

	// Session registry API (called by the control plane).
	s.mux.HandleFunc("POST /v1/sessions", s.handleRegisterSession)
	s.mux.HandleFunc("DELETE /v1/sessions/{token}", s.handleRevokeSession)
	s.mux.HandleFunc("GET /v1/sessions", s.handleListSessions)

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
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
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

	s.logger.Printf("revoked session token=%s", token)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "revoked"})
}

// sessionInfo is the JSON representation of a session in list responses.
// The APIKey is intentionally omitted.
type sessionInfo struct {
	Token       string `json:"token"`
	Provider    string `json:"provider"`
	SandboxID   string `json:"sandbox_id"`
	UpstreamURL string `json:"upstream_url,omitempty"`
}

func (s *Server) handleListSessions(w http.ResponseWriter, _ *http.Request) {
	sessions := s.store.List()

	infos := make([]sessionInfo, len(sessions))
	for i, sess := range sessions {
		infos[i] = sessionInfo{
			Token:       sess.Token,
			Provider:    sess.Provider,
			SandboxID:   sess.SandboxID,
			UpstreamURL: sess.UpstreamURL,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(infos)
}
