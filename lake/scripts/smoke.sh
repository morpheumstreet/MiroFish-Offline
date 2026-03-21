#!/usr/bin/env bash
# CI-friendly: compile + unit tests only (no server, no Neo4j).
set -euo pipefail
cd "$(dirname "$0")/.."
go vet ./...
go test ./... -count=1
go build -o /tmp/lake-smoke ./cmd/lake
echo "OK: vet, test, build"
