#!/usr/bin/env bash
# Load fixtures/datasets/v1 into local DBs. Usage: ./scripts/dev/dev-seed-fake.sh [--regenerate] [customer_count]
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
seed_dir="$root/fixtures/seed"
venv_dir="$seed_dir/.venv"
venv_py="$venv_dir/bin/python"

if [[ ! -x "$venv_py" ]]; then
  echo "Creating seed venv ..."
  python3 -m venv "$venv_dir"
fi
echo "Using seed venv: $venv_py"
"$venv_py" -m pip install --upgrade pip
"$venv_py" -m pip install -r "$seed_dir/requirements.txt"

export QLLM_CRM_PG_HOST="${QLLM_CRM_PG_HOST:-127.0.0.1}"
export QLLM_CRM_PG_USER="${QLLM_CRM_PG_USER:-qllm}"
export QLLM_CRM_PG_PASSWORD="${QLLM_CRM_PG_PASSWORD:-qllm}"
export QLLM_BILLING_MYSQL_HOST="${QLLM_BILLING_MYSQL_HOST:-127.0.0.1}"
export QLLM_BILLING_MYSQL_USER="${QLLM_BILLING_MYSQL_USER:-qllm}"
export QLLM_BILLING_MYSQL_PASSWORD="${QLLM_BILLING_MYSQL_PASSWORD:-qllm}"
export QLLM_EVENTS_MONGO_URI="${QLLM_EVENTS_MONGO_URI:-mongodb://127.0.0.1:27017}"

regenerate=0
customers=40
for a in "$@"; do
  if [[ "$a" == "--regenerate" ]]; then regenerate=1
  elif [[ "$a" =~ ^[0-9]+$ ]]; then customers="$a"
  fi
done
if [[ "$regenerate" -eq 1 ]]; then
  "$venv_py" "$seed_dir/generate_dataset.py" --customers "$customers" --seed 42
fi
"$venv_py" "$seed_dir/load_dataset.py"
echo "dataset loaded from fixtures/datasets/v1 (rebuild/restart compose test-api if data.json changed)"
