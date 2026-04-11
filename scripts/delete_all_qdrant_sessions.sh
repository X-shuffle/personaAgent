#!/usr/bin/env bash
set -euo pipefail

# Delete all session_id groups from Qdrant collection.
# Usage:
#   ./scripts/delete_all_qdrant_sessions.sh
#   QDRANT_URL=... QDRANT_COLLECTION=... ./scripts/delete_all_qdrant_sessions.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIST_SCRIPT="${SCRIPT_DIR}/list_qdrant_sessions.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/_qdrant_common.sh"

if [[ ! -x "$LIST_SCRIPT" ]]; then
  echo "Missing executable list script: $LIST_SCRIPT" >&2
  exit 1
fi

load_qdrant_env
require_qdrant_env

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
