package proxy

import "net/http"

const (
	// ProviderAnthropic is the Anthropic LLM provider.
	ProviderAnthropic = "anthropic"

	// ProviderOpenAI is the OpenAI LLM provider.
	ProviderOpenAI = "openai"

	// ProviderOllama is the Ollama local LLM provider.
	ProviderOllama = "ollama"
)

// InjectAuth sets the provider-specific authentication headers on the
// outgoing upstream request. It removes any existing auth headers from
// the sandbox request first, then injects the real credentials.
func InjectAuth(req *http.Request, provider, apiKey string) {
	// Remove sandbox auth — these carried the session token, not real keys.
	req.Header.Del("Authorization")
	req.Header.Del("x-api-key")

	switch provider {
	case ProviderAnthropic:
		req.Header.Set("x-api-key", apiKey)
	case ProviderOpenAI:
		req.Header.Set("Authorization", "Bearer "+apiKey)
	case ProviderOllama:
		// No auth needed for Ollama.
	}
}

// DefaultUpstream returns the default upstream URL for a provider.
func DefaultUpstream(provider string) string {
	switch provider {
	case ProviderAnthropic:
		return "https://api.anthropic.com"
	case ProviderOpenAI:
		return "https://api.openai.com"
	case ProviderOllama:
		return "http://localhost:11434"
	default:
		return ""
	}
}
