#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PIDS_DIR="$ROOT_DIR/.pids"

stop_service() {
  local name="$1"
  local pid_file="$PIDS_DIR/${name}.pid"

  if [[ ! -f "$pid_file" ]]; then
    echo "$name is not running (pid file missing)"
    return
  fi

  local pid
  pid="$(cat "$pid_file")"
  if kill -0 "$pid" >/dev/null 2>&1; then
    kill "$pid"
    echo "Stopped $name (pid=$pid)"
  else
    echo "$name pid $pid is not running"
  fi

  rm -f "$pid_file"
}

stop_service "api-gateway"
stop_service "object-api"
stop_service "auth-service"

echo "Done."
