# Fontes (conectores)

Matriz normativa: [`planning/04-connectors.md`](../../planning/04-connectors.md). Formatos de conexão: [`planning/03-protocol-schemas.md`](../../planning/03-protocol-schemas.md) e [`preset.schema.json`](../../planning/schemas/preset.schema.json).

## Tipos

| `type` | Harness/CI | Observações |
|--------|------------|-------------|
| `postgres` | sim | `sslMode`, `statementTimeoutMs` |
| `mysql` | sim | |
| `mongodb` | sim | `uriEnv`, `database`; join na mesma fonte passa pelo DuckDB |
| `rest` | sim | `baseUrlEnv`, auth `none`\|`bearer`\|`header`\|`basic`; agregações no DuckDB; `options.resources` |
| `mssql` | experimental | `encrypt` |
| `sqlite` | experimental | `pathEnv`; `binding.schema: main` |
| `clickhouse` | experimental | porta nativa típica 9000 |
| `dynamodb` | experimental | credenciais AWS e `region`; `endpointEnv` (Local); `accessPath` pk/sk |
| `cassandra` | experimental | `hostEnv`/`hostsEnv`, keyspace; `accessPath.partition` |
| `ksql` | experimental | **somente pull queries**; `baseUrlEnv`; `accessPath.ksqlKey` |

Experimental significa que o tipo está no binário, mas **não** tem Compose nem goldens neste repositório. Só considere que funciona depois de testar na sua própria instância.

Não são suportados: Oracle, BigQuery, Snowflake, Elasticsearch, fonte GraphQL, S3 como tabela e similares.

## Binding no catálogo

| Tipo | `binding.kind` | Campos |
|------|----------------|--------|
| SQL tabular | `table` | `schema`, `table` |
| mongodb | `collection` | `collection` |
| rest | `rest_resource` | `resource` |
| dynamodb | `table` | `table` e `accessPath.pk`/`partition`, `sk`/`sort` opcional |
| cassandra | `table` | `table` e `accessPath.partition` (nomes **lógicos**) |
| ksql | `table` | `table` e `accessPath.ksqlKey` |

Um Query IR **sem** igualdade na chave KV ou de stream retorna `UNSUPPORTED`; o runtime nunca faz varredura completa.

## Capacidades (IR e pushdown)

- **Escritas:** nunca.
- **Join na mesma fonte:** engines SQL fazem pushdown; mongo, rest e KV buscam os dados e fazem o join no DuckDB.
- **Join entre fontes:** sempre no DuckDB.
- **REST:** só filtros `eq`, `limit` e `offset` chegam à API (como query params); uma requisição, sem paginação; agregações rodam no DuckDB. `getById` é só documentação. Veja [`resources` do REST em detalhe](field-reference.md#resources-do-rest-em-detalhe).
- **Timeout:** `min(options.timeoutMs|statementTimeoutMs, limits.maxSourceMs)` mais cancelamento do contexto.

No **caminho de SQL de catálogo**, o runtime busca um conjunto mais amplo e o DuckDB faz o trabalho. A matriz de pushdown do IR **não** se aplica ao `WHERE` do SQL.

## Autenticação de conexão (não confundir com o Bearer do serve)

- SQL: `userEnv` / `passwordEnv`.
- SQLite: caminho do arquivo em uma variável de ambiente.
- Mongo: `uriEnv`.
- REST e ksql: `none` / `bearer` / `header` / `basic`.
- Dynamo: cadeia de credenciais da AWS; `region` é um valor literal.
- Cassandra: host(s) mais usuário e senha opcionais.

## Criação do catálogo

- `qllm catalog introspect`: **somente postgres e mysql**.
- `qllm catalog from-openapi`: gera um rascunho REST; você precisa colar `resources` no preset.

A descoberta automática de "sample collection" do Mongo está fora do escopo deste MVP.
