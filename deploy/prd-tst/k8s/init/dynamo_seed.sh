#!/bin/sh
set -e
EP="${QLLM_FLEET_DDB_ENDPOINT:-http://fleet-ddb:8000}"
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-local}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-local}"
export AWS_DEFAULT_REGION=us-east-1
i=0
until aws dynamodb list-tables --endpoint-url "$EP" >/dev/null 2>&1; do
  i=$((i + 1))
  if [ "$i" -gt 60 ]; then
    echo "dynamodb not ready"
    exit 1
  fi
  sleep 2
done
aws dynamodb delete-table --endpoint-url "$EP" --table-name stock_items >/dev/null 2>&1 || true
sleep 1
aws dynamodb create-table --endpoint-url "$EP" \
  --table-name stock_items \
  --attribute-definitions AttributeName=pk,AttributeType=S AttributeName=sk,AttributeType=S \
  --key-schema AttributeName=pk,KeyType=HASH AttributeName=sk,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST
aws dynamodb put-item --endpoint-url "$EP" --table-name stock_items \
  --item '{"pk":{"S":"WH-N"},"sk":{"S":"FILTER-01"},"sku":{"S":"oil-filter"},"qty":{"N":"4"}}'
echo "dynamodb seed ok"
