package proxy

import (
	"net/http"
	"testing"
)

func TestInjectAuth_Anthropic(t *testing.T) {
	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", nil)
	req.Header.Set("x-api-key", "session-token-from-sandbox")

	InjectAuth(req, ProviderAnthropic, "sk-ant-real-key")

	got := req.Header.Get("x-api-key")
	if got != "sk-ant-real-key" {
		t.Errorf("x-api-key = %q, want %q", got, "sk-ant-real-key")
	}
	if auth := req.Header.Get("Authorization"); auth != "" {
		t.Errorf("Authorization should be empty, got %q", auth)
	}
}

func TestInjectAuth_OpenAI(t *testing.T) {
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer session-token-from-sandbox")

	InjectAuth(req, ProviderOpenAI, "sk-openai-real-key")

	got := req.Header.Get("Authorization")
	want := "Bearer sk-openai-real-key"
	if got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
	if apiKey := req.Header.Get("x-api-key"); apiKey != "" {
		t.Errorf("x-api-key should be empty, got %q", apiKey)
	}
}

func TestInjectAuth_Ollama(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://localhost:11434/api/chat", nil)
	req.Header.Set("Authorization", "Bearer session-token")

	InjectAuth(req, ProviderOllama, "")

	if auth := req.Header.Get("Authorization"); auth != "" {
		t.Errorf("Authorization should be empty for Ollama, got %q", auth)
	}
}

func TestDefaultUpstream(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{ProviderAnthropic, "https://api.anthropic.com"},
		{ProviderOpenAI, "https://api.openai.com"},
		{ProviderOllama, "http://localhost:11434"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		got := DefaultUpstream(tt.provider)
		if got != tt.want {
			t.Errorf("DefaultUpstream(%q) = %q, want %q", tt.provider, got, tt.want)
		}
	}
}
