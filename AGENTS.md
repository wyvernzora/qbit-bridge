# AGENTS.md

Drop-in operating instructions for coding agents working on **qbittorrent-mcp**. Read the user-global rules first:

- `~/.agents/AGENTS.md` — universal agent-behavior rules (non-negotiables, simplicity, surgical changes, communication style, grilling, etc.)
- `~/.agents/go.md` — Go engineering rules (loaded because this repo has `go.mod`)

This file holds project-specific context, learnings, and overrides only. Rules in the global files apply unless explicitly contradicted here.

---

## 1. Project context

### About qbittorrent-mcp

- **Name:** qbittorrent-mcp.
- **Domain:** MCP server wrapping the qBittorrent WebUI v2 API.
- **Tools:** (none yet — scaffolding only; add tools in `internal/mcp/tools.go`).
- **Transports:** stdio (default), streamable HTTP (`--transport=http --addr=:8080`, MCP mounted at `/mcp`).
- **Deployment:** sidecar to the qBittorrent container, reaching it over loopback. qBittorrent must have "Bypass authentication for clients on localhost" enabled — the MCP server performs no login.
- **No auth, no REST API, no web UI.**
- **Distribution:** Go binary, Docker container.

### Stack

- **Language:** Go 1.25.0+. Pinned in `go.mod`.
- **Entry point:** `cmd/qbit-mcp/main.go` — flag-driven, env fallbacks for all flags (prefix `QBITTORRENT_`).
- **MCP SDK:** `github.com/modelcontextprotocol/go-sdk`; streamable HTTP handler at `/mcp`, health check at `/healthz`.
- **qBittorrent client:** [`github.com/autobrr/go-qbittorrent`](https://github.com/autobrr/go-qbittorrent), constructed directly in `cmd/qbit-mcp/main.go`. Username and Password are intentionally empty so `LoginCtx` no-ops; the sidecar relies on qBittorrent's loopback-auth-bypass.
- **Server wiring:** `internal/mcp/server.go` (transport setup + HTTP handler), `internal/mcp/tools.go` (tool definitions), `internal/mcp/errors.go` (ToolError + ErrCode shared by all tools).

### Commands

```sh
go run ./cmd/qbit-mcp          # run from source (stdio)
go test ./...                  # full test suite
go build -o bin/qbit-mcp ./cmd/qbit-mcp
make devserver-build           # build dev image (hot-reload + inspector)
make devserver-run             # start dev container
```

### Relevant flags / env vars

```
--transport=http                # enables HTTP transport (QBITTORRENT_TRANSPORT)
--addr=:8080                    # listen address for HTTP (QBITTORRENT_ADDR)
--qb-url=http://localhost:8080  # qBittorrent WebUI base URL (QBITTORRENT_URL)
--qb-timeout=15s                # per-request HTTP timeout (QBITTORRENT_TIMEOUT)
--log-level=debug               # structured JSON log level (QBITTORRENT_LOG_LEVEL)
```

---

## 2. Project Learnings

**Accumulated corrections. This section is for the agent to maintain, not just the human.**

When the user corrects your approach, append a one-line rule here before ending the session. Write it concretely ("Always use X for Y"), never abstractly ("be careful with Y"). If an existing line already covers the correction, tighten it instead of adding a new one.

- (empty)
