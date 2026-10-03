#!/usr/bin/env bash
# Run SQL goldens from the repo root against the live qLLM MCP (compose).
# Usage: ./scripts/dev/check-live.sh [--oracle] [--filter rest_json]
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
py="$root/fixtures/seed/.venv/bin/python"
if [[ ! -x "$py" ]]; then
  py="$root/fixtures/seed/.venv/Scripts/python.exe"
fi
if [[ ! -x "$py" && ! -f "$py" ]]; then
  echo "seed venv missing — run ./scripts/dev/dev-seed-fake.sh first" >&2
  exit 1
fi
cmd=mcp
filt=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --oracle) cmd=check-oracle ;;
    --filter) filt="${2:-}"; shift ;;
    *) echo "unknown arg: $1" >&2; exit 1 ;;
  esac
  shift
done
args=(-m sqlcheck "$cmd")
if [[ -n "$filt" ]]; then
  args+=(--filter "$filt")
fi
cd "$root/fixtures"
exec "$py" "${args[@]}"
