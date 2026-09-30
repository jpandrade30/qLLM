#!/usr/bin/env bash
# Enable CGO for -tags duckdb on Linux/macOS. Source it:  source ./scripts/dev-shell.sh
# Windows gcc/duckdblib: use scripts/dev-shell.ps1
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
if [[ ! -f "$root/go.mod" ]]; then
  echo "go.mod not found next to scripts/" >&2
  return 1 2>/dev/null || exit 1
fi
export CGO_ENABLED=1
export QLLM_CRM_PG_HOST="${QLLM_CRM_PG_HOST:-127.0.0.1}"
export QLLM_CRM_PG_USER="${QLLM_CRM_PG_USER:-qllm}"
export QLLM_CRM_PG_PASSWORD="${QLLM_CRM_PG_PASSWORD:-qllm}"
export QLLM_BILLING_MYSQL_HOST="${QLLM_BILLING_MYSQL_HOST:-127.0.0.1}"
export QLLM_BILLING_MYSQL_USER="${QLLM_BILLING_MYSQL_USER:-qllm}"
export QLLM_BILLING_MYSQL_PASSWORD="${QLLM_BILLING_MYSQL_PASSWORD:-qllm}"
export QLLM_EVENTS_MONGO_URI="${QLLM_EVENTS_MONGO_URI:-mongodb://127.0.0.1:27017}"
echo "CGO_ENABLED=1  repo=$root"
echo "Build: go build -tags duckdb -o qllm ./cmd/qllm"
