# 04 — Connectors

## Capability matrix

Harness / CI: postgres, mysql, mongodb, rest.

Experimental 0.2.0 (no compose, no goldens): mssql, sqlite, clickhouse, dynamodb, cassandra, ksql pull, redis, kafka.

Wire aliases (same driver as the parent, experimental, no harness): mysql → `mariadb` `tidb` `vitess` `aurora_mysql` `planetscale`; postgres → `cockroach` `yugabyte` `alloydb` `aurora_postgres` `neon` `supabase` `timescale` `redshift`.

| Capacidade | postgres | mysql | mssql | sqlite | clickhouse | mongodb | rest | dynamodb | cassandra | ksql | redis | kafka |
|------------|----------|-------|-------|--------|------------|---------|------|----------|-----------|------|-------|-------|
| filter | yes | yes | yes | yes | yes | yes | partial | partial (key eq) | partial (partition eq) | partial (key eq) | key eq | partition+offset / key / time |
| project | yes | yes | yes | yes | yes | yes | yes | yes | yes | yes | yes | yes |
| orderBy | yes | yes | yes | yes | yes | yes | partial | no | no | no | no | no |
| limit/offset | yes | yes | yes (TOP/OFFSET) | yes | yes | yes | partial | limit | limit | limit | limit | limit |
| agg + groupBy | yes | yes | yes | yes | yes | yes | no → DuckDB | no | no | no | no | no |
| join same source | yes | yes | yes | yes | yes | no → DuckDB | no | no | no | no | no | no |
| join cross source | DuckDB | DuckDB | DuckDB | DuckDB | DuckDB | DuckDB | DuckDB | DuckDB | DuckDB | DuckDB | DuckDB | DuckDB |
| writes | no | no | no | no | no | no | no | no | no | no | no (D19) | no (D19) |

Dynamo/Cassandra/ksql/redis/kafka: missing `binding.accessPath` equality in WHERE → `UNSUPPORTED` (no Scan / ALLOW FILTERING / EMIT CHANGES / `KEYS` / unbounded consume). Redis never mutates keys; Kafka never joins a group or commits offsets.

## Pushdown vs DuckDB

1. Planner gera plano por entidade/fonte.
2. Se todas as ops do step são `yes` na matrix → pushdown.
3. Se join cross-source ou REST/KV agg → fetch com filter/limit máximos → DuckDB.
4. Se op pedida é impossível sem scan absurdo → `UNSUPPORTED` (não “puxar a tabela inteira”).

Authoring (não é query): `qllm catalog from-openapi` gera entities `rest_resource` e um fragmento `options.resources` a partir de GET listáveis. O connector lê `list` e, quando o WHERE tem igualdade nos path params, `getById` (`{id}` é substituído). `list.itemsKey` escolhe a chave do array; `maxPages`/`pageSize` paginam por offset. Campo de catálogo `fromFilter: true` (D20, só REST) preenche uma coluna omitida no JSON com o `eq` do WHERE. `introspect` é postgres/mysql e os aliases de fio.

## Bindings físicos

| type | `binding.kind` | Campos |
|------|----------------|--------|
| postgres/mysql (+ aliases)/mssql/sqlite/clickhouse | `table` | `schema`, `table` (sqlite: `schema: main`) |
| mongodb | `collection` | `collection` |
| rest | `rest_resource` | `resource` |
| dynamodb | `table` | `table` + `accessPath.pk` / `partition`, optional `sk`/`sort` |
| cassandra | `table` | `table` + `accessPath.partition` (logical field names) |
| ksql | `table` | `table` + `accessPath.ksqlKey` (pull only) |
| redis | `key` | `keyPattern` (`user:{id}`) + `accessPath.partition` (logical fields in the pattern) |
| kafka | `topic` | `topic` + equality on partition+offset, `accessPath.key`, or timestamp |

## Auth suportada (connection)

- **SQL (pg/mysql/mssql/ch):** user/password via env; `sslMode` postgres; `encrypt` mssql; `secure` clickhouse
- **SQLite:** `pathEnv` (file path)
- **Mongo:** `uriEnv`
- **REST / ksql:** `none` \| `bearer` \| `header` \| `basic`
- **DynamoDB:** AWS default credential chain; `region` literal; optional `endpointEnv`
- **Cassandra:** `hostEnv` or `hostsEnv`; optional user/password env
- **Redis:** `addrEnv` (or `hostEnv`+`port`); optional `db`, `userEnv`, `passwordEnv`, `tls`, `readReplica` (READONLY). Recommended ACL: `+get +hget +hgetall +lrange +sscan +zrange +xrange +type +exists`
- **Kafka:** `brokersEnv`; optional `tls`, SASL `userEnv`/`passwordEnv` (`sasl`: `plain`/`scram`). Recommended ACL: topic `Read`+`Describe`, no group, no Write. Phase 1: JSON/raw values only.

## Timeouts

Cada connector aplica `min(options.timeoutMs|statementTimeoutMs, limits.maxSourceMs)` e propaga cancel do context Go.
