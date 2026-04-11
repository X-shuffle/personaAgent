#!/usr/bin/env bash
set -euo pipefail

# List all unique session_id values stored in Qdrant collection.
# Usage:
#   ./list_qdrant_sessions.sh
#   QDRANT_URL=... QDRANT_COLLECTION=... ./list_qdrant_sessions.sh

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
