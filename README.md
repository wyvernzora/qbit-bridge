# qbittorrent-mcp

MCP server wrapping the [qBittorrent](https://www.qbittorrent.org) WebUI v2 API for agent-driven download management.

Designed to run as a sidecar to the qBittorrent container, reaching the daemon over loopback. qBittorrent must have **"Bypass authentication for clients on localhost"** enabled in WebUI settings — the MCP server performs no login.

## Tools

Eight tools across four groups (see [`docs/tools.md`](docs/tools.md) for the full schema spec).

| Group | Tool | What |
| --- | --- | --- |
| Downloads | `list_downloads` | Filtered/sorted/paginated download list with opt-in field projection (incl. per-hash `trackers` / `files`). |
| Downloads | `add_download` | Magnet-only add. Idempotent — re-adding a hash already known to qBittorrent leaves the live download untouched and reports `already_existed: true`. |
| Downloads | `remove_downloads` | Bulk remove by explicit `hashes` or by `filter` (states/tags). On-disk files are never deleted by this tool. |
| Tags | `list_tags` | List the configured tags. Unknown tags auto-create on `add_download.tags`. |
| Destinations | `list_destinations` | List the deploy-time-configured save-path aliases (name → absolute path). Useful for reverse-lookups from a raw `save_path` to an alias name. |
| Subscriptions | `list_subscriptions` | RSS-feed-plus-rule joined as a single concept. Summary by default; opt-in `recent_items`. |
| Subscriptions | `set_subscription` | Atomic upsert of a subscription. Creates (or replaces) the feed and the auto-download rule pointing at it. `feed_url` is immutable on existing subscriptions. |
| Subscriptions | `delete_subscription` | Removes the rule; removes the synthetic feed too when no other subscription still references the same `feed_url`. |

### Destination aliases

Tools that direct download storage (`add_download`, `set_subscription`) **do not accept arbitrary filesystem paths**. The operator declares aliases at boot via `--save-paths` (or `QBITTORRENT_SAVE_PATHS`):

```
--save-paths='kura-inbox=/mnt/kura,downloads=/mnt/downloads'
```

Callers pass alias names; the server resolves to a path before calling qBittorrent. Untrusted agents cannot redirect downloads outside the configured set.

### Audit logging

Every mutation (`add`, `remove`, `subscription_set`, `subscription_delete`) emits a structured slog record with `audit=true`, the affected hashes/names, and tool-specific extras. Destructive ops (`remove`, `subscription_delete`) log at WARN so log aggregators filtering on level surface them.

## Build & run

```sh
go build ./cmd/qbit-mcp
./qbit-mcp --transport=stdio
./qbit-mcp --transport=http --addr=:8080
```

HTTP transport exposes the MCP endpoint at `/mcp` and a k8s liveness probe at `/healthz`.

## Container

Prebuilt images are published to GHCR on every push to `main` (as `:dev`) and on tag pushes (as `:vX.Y.Z` and `:latest`).

```sh
docker pull ghcr.io/wyvernzora/qbittorrent-mcp:latest
docker run --rm --network host \
  -e QBITTORRENT_URL=http://localhost:8080 \
  -e QBITTORRENT_SAVE_PATHS='kura-inbox=/mnt/kura' \
  ghcr.io/wyvernzora/qbittorrent-mcp:latest
```

Local build:

```sh
docker build -t qbit-mcp .
docker run --rm --network host qbit-mcp           # sidecar-style: shares loopback with qBittorrent
docker run --rm -i qbit-mcp --transport=stdio     # stdio
```

## Devserver (hot reload + MCP inspector)

```sh
make devserver-build
QBITTORRENT_URL=http://host.docker.internal:8080 make devserver-run
```

The container runs [air](https://github.com/air-verse/air) (rebuilds on `.go` save) alongside [@modelcontextprotocol/inspector](https://github.com/modelcontextprotocol/inspector). On startup it prints a prefilled inspector URL to copy into a browser.

## Configuration

| Flag | Env | Default |
| --- | --- | --- |
| `--transport` | `QBITTORRENT_TRANSPORT` | `stdio` |
| `--addr` | `QBITTORRENT_ADDR` | `:8080` |
| `--qb-url` | `QBITTORRENT_URL` | `http://localhost:8080` |
| `--qb-timeout` | `QBITTORRENT_TIMEOUT` | `15s` |
| `--save-paths` | `QBITTORRENT_SAVE_PATHS` | _(empty)_ |
| `--log-level` | `QBITTORRENT_LOG_LEVEL` | `info` |

## Errors

Tool errors are returned as `IsError: true` with a JSON body:

```json
{ "code": "upstream_forbidden", "message": "...", "retriable": false }
```

Codes: `invalid_argument`, `upstream_unavailable`, `upstream_forbidden`, `upstream_not_found`, `internal`.

`upstream_forbidden` signals the loopback-auth-bypass assumption was wrong — re-check qBittorrent's WebUI settings.

The qBittorrent WebUI v2 calls go through [`github.com/autobrr/go-qbittorrent`](https://github.com/autobrr/go-qbittorrent); errors from the SDK are translated to the codes above at the MCP tool boundary in [`internal/mcp/errors.go`](internal/mcp/errors.go).
