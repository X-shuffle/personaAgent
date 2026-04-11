#!/usr/bin/env bash
set -euo pipefail

# List all unique session_id values stored in Qdrant collection.
# Usage:
#   ./scripts/list_qdrant_sessions.sh
#   QDRANT_URL=... QDRANT_COLLECTION=... ./scripts/list_qdrant_sessions.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/_qdrant_common.sh"

load_qdrant_env
require_qdrant_env

TMP_SESSIONS="$(mktemp)"
trap 'rm -f "$TMP_SESSIONS"' EXIT

NEXT_OFFSET="__NONE__"
while :; do
  BODY="$(python3 - "$NEXT_OFFSET" <<'PY'
import json, sys
offset_raw = sys.argv[1]
body = {
    "limit": 256,
    "with_payload": ["session_id"],
    "with_vector": False,
}
if offset_raw != "__NONE__":
    body["offset"] = json.loads(offset_raw)
print(json.dumps(body, ensure_ascii=False))
PY
)"

  RESP="$(curl_qdrant "/collections/${QDRANT_COLLECTION}/points/scroll" "$BODY")"

  NEXT_OFFSET="$(python3 - "$RESP" "$TMP_SESSIONS" <<'PY'
import json, sys
resp = json.loads(sys.argv[1])
out_file = sys.argv[2]
res = resp.get("result", {})
points = res.get("points", []) or []
with open(out_file, "a", encoding="utf-8") as f:
    for p in points:
        sid = (p.get("payload") or {}).get("session_id")
        if sid is not None and str(sid).strip():
            f.write(str(sid).strip() + "\n")
print(json.dumps(res.get("next_page_offset", None), ensure_ascii=False))
PY
)"

  if [[ "$NEXT_OFFSET" == "null" ]]; then
    break
  fi
done

if [[ ! -s "$TMP_SESSIONS" ]]; then
  echo "(no session_id found)"
  exit 0
fi

sort -u "$TMP_SESSIONS"
