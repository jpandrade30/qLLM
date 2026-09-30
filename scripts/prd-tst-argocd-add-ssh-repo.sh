#!/usr/bin/env bash
# Registers an SSH git repo for Argo CD (in-cluster). Local ssh-agent is NOT visible to Argo pods.
set -euo pipefail
KEY_PATH="${1:-}"
REPO_SSH="${2:-git@github.com:jpandrade30/qLLM.git}"
if [[ -z "$KEY_PATH" ]]; then
  for c in "$HOME/.ssh/id_ed25519" "$HOME/.ssh/id_rsa"; do
    if [[ -f "$c" ]]; then KEY_PATH="$c"; break; fi
  done
fi
if [[ -z "$KEY_PATH" || ! -f "$KEY_PATH" ]]; then
  echo "Private key not found. Pass path as first arg (OpenSSH private key, not .pub)." >&2
  exit 1
fi
if grep -q PuTTY-User-Key-File "$KEY_PATH"; then
  echo "This is a PuTTY .ppk. Export an OpenSSH key first." >&2
  exit 1
fi
if ! grep -q "BEGIN .*PRIVATE KEY" "$KEY_PATH"; then
  echo "File does not look like an OpenSSH private key (do not use .pub)." >&2
  exit 1
fi

kubectl create secret generic repo-qllm-ssh \
  --namespace argocd \
  --from-literal=type=git \
  --from-literal=url="$REPO_SSH" \
  --from-file=sshPrivateKey="$KEY_PATH" \
  --dry-run=client -o yaml |
  kubectl label --local -f - argocd.argoproj.io/secret-type=repository -o yaml |
  kubectl apply -f -

kubectl -n argocd patch application qllm-prd-sim --type merge \
  -p "{\"spec\":{\"source\":{\"repoURL\":\"${REPO_SSH}\"}}}"

echo "Secret repo-qllm-ssh created. Application repoURL -> $REPO_SSH"
echo "In Argo UI: delete any broken SSH repo (the one that had no key), then Refresh the app."
