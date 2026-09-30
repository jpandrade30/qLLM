#!/usr/bin/env bash
set -euo pipefail
b64=$(kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" || true)
if [[ -z "${b64}" ]]; then
  echo "secret argocd-initial-admin-secret missing (install Argo: ./scripts/prd-tst-argocd-up.sh)" >&2
  exit 1
fi
pass=$(printf '%s' "$b64" | base64 -d 2>/dev/null || printf '%s' "$b64" | base64 -D)
echo "user:     admin"
echo "password: $pass"
