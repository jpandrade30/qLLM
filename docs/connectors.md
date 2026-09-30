# Fontes (connectors)

Matriz normativa: [`planning/04-connectors.md`](../planning/04-connectors.md). Connection shapes: [`planning/03-protocol-schemas.md`](../planning/03-protocol-schemas.md) + [`preset.schema.json`](../planning/schemas/preset.schema.json).

## Tipos

| `type` | Harness/CI | Notas |
|--------|------------|--------|
| `postgres` | sim | `sslMode`, `statementTimeoutMs` |
| `mysql` | sim | |
| `mongodb` | sim | `uriEnv`, `database`; join same-source → DuckDB |
| `rest` | sim | `baseUrlEnv`, auth `none`\|`bearer`\|`header`\|`basic`; agg → DuckDB; `options.resources` |
| `mssql` | experimental | `encrypt` |
| `sqlite` | experimental | `pathEnv`; `binding.schema: main` |
| `clickhouse` | experimental | native port típico 9000 |
| `dynamodb` | experimental | AWS creds + `region`; `endpointEnv` (Local); `accessPath` pk/sk |
| `cassandra` | experimental | `hostEnv`/`hostsEnv`, keyspace; `accessPath.partition` |
| `ksql` | experimental | **só pull**; `baseUrlEnv`; `accessPath.ksqlKey` |

Experimental = no binário, **sem** compose/goldens neste repo. Assumir “funciona” só depois de testares a tua instância.

Não há: Oracle, BigQuery, Snowflake, Elasticsearch, GraphQL source, S3-as-table, etc.

## Binding no catalog

| Tipo | `binding.kind` | Campos |
|------|----------------|--------|
| SQL tabular | `table` | `schema`, `table` |
| mongodb | `collection` | `collection` |
| rest | `rest_resource` | `resource` |
| dynamodb | `table` | `table` + `accessPath.pk`/`partition`, `sk`/`sort` opcional |
| cassandra | `table` | `table` + `accessPath.partition` (nomes **lógicos**) |
| ksql | `table` | `table` + `accessPath.ksqlKey` |

Query IR **sem** igualdade na chave KV/stream → `UNSUPPORTED` (não faz scan completo).

## Capacidades (IR / pushdown)

- **Writes:** nunca.
- **Join same source:** SQL engines sim; mongo/rest/KV não → fetch + DuckDB.
- **Join cross-source:** sempre DuckDB.
- **REST:** filter/order/limit parciais; agg no DuckDB.
- **Timeout:** `min(options.timeoutMs|statementTimeoutMs, limits.maxSourceMs)` + cancel do context.

Caminho **SQL de catálogo:** fetch alargado + DuckDB; a matrix de pushdown do IR **não** se aplica ao `WHERE` SQL.

## Auth de conexão (não confundir com Bearer do serve)

- SQL: `userEnv` / `passwordEnv`.
- SQLite: path em env.
- Mongo: `uriEnv`.
- REST/ksql: `none` / `bearer` / `header` / `basic`.
- Dynamo: chain AWS; `region` literal.
- Cassandra: host(s) + user/password opcionais.

## Authoring

- `qllm catalog introspect`: **só** postgres/mysql.
- `qllm catalog from-openapi`: gera draft REST; tens de colar `resources` no preset.

Mongo “sample collection” automático: fora deste MVP.
