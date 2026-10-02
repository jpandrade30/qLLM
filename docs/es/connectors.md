# Fuentes (conectores)

Matriz normativa: [`planning/04-connectors.md`](../../planning/04-connectors.md). Formatos de conexión: [`planning/03-protocol-schemas.md`](../../planning/03-protocol-schemas.md) y [`preset.schema.json`](../../planning/schemas/preset.schema.json).

## Tipos

| `type` | Harness/CI | Notas |
|--------|------------|-------|
| `postgres` | sí | `sslMode`, `statementTimeoutMs`. Alias: `cockroach` `yugabyte` `alloydb` `aurora_postgres` `neon` `supabase` `timescale` `redshift` |
| `mysql` | sí | Alias: `mariadb` `tidb` `vitess` `aurora_mysql` `planetscale` |
| `mongodb` | sí | `uriEnv`, `database`; el join en la misma fuente pasa por DuckDB |
| `rest` | sí | `baseUrlEnv`, auth `none`\|`bearer`\|`header`\|`basic`; agregaciones en DuckDB; `options.resources` |
| `mssql` | experimental | `encrypt` |
| `sqlite` | experimental | `pathEnv`; `binding.schema: main` |
| `clickhouse` | experimental | el puerto nativo habitual es 9000 |
| `dynamodb` | experimental | credenciales de AWS y `region`; `endpointEnv` (Local); `accessPath` pk/sk |
| `cassandra` | experimental | `hostEnv`/`hostsEnv`, keyspace; `accessPath.partition` |
| `ksql` | experimental | **solo pull queries**; `baseUrlEnv`; `accessPath.ksqlKey` |
| `redis` | experimental | `addrEnv` o `hostEnv`+`port`; `binding.kind: key` + `keyPattern`; nunca muta claves |
| `kafka` | experimental | `brokersEnv`; `binding.kind: topic`; sin grupo ni commit; JSON/raw |

Experimental significa que el tipo está en el binario, pero **no** tiene Compose ni goldens en este repositorio. Considera que funciona solo después de probarlo en tu propia instancia.

No son un tipo propio: Oracle, BigQuery, Snowflake, Elasticsearch, GraphQL, S3 como tabla. Muchas APIs HTTP JSON usan `rest`.

## Binding en el catálogo

| Tipo | `binding.kind` | Campos |
|------|----------------|--------|
| SQL tabular | `table` | `schema`, `table` |
| mongodb | `collection` | `collection` |
| rest | `rest_resource` | `resource` |
| dynamodb | `table` | `table` y `accessPath.pk`/`partition`, `sk`/`sort` opcional |
| cassandra | `table` | `table` y `accessPath.partition` (nombres **lógicos**) |
| ksql | `table` | `table` y `accessPath.ksqlKey` |
| redis | `key` | `keyPattern` (`user:{id}`) y `accessPath.partition` |
| kafka | `topic` | `topic` e igualdad en partition+offset, `accessPath.key` o timestamp |

Un Query IR **sin** igualdad sobre la clave KV o de stream devuelve `UNSUPPORTED`; el runtime nunca hace un escaneo completo.

## Capacidades (IR y pushdown)

- **Escrituras:** nunca.
- **Join en la misma fuente:** los motores SQL hacen pushdown; mongo, rest y KV traen los datos y hacen el join en DuckDB.
- **Join entre fuentes:** siempre en DuckDB.
- **REST:** filtros `eq`, `limit` y `offset` llegan a la API; `getById` corre cuando los path params están completos; `maxPages` pagina por offset; agregaciones en DuckDB. Ver [`resources` de REST en detalle](field-reference.md#resources-de-rest-en-detalle).
- **Timeout:** `min(options.timeoutMs|statementTimeoutMs, limits.maxSourceMs)` más la cancelación del contexto.

En la **ruta de SQL de catálogo**, el runtime trae un conjunto más amplio y DuckDB hace el trabajo. La matriz de pushdown del IR **no** se aplica al `WHERE` del SQL.

## Autenticación de conexión (no la confundas con el Bearer del serve)

- SQL: `userEnv` / `passwordEnv`.
- SQLite: ruta del archivo en una variable de entorno.
- Mongo: `uriEnv`.
- REST y ksql: `none` / `bearer` / `header` / `basic`.
- Dynamo: cadena de credenciales de AWS; `region` es un valor literal.
- Cassandra: host(s) más usuario y contraseña opcionales.
- Redis: `addrEnv`; user/password/tls opcionales. Prefiere ACL solo con get/hget/hgetall/lrange/sscan/zrange/xrange/type/exists.
- Kafka: `brokersEnv`; TLS/SASL opcionales. Prefiere `Read`+`Describe` en el topic y **sin** grupo. Fase 1: JSON/raw.

## Creación del catálogo

- `qllm catalog introspect`: postgres, mysql y sus alias de cable.
- `qllm catalog from-openapi`: genera un borrador REST; debes pegar `resources` en el preset.

El descubrimiento automático de "sample collection" de Mongo queda fuera del alcance de este MVP.
