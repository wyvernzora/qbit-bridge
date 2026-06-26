# qbit-bridge

qBittorrent automation bridge for MCP, REST, and n8n workflows.

Designed to run as a sidecar to the qBittorrent container, reaching the daemon over loopback. qBittorrent must have **"Bypass authentication for clients on localhost"** enabled in WebUI settings — the MCP server performs no login.

## Tools

Eight tools across four groups (see [`docs/tools.md`](docs/tools.md) for the full schema spec).

All tools carry the `qbit_` prefix so generic verbs don't collide with other MCP servers in the same agent session.

| Group | Tool | What |
| --- | --- | --- |
| Downloads | `qbit_search_downloads` | Filtered/sorted/paginated download list with opt-in field projection (incl. per-hash `trackers` / `files`). |
| Downloads | `qbit_add_download` | Magnet-only add. Idempotent — re-adding a hash already known to qBittorrent leaves the live download untouched and reports `already_existed: true`. |
| Downloads | `qbit_remove_downloads` | Bulk remove by explicit `hashes` or by `filter` (states/tags). On-disk files are never deleted by this tool. |
| Tags | `qbit_list_tags` | List the configured tags. Unknown tags auto-create on `qbit_add_download.tags`. |
| Destinations | `qbit_list_destinations` | List the deploy-time-configured save-path aliases (name → absolute path). Useful for reverse-lookups from a raw `save_path` to an alias name. |
| Subscriptions | `qbit_search_subscriptions` | RSS-feed-plus-rule joined as a single concept. Summary by default; opt-in `recent_items`. Filter by `name_glob`/`feed_url_substring`, paginate via `limit`/`offset`. |
| Subscriptions | `qbit_subscribe` | Create or replace a subscription by name. Atomically creates (or replaces) the feed and the auto-download rule pointing at it. `feed_url` is immutable on existing subscriptions. |
| Subscriptions | `qbit_unsubscribe` | Removes the rule; removes the synthetic feed too when no other subscription still references the same `feed_url`. |

## REST API

HTTP transport also exposes a small REST facade for n8n-style workflows. See [`docs/rest.md`](docs/rest.md).

| Method | Path | What |
| --- | --- | --- |
| `POST` | `/api/v1/downloads` | Add one magnet download. |
| `GET` | `/api/v1/downloads` | List downloads; filter with repeated `states`, `tags`, and `hashes` query params. |
| `GET` | `/api/v1/downloads/{hash}` | Get one download. |
| `DELETE` | `/api/v1/downloads/{hash}` | Remove one download from qBittorrent tracking; files are not deleted. |

## n8n nodes

The `integrations/n8n` package exports a community node for the REST facade. It
ships the **qBittorrent** node with Download **Add**, **List**, **Get**, and
**Remove** operations, plus a `qBit Bridge API` credential.

```sh
cd integrations/n8n
corepack pnpm install --frozen-lockfile
corepack pnpm build
```

### Destination aliases

Tools that direct download storage (`qbit_add_download`, `qbit_subscribe`) **do not accept arbitrary filesystem paths**. The operator declares aliases at boot via `--save-paths` (or `QBITTORRENT_SAVE_PATHS`):

```
--save-paths='kura-inbox=/mnt/kura,downloads=/mnt/downloads'
```

Callers pass alias names; the server resolves to a path before calling qBittorrent. Untrusted agents cannot redirect downloads outside the configured set.

### Audit logging

Every mutation (`add`, `remove`, `subscribe`, `unsubscribe`) emits a structured slog record with `audit=true`, the affected hashes/names, and tool-specific extras. Destructive ops (`remove`, `unsubscribe`) log at WARN so log aggregators filtering on level surface them.

## Build & run

```sh
go build ./cmd/qbit-bridge
./qbit-bridge --transport=stdio
./qbit-bridge --transport=http --addr=:8080
```

HTTP transport exposes the MCP endpoint at `/mcp`, REST downloads under `/api/v1/downloads`, and a k8s liveness probe at `/healthz`.

Run `lefthook install` once to enable the pre-commit hook; `lefthook run pre-commit --all-files` mirrors the Go CI gate.

## Container

Prebuilt images are published to GHCR on every push to `main` (as `:dev`) and on tag pushes (as `:vX.Y.Z` and `:latest`).

```sh
docker pull ghcr.io/wyvernzora/qbit-bridge:latest
docker pull ghcr.io/wyvernzora/qbit-bridge/n8n-nodes:latest

docker run --rm --network host \
  -e QBITTORRENT_URL=http://localhost:8080 \
  -e QBITTORRENT_SAVE_PATHS='kura-inbox=/mnt/kura' \
  ghcr.io/wyvernzora/qbit-bridge:latest
```

Local build:

```sh
docker build -t qbit-bridge .
docker run --rm --network host qbit-bridge           # sidecar-style: shares loopback with qBittorrent
docker run --rm -i qbit-bridge --transport=stdio     # stdio
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

The qBittorrent WebUI v2 calls go through [`github.com/autobrr/go-qbittorrent`](https://github.com/autobrr/go-qbittorrent); SDK errors are translated into the structured codes above for both MCP and REST clients.
