#!/bin/sh
# Copy the baked-in qbittorrent-mcp node package into the shared volume n8n scans via
# N8N_CUSTOM_EXTENSIONS. Intended for an initContainer with an emptyDir target.
set -eu

TARGET="${QBITTORRENT_MCP_NODES_TARGET:-/opt/n8n/custom}"
DEST="${TARGET}/n8n-nodes-qbittorrent-mcp"

echo "qbittorrent-mcp-n8n-nodes: installing into ${DEST}"
mkdir -p "${DEST}"
cp -r /qbittorrent-mcp-nodes/. "${DEST}/"
chmod -R a+rX "${DEST}"
echo "qbittorrent-mcp-n8n-nodes: installed"
ls -la "${DEST}"
