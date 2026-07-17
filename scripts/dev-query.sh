#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IR="${1:?usage: dev-query.sh path/to/ir.json}"
cd "$ROOT"
go run ./cmd/qllm query --config-dir "$ROOT/fixtures/presets" -f "$IR"
