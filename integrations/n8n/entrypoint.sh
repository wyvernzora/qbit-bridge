#!/bin/sh
# Copy the baked-in qbit-bridge node package into the shared volume n8n scans via
# N8N_CUSTOM_EXTENSIONS. Intended for an initContainer with an emptyDir target.
set -eu

TARGET="${QBIT_BRIDGE_NODES_TARGET:-/opt/n8n/custom}"
DEST="${TARGET}/n8n-nodes-qbit-bridge"

echo "qbit-bridge-n8n-nodes: installing into ${DEST}"
mkdir -p "${DEST}"
cp -r /qbit-bridge-nodes/. "${DEST}/"
chmod -R a+rX "${DEST}"
echo "qbit-bridge-n8n-nodes: installed"
ls -la "${DEST}"
