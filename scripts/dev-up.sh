#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
kubectl apply -k "$ROOT/deploy/dev"
kubectl -n qllm-dev rollout status deploy/postgres --timeout=180s
kubectl -n qllm-dev rollout status deploy/mysql --timeout=300s
kubectl -n qllm-dev rollout status deploy/mongodb --timeout=180s
kubectl -n qllm-dev rollout status deploy/test-api --timeout=300s
echo "dev harness applied"
