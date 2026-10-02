#!/usr/bin/env bash
# Create a slim qLLM project folder. Usage: ./scripts/init-standalone.sh --user Alice [--out DIR] [--force]
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
if command -v python3 >/dev/null 2>&1; then
  exec python3 "$root/scripts/init-standalone.py" "$@"
fi
if command -v python >/dev/null 2>&1; then
  exec python "$root/scripts/init-standalone.py" "$@"
fi
echo "python3 or python is required" >&2
exit 1
