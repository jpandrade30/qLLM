#!/usr/bin/env bash
# Registers Application qllm-prd-sim in Argo CD. Run from repo root.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
yaml="$root/deploy/prd-tst/argocd/application.yaml"
[[ -f "$yaml" ]] || { echo "missing $yaml — run from the qLLM clone" >&2; exit 1; }
kubectl apply -f "$yaml"
branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null | tr -d '\r' || true)
if [[ -n "$branch" ]]; then
  kubectl -n argocd patch application qllm-prd-sim --type merge \
    -p "{\"spec\":{\"source\":{\"targetRevision\":\"${branch}\"}}}"
  echo "targetRevision set to branch: $branch"
fi
echo "Created Application qllm-prd-sim (namespace argocd)."
echo "Refresh the Argo UI. Push this branch if deploy/prd-tst is not on the remote yet."
echo "Private GitHub: Settings -> Repositories in Argo, then Refresh."
