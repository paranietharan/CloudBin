#!/usr/bin/env bash
set -euo pipefail

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required"
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required"
  exit 1
fi

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
EMAIL="${EMAIL:-}"
PASSWORD="${PASSWORD:-}"
FILE_PATH="${FILE_PATH:-}"

if [[ -z "$EMAIL" || -z "$PASSWORD" || -z "$FILE_PATH" ]]; then
  echo "Usage: EMAIL=<email> PASSWORD=<password> FILE_PATH=/abs/path/file ./scripts/integration-main-flow.sh"
  exit 1
fi

echo "1) Login"
LOGIN_RESP="$(curl -fsS -X POST "$GATEWAY_URL/api/v1/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")"

TOKEN="$(echo "$LOGIN_RESP" | jq -r '.token')"
USER_ID="$(echo "$LOGIN_RESP" | jq -r '.user_id')"
if [[ -z "$TOKEN" || "$TOKEN" == "null" ]]; then
  echo "login failed: $LOGIN_RESP"
  exit 1
fi

echo "2) Upload"
UPLOAD_RESP="$(curl -fsS -X POST "$GATEWAY_URL/api/v1/upload-file" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/octet-stream' \
  --data-binary "@$FILE_PATH")"
OBJECT_KEY="$(echo "$UPLOAD_RESP" | jq -r '.object_key')"
if [[ -z "$OBJECT_KEY" || "$OBJECT_KEY" == "null" ]]; then
  echo "upload failed: $UPLOAD_RESP"
  exit 1
fi

echo "3) List files"
LIST_RESP="$(curl -fsS "$GATEWAY_URL/api/v1/get-user-files?limit=10&offset=0" \
  -H "Authorization: Bearer $TOKEN")"
echo "$LIST_RESP" | jq -e --arg key "$OBJECT_KEY" '.files[] | select(.ObjectKey == $key)' >/dev/null

echo "4) Make public"
curl -fsS -X PUT "$GATEWAY_URL/api/v1/make-public-read" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"object_key\":\"$OBJECT_KEY\"}" >/dev/null

echo "5) Public download"
curl -fsS "$GATEWAY_URL/api/v1/public/download-file?owner_id=$USER_ID&object_key=$OBJECT_KEY" -o /tmp/cloudbin_public_download.bin

echo "6) Make private"
curl -fsS -X PUT "$GATEWAY_URL/api/v1/make-private-read" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"object_key\":\"$OBJECT_KEY\"}" >/dev/null

echo "Integration main flow passed"
