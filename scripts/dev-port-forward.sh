#!/usr/bin/env bash
# Start port-forwards and print env exports for host-side qllm.
set -euo pipefail

kubectl -n qllm-dev port-forward svc/postgres 5432:5432 >/tmp/qllm-pf-pg.log 2>&1 &
kubectl -n qllm-dev port-forward svc/mysql 3306:3306 >/tmp/qllm-pf-mysql.log 2>&1 &
kubectl -n qllm-dev port-forward svc/mongodb 27017:27017 >/tmp/qllm-pf-mongo.log 2>&1 &
kubectl -n qllm-dev port-forward svc/test-api 18080:8080 >/tmp/qllm-pf-api.log 2>&1 &

sleep 2
cat <<'EOF'
export QLLM_CRM_PG_HOST=127.0.0.1
export QLLM_CRM_PG_USER=qllm
export QLLM_CRM_PG_PASSWORD=qllm
export QLLM_BILLING_MYSQL_HOST=127.0.0.1
export QLLM_BILLING_MYSQL_USER=qllm
export QLLM_BILLING_MYSQL_PASSWORD=qllm
export QLLM_EVENTS_MONGO_URI=mongodb://127.0.0.1:27017
export QLLM_LEGACY_API_BASE_URL=http://127.0.0.1:18080
EOF
echo "port-forwards started (pg 5432, mysql 3306, mongo 27017, api 18080)" >&2
