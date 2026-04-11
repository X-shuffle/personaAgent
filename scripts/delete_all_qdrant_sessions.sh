#!/usr/bin/env bash
set -euo pipefail

# Delete all session_id groups from Qdrant collection.
# Usage:
#   ./scripts/delete_all_qdrant_sessions.sh
#   QDRANT_URL=... QDRANT_COLLECTION=... ./scripts/delete_all_qdrant_sessions.sh

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIST_SCRIPT="${ROOT_DIR}/list_qdrant_sessions.sh"

if [[ ! -x "$LIST_SCRIPT" ]]; then
  echo "Missing executable list script: $LIST_SCRIPT" >&2
  exit 1
fi

if [[ -f "${ROOT_DIR}/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${ROOT_DIR}/.env"
  set +a
fi

QDRANT_URL="${QDRANT_URL:-}"
QDRANT_COLLECTION="${QDRANT_COLLECTION:-}"
QDRANT_API_KEY="${QDRANT_API_KEY:-}"

if [[ -z "$QDRANT_URL" || -z "$QDRANT_COLLECTION" ]]; then
  echo "QDRANT_URL and QDRANT_COLLECTION are required (from env or .env)." >&2
  exit 1
fi

curl_qdrant() {
  local endpoint="$1"
  local body="$2"
  if [[ -n "$QDRANT_API_KEY" ]]; then
    curl -sS -X POST "${QDRANT_URL%/}${endpoint}" \
      -H "Content-Type: application/json" \
      -H "api-key: ${QDRANT_API_KEY}" \
      -d "$body"
  else
    curl -sS -X POST "${QDRANT_URL%/}${endpoint}" \
      -H "Content-Type: application/json" \
      -d "$body"
  fi
}

TMP_SESSIONS="$(mktemp)"
trap 'rm -f "$TMP_SESSIONS"' EXIT

"$LIST_SCRIPT" > "$TMP_SESSIONS"

if [[ ! -s "$TMP_SESSIONS" ]]; then
  echo "No session_id found. Nothing to delete."
  exit 0
fi

DELETED=0
while IFS= read -r SESSION_ID; do
  [[ -z "$SESSION_ID" ]] && continue

  echo "Deleting session_id=${SESSION_ID}"
  curl_qdrant "/collections/${QDRANT_COLLECTION}/points/delete?wait=true" \
    "{\"filter\":{\"must\":[{\"key\":\"session_id\",\"match\":{\"value\":\"${SESSION_ID}\"}}]}}" > /dev/null

  DELETED=$((DELETED + 1))
done < "$TMP_SESSIONS"

echo "Deleted ${DELETED} session_id group(s)."
