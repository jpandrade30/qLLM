# 07 — SQL dialect (Databricks-like inventory)

Query IR `protocolVersion` **0.1.0** shape does not change (document **0.2.0** adds source types). Rich expressions live on **`POST /v1/sql` / `execute_sql`**, executed in DuckDB after catalog-table fetch (D15). Databricks SQL is a **naming inventory**, not a clone (no Unity Catalog, no Spark `LATERAL VIEW`, no `MERGE`).

`test` values: `parse+exec` | `reject` | `skip-reason`. Status `sql-1` / `sql-2` requires `parse+exec`. `never` requires `reject`.

Omitted `version` = latest dialect **`"2"`**. `"1"` remains valid and frozen (no set ops, no `QUALIFY`).

## Product never (not a feature backlog)

| Item | test |
|------|------|
| Agent API is Query IR + catalog SQL only; GraphQL is never a qLLM API | skip-reason: HTTP/MCP contract (D17) |
| Agent API is not Mongo / raw REST URLs | skip-reason: HTTP/MCP contract |
| SQL only via `execute_sql` / `POST /v1/sql`; tables = catalog entity names | reject: `public.customers` |
| Do not invent entities/fields/join keys | skip-reason: catalog validation |
| Query IR `where` is `{op,args}` not `{and:[…]}` | reject: llmlint |
| Query IR compare ops are `eq`/`contains`/… not `=`/`LIKE` | reject: llmlint |
| Catalog field spelling (no invented camelCase) | skip-reason: catalog |
| Query IR: bare field + agg without `groupBy` | reject: llmlint |
| Aliases only from `as` / entity name | skip-reason: IR validate |

## Security never (`reject`)

| Construct | test |
|-----------|------|
| `INSERT` `UPDATE` `DELETE` `MERGE` `REPLACE` | reject |
| `CREATE` `DROP` `ALTER` `TRUNCATE` `COPY` `ATTACH` `DETACH` | reject |
| `INSTALL` `LOAD` `PRAGMA` `SET` `CALL` `GRANT` | reject |
| `read_csv` `read_parquet` `read_json` `postgres_scan` `httpfs` `glob` `read_text` `read_blob` | reject |
| Multi-statement (`;`) | reject |
| Schema-qualified tables (`public.x`) | reject |
| Arbitrary table functions | reject |

## OFFSET / LIMIT

| Item | status | test |
|------|--------|------|
| `LIMIT` required or injected (`defaultLimit`) | sql-1 | parse+exec |
| `OFFSET` with `LIMIT` | sql-1 | parse+exec |
| `OFFSET` without `LIMIT` | inject `LIMIT defaultLimit`; not unbounded | parse+exec (inject) |
| Mutations | never | reject |

## Dialect `"1"` (`parse+exec`)

| Family | Examples | status | test |
|--------|----------|--------|------|
| Filter | `WHERE`, `AND`/`OR`/`NOT`, `IN`, `IS NULL` | sql-1 | parse+exec |
| HAVING | `HAVING COUNT(*) > 1` | sql-1 | parse+exec |
| DISTINCT | `SELECT DISTINCT status` | sql-1 | parse+exec |
| CASE | `CASE WHEN … THEN … END` | sql-1 | parse+exec |
| LIKE / ILIKE | `status LIKE 'p%'` | sql-1 | parse+exec |
| BETWEEN | `total BETWEEN 1 AND 9` | sql-1 | parse+exec |
| CTE | `WITH t AS (SELECT …) SELECT … FROM t` | sql-1 | parse+exec |
| Subquery FROM | `FROM (SELECT …) s` | sql-1 | parse+exec |
| Arithmetic | `total + 1`, `total * 2` | sql-1 | parse+exec |
| Joins | `INNER`/`LEFT` (+ `FULL`/`CROSS` if DuckDB accepts) | sql-1 | parse+exec |
| Agg basic | `COUNT` `SUM` `AVG` `MIN` `MAX` | sql-1 | parse+exec |
| Conditional | `COALESCE` `NULLIF` `IF`/`IFF` | sql-1 | parse+exec |
| String | `CONCAT` `\|\|` `LOWER` `UPPER` `TRIM` `SUBSTRING` `REPLACE` | sql-1 | parse+exec |
| Numeric | `ABS` `CEIL` `FLOOR` `ROUND` `POWER` `SQRT` `LN` `MOD` | sql-1 | parse+exec |
| Cast | `CAST` `TRY_CAST` | sql-1 | parse+exec |
| JSON (DuckDB names) | `json_extract`, `->`, `->>` | sql-1 / duckdb-name-diff | parse+exec |
| Array (DuckDB names) | `list_contains`, `list_value`, `unnest` in SELECT | sql-1 / duckdb-name-diff | parse+exec |
| Datetime (DuckDB names) | `date_trunc`, `EXTRACT(YEAR FROM ts)`, `CURRENT_DATE` | sql-1 / duckdb-name-diff | parse+exec |
| Extra agg | `COUNT(*) FILTER`, `bool_or`/`bool_and`, `stddev`, `array_agg` | sql-1 | parse+exec |

Databricks aliases (`GET_JSON_OBJECT`, `DATEADD`, `NVL`, `COLLECT_LIST`, `RLIKE`) are **duckdb-name-diff**: agents should use DuckDB spelling. Not rejected if DuckDB implements them.

## Dialect `"2"` (`parse+exec`; `"1"` `reject` for set ops / QUALIFY)

| Family | Examples | status | test |
|--------|----------|--------|------|
| Set ops | `UNION` `UNION ALL` `INTERSECT` `EXCEPT` | sql-2 | parse+exec |
| QUALIFY | `QUALIFY ROW_NUMBER() OVER (…) = 1`; SELECT-list aliases (`QUALIFY posicao <= 10`) OK in DuckDB | sql-2 | parse+exec |
| COUNT DISTINCT | `COUNT(DISTINCT status)` | sql-2 (also parsed in 1) | parse+exec |
| XOR | boolean `XOR` | sql-2 | parse+exec |
| Window | `ROW_NUMBER` `RANK` `DENSE_RANK` `LAG` `LEAD` `NTILE` `SUM() OVER` | sql-2 | parse+exec |

## skip-reason (out of scope)

| Item | skip-reason |
|------|-------------|
| Spark `PIVOT`/`UNPIVOT`, `LATERAL VIEW EXPLODE` | not DuckDB/Spark SQL |
| `IDENTIFIER()`, Unity Catalog, `STREAM`, `CACHE` | Databricks platform |
| `ai_*` functions | not in runtime |
| Delta `DESCRIBE HISTORY` | not a query runtime feature |
| Query IR having/union/case/like/xor | IR stays 0.1.0; use SQL |

## Execution rules (unchanged)

- Tables in SQL = catalog **entity names** (logical), not `schema.table`.
- CTE names and their `FROM` aliases (`WITH revenue AS (…) SELECT … FROM revenue r`) are **not** catalog entities. Fetch uses tables cited **inside** the CTE (and any real `JOIN`s). Computed CTE columns (`SUM(…) AS paid_total`) are not catalog fields.
- `QUALIFY` and `LIMIT` may appear on the same statement (one `LIMIT`; `QUALIFY` is a window filter, not a second `LIMIT`).
- `EXTRACT(YEAR FROM col)` is DuckDB datetime spelling (`FROM` inside the function is not the query `FROM`).
- Fetch cited columns; no WHERE pushdown on the SQL path.
- DuckDB `SET enable_external_access=false`.
- Build `-tags duckdb` required for `ExecSQL`.
