# 04 — Connectors

## Capability matrix

Harness / CI: postgres, mysql, mongodb, rest.

Experimental 0.2.0 (no compose, no goldens): mssql, sqlite, clickhouse, dynamodb, cassandra, ksql pull.

| Capacidade | postgres | mysql | mssql | sqlite | clickhouse | mongodb | rest | dynamodb | cassandra | ksql |
|------------|----------|-------|-------|--------|------------|---------|------|----------|-----------|------|
| filter | yes | yes | yes | yes | yes | yes | partial | partial (key eq) | partial (partition eq) | partial (key eq) |
| project | yes | yes | yes | yes | yes | yes | yes | yes | yes | yes |
| orderBy | yes | yes | yes | yes | yes | yes | partial | no | no | no |
| limit/offset | yes | yes | yes (TOP/OFFSET) | yes | yes | yes | partial | limit | limit | limit |
| agg + groupBy | yes | yes | yes | yes | yes | yes | no → DuckDB | no | no | no |
| join same source | yes | yes | yes | yes | yes | no → DuckDB | no | no | no | no |
| join cross source | DuckDB | DuckDB | DuckDB | DuckDB | DuckDB | DuckDB | DuckDB | DuckDB | DuckDB | DuckDB |
| writes | no | no | no | no | no | no | no | no | no | no |

Dynamo/Cassandra/ksql: missing `binding.accessPath` equality in WHERE → `UNSUPPORTED` (no Scan / ALLOW FILTERING / EMIT CHANGES).

## Pushdown vs DuckDB

1. Planner gera plano por entidade/fonte.
2. Se todas as ops do step são `yes` na matrix → pushdown.
3. Se join cross-source ou REST/KV agg → fetch com filter/limit máximos → DuckDB.
4. Se op pedida é impossível sem scan absurdo → `UNSUPPORTED` (não “puxar a tabela inteira”).

Authoring (não é query): `qllm catalog from-openapi` gera entities `rest_resource` e um fragmento `options.resources` a partir de GET listáveis. O connector continua lendo só o YAML já no preset. `introspect` continua postgres/mysql only.

## Bindings físicos

| type | `binding.kind` | Campos |
|------|----------------|--------|
| postgres/mysql/mssql/sqlite/clickhouse | `table` | `schema`, `table` (sqlite: `schema: main`) |
| mongodb | `collection` | `collection` |
| rest | `rest_resource` | `resource` |
| dynamodb | `table` | `table` + `accessPath.pk` / `partition`, optional `sk`/`sort` |
| cassandra | `table` | `table` + `accessPath.partition` (logical field names) |
| ksql | `table` | `table` + `accessPath.ksqlKey` (pull only) |

## Auth suportada (connection)

- **SQL (pg/mysql/mssql/ch):** user/password via env; `sslMode` postgres; `encrypt` mssql; `secure` clickhouse
- **SQLite:** `pathEnv` (file path)
- **Mongo:** `uriEnv`
- **REST / ksql:** `none` \| `bearer` \| `header` \| `basic`
- **DynamoDB:** AWS default credential chain; `region` literal; optional `endpointEnv`
- **Cassandra:** `hostEnv` or `hostsEnv`; optional user/password env

## Timeouts

Cada connector aplica `min(options.timeoutMs|statementTimeoutMs, limits.maxSourceMs)` e propaga cancel do context Go.
