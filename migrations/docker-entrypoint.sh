#!/bin/sh
set -eu

BIN="/app/migrations/migrations"

wait_for_postgres() {
  DSN="$1"
  LABEL="$2"
  echo "Waiting for $LABEL to be reachable..."
  i=0
  while [ $i -lt 30 ]; do
    if "$BIN" -migrate=up -target="$LABEL" 2>/dev/null; then
      return 0
    fi
    # Check if it's a connection error vs a real error
    # Try a ping-style attempt
    i=$((i + 1))
    echo "  $LABEL not ready yet (attempt $i/30), retrying in 2s..."
    sleep 2
  done
  echo "ERROR: $LABEL failed to become ready after 30 attempts"
  return 1
}

case "${1:-}" in
  migrate-all)
    shift || true
    printf '%s\n' 'running auth migrations'
    i=0
    while [ $i -lt 30 ]; do
      if "$BIN" -migrate=up -target=auth; then
        break
      fi
      i=$((i + 1))
      echo "auth migration attempt $i/30 failed, retrying in 2s..."
      sleep 2
    done
    if [ $i -eq 30 ]; then
      echo "ERROR: auth migrations failed after 30 attempts"
      exit 1
    fi

    printf '%s\n' 'running object migrations'
    i=0
    while [ $i -lt 30 ]; do
      if "$BIN" -migrate=up -target=object; then
        break
      fi
      i=$((i + 1))
      echo "object migration attempt $i/30 failed, retrying in 2s..."
      sleep 2
    done
    if [ $i -eq 30 ]; then
      echo "ERROR: object migrations failed after 30 attempts"
      exit 1
    fi
    ;;
  seed-auth)
    shift || true
    printf '%s\n' 'running auth seed'
    "$BIN" -seed -target=auth
    ;;
  *)
    exec "$BIN" "$@"
    ;;
esac