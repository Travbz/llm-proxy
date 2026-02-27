# API Reference

The proxy exposes two sets of endpoints on the same port:

1. **Session registry API** -- used by the control plane to manage sessions. These are internal endpoints, not called by sandboxes.
2. **Proxy handler** -- the catch-all that handles LLM API calls from sandboxes.

Default listen address: `:8090` (override with `-addr`).

---

## Session Registry API

### POST /v1/sessions

Register a new session. Called by the control plane when a sandbox boots.

**Request:**

```json
{
  "token": "abc123",
  "provider": "anthropic",
  "api_key": "sk-ant-api03-real-key-here",
  "upstream_url": "https://api.anthropic.com",
  "sandbox_id": "my-sandbox"
}
```

| Field | Required | Description |
|---|---|---|
| `token` | yes | The session token the sandbox will use to authenticate. |
| `provider` | yes | LLM provider: `"anthropic"`, `"openai"`, or `"ollama"`. |
| `api_key` | yes | The real API key. Never sent to the sandbox. |
| `upstream_url` | no | Override the default upstream URL for this provider. |
| `sandbox_id` | no | Identifier for the associated sandbox (for logging). |

**Response (201 Created):**

```json
{
  "status": "registered"
}
```

**Errors:**

| Status | Body | Cause |
|---|---|---|
| 400 | `{"error":"token, provider, and api_key are required"}` | Missing required fields. |
| 400 | `{"error":"invalid request: ..."}` | Malformed JSON body. |

**curl example:**

```bash
curl -X POST http://localhost:8090/v1/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "token": "my-session-token",
    "provider": "anthropic",
    "api_key": "sk-ant-api03-...",
    "sandbox_id": "dev-sandbox"
  }'
```

---

### DELETE /v1/sessions/{token}

Revoke a session. Called by the control plane when a sandbox shuts down.

**Path parameters:**

| Parameter | Description |
|---|---|
| `token` | The session token to revoke. |

**Response (200 OK):**

```json
{
  "status": "revoked"
}
```

Revoking a non-existent token is not an error -- it returns 200 regardless.

**curl example:**

```bash
curl -X DELETE http://localhost:8090/v1/sessions/my-session-token
```

---

### GET /v1/sessions

List all active sessions. API keys are omitted from the response.

**Response (200 OK):**

```json
[
  {
    "token": "my-session-token",
    "provider": "anthropic",
    "sandbox_id": "dev-sandbox",
    "upstream_url": ""
  }
]
```

Returns an empty array `[]` if no sessions are registered.

**curl example:**

```bash
curl http://localhost:8090/v1/sessions
```

---

### GET /v1/health

Health check endpoint.

**Response (200 OK):**

```json
{
  "status": "ok"
}
```

---

## Proxy Handler

Everything that doesn't match the registry API routes goes to the proxy handler. This is where sandboxes send their LLM API calls.

### Authentication

The proxy extracts the session token from one of two headers:

| Header | Format | Used by |
|---|---|---|
| `Authorization` | `Bearer session-<token>` or `Bearer <token>` | OpenAI SDK |
| `x-api-key` | `session-<token>` or `<token>` | Anthropic SDK |

The `session-` prefix is optional and stripped during extraction. Both headers are checked -- `Authorization` first, then `x-api-key`.

### Request flow

1. Extract token from auth header.
2. Look up session in the memory store.
3. Build upstream URL: `{session.UpstreamURL || DefaultUpstream(provider)}{request.Path}?{request.Query}`.
4. Copy request headers (excluding hop-by-hop: `Connection`, `Keep-Alive`, `Transfer-Encoding`, `Te`, `Trailer`, `Upgrade`, `Host`).
5. Replace auth headers with real credentials via `InjectAuth`.
6. Forward request to upstream.
7. Copy response headers and status code.
8. Stream or copy response body.

### Error responses

| Status | Body | Cause |
|---|---|---|
| 401 | `{"error":"missing or invalid authorization header"}` | No auth header or unrecognized format. |
| 401 | `{"error":"invalid session token"}` | Token not found in session store (expired or revoked). |
| 400 | `{"error":"unknown provider"}` | Session has no upstream URL and provider has no default. |
| 500 | `{"error":"internal error"}` | Failed to create upstream request. |
| 502 | `{"error":"upstream request failed"}` | Network error reaching the LLM provider. |

### curl example (proxied Anthropic call)

```bash
# Assumes session "my-token" is registered for provider "anthropic"
curl -X POST http://localhost:8090/v1/messages \
  -H "x-api-key: session-my-token" \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "max_tokens": 256,
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

The proxy forwards this to `https://api.anthropic.com/v1/messages` with `x-api-key: sk-ant-real-key` replacing the session token.
