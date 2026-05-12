#!/bin/sh
# air invokes this as the bin path after each successful build.
# The binary lives at /src/tmp/qbit-mcp (built by air's build.cmd).
# QBITTORRENT_ADDR defaults to 0.0.0.0:8091 (set in Dockerfile ENV).
# QBITTORRENT_URL is passed through from `make devserver-run`; when reaching
# a qBittorrent on the host from inside the container, set it to
# http://host.docker.internal:8080 on the host shell.

set -e

exec /src/tmp/qbit-mcp \
  --transport=http \
  --addr="${QBITTORRENT_ADDR:-0.0.0.0:8091}" \
  --log-level="${QBITTORRENT_LOG_LEVEL:-debug}"
