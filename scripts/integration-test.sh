#!/usr/bin/env bash
set -euo pipefail

export POSTGRES_PORT="${TEST_POSTGRES_PORT:-55432}"
docker compose -f deploy/compose.yaml down --remove-orphans >/dev/null 2>&1 || true
cleanup() { docker compose -f deploy/compose.yaml down --remove-orphans >/dev/null 2>&1 || true; }
trap cleanup EXIT
docker compose -f deploy/compose.yaml up -d postgres
for _ in $(seq 1 30); do
  docker compose -f deploy/compose.yaml exec -T postgres pg_isready -U mlaiops && break
  sleep 1
done

export TEST_DATABASE_URL="postgres://mlaiops:mlaiops-local@localhost:${POSTGRES_PORT}/mlaiops?sslmode=disable"
(cd go && go test -buildvcs=false -tags=integration ./integration)
