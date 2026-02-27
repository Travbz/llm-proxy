# Providers

The proxy supports three LLM providers out of the box. Each has its own auth header format and default upstream URL.

## Supported providers

| Provider | Constant | Default upstream | Auth header |
|---|---|---|---|
| Anthropic | `ProviderAnthropic` | `https://api.anthropic.com` | `x-api-key: <key>` |
| OpenAI | `ProviderOpenAI` | `https://api.openai.com` | `Authorization: Bearer <key>` |
| Ollama | `ProviderOllama` | `http://localhost:11434` | None |

## How InjectAuth works

The `InjectAuth` function in `pkg/proxy/provider.go` does three things on every proxied request:

1. **Removes the sandbox's auth headers.** Both `Authorization` and `x-api-key` are deleted. These held the session token, not real credentials.
2. **Sets the provider-specific header.** Based on the provider string from the session, it sets the correct header with the real API key.
3. **Leaves everything else alone.** Other headers (`Content-Type`, `anthropic-version`, model-specific headers) pass through untouched.

```
Before InjectAuth:
  x-api-key: session-abc123
  Content-Type: application/json
  anthropic-version: 2023-06-01

After InjectAuth (provider=anthropic, apiKey=sk-ant-real-key):
  x-api-key: sk-ant-real-key
  Content-Type: application/json
  anthropic-version: 2023-06-01
```

For Ollama, both auth headers are removed and nothing is set -- Ollama doesn't need authentication.

## Provider-specific notes

### Anthropic

- Auth header: `x-api-key`
- The Anthropic SDK also sends `anthropic-version` -- this passes through untouched.
- Streaming uses SSE (`text/event-stream`).
- The control plane sets `ANTHROPIC_BASE_URL` inside the sandbox so the SDK routes through the proxy.

### OpenAI

- Auth header: `Authorization: Bearer <key>`
- Compatible with any OpenAI-compatible API (Azure OpenAI, Together, Groq, etc.) by setting a custom `upstream_url` in the session.
- Streaming uses SSE (`text/event-stream`).
- The control plane sets `OPENAI_BASE_URL` inside the sandbox.

### Ollama

- No auth required. The proxy still strips auth headers for consistency.
- Streaming uses NDJSON (`application/x-ndjson`).
- Default upstream is `http://localhost:11434` -- assumes Ollama is running on the host.
- The control plane sets `OLLAMA_HOST` inside the sandbox.

## Adding a new provider

To add support for a new provider:

1. **Add a constant** in `pkg/proxy/provider.go`:

```go
const ProviderMyProvider = "myprovider"
```

2. **Add auth injection** in the `InjectAuth` switch:

```go
case ProviderMyProvider:
    req.Header.Set("Authorization", "Token "+apiKey)
```

3. **Add default upstream** in the `DefaultUpstream` switch:

```go
case ProviderMyProvider:
    return "https://api.myprovider.com"
```

4. **Add base URL injection** in the orchestrator (`control-plane/pkg/orchestrator/orchestrator.go`):

```go
case "myprovider":
    env["MYPROVIDER_BASE_URL"] = proxyBaseURL
```

5. **Register a session** with the new provider name:

```bash
curl -X POST http://localhost:8090/v1/sessions \
  -H "Content-Type: application/json" \
  -d '{"token":"t1","provider":"myprovider","api_key":"real-key"}'
```

The proxy doesn't validate provider names -- any string works. If the provider isn't in the `InjectAuth` switch, no auth header is set (same as Ollama behavior). If the provider isn't in `DefaultUpstream`, you must provide `upstream_url` in the session registration.
