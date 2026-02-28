// Package proxy implements a credential-injecting HTTP reverse proxy for
// LLM API calls. It validates session tokens, swaps in real API keys,
// and streams responses back to sandboxes.
package proxy

import (
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"llm-proxy/pkg/session"
)

// Proxy is the credential-injecting LLM reverse proxy.
type Proxy struct {
	store      session.Store
	usage      *session.UsageTracker
	httpClient *http.Client
	logger     *log.Logger
}

// New creates a new Proxy with the given session store and logger.
func New(store session.Store, usage *session.UsageTracker, logger *log.Logger) *Proxy {
	return &Proxy{
		store: store,
		usage: usage,
		httpClient: &http.Client{
			// LLM requests can be slow, especially with thinking blocks.
			Timeout: 5 * time.Minute,
			// Don't follow redirects — return them to the caller.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		logger: logger,
	}
}

// ServeHTTP implements http.Handler. Every request is authenticated via
// session token, has its credentials swapped, and is forwarded upstream.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract session token from auth header.
	token := extractToken(r)
	if token == "" {
		http.Error(w, `{"error":"missing or invalid authorization header"}`, http.StatusUnauthorized)
		return
	}

	// Look up session.
	sess, err := p.store.Lookup(token)
	if err != nil {
		http.Error(w, `{"error":"invalid session token"}`, http.StatusUnauthorized)
		return
	}

	// Resolve upstream URL.
	upstream := sess.UpstreamURL
	if upstream == "" {
		upstream = DefaultUpstream(sess.Provider)
	}
	if upstream == "" {
		http.Error(w, `{"error":"unknown provider"}`, http.StatusBadRequest)
		return
	}

	upstreamURL := upstream + r.URL.Path
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}

	// Build upstream request.
	upstreamReq, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, r.Body)
	if err != nil {
		p.logger.Printf("error creating upstream request: %v", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	// Copy client headers, then inject real credentials.
	copyHeaders(upstreamReq.Header, r.Header)
	InjectAuth(upstreamReq, sess.Provider, sess.APIKey)

	p.logger.Printf("proxying %s %s -> %s (provider=%s sandbox=%s)",
		r.Method, r.URL.Path, upstreamURL, sess.Provider, sess.SandboxID)

	// Execute upstream request.
	resp, err := p.httpClient.Do(upstreamReq)
	if err != nil {
		p.logger.Printf("upstream request failed: %v", err)
		http.Error(w, `{"error":"upstream request failed"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers.
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	// Wrap body in a metering reader to capture response for usage extraction.
	mr := newMeteringReader(resp.Body)

	// Stream or copy response body.
	if isStreamingResponse(resp) {
		StreamResponse(w, mr)
		// Extract usage from the captured SSE data.
		input, output := extractUsageFromSSE(mr.Bytes(), sess.Provider)
		if input > 0 || output > 0 {
			p.usage.Record(token, input, output)
			p.logger.Printf("usage: session=%s input=%d output=%d", sess.SandboxID, input, output)
		}
	} else {
		_, _ = io.Copy(w, mr)
		// Extract usage from the captured JSON body.
		input, output := extractUsage(mr.Bytes(), sess.Provider)
		if input > 0 || output > 0 {
			p.usage.Record(token, input, output)
			p.logger.Printf("usage: session=%s input=%d output=%d", sess.SandboxID, input, output)
		}
	}
}

// extractToken extracts the session token from the request's auth headers.
// Supports both OpenAI-style (Authorization: Bearer) and Anthropic-style
// (x-api-key) headers. The token is returned as-is (including the "session-"
// prefix) since the session store keys include the prefix.
func extractToken(r *http.Request) string {
	// Check Authorization header (OpenAI style).
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	// Check x-api-key header (Anthropic style).
	if apiKey := r.Header.Get("x-api-key"); apiKey != "" {
		return apiKey
	}

	return ""
}

// copyHeaders copies HTTP headers, excluding hop-by-hop headers that
// should not be forwarded between connections.
func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		switch strings.ToLower(k) {
		case "connection", "keep-alive", "transfer-encoding",
			"te", "trailer", "upgrade", "host":
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// isStreamingResponse checks if the upstream response is a streaming
// response that should be flushed incrementally.
func isStreamingResponse(resp *http.Response) bool {
	ct := resp.Header.Get("Content-Type")
	return strings.HasPrefix(ct, "text/event-stream") ||
		strings.Contains(ct, "application/x-ndjson") ||
		resp.Header.Get("Transfer-Encoding") == "chunked"
}
