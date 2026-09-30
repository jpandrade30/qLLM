# Queries: catalog SQL and Query IR

Two paths. MCP has **only** SQL. HTTP has both.

Schemas: [`sql-request.schema.json`](../../planning/schemas/sql-request.schema.json), [`query-ir.schema.json`](../../planning/schemas/query-ir.schema.json), [`query-response.schema.json`](../../planning/schemas/query-response.schema.json). Dialect: [`planning/07-sql-dialect.md`](../../planning/07-sql-dialect.md).

## Catalog SQL (`execute_sql` / `POST /v1/sql` / `qllm sql`)

Body: `{ "sql": "SELECT …", "version": "1"|"2" }`. An omitted `version` means **`"2"`**. An unknown value returns `UNSUPPORTED_VERSION`.

### Accepted (summary)

- A single `SELECT` statement (optionally with `WITH`).
- Tables are **entity names** (plus CTE aliases, which are not entities).
- `LIMIT` is required or injected (`defaultLimit`). A lone `OFFSET` also injects a limit.
- Dialect `"1"`: `WHERE`, `HAVING`, `DISTINCT`, `CASE`, `LIKE`, `ILIKE`, `BETWEEN`, `IN`, `IS NULL`, CTEs, `FROM` subqueries, joins, basic aggregations, string/number/cast functions, and JSON/array/datetime functions **using DuckDB names**.
- Dialect `"2"`: everything in `"1"` plus `UNION` / `UNION ALL` / `INTERSECT` / `EXCEPT`, `QUALIFY`, window functions (`ROW_NUMBER`, …), and `XOR`.

Databricks functions with a different name (`GET_JSON_OBJECT`, `DATEADD`, …) are not rejected if DuckDB has them; the guide asks for DuckDB spelling.

### Refused (`INVALID_SQL` or equivalent)

- `INSERT` `UPDATE` `DELETE` `MERGE` `REPLACE`
- `CREATE` `DROP` `ALTER` `TRUNCATE` `COPY` `ATTACH` `DETACH`
- `INSTALL` `LOAD` `PRAGMA` `SET` `CALL` `GRANT`
- `read_csv` `read_parquet` `read_json` `postgres_scan` `httpfs` `glob` `read_text` `read_blob`
- Multiple statements (`;`)
- `schema.table` (`public.customers`)
- Arbitrary table functions
- `LIMIT` greater than `maxLimit` returns `LIMIT_EXCEEDED`
- A missing entity or field returns `UNKNOWN_ENTITY` / `UNKNOWN_FIELD`
- An entity outside the ACL returns `FORBIDDEN`

### Execution (important if you model the planner in your head)

1. Parse, then validate names and ACL.
2. **Fetch** the referenced tables (only the columns needed). There is **no** `WHERE` pushdown on this path.
3. DuckDB runs the `SELECT` (`enable_external_access=false`).
4. A build **without** `-tags duckdb` cannot run this path.

For KV and stream sources, `WHERE` must include an equality on the `accessPath` or the fetch fails with `UNSUPPORTED`. The SQL "looks" valid, but the source refuses it.

## Query IR (`POST /v1/queries` / `qllm query`)

JSON with `additionalProperties: false`. Required: `from`, `select`.

### Accepted

- `from` and join `from`: an entity name or catalog alias (`[a-z][a-z0-9_]*`).
- `as` in the query: a unique local alias.
- `select`: field refs (`entity.field` or `alias.field`) **or** `{ "agg": "count|sum|avg|min|max", "field"?, "as" }`. `count` may omit `field`.
- `joins[]`: only `inner` and `left`; `on` uses `{left, right}` pairs.
- `where`: `{ "op", "args" }` for `and` / `or` / `not`; comparisons use `{ "field", "op", "value"? }`.
- Compare `op`: `eq` `neq` `gt` `gte` `lt` `lte` `in` `nin` `contains` `is_null` `not_null`. Do **not** use `=`, `LIKE`, or `{and:[…]}` in place of `op` plus `args`.
- `groupBy` when you mix aggregate and non-aggregate columns.
- `orderBy`: `{ "field", "dir": "asc"|"desc" }`.
- `limit` ≥ 1 (or the preset default); `offset` ≥ 0.
- `mode`: `sync` or `async`.
- With more than one entity, qualify fields or you get `AMBIGUOUS_FIELD`.

### Refused or absent in the IR

- `full` and `cross` joins in the IR (SQL on DuckDB may accept them on the SQL path).
- `having`, `union`, `case`, `like`, and `xor` in the IR; use SQL instead.
- Mutations.
- Inventing entities or join keys.

`GET /v1/howtouseme` describes `never`, shapes, and `invalidExamples`. The agent should read it **before** inventing an IR.

## Response

An envelope with `protocolVersion`, `queryId`, `status` (`succeeded` / `failed` / `accepted`), a tabular `result`, `meta` (`elapsedMs`, `app`, `plan.usedDuckDB`, steps), or a typed `error`.

Do not treat HTTP 200 as success without checking `status` and `error`.
