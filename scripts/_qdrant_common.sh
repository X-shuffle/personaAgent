#!/usr/bin/env bash

load_qdrant_env() {
  local script_dir repo_root
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  repo_root="$(cd "${script_dir}/.." && pwd)"

  if [[ -f "${repo_root}/.env" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "${repo_root}/.env"
    set +a
    return
  fi

  if [[ -f ".env" ]]; then
    echo "Warning: using .env from current directory; prefer repo root .env" >&2
    set -a
    # shellcheck disable=SC1091
    source .env
    set +a
  fi
}

require_qdrant_env() {
  QDRANT_URL="${QDRANT_URL:-}"
  QDRANT_COLLECTION="${QDRANT_COLLECTION:-}"
  QDRANT_API_KEY="${QDRANT_API_KEY:-}"

  if [[ -z "$QDRANT_URL" || -z "$QDRANT_COLLECTION" ]]; then
    echo "QDRANT_URL and QDRANT_COLLECTION are required (from env or .env)." >&2
    exit 1
  fi
}

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
