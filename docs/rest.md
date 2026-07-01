# REST API

HTTP transport exposes a small REST facade for n8n-style workflows beside MCP:

```sh
./qbit-bridge --transport=http --addr=:8080
```

Base path: `/api/v1`. The REST surface is unauthenticated for the same sidecar/trusted-network reason as MCP.

Errors use the same shape as MCP tool errors:

```json
{ "code": "invalid_argument", "message": "...", "retriable": false }
```

Status mapping: `invalid_argument` is `400`, `upstream_not_found` is `404`, qBittorrent upstream failures are `502`, and `internal` is `500`.

## Downloads

### `POST /api/v1/downloads`

Add one magnet download. Request:

```json
{
  "magnet": "magnet:?xt=urn:btih:deadbeef...",
  "tags": ["tvdb:12345", "weekly"],
  "destination": "kura-inbox:show-name",
  "rename": "optional display name"
}
```

Response:

```json
{
  "hash": "deadbeef...",
  "accepted": true,
  "already_existed": false
}
```

### `GET /api/v1/downloads`

List downloads. Query parameters:

| Param | Repeatable | Meaning |
| --- | --- | --- |
| `states` | yes | Normalized states: `downloading`, `seeding`, `paused`, `stalled`, `queued`, `checking`, `errored`, `unknown` |
| `tags` | yes | Tag patterns using Go `path.Match` glob syntax, e.g. `tvdb:*` |
| `not_tags` | yes | Exclude downloads matching any tag pattern, same syntax as `tags` |
| `hashes` | yes | Exact hash set |
| `sort` | no | Same values as `search_downloads`, default `added_on_desc` |
| `limit` | no | Default `50`, max `200` |
| `offset` | no | Default `0` |
| `include_fields` | yes | Same projection keys as `search_downloads` |

Response:

```json
{
  "count": 1,
  "has_more": false,
  "downloads": [
    {
      "hash": "deadbeef...",
      "name": "Show - 01",
      "state": "downloading",
      "progress": 0.42,
      "size_bytes": 12345678,
      "dlspeed_bytes_per_sec": 524288,
      "upspeed_bytes_per_sec": 0,
      "eta_seconds": 1234,
      "ratio": 0,
      "tags": ["tvdb:12345"],
      "added_on": 1714851923
    }
  ]
}
```

### `GET /api/v1/downloads/{hash}`

Return one download. Supports `include_fields` query parameters.

Response is a single `Download` object, not a list wrapper. Missing hashes return `404` with `code: "upstream_not_found"`.

### `DELETE /api/v1/downloads/{hash}`

Remove one download from qBittorrent tracking. Files on disk are not deleted.

Response:

```json
{
  "affected_count": 1,
  "affected_hashes": ["deadbeef..."]
}
```

### `PUT /api/v1/downloads/{hash}/tags`

Add and/or remove literal tags on one download.

```json
{
  "add": ["require-review"],
  "remove": ["auto-adopt"]
}
```

Response:

```json
{
  "affected_count": 1,
  "affected_hashes": ["deadbeef..."]
}
```

## n8n usage

n8n should call one item at a time:

- Add node: one item to `POST /api/v1/downloads`.
- List node: call `GET /api/v1/downloads`, then emit each `downloads[]` entry as an n8n item.
- Get node: one hash to `GET /api/v1/downloads/{hash}`.
- Remove node: one hash to `DELETE /api/v1/downloads/{hash}`.
- Update Tags node: one hash to `PUT /api/v1/downloads/{hash}/tags`.
