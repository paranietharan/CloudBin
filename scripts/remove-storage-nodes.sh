#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTAINER_PREFIX="cloudbin-storage-node"
IMAGE_NAME="cloudbin-storage-node:latest"
NETWORK_NAME="cloudbin-net"

REMOVE_IMAGE=false
REMOVE_NETWORK=false
REMOVE_DATA=false

for arg in "$@"; do
  case "$arg" in
    --image)
      REMOVE_IMAGE=true
      ;;
    --network)
      REMOVE_NETWORK=true
      ;;
    --data)
      REMOVE_DATA=true
      ;;
    *)
      echo "Unknown option: $arg"
      echo "Usage: ./scripts/remove-storage-nodes.sh [--image] [--network] [--data]"
      exit 1
      ;;
  esac
done

for i in 1 2 3; do
  name="${CONTAINER_PREFIX}-${i}"
  docker rm -f "$name" >/dev/null 2>&1 || true
  echo "Removed $name"
done

if [ "$REMOVE_IMAGE" = true ]; then
  docker rmi "$IMAGE_NAME" >/dev/null 2>&1 || true
  echo "Removed image $IMAGE_NAME"
fi

if [ "$REMOVE_NETWORK" = true ]; then
  docker network rm "$NETWORK_NAME" >/dev/null 2>&1 || true
  echo "Removed network $NETWORK_NAME"
fi

if [ "$REMOVE_DATA" = true ]; then
  rm -rf "$ROOT_DIR/storage-data/node1" "$ROOT_DIR/storage-data/node2" "$ROOT_DIR/storage-data/node3"
  echo "Removed storage data directories"
fi

echo "Storage-node cleanup completed."
