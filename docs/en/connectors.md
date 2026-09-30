# Sources (connectors)

Normative matrix: [`planning/04-connectors.md`](../../planning/04-connectors.md). Connection shapes: [`planning/03-protocol-schemas.md`](../../planning/03-protocol-schemas.md) and [`preset.schema.json`](../../planning/schemas/preset.schema.json).

## Types

| `type` | Harness/CI | Notes |
|--------|------------|-------|
| `postgres` | yes | `sslMode`, `statementTimeoutMs` |
| `mysql` | yes | |
| `mongodb` | yes | `uriEnv`, `database`; same-source join goes through DuckDB |
| `rest` | yes | `baseUrlEnv`, auth `none`\|`bearer`\|`header`\|`basic`; aggregations in DuckDB; `options.resources` |
| `mssql` | experimental | `encrypt` |
| `sqlite` | experimental | `pathEnv`; `binding.schema: main` |
| `clickhouse` | experimental | native port is typically 9000 |
| `dynamodb` | experimental | AWS credentials plus `region`; `endpointEnv` (Local); `accessPath` pk/sk |
| `cassandra` | experimental | `hostEnv`/`hostsEnv`, keyspace; `accessPath.partition` |
| `ksql` | experimental | **pull queries only**; `baseUrlEnv`; `accessPath.ksqlKey` |

Experimental means the type is in the binary but has **no** Compose setup or goldens in this repository. Treat it as working only after you test your own instance.

Not supported: Oracle, BigQuery, Snowflake, Elasticsearch, a GraphQL source, S3-as-table, and similar.

## Catalog binding

| Type | `binding.kind` | Fields |
|------|----------------|--------|
| Tabular SQL | `table` | `schema`, `table` |
| mongodb | `collection` | `collection` |
| rest | `rest_resource` | `resource` |
| dynamodb | `table` | `table` plus `accessPath.pk`/`partition`, optional `sk`/`sort` |
| cassandra | `table` | `table` plus `accessPath.partition` (**logical** names) |
| ksql | `table` | `table` plus `accessPath.ksqlKey` |

A Query IR **without** an equality on the KV or stream key returns `UNSUPPORTED`; the runtime never does a full scan.

## Capabilities (IR and pushdown)

- **Writes:** never.
- **Same-source join:** SQL engines push it down; mongo, rest, and KV stores fetch and join in DuckDB.
- **Cross-source join:** always DuckDB.
- **REST:** only `eq` filters, `limit`, and `offset` reach the API (as query parameters); one request, no pagination; aggregations run in DuckDB. `getById` is documentation only. See [REST `resources` in detail](field-reference.md#rest-resources-in-detail).
- **Timeout:** `min(options.timeoutMs|statementTimeoutMs, limits.maxSourceMs)` plus context cancellation.

On the **catalog SQL path** the runtime fetches a wider set and DuckDB does the work. The IR pushdown matrix does **not** apply to SQL `WHERE`.

## Connection auth (not the same as the serve Bearer)

- SQL: `userEnv` / `passwordEnv`.
- SQLite: file path from an env var.
- Mongo: `uriEnv`.
- REST and ksql: `none` / `bearer` / `header` / `basic`.
- Dynamo: the AWS credential chain; `region` is a literal.
- Cassandra: host(s) plus optional user and password.

## Authoring

- `qllm catalog introspect`: **postgres and mysql only**.
- `qllm catalog from-openapi`: generates a REST draft; you must paste `resources` into the preset.

Automatic Mongo "sample collection" discovery is out of scope for this MVP.
