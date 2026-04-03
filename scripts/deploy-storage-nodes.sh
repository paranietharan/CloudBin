#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE_NAME="cloudbin-storage-node:latest"
CONTAINER_PREFIX="cloudbin-storage-node"
NETWORK_NAME="cloudbin-net"

# Build storage-node image
cd "$ROOT_DIR/storage-node"
docker build -t "$IMAGE_NAME" .

# Ensure network exists
if ! docker network inspect "$NETWORK_NAME" >/dev/null 2>&1; then
  docker network create "$NETWORK_NAME" >/dev/null
fi

# Prepare host data directories
mkdir -p "$ROOT_DIR/storage-data/node1" "$ROOT_DIR/storage-data/node2" "$ROOT_DIR/storage-data/node3"

# Recreate three storage node containers
for i in 1 2 3; do
  name="${CONTAINER_PREFIX}-${i}"
  port=$((8082 + i))
  data_dir="$ROOT_DIR/storage-data/node${i}"

  docker rm -f "$name" >/dev/null 2>&1 || true

  docker run -d \
    --name "$name" \
    --network "$NETWORK_NAME" \
    -p "${port}:${port}" \
    -e STORAGE_NODE_ID="node-${i}" \
    -e STORAGE_PORT="${port}" \
    -e STORAGE_ROOT="/data" \
    -v "$data_dir:/data" \
    "$IMAGE_NAME" >/dev/null

  echo "Started $name on port $port"
done

echo "Storage nodes deployed successfully."
echo "Health checks:"
echo "  curl http://localhost:8083/healthz"
echo "  curl http://localhost:8084/healthz"
echo "  curl http://localhost:8085/healthz"
