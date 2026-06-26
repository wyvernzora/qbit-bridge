#!/bin/sh
# Starts mcp-inspector in background, waits for qbit-bridge to bind its
# MCP HTTP port, prints a prefilled inspector URL, then execs air (hot-reload).
#
# tini -g (PID 1, set in ENTRYPOINT) forwards SIGTERM to the whole process
# group so inspector + air + qbit-bridge all die together on docker stop.

set -e

if [ ! -d "/src/cmd/qbit-bridge" ]; then
  echo "entrypoint: /src/cmd/qbit-bridge missing — bind-mount the qbit-bridge repo at /src" >&2
  exit 1
fi

# Pin the inspector proxy auth token up front so we can embed it in the
# prefilled URL before the server starts.
if [ -z "${MCP_PROXY_AUTH_TOKEN:-}" ]; then
  MCP_PROXY_AUTH_TOKEN="$(tr -dc 'a-f0-9' < /dev/urandom | head -c 64)"
  export MCP_PROXY_AUTH_TOKEN
fi

MCP_PORT="${QBITTORRENT_ADDR##*:}"

echo "devserver: MCP HTTP on container 0.0.0.0:${MCP_PORT}" >&2
echo "devserver: edit any .go file under cmd/ or internal/ and air rebuilds in ~3s" >&2

# Waits for qbit-bridge to bind the MCP port, then prints a copy-paste
# inspector URL with all prefill query params. Backgrounded so air can
# exec in the foreground. Runs once per container start; air restarts
# do not retrigger this (the URL stays valid as long as the port stays bound).
(
  i=0
  while ! nc -z 127.0.0.1 "${MCP_PORT}" 2>/dev/null; do
    i=$((i + 1))
    if [ "${i}" -gt 600 ]; then
      echo "devserver: qbit-bridge did not bind port ${MCP_PORT} within 60s; skipping inspector URL" >&2
      exit 0
    fi
    sleep 0.1
  done

  # serverUrl points at the /mcp endpoint inside the container; the inspector
  # proxy reaches it via loopback. The /mcp path suffix is required —
  # qbit-bridge mounts the MCP handler there, not at root.
  SERVER_URL="http%3A%2F%2F127.0.0.1%3A${MCP_PORT}%2Fmcp"
  # MCP_PROXY_PORT tells the inspector UI bundle (running in the browser) which
  # host port the proxy listens on. UI defaults to 6277; we shift to 6477 to
  # avoid colliding with dmhy-mcp's (6377) and kura's (6277) devservers, so we
  # must override via URL param.
  INSPECTOR_URL="http://localhost:${CLIENT_PORT}/?MCP_PROXY_AUTH_TOKEN=${MCP_PROXY_AUTH_TOKEN}&MCP_PROXY_PORT=${SERVER_PORT}&transport=streamable-http&serverUrl=${SERVER_URL}"

  echo "devserver: open the inspector UI at:" >&2
  echo "  ${INSPECTOR_URL}" >&2
) &

mkdir -p /src/tmp

# mcp-inspector reads MCP_PROXY_AUTH_TOKEN, HOST, CLIENT_PORT, SERVER_PORT,
# ALLOWED_ORIGINS, MCP_AUTO_OPEN_ENABLED from env (set in Dockerfile ENV).
mcp-inspector &

exec air -c /etc/qbit-bridge/air.toml
