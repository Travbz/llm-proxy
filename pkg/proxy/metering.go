package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

// usageBlock represents the token usage returned by LLM providers.
// Both Anthropic and OpenAI use this same field naming.
type usageBlock struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	// OpenAI uses these field names:
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
}

// extractUsage parses the response body for token usage information.
// It handles both Anthropic and OpenAI response formats.
// Returns (inputTokens, outputTokens). Non-streaming only — for streaming
// responses, the final SSE event is parsed separately.
func extractUsage(body []byte, provider string) (input, output int64) {
	// Try to find a "usage" key in the top-level JSON.
	var envelope struct {
		Usage usageBlock `json:"usage"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return 0, 0
	}

	u := envelope.Usage
	switch provider {
	case ProviderAnthropic:
		return u.InputTokens, u.OutputTokens
	case ProviderOpenAI:
		// OpenAI uses prompt_tokens / completion_tokens.
		if u.PromptTokens > 0 || u.CompletionTokens > 0 {
			return u.PromptTokens, u.CompletionTokens
		}
		return u.InputTokens, u.OutputTokens
	default:
		// Best-effort for unknown providers.
		if u.InputTokens > 0 {
			return u.InputTokens, u.OutputTokens
		}
		return u.PromptTokens, u.CompletionTokens
	}
}

// extractUsageFromSSE scans SSE event lines for the final usage data.
// Anthropic sends a message_delta event with usage at the end of a stream.
// OpenAI includes usage in the final [DONE]-adjacent chunk.
func extractUsageFromSSE(data []byte, provider string) (input, output int64) {
	lines := strings.Split(string(data), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			continue
		}
		in, out := extractUsage([]byte(payload), provider)
		if in > 0 || out > 0 {
			return in, out
		}
	}
	return 0, 0
}

// meteringReader wraps a response body reader and captures the content
// for post-response usage extraction.
type meteringReader struct {
	reader io.Reader
	buf    bytes.Buffer
}

func newMeteringReader(r io.Reader) *meteringReader {
	return &meteringReader{reader: r}
}

func (m *meteringReader) Read(p []byte) (int, error) {
	n, err := m.reader.Read(p)
	if n > 0 {
		m.buf.Write(p[:n])
	}
	return n, err
}

func (m *meteringReader) Bytes() []byte {
	return m.buf.Bytes()
}
