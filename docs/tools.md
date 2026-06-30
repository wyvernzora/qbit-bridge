# Tool surface

Design spec for the MCP tools `qbit-bridge` exposes. Six tools across three groups: downloads (4), tags (1), destinations (1).

Design priorities:

- **Caller context budget is finite.** Tool descriptions stay short; outputs use lean default projections with opt-in expansion; lists default to 50 results, max 200.
- **Discrete narrow tools, not fat polymorphic ones.** Agents pick narrow tools more reliably than discriminated-union schemas.
- **Agent intent over qBittorrent mechanism.** The surface reflects the agent's mental model of "manage downloads" — observe, add, remove — not qbit's torrent/feed/rule abstraction. Pause/resume, mid-life updates, and a separate single-hash get all fold away because they don't match any real agent workflow.
- **Magnet-only `add_download`.** No URL or .torrent file uploads in v1 — keeps the input shape small and the handler synchronous (hash is known before the qbit call).
- **Filter-vs-hashes mutual exclusion on bulk ops.** Caller passes either explicit `hashes[]` or a `filter` object, never both. Forgetting `hashes` cannot accidentally remove every download.
- **Normalized state enum.** qBittorrent's 21 raw states collapse to 8 logical buckets the agent cares about. State values are case-insensitive on input — `"Downloading"` and `"downloading"` both pass validation.
- **Tool naming reflects behavior.** Read tools that filter and paginate use search verbs; read tools that return small fixed sets use list verbs.

## Agent anti-patterns

Things the surface is designed to make unnecessary; doing them anyway wastes tokens or risks correctness:

- **Pre-checking with `search_downloads` before `add_download`.** The add is idempotent and reports `already_existed`. One call, not two.
- **Fabricating destination alias names.** The valid set lives in the tool description (`destHint`) and is also queryable via `list_destinations`. Unknown aliases return `invalid_argument` with the configured set listed.
- **Calling `add_download` with magnet + assuming `name` is in the response.** Magnet metadata fetch is async in qbit; `name` resolves later. Follow up with `search_downloads` filtered to the returned hash.
- **Passing `hashes` and `filter` together to `remove_downloads`.** Rejected. Pick one.
- **Asking for `trackers` or `files` across a wide multi-hash `search_downloads` without thinking about size.** Not rejected — but every hash triggers a separate upstream fetch, and the response payload scales linearly. Scope the query before opting in to per-hash enrichments.
- **Retrying after `upstream_forbidden`.** That's the loopback-auth-bypass check failing; retry won't fix it. Surface the operator-action message to the user.

## Operating model

qbit-bridge is designed as a **sidecar in a trusted environment**: one agent, loopback-only access to one qBittorrent instance, operator-controlled deploy. This shapes a few deliberate choices the agent should be aware of:

- **No rate limiting.** The MCP spec recommends per-tool rate limits; qbit-bridge omits them because the deploy assumption is a single trusted caller. If you embed qbit-bridge in a multi-tenant or untrusted-caller deployment, add a fronting proxy with rate limits — the surface here does not enforce them.
- **No authentication on the MCP or REST endpoints.** Same rationale: loopback or container-internal traffic only.
- **qBittorrent must have "Bypass authentication for clients on localhost" enabled.** qbit-bridge performs no login against qBittorrent; the loopback-auth-bypass is load-bearing.

## Conventions

### Destinations (save-path aliases)

Tools that direct download storage (`add_download`) **do not accept arbitrary filesystem paths**. Callers pass the *name* of a deploy-time-configured alias via a `destination` field; the server resolves the name to a path before calling qBittorrent. Untrusted agents cannot redirect download storage outside the configured set.

The operator declares aliases at boot time via `--save-paths` / `QBITTORRENT_SAVE_PATHS`:

```
--save-paths='kura-inbox=/mnt/kura,downloads=/mnt/downloads'
```

Format: `name=path,name=path`. Names match `[a-z0-9][a-z0-9-]*`. Empty input is allowed (no aliases configured); in that case, tools that accept a `destination` only accept the empty string, which means "leave save_path unset, qBittorrent uses its account default".

Tool descriptions include the current alias list at session start, so agents see exactly which names are valid:

```
Valid destinations: downloads, kura-inbox. Leave empty to use qBittorrent's account default.
```

**Every** path on input and on regular output projections uses the form `"<prefix>:<rest>"` (or the bare `"<prefix>"` form when there is no rest). No raw absolute paths cross the wire on those surfaces.

The one carve-out is `list_destinations`, which intentionally exposes the alias→absolute-path map so operators can verify the deploy-time configuration and so agents can translate an `unspecified:…` projection back to a real filesystem location. It is the only tool that surfaces raw absolute paths.

**Output projections** (`save_path`, `content_path` on `search_downloads`):

| qBittorrent stored path | Output |
| --- | --- |
| Path **equals** an alias root, e.g. `/mnt/kura` with `kura-inbox=/mnt/kura` | `"kura-inbox"` |
| Path **nested** below an alias root, e.g. `/mnt/kura/anime/show` | `"kura-inbox:anime/show"` |
| Path **not** under any alias, e.g. `/data/random` | `"unspecified:data/random"` |

The `unspecified:` prefix is a reserved sentinel for paths that no configured alias covers. The leading `/` on the absolute path is stripped after the colon for format symmetry with the alias-relative form. Agents that need to surface the underlying filesystem path to a user can prepend a `/` to the part after `unspecified:`.

Boundary-safe: `/mnt/kura-other/foo` does **not** match an alias rooted at `/mnt/kura` — only `/mnt/kura` or `/mnt/kura/…` do. Falls through to `unspecified:mnt/kura-other/foo`. Longest-prefix wins when two aliases nest (e.g. `outer=/x` and `inner=/x/y` → `/x/y/z` resolves as `inner:z`).

**Input fields** (`destination` on `add_download`):

| Input | Meaning |
| --- | --- |
| `"kura-inbox"` | alias root (resolves to the configured filesystem path) |
| `"kura-inbox:season-1"` | alias root joined with the relpath → `/mnt/kura/season-1` |
| `"unspecified:…"` | **rejected** with `invalid_argument` — output-only sentinel |
| Unknown alias name | **rejected** with `invalid_argument` (configured set listed in the error message) |

Relpath validation: absolute relpaths (`kura-inbox:/etc/passwd`) are rejected; relpaths containing `..` components that would escape the alias root (`kura-inbox:foo/../../etc`) are rejected. Internal redundancy is silently normalized (`kura-inbox:./foo/./bar/` → `/mnt/kura/foo/bar`).

**Roundtrip works** for aliased paths: an agent that observed `save_path: "kura-inbox:anime/show"` can pass the same value back as `add_download.destination` to point a new download at the same directory. Paths surfaced as `unspecified:…` cannot be roundtripped — they were placed by an operator outside the alias map and the agent has no authority to create new downloads there.

**Reserved alias name**: `unspecified` is rejected at boot if an operator tries to configure it via `--save-paths`; it exists solely as the no-alias output sentinel.

The `list_destinations` tool exposes the alias-to-root map so an agent can confirm which prefixes are valid on input.

### Audit logging

Every mutation tool (`add_download`, `remove_downloads`, `update_download_tags`) emits a structured slog record **before** the upstream call so the action is visible even when upstream rejects the request. The records share a fixed shape:

```
msg=tool audit
audit=true
action=<verb>        ← add | remove | tag
hashes=[h1 h2 ...]
count=N
[tool-specific extras]
```

Severity differentiates ops so log aggregators filtering on level can surface the destructive ones:

| Action | Level | Extras | Why |
|---|---|---|---|
| `add` | `INFO` | `destination`, `tags`, `already_existed` | Reversible by `remove_downloads`. `already_existed=true|false` distinguishes the idempotent re-add (skipped upstream call) from a fresh add. |
| `remove` | `WARN` | _(none)_ | Even without on-disk deletion, "qbit forgot this download" is the kind of event operators investigate when something downstream breaks. |
| `tag` | `INFO` | `add`, `remove` | Reversible tag metadata patch on explicit hashes. |

Operators investigating "what did the agent do" can `grep audit=true` on the structured JSON stderr stream. The `hashes` field carries the full hash list so per-hash forensics work too.

`wrap`'s per-call timing log (logged at INFO with `tool=<name>` and `duration_ms`) continues to capture every tool call including reads. The audit layer is additive — finer-grained, mutation-only.

### Tag-pattern matching

`tags` filter fields on `search_downloads` and `remove_downloads.filter` accept shell-style globs using Go's `path.Match` syntax:

| Pattern token | Matches |
|---|---|
| `*` | any run of characters |
| `?` | exactly one character |
| `[abc]` | any one of `a`, `b`, `c` |
| `[a-z]` | any character in the range |
| plain string | exact tag name |

OR semantics across the patterns list — a download is included if any pattern matches any of its tags.

Use case: dmhy-mcp tags downloads it adds with `tvdb:<series-id>`. `search_downloads` with `tags: ["tvdb:*"]` returns every kura-related job. `tags: ["tvdb:12345", "tvdb:67890"]` returns just those two series (literal match).

Mutation fields (`tags` on `add_download`) are **literal tag names**, not patterns — qBittorrent's API requires exact strings.

Malformed patterns return `invalid_argument` naming the offending entry.

### Cross-server integration with dmhy-mcp

For one-shot grabs (one specific episode), an agent can go through `dmhy_get_magnets(info_hashes=[...])` → `add_download(magnet=...)`, and qbit-bridge's idempotent re-add covers retry safety on that path.

### Hash format

Full 40-char SHA-1 lowercase hex (qBittorrent's `infohash_v1` form). Echoed unchanged from upstream — no truncation, no normalization beyond what qbit returns. Agents need exact match for follow-up calls.

### Numeric units

Sizes in bytes, speeds in bytes-per-second, durations in seconds, timestamps as epoch seconds. No humanized strings. Agents do their own formatting; locale-aware formatting wastes tokens.

### Error shape

Every tool returns the standard `*ToolError` (`internal/mcp/errors.go`) on failure:

```json
{ "code": "upstream_unavailable", "message": "...", "retriable": true }
```

Codes used by this tool surface:

| Code | When |
|---|---|
| `invalid_argument` | Client-side validation rejected the input (bad magnet, mutually-exclusive fields, etc.) |
| `upstream_unavailable` | Network error, 5xx from qBittorrent, or context cancellation |
| `upstream_forbidden` | 401/403 from qBittorrent — loopback-auth-bypass is misconfigured |
| `upstream_not_found` | Hash, rule, or path that the request references does not exist |
| `internal` | Bug in qbit-bridge (e.g. response decode failure) |

### Normalized download state

| Normalized | Maps from raw qBittorrent states |
|---|---|
| `downloading` | `downloading`, `metaDL`, `forcedDL`, `allocating` |
| `seeding` | `uploading`, `forcedUP` |
| `paused` | `pausedDL`, `pausedUP`, `stoppedDL`, `stoppedUP` |
| `stalled` | `stalledDL`, `stalledUP` |
| `queued` | `queuedDL`, `queuedUP` |
| `checking` | `checkingDL`, `checkingUP`, `checkingResumeData`, `moving` |
| `errored` | `error`, `missingFiles` |
| `unknown` | `unknown` (or anything qBittorrent adds later) |

Every download in `search_downloads` output carries `state` as one of the normalized values above. qBittorrent's raw state string is not echoed back.

---

## Download tools

### `search_downloads`

Primary read. Filtered, sorted, paginated.

**Input:**

```json
{
  "states": ["downloading", "stalled"],   // optional; OR semantics
  "tags": ["tvdb:*", "weekly"],            // optional; OR; shell-style globs (path.Match)
  "hashes": ["aabbcc..."],                 // optional; exact set
  "sort": "added_on_desc",                 // see enum below; default added_on_desc
  "limit": 50,                             // default 50, max 200
  "offset": 0,                             // default 0
  "include_fields": ["save_path"]          // see opt-in fields below
}
```

`states` accepts the eight normalized values listed above, **case-insensitively** — `"Downloading"`, `"DOWNLOADING"`, and `"downloading"` all pass. Unknown states return `invalid_argument`.

`sort` enum: `name_asc`, `name_desc`, `added_on_asc`, `added_on_desc` (default), `size_asc`, `size_desc`, `progress_asc`, `progress_desc`, `dlspeed_desc`, `eta_asc`, `ratio_desc`.

`include_fields` opt-in values:

- **Field-level:** `save_path`, `content_path`, `magnet_uri`, `completion_on`, `last_activity`, `total_uploaded`, `total_downloaded`, `total_size`, `seeds`, `seeds_incomplete`, `peers`, `tracker_count`, `seeding_time`, `private`. The two path-shaped fields (`save_path`, `content_path`) are rewritten to the `<alias>:<relpath>` form per the Destinations convention — see above.
- **Per-hash enrichments:** `trackers`, `files`. Each triggers one additional upstream call per result hash. Response size scales with `len(hashes) * (trackers_per_torrent + files_per_torrent)`, so opt in deliberately — `trackers` on a 50-torrent page is ~500 sub-entries. No hard limit is enforced; the sidecar is trusted and the agent is expected to scope its own queries.
- **Convenience:** `"all"` expands every field-level key (not trackers/files). Use `["all", "trackers", "files"]` for the kitchen-sink projection.

`private` indicates a private-tracker torrent — agents that prune downloads should treat it as a "do not bulk-remove" hint to avoid hit-and-run penalties before the ratio is met.

Off by default.

**Output:**

```json
{
  "count": 12,
  "has_more": false,
  "downloads": [
    {
      "hash": "deadbeef...",
      "name": "[Erai-raws] Show - 03",
      "state": "downloading",
      "progress": 0.42,
      "size_bytes": 12345678,
      "dlspeed_bytes_per_sec": 524288,
      "upspeed_bytes_per_sec": 0,
      "eta_seconds": 1234,
      "ratio": 0.0,
      "tags": ["weekly"],
      "added_on": 1714851923
    }
  ]
}
```

`tags` is an array; `eta_seconds` is `-1` when unknown (qBittorrent's `8640000` sentinel collapses to `-1`).

### `add_download`

Add a single download by magnet URI.

**Input:**

```json
{
  "magnet": "magnet:?xt=urn:btih:deadbeef...&dn=Name&tr=udp://...",
  "tags": ["weekly"],             // optional; literal tag names, no commas
  "destination": "kura-inbox",    // optional; alias name only — see Destinations above
  "rename": "Custom name"         // optional; qBittorrent display-name override
}
```

Client-side validation rejects with `invalid_argument` when `magnet` is missing, has no `xt=urn:btih:<hash>` parameter, or the hash is not 40-char hex / 32-char base32; when `destination` is set to an unknown alias name; or when any tag contains a comma. Hash is parsed before the upstream call so the response carries it deterministically.

Magnet hash is normalized to 40-char lowercase hex in the response — base32 hashes are decoded to bytes and re-encoded as hex.

`auto_tmm` is always forced to `false` on the upstream call so the resolved destination is not silently overridden by qBittorrent's category-based routing. There is no input knob to change this — exposing one would defeat the destination-alias security boundary.

`paused`, `sequential`, and `auto_tmm` are not exposed as inputs. Magnets cannot fetch metadata while paused, sequential download is a power-user knob with no agent workflow, and auto_tmm would override the destination alias. If a workflow ever needs them, configure directly via the qBittorrent UI.

**Output:**

```json
{
  "hash": "deadbeef...",
  "accepted": true,
  "already_existed": false
}
```

`accepted: true` means qBittorrent acknowledged the add (or the hash was already present — see below). Metadata fetch for new magnets is asynchronous in qbit; agents that need the resolved `name` should follow up with `search_downloads` filtered to the returned hash.

**Idempotency.** The handler pre-checks via `/torrents/info?hashes=<h>` before issuing the add. If the hash is already known to qBittorrent, the upstream add is skipped and the response carries `already_existed: true`. The live download — tags, destination, progress — is left untouched; re-add does not mutate existing torrent state. The audit record always fires (the agent's intent to add is logged either way) with an `already_existed` field so operators can tell the noop case apart.

This makes retry-safe agent workflows simple: an agent that loses track of whether it already submitted a magnet can call `add_download` again without risk of duplicate adds or destination drift.

### `remove_downloads`

Remove downloads from qBittorrent's tracking. Pass exactly one of `hashes` or `filter`.

**Input:**

```json
{
  "hashes": ["aabbcc..."]
}
```

or:

```json
{
  "filter": { "states": ["downloading"], "tags": ["weekly"] }
}
```

Filter accepts `states` and `tags` (same semantics as `search_downloads`; tags use shell-style globs). Passing both `hashes` and `filter` returns `invalid_argument`. Passing **neither** also returns `invalid_argument` (refuses to operate on every download).

**Empty selectors are a no-op success.** An explicitly-empty `hashes: []` returns `{ "affected_count": 0, "affected_hashes": [] }` with no upstream call. A `filter` that resolves to zero matches does the same. Agents building a hash list dynamically can call without guarding against the empty case. The distinction is between *empty* (provided, resolves to nothing → no-op) and *absent* (forgot to pass it at all → rejected, because we can't tell whether you meant no-op or every-download).

**There is no `delete_files` field** — files on disk are never deleted by this tool. The qBittorrent entry is removed; the underlying files are an operator concern (cron, kura's trash, manual rm). Exposing on-disk deletion would punch through the destination-alias security boundary.

**Output:**

```json
{
  "affected_count": 3,
  "affected_hashes": ["aabbcc...", "ddeeff...", "112233..."]
}
```

### `update_download_tags`

Add and/or remove literal tags on explicitly selected download hashes.

**Input:**

```json
{
  "hashes": ["aabbcc..."],
  "add": ["kura:reviewed"],
  "remove": ["weekly"]
}
```

`hashes` is required. An explicitly-empty `hashes: []` is a no-op success with no upstream call. At least one of `add` or `remove` must be non-empty. Tag names are literal, not glob patterns, and must not contain commas.

**Output:**

```json
{
  "affected_count": 1,
  "affected_hashes": ["aabbcc..."]
}
```

---

## Tag tools

### `list_tags`

Read all tags configured in qBittorrent.

**Output:**

```json
{
  "tags": ["weekly", "movies", "complete"]
}
```

Tags auto-create when `add_download.tags` references an unknown tag. No `create_tag` / `delete_tag` tools in v1.

---

## Destination tools

### `list_destinations`

Read the deploy-time-configured save-path aliases. No upstream call — the map is fixed for the lifetime of the qbit-bridge process. Restart with a different `--save-paths` / `QBITTORRENT_SAVE_PATHS` to change it.

**Output:**

```json
{
  "destinations": [
    { "name": "kura-inbox", "path": "/mnt/kura" },
    { "name": "downloads",  "path": "/mnt/downloads" }
  ]
}
```

`name` is the value to pass on `add_download.destination`. `path` is the resolved absolute filesystem path qBittorrent stores under that alias — the one place on the surface where a raw absolute path is intentionally exposed. Two use cases:

1. **Confirm which alias names are valid** before invoking a destination-using tool. Same content as the description-time `destHint`, but queryable on demand if it's stale.
2. **Translate an `unspecified:<rest>` projection back to a real location.** If a Download was added outside the alias map (operator action, legacy download), its `save_path` surfaces as `unspecified:data/some/path`. `list_destinations` lets the agent compare that against the configured roots and decide whether to leave it alone, rename, or surface to the user with the real `/data/some/path` for context.

Returns an empty array when no aliases are configured. Names are sorted alphabetically.

---

## Deferred to follow-ups (not in v1)

- `update_downloads` — broader mid-life metadata edits (destination, name). Dropped because everything is set at `add_download` time; metadata churn isn't an agent workflow. Tags are covered by `update_download_tags`.
- `pause_downloads` / `resume_downloads` — operator concern (maintenance windows, bandwidth scheduling), not agent workflow. Re-add if a workflow surfaces.
- `get_download` — folded into `search_downloads` via `include_fields=["all", "trackers", "files"]` with a single-hash query.
- `recheck_torrents` — rare workflow; add when there is demand.
- Download projection fields previously offered but dropped because no agent workflow used them: `download_path` (only meaningful with qBit's separate-incomplete-dir option), `auto_tmm` (forced false on every add by design), `sequential` and `first_last_piece_prio` (video-streaming optimizations), `force_start` and `super_seeding` (operator-level queue / seeding knobs), `ratio_limit` and `seeding_time_limit` (operator-set caps). Each is a one-line setter restore in `downloadFieldSetters` if a real workflow surfaces.
- Queue event stream — push notifications to the agent on `add`/`complete`/`error` events, so the agent can intervene at the moment a download lands rather than polling `search_downloads`. qBittorrent exposes this via `/api/v2/sync/maindata` polling; an MCP `notifications/*` channel could surface it. Not in v1 because it requires the server to hold a long-lived connection per agent.
- `set_torrent_speed_limits`, `set_torrent_share_limit` — agent-uncommon power-user knobs.
- `recheck`, `reannounce`, `set_force_start`, `set_super_seeding` — download-level toggles that complicate the v1 surface without a clear workflow story.
- Download file upload (raw bytes) — magnet URIs cover the agentic flow we ship dmhy-mcp + qbit-bridge for.
- Tracker / peer / file management (`add_trackers`, `ban_peers`, `set_file_priority`, `rename_file`).
- Search-plugin tools.

These all map cleanly onto the established `internalHandler` + `wrap` pattern; adding any one is one new struct pair plus one `mcpsdk.AddTool` call.

---

## Context-budget accounting

| Component | Approx tokens |
|---|---|
| Tool list (6 names + descriptions) loaded per turn | 0.5k – 0.7k |
| `search_downloads` default response, 50 downloads | 3.5k – 4.5k |
| `search_downloads` default response, 10 downloads | 0.7k – 1.0k |
| `search_downloads` single-hash with `include_fields=["all"]` (no trackers/files) | 0.3k |
| `search_downloads` single-hash with `include_fields=["all","trackers","files"]` on a typical anime release | 1.0k – 2.0k |

Rule of thumb: a download-aware agent that lists 20 active downloads and inspects one in detail eats ~2.5k tokens per probe loop. Comfortable budget at modern context sizes; would not be on smaller models.
