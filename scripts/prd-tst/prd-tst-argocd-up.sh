#!/usr/bin/env bash
# Installs Argo CD with --insecure so the UI is plain HTTP on Service port 80.
set -euo pipefail

command -v kubectl >/dev/null || { echo "kubectl not on PATH" >&2; exit 1; }

kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
echo "waiting for argocd-server..."
kubectl -n argocd rollout status deploy/argocd-server --timeout=300s

kubectl -n argocd get deploy argocd-server -o json | python3 -c '
import json, sys
d = json.load(sys.stdin)
c = d["spec"]["template"]["spec"]["containers"][0]
a = list(c.get("args") or [])
if "--insecure" not in a:
    a.append("--insecure")
print(json.dumps({"spec": {"template": {"spec": {"containers": [{"name": c["name"], "args": a}]}}}}))
' > /tmp/argocd-insecure.json
kubectl -n argocd patch deploy argocd-server --type strategic --patch-file /tmp/argocd-insecure.json
rm -f /tmp/argocd-insecure.json
kubectl -n argocd rollout status deploy/argocd-server --timeout=180s

b64=$(kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}")
pass=$(printf '%s' "$b64" | base64 -d 2>/dev/null || printf '%s' "$b64" | base64 -D)
echo
echo "Argo CD user: admin"
echo "password:     $pass"
echo "Stop port-forward (Ctrl+C) if it is running, then:"
echo "  ./scripts/prd-tst/prd-tst-port-forward.sh"
echo "Open: http://127.0.0.1:18081"
