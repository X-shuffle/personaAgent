#!/usr/bin/env bash
set -euo pipefail

# Delete all vectors for a session_id from Qdrant.
# Usage:
#   ./scripts/delete_qdrant_session.sh <session_id>
#   QDRANT_URL=... QDRANT_COLLECTION=... ./scripts/delete_qdrant_session.sh <session_id>

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <session_id>" >&2
  exit 1
fi

SESSION_ID="$1"

# Auto-load .env if present (does not overwrite existing env vars)
if [[ -f ".env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
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

echo "Deleting points for session_id=${SESSION_ID} from collection=${QDRANT_COLLECTION}"
curl_qdrant "/collections/${QDRANT_COLLECTION}/points/delete?wait=true" \
  "{\"filter\":{\"must\":[{\"key\":\"session_id\",\"match\":{\"value\":\"${SESSION_ID}\"}}]}}"

echo "\nChecking remaining count..."
curl_qdrant "/collections/${QDRANT_COLLECTION}/points/count" \
  "{\"filter\":{\"must\":[{\"key\":\"session_id\",\"match\":{\"value\":\"${SESSION_ID}\"}}]},\"exact\":true}"

echo
