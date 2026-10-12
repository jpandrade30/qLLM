#!/usr/bin/env bash
# Run Go unit tests (pure + -tags duckdb) inside Linux via nerdctl.
# Usage:
#   ./scripts/dev/test-in-container.sh
#   ./scripts/dev/test-in-container.sh --fail-build
#   ./scripts/dev/test-in-container.sh --remount
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

FAIL_BUILD=0
REMOUNT=0
for arg in "$@"; do
  case "$arg" in
    --fail-build) FAIL_BUILD=1 ;;
    --remount) REMOUNT=1 ;;
    -h|--help)
      echo "Usage: $0 [--fail-build|--remount]"
      exit 0
      ;;
    *)
      echo "unknown arg: $arg" >&2
      exit 2
      ;;
  esac
done

if [[ "$FAIL_BUILD" -eq 1 ]]; then
  echo "nerdctl build -f Dockerfile.test --target test ."
  exec nerdctl build -f Dockerfile.test --target test .
fi

echo "nerdctl build -f Dockerfile.test -t qllm-test ."
nerdctl build -f Dockerfile.test -t qllm-test .

# -t so ANSI colors from run-go-tests.sh render in the host terminal
if [[ "$REMOUNT" -eq 1 ]]; then
  echo "nerdctl run --rm -t -v ${ROOT}:/src -w /src qllm-test"
  exec nerdctl run --rm -t -v "${ROOT}:/src" -w /src qllm-test
fi

echo "nerdctl run --rm -t qllm-test"
exec nerdctl run --rm -t qllm-test
