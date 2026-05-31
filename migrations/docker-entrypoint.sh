#!/bin/sh
set -eu

BIN="/app/migrations/migrations"

case "${1:-}" in
  migrate-all)
    shift || true
    printf '%s\n' 'running auth migrations'
    "$BIN" -migrate=up -target=auth
    printf '%s\n' 'running object migrations'
    "$BIN" -migrate=up -target=object
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