#!/usr/bin/env bash
# Colored, verbose Go unit suite (same packages as CI / Dockerfile.test).
# Env: QLLM_TEST_COLOR=0 to disable ANSI; QLLM_TEST_V=0 for quiet (no -v).
set -euo pipefail

COLOR="${QLLM_TEST_COLOR:-1}"
VERBOSE="${QLLM_TEST_V:-1}"
VFLAG=()
if [[ "$VERBOSE" == "1" ]]; then
  VFLAG=(-v)
fi

c_reset=$'\033[0m'
c_green=$'\033[32m'
c_red=$'\033[31m'
c_yellow=$'\033[33m'
c_cyan=$'\033[36m'
c_dim=$'\033[2m'

paint() {
  local color="$1"
  shift
  if [[ "$COLOR" == "1" ]]; then
    printf '%s%s%s\n' "$color" "$*" "$c_reset"
  else
    printf '%s\n' "$*"
  fi
}

header() {
  paint "$c_cyan" "══ $* ══"
}

# Only color go test status lines. Do NOT match substrings like "Error" in
# test names (TestFooError) or t.Log output — those are false positives.
colorize_line() {
  local line="$1"
  local trimmed="${line#"${line%%[![:space:]]*}"}" # strip leading whitespace
  if [[ "$trimmed" == ---[[:space:]]PASS:* ]] || [[ "$trimmed" == ok[[:space:]]* ]] || [[ "$trimmed" == PASS ]]; then
    paint "$c_green" "$line"
  elif [[ "$trimmed" == ---[[:space:]]FAIL:* ]] || [[ "$trimmed" == FAIL ]] || [[ "$trimmed" == FAIL[[:space:]]* ]] || [[ "$trimmed" == \#\ * ]]; then
    paint "$c_red" "$line"
  elif [[ "$trimmed" == ---[[:space:]]SKIP:* ]] || [[ "$trimmed" == \?[[:space:]]* ]]; then
    paint "$c_yellow" "$line"
  else
    printf '%s\n' "$line"
  fi
}

run_go_test() {
  local label="$1"
  shift
  header "$label"
  set +e
  # Line-buffer so colors appear as tests finish (when stdbuf exists).
  if command -v stdbuf >/dev/null 2>&1; then
    stdbuf -oL -eL go test "${VFLAG[@]}" -count=1 "$@" 2>&1 | while IFS= read -r line || [[ -n "$line" ]]; do
      colorize_line "$line"
    done
  else
    go test "${VFLAG[@]}" -count=1 "$@" 2>&1 | while IFS= read -r line || [[ -n "$line" ]]; do
      colorize_line "$line"
    done
  fi
  local st="${PIPESTATUS[0]}"
  set -e
  if [[ "$st" -ne 0 ]]; then
    paint "$c_red" "✗ $label failed (exit $st)"
    return "$st"
  fi
  paint "$c_green" "✓ $label passed"
  return 0
}

header "go vet ./..."
if ! go vet ./...; then
  paint "$c_red" "✗ go vet failed"
  exit 1
fi
paint "$c_green" "✓ go vet passed"
paint "$c_dim" ""

run_go_test "go test ./... (pure Go)" ./...
run_go_test "go test -tags duckdb (duckdblocal + executor)" -tags duckdb ./internal/duckdblocal/... ./internal/executor/...

paint "$c_green" "══ all unit tests passed ══"
