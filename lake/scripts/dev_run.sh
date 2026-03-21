#!/usr/bin/env bash
# Run Lake locally: tests, build, then HTTP server.
# Usage (from repo root or lake/):
#   bash lake/scripts/dev_run.sh
# Optional: load secrets from repo-root .env (gitignored):
#   set -a && [ -f ../.env ] && . ../.env; set +a
#   bash lake/scripts/dev_run.sh

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "== go test ./... =="
go test ./... -count=1

echo "== go build =="
go build -o "${LAKE_BIN:-/tmp/lake-dev}" ./cmd/lake

if [[ -f ../.env ]]; then
  echo "== sourcing ../.env (non-fatal if missing keys) =="
  set -a
  # shellcheck disable=SC1091
  source ../.env
  set +a
fi

: "${LLM_API_KEY:?Set LLM_API_KEY in environment or ../.env}"
: "${NEO4J_URI:?Set NEO4J_URI}"
: "${NEO4J_PASSWORD:?Set NEO4J_PASSWORD}"

echo "== starting Lake on ${LAKE_HTTP_HOST:-0.0.0.0}:${LAKE_HTTP_PORT:-5001} =="
echo "    (first request may be slow: Neo4j schema step can take up to ~90s if Bolt is slow/unreachable)"
echo "    health: http://${LAKE_HTTP_HOST:-127.0.0.1}:${LAKE_HTTP_PORT:-5001}/health"
exec "${LAKE_BIN:-/tmp/lake-dev}"
