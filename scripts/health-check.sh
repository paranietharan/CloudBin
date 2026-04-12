#!/usr/bin/env bash
set -euo pipefail

check() {
  local name="$1"
  local url="$2"

  if curl -fsS "$url" >/dev/null 2>&1; then
    echo "[OK]   $name -> $url"
  else
    echo "[FAIL] $name -> $url"
    return 1
  fi
}

status=0
check "api-gateway" "http://localhost:8080/healthz" || status=1
check "auth-service" "http://localhost:8081/healthz" || status=1
check "object-api" "http://localhost:8082/healthz" || status=1
check "storage-node-1" "http://localhost:8083/healthz" || status=1
check "storage-node-2" "http://localhost:8084/healthz" || status=1
check "storage-node-3" "http://localhost:8085/healthz" || status=1

exit "$status"
