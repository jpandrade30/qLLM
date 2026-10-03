#!/usr/bin/env bash
# One window: qllm-prd (qLLM + DBs + crew API) and Argo CD UI if installed.
# Ctrl+C stops every kubectl this script started.
set -euo pipefail

command -v kubectl >/dev/null || { echo "kubectl not on PATH" >&2; exit 1; }

svc_ok() {
  kubectl get svc -n "$1" "$2" -o name >/dev/null 2>&1
}

pids=()
cleanup() {
  local p
  for p in "${pids[@]:-}"; do
    kill "$p" 2>/dev/null || true
  done
}
trap cleanup EXIT INT TERM

hints=()
start_fw() {
  local ns="$1" svc="$2"
  shift 2
  if ! svc_ok "$ns" "$svc"; then
    echo "skip ${ns}/${svc} (not in cluster)"
    if [[ "$ns" == argocd ]]; then
      echo "  Argo is not installed. HTTP :18081 will not work until:"
      echo "  ./scripts/prd-tst/prd-tst-argocd-up.sh"
    fi
    return 0
  fi
  kubectl -n "$ns" port-forward "svc/${svc}" "$@" >/dev/null &
  pids+=("$!")
  echo "started pid $!: kubectl -n ${ns} port-forward svc/${svc} $*"
}

start_fw qllm-prd qllm 18088:8088 18089:8089
hints+=("qLLM HTTP http://127.0.0.1:18088  MCP http://127.0.0.1:18089")
start_fw qllm-prd fleet-pg 15432:5432
hints+=("Postgres 127.0.0.1:15432")
start_fw qllm-prd fleet-ch 19000:9000 18123:8123
hints+=("ClickHouse native 127.0.0.1:19000  HTTP http://127.0.0.1:18123")
start_fw qllm-prd fleet-ddb 18000:8000
hints+=("DynamoDB Local http://127.0.0.1:18000")
start_fw qllm-prd fleet-api 18080:8080
hints+=("crew API http://127.0.0.1:18080")
start_fw argocd argocd-server 18081:80
hints+=("Argo CD http://127.0.0.1:18081  (needs ./scripts/prd-tst/prd-tst-argocd-up.sh --insecure)")

if [[ ${#pids[@]} -eq 0 ]]; then
  echo "nothing to forward (is qllm-prd applied? is Argo in namespace argocd?)" >&2
  exit 1
fi
echo
printf '%s\n' "${hints[@]}"
echo
echo "Ctrl+C to stop all forwards."
while true; do sleep 3600; done
