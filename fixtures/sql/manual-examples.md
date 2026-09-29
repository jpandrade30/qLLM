# Manual SQL examples (execute_sql)

Use with MCP tool **`execute_sql`** or `POST /v1/sql` (Bearer `change-me` in the image stack).

Tables and fields are **catalog entity names** from [`deploy/image/config/qllm.catalog.yaml`](../../deploy/image/config/qllm.catalog.yaml) — not `billing.invoices`.

Omit `version` (latest = `"2"`). Dialeto `"1"` rejects `UNION` / `QUALIFY`.

**Prereq:** compose up + seed (`load_dataset.py` or `dev-seed-fake.ps1`). Rebuild qllm if you just pulled dialect-2 code.

```bash
# HTTP smoke
curl.exe -s -H "Authorization: Bearer change-me" -H "Content-Type: application/json" ^
  --data-binary "@fixtures/sql/body.json" http://127.0.0.1:8088/v1/sql
```

Body shape:

```json
{ "sql": "SELECT id, status FROM invoices LIMIT 20" }
```

---

## 1. Basics

```sql
SELECT id, total, status FROM invoices LIMIT 20
```

```sql
SELECT id, email, country FROM customers LIMIT 20
```

```sql
SELECT id, total, status FROM invoices ORDER BY total DESC LIMIT 10
```

```sql
SELECT id, status FROM invoices LIMIT 10 OFFSET 5
```

---

## 2. Filters

```sql
SELECT id, total, status FROM invoices WHERE status = 'paid' LIMIT 50
```

```sql
SELECT id, total FROM invoices WHERE status IN ('paid', 'open') LIMIT 50
```

```sql
SELECT id, total FROM invoices WHERE total BETWEEN 10000 AND 200000 LIMIT 50
```

```sql
SELECT id, status FROM invoices WHERE status LIKE 'p%' LIMIT 50
```

```sql
SELECT id, email FROM customers WHERE email ILIKE '%@%' LIMIT 50
```

```sql
SELECT id, total, status FROM invoices
WHERE status = 'paid' AND total >= 50000
LIMIT 50
```

```sql
SELECT id, country FROM customers WHERE country IS NOT NULL LIMIT 50
```

---

## 3. Aggregations

```sql
SELECT COUNT(*) AS n FROM invoices LIMIT 1
```

```sql
SELECT COUNT(DISTINCT status) AS statuses FROM invoices LIMIT 1
```

```sql
SELECT
  status,
  COUNT(*) AS n,
  SUM(total) AS sum_total,
  AVG(total) AS avg_total,
  MIN(total) AS min_total,
  MAX(total) AS max_total
FROM invoices
GROUP BY status
LIMIT 20
```

```sql
SELECT status, COUNT(*) AS n
FROM invoices
GROUP BY status
HAVING COUNT(*) > 5
LIMIT 20
```

```sql
SELECT COUNT(*) FILTER (WHERE status = 'paid') AS paid_n
FROM invoices
LIMIT 1
```

```sql
SELECT
  status,
  stddev_samp(total) AS sd,
  array_agg(id) AS ids
FROM invoices
GROUP BY status
LIMIT 20
```

---

## 4. CASE / conditional

```sql
SELECT
  id,
  status,
  CASE
    WHEN status = 'paid' THEN 'ok'
    WHEN status = 'open' THEN 'todo'
    ELSE 'other'
  END AS bucket
FROM invoices
LIMIT 20
```

```sql
SELECT id, COALESCE(NULLIF(status, ''), 'unknown') AS status_safe
FROM invoices
LIMIT 20
```

```sql
SELECT id, IFF(total >= 100000, 'high', 'low') AS band
FROM invoices
LIMIT 20
```

---

## 5. String

```sql
SELECT
  id,
  LOWER(status) AS lo,
  UPPER(status) AS up,
  CONCAT(status, ':', id) AS label
FROM invoices
LIMIT 20
```

```sql
SELECT id, email, LENGTH(email) AS elen
FROM customers
LIMIT 20
```

```sql
SELECT id, REPLACE(status, 'paid', 'PAID') AS status2
FROM invoices
LIMIT 20
```

```sql
SELECT id, SUBSTRING(email, 1, 5) AS prefix
FROM customers
LIMIT 20
```

---

## 6. Numeric / arithmetic

```sql
SELECT
  id,
  total,
  total + 1 AS bumped,
  ROUND(total / 100.0, 2) AS dollars,
  ABS(total - 50000) AS dist
FROM invoices
LIMIT 20
```

```sql
SELECT id, CEIL(total / 1000.0) AS k_ceil, FLOOR(total / 1000.0) AS k_floor
FROM invoices
LIMIT 20
```

```sql
SELECT id, POWER(2, 3) AS eight, SQRT(total) AS root
FROM invoices
LIMIT 5
```

```sql
SELECT CAST(total AS DOUBLE) AS t_f, TRY_CAST(id AS INTEGER) AS id_maybe
FROM invoices
LIMIT 10
```

---

## 7. Joins (cross-source OK)

```sql
SELECT c.id, c.email, i.total, i.status
FROM customers c
INNER JOIN invoices i ON i.customer_id = c.id
LIMIT 50
```

```sql
SELECT c.id, c.email, a.city, a.country
FROM customers c
LEFT JOIN addresses a ON a.customer_id = c.id
LIMIT 50
```

```sql
SELECT i.id, i.total, p.sku, ii.qty
FROM invoices i
INNER JOIN invoice_items ii ON ii.invoice_id = i.id
INNER JOIN products p ON p.id = ii.product_id
LIMIT 50
```

---

## 8. CTE / subquery

```sql
WITH paid AS (
  SELECT id, customer_id, total FROM invoices WHERE status = 'paid' LIMIT 500
)
SELECT customer_id, SUM(total) AS spent
FROM paid
GROUP BY customer_id
ORDER BY spent DESC
LIMIT 20
```

```sql
SELECT s.customer_id, s.n
FROM (
  SELECT customer_id, COUNT(*) AS n
  FROM invoices
  GROUP BY customer_id
) s
WHERE s.n >= 2
LIMIT 20
```

---

## 9. DISTINCT

```sql
SELECT DISTINCT status FROM invoices LIMIT 20
```

```sql
SELECT DISTINCT country FROM customers LIMIT 50
```

---

## 10. Windows (dialect 2)

```sql
SELECT
  id,
  status,
  total,
  ROW_NUMBER() OVER (PARTITION BY status ORDER BY total DESC) AS rn
FROM invoices
LIMIT 50
```

```sql
SELECT
  id,
  status,
  total,
  RANK() OVER (ORDER BY total DESC) AS rnk,
  LAG(total) OVER (ORDER BY total) AS prev_total
FROM invoices
LIMIT 50
```

```sql
SELECT status, total
FROM invoices
QUALIFY ROW_NUMBER() OVER (PARTITION BY status ORDER BY total DESC) = 1
LIMIT 20
```

---

## 11. Set ops (dialect 2)

```sql
SELECT id FROM invoices LIMIT 20
UNION ALL
SELECT id FROM customers LIMIT 20
```

```sql
SELECT customer_id AS id FROM invoices LIMIT 100
INTERSECT
SELECT id FROM customers LIMIT 100
```

```sql
SELECT id FROM customers LIMIT 100
EXCEPT
SELECT customer_id FROM invoices LIMIT 100
```

---

## 12. Datetime (DuckDB names)

```sql
SELECT id, issued_at, date_trunc('month', issued_at::TIMESTAMP) AS month
FROM invoices
LIMIT 20
```

```sql
SELECT id, created_at, CURRENT_DATE AS today
FROM customers
LIMIT 20
```

```sql
SELECT id, EXTRACT(year FROM issued_at::TIMESTAMP) AS y
FROM invoices
LIMIT 20
```

---

## 13. Boolean / XOR (dialect 2)

```sql
SELECT id, (status = 'paid') XOR (total > 100000) AS flag
FROM invoices
LIMIT 20
```

---

## 14. Expect reject (security / ACL shape)

These **must** fail with `INVALID_SQL` (or similar):

```sql
INSERT INTO invoices VALUES ('x')
```

```sql
SELECT * FROM public.customers LIMIT 10
```

```sql
SELECT * FROM read_csv('x.csv')
```

```sql
SELECT 1; SELECT 2
```

Dialeto `"1"` only — set `version: "1"` and expect reject:

```sql
SELECT id FROM invoices LIMIT 5 UNION SELECT id FROM customers LIMIT 5
```

```sql
SELECT status FROM invoices QUALIFY COUNT(*) OVER () > 0 LIMIT 5
```

---

## Catalog cheat sheet

| Entity | Useful fields |
|--------|----------------|
| `customers` | `id`, `email`, `full_name`, `country`, `created_at` |
| `addresses` | `customer_id`, `city`, `country` |
| `invoices` | `id`, `customer_id`, `total`, `status`, `issued_at` |
| `invoice_items` | `invoice_id`, `product_id`, `qty`, `unit` |
| `products` | `id`, `sku`, `name`, `price`, `category` |
| `payments` | `invoice_id`, `customer_id`, `amount`, `method` |
| `subscriptions` | `customer_id`, `plan`, `status`, `mrr` |
| `support_tickets` | `customer_id`, `subject`, `status`, `priority` |

Normative inventory: [`planning/07-sql-dialect.md`](../planning/07-sql-dialect.md).
