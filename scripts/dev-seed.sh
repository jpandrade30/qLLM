#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

kubectl -n qllm-dev exec deploy/postgres -- psql -U qllm -d crm -f - < "$ROOT/fixtures/seed/postgres.sql"
kubectl -n qllm-dev exec deploy/mysql -- mysql -uqllm -pqllm < "$ROOT/fixtures/seed/mysql.sql"
kubectl -n qllm-dev exec deploy/mongodb -- mongosh --quiet < "$ROOT/fixtures/seed/mongo.js"
echo "seeds applied"
