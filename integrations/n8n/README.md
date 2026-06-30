# qbit-bridge n8n nodes

Custom n8n node for qbit-bridge. The package is built as an init-container
image that installs into the directory n8n reads from `N8N_CUSTOM_EXTENSIONS`.

```sh
docker pull ghcr.io/wyvernzora/qbit-bridge/n8n-nodes:latest
```

## Node

| Node | What it does |
| --- | --- |
| qBit Bridge | Resource **Download**, operations **Add**, **List**, **Get**, and **Remove**. |
| qBit Bridge Download Finished Trigger | Polls qbit-bridge for completed downloads and emits each hash at most once per lease window. |

**Download -> List** emits each returned `downloads[]` row as its own n8n item. It supports tag include/exclude filters via **Tags** and **Not Tags**.

**Download -> Add/Get/Remove** operate one input item at a time.

The download-finished trigger treats a job as complete when `completion_on > 0`
or `progress >= 1`, with the same **Tags** and **Not Tags** filters. Downstream
removal of the qBittorrent job acts as the ack; if the job remains visible, it
is emitted again after the lease expires.

## Development

```sh
corepack enable
corepack pnpm install --frozen-lockfile
corepack pnpm typecheck
corepack pnpm build
```
