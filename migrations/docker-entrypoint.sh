#!/bin/sh
set -eu

BIN="/app/migrations/migrations"

migrate_target() {
  target="$1"
  echo "running $target migrations"
  attempt=0
  while [ "$attempt" -lt 30 ]; do
    if "$BIN" -migrate=up -target="$target"; then
      return 0
    fi
    attempt=$((attempt + 1))
    echo "$target migration attempt $attempt/30 failed, retrying in 2s..."
    sleep 2
  done
  echo "ERROR: $target migrations failed after 30 attempts"
  return 1
}

case "${1:-}" in
  migrate-all)
    migrate_target auth
    migrate_target object
    ;;
  seed-auth)
    printf '%s\n' 'running auth seed'
    "$BIN" -seed -target=auth
    ;;
  *)
    exec "$BIN" "$@"
    ;;
esac
