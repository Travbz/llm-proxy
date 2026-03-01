# GhostProxy

A credential-injecting reverse proxy for LLM API calls. Sits between sandboxed agents and upstream providers (Anthropic, OpenAI, Ollama), swapping session tokens for real API keys on every request. The sandbox never sees the real credentials.

Built as part of a three-service agent sandbox system:

| Repo | What it does |
|---|---|
| **[CommandGrid](https://github.com/Travbz/CommandGrid)** | Orchestrator -- config, secrets, provisioning, boot sequence |
| **[GhostProxy](https://github.com/Travbz/GhostProxy)** | This repo -- credential-injecting LLM reverse proxy |
| **[RootFS](https://github.com/Travbz/RootFS)** | Container image -- entrypoint, env stripping, privilege drop |

---

## How it works

```mermaid
sequenceDiagram
    participant CP as CommandGrid
    participant Proxy as GhostProxy
    participant Sandbox as sandbox (agent)
    participant LLM as LLM Provider

    CP->>Proxy: POST /v1/sessions {token, provider, api_key}<br/>Authorization: Bearer admin-token
    Note over Proxy: Session registered in memory

    Sandbox->>Proxy: POST /v1/messages<br/>x-api-key: session-abc123
    Proxy->>Proxy: Validate token, lookup real key
    Proxy->>LLM: POST /v1/messages<br/>x-api-key: sk-ant-real-key
    LLM-->>Proxy: SSE stream
    Proxy-->>Sandbox: SSE stream (pass-through)

    CP->>Proxy: DELETE /v1/sessions/abc123<br/>Authorization: Bearer admin-token
    Note over Proxy: Session revoked
```

The proxy is completely stateless -- no conversation history, no storage, no parsing of request/response bodies. It validates the token, swaps the auth header, and forwards everything verbatim. Streaming responses (SSE and NDJSON) are flushed immediately with no buffering.

---

## Provider auth

Each provider has its own auth header format. The proxy handles the translation:

| Provider | Sandbox sends | Proxy injects upstream |
|---|---|---|
| Anthropic | `x-api-key: session-<token>` | `x-api-key: <real-key>` |
| OpenAI | `Authorization: Bearer session-<token>` | `Authorization: Bearer <real-key>` |
| Ollama | Either header format | No auth (local) |

---

## API

### Session registry (called by CommandGrid)

| Method | Path | Description |
|---|---|---|
| `POST` | `/v1/sessions` | Register a session: `{token, provider, api_key, sandbox_id}` |
| `DELETE` | `/v1/sessions/{token}` | Revoke a single session |
| `DELETE` | `/v1/sandboxes/{id}/sessions` | Revoke all sessions for a sandbox |
| `GET` | `/v1/sessions` | List active sessions (tokens and keys omitted) |
| `GET` | `/v1/health` | Health check |

Set `GHOSTPROXY_ADMIN_TOKEN` (or pass `-admin-token`) to enable registry endpoints. Requests to `/v1/sessions*` require `Authorization: Bearer <admin-token>`.

### Proxy (called by sandboxes)

Everything not matching the above routes goes through the proxy handler. The proxy extracts the session token from `Authorization` or `x-api-key`, looks up the session, and forwards to the upstream provider.

---

## Building

Requires Go 1.25+. If you use Nix, `nix develop` gets you a shell with everything you need.

```bash
make build    # builds to ./build/ghostproxy
make test     # runs all tests
make lint     # golangci-lint
make run      # builds and runs on :8090
```

### Quick test with curl

```bash
# start the proxy
./build/ghostproxy -addr :8090

# register a session
curl -X POST http://localhost:8090/v1/sessions \
  -H "Authorization: Bearer $GHOSTPROXY_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"token":"my-token","provider":"anthropic","api_key":"sk-ant-...","sandbox_id":"dev"}'

# make a proxied request (as the sandbox would)
curl -X POST http://localhost:8090/v1/messages \
  -H "x-api-key: session-my-token" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"hello"}]}'
```

---

## Project structure

```
GhostProxy/
├── main.go                         # entry point, flag parsing
├── pkg/
│   ├── proxy/
│   │   ├── proxy.go                # reverse proxy core
│   │   ├── streaming.go            # SSE + NDJSON flush-through
│   │   ├── provider.go             # provider-specific auth injection
│   │   └── provider_test.go
│   ├── session/
│   │   ├── session.go              # Store interface + Session type
│   │   ├── memory.go               # in-memory store implementation
│   │   └── memory_test.go
│   └── server/
│       └── server.go               # HTTP server, routing, registry API
├── Makefile
├── go.mod
├── flake.nix
├── .releaserc.yaml
└── .github/workflows/
    ├── ci.yaml                     # lint, test, vet on PRs
    └── release.yaml                # semantic-release on main
```

---

## Versioning

Releases are automated with [semantic-release](https://github.com/semantic-release/semantic-release) from [conventional commits](https://www.conventionalcommits.org/). No manual version bumps.

```
feat: new feature      -> minor bump
fix: bug fix           -> patch bump
BREAKING CHANGE:       -> major bump (in commit footer)
```
