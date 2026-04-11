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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/_qdrant_common.sh"

load_qdrant_env
require_qdrant_env

echo "Deleting points for session_id=${SESSION_ID} from collection=${QDRANT_COLLECTION}"
curl_qdrant "/collections/${QDRANT_COLLECTION}/points/delete?wait=true" \
  "{\"filter\":{\"must\":[{\"key\":\"session_id\",\"match\":{\"value\":\"${SESSION_ID}\"}}]}}"

echo "\nChecking remaining count..."
curl_qdrant "/collections/${QDRANT_COLLECTION}/points/count" \
  "{\"filter\":{\"must\":[{\"key\":\"session_id\",\"match\":{\"value\":\"${SESSION_ID}\"}}]},\"exact\":true}"

echo
