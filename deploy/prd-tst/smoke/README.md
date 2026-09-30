# Isolation checks (not pytest; do not add to fixtures/sqlcheck)

After port-forward:

```bash
# must succeed — fleet domain
curl -s -H "Authorization: Bearer fleet-prd-token" \
  -H "Content-Type: application/json" \
  -d "{\"sql\":\"SELECT vin FROM vehicles LIMIT 5\"}" \
  http://127.0.0.1:18088/v1/sql

# must be UNKNOWN_ENTITY — harness names
curl -s -H "Authorization: Bearer fleet-prd-token" \
  -H "Content-Type: application/json" \
  -d "{\"sql\":\"SELECT id FROM customers LIMIT 1\"}" \
  http://127.0.0.1:18088/v1/sql

# dynamo without pk — UNSUPPORTED
curl -s -H "Authorization: Bearer fleet-prd-token" \
  -H "Content-Type: application/json" \
  -d "{\"sql\":\"SELECT sku FROM stock_items LIMIT 1\"}" \
  http://127.0.0.1:18088/v1/sql
```
