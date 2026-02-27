package proxy

import (
	"io"
	"net/http"
)

// StreamResponse streams the upstream response body to the client with
// immediate flushing. This ensures real-time delivery of SSE events and
// NDJSON lines from LLM providers without buffering.
func StreamResponse(w http.ResponseWriter, body io.Reader) {
	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 32*1024) // 32KB read buffer

	for {
		n, err := body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return
			}
			if canFlush {
				flusher.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}
