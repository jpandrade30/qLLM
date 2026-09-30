#!/usr/bin/env bash
# Remove the fleet-ops overlay. Stops Argo Application qllm-prd-sim if present.
# Does not uninstall Argo CD. Stop prd-tst-port-forward (Ctrl+C) first.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

command -v kubectl >/dev/null || { echo "kubectl not on PATH" >&2; exit 1; }

echo "kubectl delete -k deploy/prd-tst"
kubectl delete -k "$root/deploy/prd-tst" --ignore-not-found || true
kubectl -n argocd delete application qllm-prd-sim --ignore-not-found || true
kubectl delete ns qllm-prd --ignore-not-found --wait=false || true

echo
echo "PRD sim removed. Ctrl+C any leftover port-forward windows."
echo "Harness: nerdctl compose up --build"
