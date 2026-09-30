#!/usr/bin/env bash
# Build qllm:local into k8s.io, apply deploy/prd-tst, wait for deploy/qllm.
# Does not start port-forward (run prd-tst-port-forward.sh after). Argo is optional.
# Usage: ./scripts/prd-tst-up.sh [--skip-compose-down] [--skip-build]
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

skip_compose=0
skip_build=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-compose-down) skip_compose=1 ;;
    --skip-build) skip_build=1 ;;
    -h|--help)
      echo "Usage: $0 [--skip-compose-down] [--skip-build]"
      exit 0
      ;;
    *)
      echo "unknown arg: $1" >&2
      exit 1
      ;;
  esac
  shift
done

command -v kubectl >/dev/null || { echo "kubectl not on PATH" >&2; exit 1; }
command -v nerdctl >/dev/null || { echo "nerdctl not on PATH" >&2; exit 1; }

if [[ "$skip_compose" -eq 0 ]]; then
  echo "nerdctl compose down -v (harness must not share this machine with fleet-ops)"
  nerdctl compose down -v
fi

if [[ "$skip_build" -eq 0 ]]; then
  echo "nerdctl --namespace k8s.io build -f Dockerfile.dev -t qllm:local ."
  nerdctl --namespace k8s.io build -f Dockerfile.dev -t qllm:local .
fi

kubectl apply -k "$root/deploy/prd-tst"
kubectl -n qllm-prd rollout restart deploy/qllm
kubectl -n qllm-prd rollout status deploy/qllm --timeout=180s

echo
echo "fleet-ops is up (namespace qllm-prd). Bearer: fleet-prd-token"
echo "  ./scripts/prd-tst-port-forward.sh"
echo "HTTP http://127.0.0.1:18088  MCP http://127.0.0.1:18089"
echo "Tear down: ./scripts/prd-tst-down.sh"
