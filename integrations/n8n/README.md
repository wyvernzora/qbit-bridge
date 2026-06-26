# qbit-bridge n8n nodes

Custom n8n node for qbit-bridge. The package is built as an init-container
image that installs into the directory n8n reads from `N8N_CUSTOM_EXTENSIONS`.

## Node

| Node | What it does |
| --- | --- |
| qBittorrent | Resource **Download**, operations **Add**, **List**, **Get**, and **Remove**. |

**Download -> List** emits each returned `downloads[]` row as its own n8n item.

**Download -> Add/Get/Remove** operate one input item at a time.

## Development

```sh
corepack enable
corepack pnpm install --frozen-lockfile
corepack pnpm typecheck
corepack pnpm build
```
