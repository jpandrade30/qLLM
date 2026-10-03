# Fontes (conectores)

Matriz normativa: [`planning/04-connectors.md`](../../planning/04-connectors.md). Formatos de conexão: [`planning/03-protocol-schemas.md`](../../planning/03-protocol-schemas.md) e [`preset.schema.json`](../../planning/schemas/preset.schema.json).

## Tipos

| `type` | Harness/CI | Observações |
|--------|------------|-------------|
| `postgres` | sim | `sslMode`, `statementTimeoutMs`. Aliases: `cockroach` `yugabyte` `alloydb` `aurora_postgres` `neon` `supabase` `timescale` `redshift` |
| `mysql` | sim | Aliases: `mariadb` `tidb` `vitess` `aurora_mysql` `planetscale` |
| `mongodb` | sim | `uriEnv`, `database`; join na mesma fonte passa pelo DuckDB |
| `rest` | sim | `baseUrlEnv`, auth `none`\|`bearer`\|`header`\|`basic`; agregações no DuckDB; `options.resources` |
| `mssql` | experimental | `encrypt` |
| `sqlite` | experimental | `pathEnv`; `binding.schema: main` |
| `clickhouse` | experimental | porta nativa típica 9000 |
| `dynamodb` | experimental | credenciais AWS e `region`; `endpointEnv` (Local); `accessPath` pk/sk |
| `cassandra` | experimental | `hostEnv`/`hostsEnv`, keyspace; `accessPath.partition` |
| `ksql` | experimental | **somente pull queries**; `baseUrlEnv`; `accessPath.ksqlKey` |
| `redis` | experimental | `addrEnv` ou `hostEnv`+`port`; `binding.kind: key` + `keyPattern`; nunca altera chaves |
| `kafka` | experimental | `brokersEnv`; `binding.kind: topic`; sem grupo, sem commit; JSON/raw |

Experimental significa que o tipo está no binário, mas **não** tem Compose nem goldens neste repositório. Só considere que funciona depois de testar na sua própria instância.

Não são um tipo próprio: Oracle, BigQuery, Snowflake, Elasticsearch, GraphQL, S3 como tabela. Muitas APIs HTTP JSON usam `rest`.

## Binding no catálogo

| Tipo | `binding.kind` | Campos |
|------|----------------|--------|
| SQL tabular | `table` | `schema`, `table` |
| mongodb | `collection` | `collection` |
| rest | `rest_resource` | `resource` |
| dynamodb | `table` | `table` e `accessPath.pk`/`partition`, `sk`/`sort` opcional |
| cassandra | `table` | `table` e `accessPath.partition` (nomes **lógicos**) |
| ksql | `table` | `table` e `accessPath.ksqlKey` |
| redis | `key` | `keyPattern` (`user:{id}`) e `accessPath.partition` |
| kafka | `topic` | `topic` e igualdade em partition+offset, `accessPath.key` ou timestamp |

Um Query IR **sem** igualdade na chave KV ou de stream retorna `UNSUPPORTED`; o runtime nunca faz varredura completa.

## Capacidades (IR e pushdown)

- **Escritas:** nunca.
- **Join na mesma fonte:** engines SQL fazem pushdown; mongo, rest e KV buscam os dados e fazem o join no DuckDB.
- **Join entre fontes:** sempre no DuckDB.
- **REST:** filtros `eq`, `limit` e `offset` chegam à API; `getById` roda quando os path params estão preenchidos; `maxPages` pagina por offset; agregações no DuckDB. Use `fields[].fromFilter: true` quando a API recebe a chave na request mas omite no JSON (por exemplo `{"saldo":5300}`) para `GROUP BY` e joins ainda terem essa coluna. Veja [`resources` do REST em detalhe](field-reference.md#resources-do-rest-em-detalhe).
- **Timeout:** `min(options.timeoutMs|statementTimeoutMs, limits.maxSourceMs)` mais cancelamento do contexto.

No **caminho de SQL de catálogo**, o runtime busca um conjunto mais amplo e o DuckDB faz o trabalho. A matriz de pushdown do IR **não** se aplica ao `WHERE` do SQL.

## Autenticação de conexão (não confundir com o Bearer do serve)

- SQL: `userEnv` / `passwordEnv`.
- SQLite: caminho do arquivo em uma variável de ambiente.
- Mongo: `uriEnv`.
- REST e ksql: `none` / `bearer` / `header` / `basic`.
- Dynamo: cadeia de credenciais da AWS; `region` é um valor literal.
- Cassandra: host(s) mais usuário e senha opcionais.
- Redis: `addrEnv`; user/password/tls opcionais. Prefira ACL só com get/hget/hgetall/lrange/sscan/zrange/xrange/type/exists.
- Kafka: `brokersEnv`; TLS/SASL opcionais. Prefira `Read`+`Describe` no tópico e **sem** grupo. Fase 1: JSON/raw.

## Criação do catálogo

- `qllm catalog introspect`: postgres, mysql e os aliases de fio.
- `qllm catalog from-openapi`: gera um rascunho REST; você precisa colar `resources` no preset.

A descoberta automática de "sample collection" do Mongo está fora do escopo deste MVP.
