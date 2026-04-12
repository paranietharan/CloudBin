#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PIDS_DIR="$ROOT_DIR/.pids"
LOGS_DIR="$ROOT_DIR/.logs"

mkdir -p "$PIDS_DIR" "$LOGS_DIR"

start_service() {
  local name="$1"
  local dir="$2"
  local pid_file="$PIDS_DIR/${name}.pid"
  local log_file="$LOGS_DIR/${name}.log"

  if [[ -f "$pid_file" ]]; then
    local existing_pid
    existing_pid="$(cat "$pid_file")"
    if kill -0 "$existing_pid" >/dev/null 2>&1; then
      echo "$name is already running (pid=$existing_pid)"
      return
    fi
    rm -f "$pid_file"
  fi

  (
    cd "$dir"
    nohup go run ./cmd >"$log_file" 2>&1 &
    echo $! >"$pid_file"
  )

  echo "Started $name"
  echo "  pid file: $pid_file"
  echo "  log file: $log_file"
}

start_service "auth-service" "$ROOT_DIR/auth-service"
start_service "object-api" "$ROOT_DIR/object-api"
start_service "api-gateway" "$ROOT_DIR/api-gateway"

echo
echo "All services started."
echo "Run ./scripts/health-check.sh to verify health endpoints."
