# Consultas: SQL de catálogo e Query IR

Dois caminhos. O MCP **só** tem SQL. HTTP tem os dois.

Schemas: [`sql-request.schema.json`](../planning/schemas/sql-request.schema.json), [`query-ir.schema.json`](../planning/schemas/query-ir.schema.json), [`query-response.schema.json`](../planning/schemas/query-response.schema.json). Dialeto: [`planning/07-sql-dialect.md`](../planning/07-sql-dialect.md).

## Catalog SQL (`execute_sql` / `POST /v1/sql` / `qllm sql`)

Body: `{ "sql": "SELECT …", "version": "1"|"2" }`. `version` omitido = **`"2"`**. Valor desconhecido → `UNSUPPORTED_VERSION`.

### Aceite (resumo)

- Uma única statement `SELECT` (opcional `WITH`).
- Tabelas = **entity names** (e aliases de CTE, que não são entidades).
- `LIMIT` obrigatório ou injectado (`defaultLimit`). `OFFSET` sozinho também injecta limit.
- Dialeto `"1"`: `WHERE`/`HAVING`/`DISTINCT`/`CASE`/`LIKE`/`ILIKE`/`BETWEEN`/`IN`/`IS NULL`, CTE, subquery `FROM`, joins, aggs básicas, strings/nums/cast, JSON/array/datetime **com nomes DuckDB**.
- Dialeto `"2"`: tudo o de `"1"` mais `UNION`/`UNION ALL`/`INTERSECT`/`EXCEPT`, `QUALIFY`, janelas (`ROW_NUMBER`…), `XOR`.

Funções Databricks com outro nome (`GET_JSON_OBJECT`, `DATEADD`, …) não são rejeitadas se o DuckDB as tiver; o guia pede spelling DuckDB.

### Recusado (`INVALID_SQL` ou equivalente)

- `INSERT` `UPDATE` `DELETE` `MERGE` `REPLACE`
- `CREATE` `DROP` `ALTER` `TRUNCATE` `COPY` `ATTACH` `DETACH`
- `INSTALL` `LOAD` `PRAGMA` `SET` `CALL` `GRANT`
- `read_csv` `read_parquet` `read_json` `postgres_scan` `httpfs` `glob` `read_text` `read_blob`
- Várias statements (`;`)
- `schema.tabela` (`public.customers`)
- Funções de tabela arbitrárias
- `LIMIT` > `maxLimit` → `LIMIT_EXCEEDED`
- Entidade/campo inexistente → `UNKNOWN_ENTITY` / `UNKNOWN_FIELD`
- Entidade fora do ACL → `FORBIDDEN`

### Execução (importante para quem implementa o planner mental)

1. Parse + validação de nomes/ACL.
2. **Fetch** das tabelas citadas (colunas necessárias). **Não** há pushdown de `WHERE` neste caminho.
3. DuckDB corre o `SELECT` (`enable_external_access=false`).
4. Build **sem** `-tags duckdb` não executa este caminho.

KV/stream: `WHERE` tem de incluir igualdade na `accessPath` ou o fetch falha `UNSUPPORTED` — o SQL “parece” válido mas a fonte recusa.

## Query IR (`POST /v1/queries` / `qllm query`)

JSON, `additionalProperties: false`. Obrigatório: `from`, `select`.

### Aceite

- `from` / joins `from`: entity name ou alias de catalog (`[a-z][a-z0-9_]*`).
- `as` no query: alias local único.
- `select`: field refs (`entity.field` ou `alias.field`) **ou** `{ "agg": "count|sum|avg|min|max", "field"?, "as" }`. `count` pode omitir `field`.
- `joins[]`: só `inner` | `left`; `on`: pares `{left, right}`.
- `where`: `{ "op", "args" }` para `and`/`or`/`not`; compares `{ "field", "op", "value"? }`.
- Compare `op`: `eq` `neq` `gt` `gte` `lt` `lte` `in` `nin` `contains` `is_null` `not_null`. **Não** uses `=` / `LIKE` / `{and:[…]}` no sítio de `op`+`args`.
- `groupBy` se misturas agg e não-agg.
- `orderBy`: `{ "field", "dir": "asc"|"desc" }`.
- `limit` ≥ 1 (ou default do preset); `offset` ≥ 0.
- `mode`: `sync` | `async`.
- Com >1 entidade, qualifica campos ou `AMBIGUOUS_FIELD`.

### Recusado / inexistente no IR

- Joins `full` / `cross` no IR (SQL no DuckDB pode aceitar no caminho SQL).
- `having`, `union`, `case`, `like`, `xor` no IR — usa SQL.
- Mutações.
- Inventar entidades ou chaves de join.

`GET /v1/howtouseme` descreve `never`, shapes e `invalidExamples` — o agente deve ler isto **antes** de inventar IR.

## Resposta

Envelope com `protocolVersion`, `queryId`, `status` (`succeeded` / `failed` / `accepted`), `result` tabular, `meta` (`elapsedMs`, `app`, `plan.usedDuckDB`, steps), ou `error` tipado.

Não trates HTTP 200 como sucesso sem olhar `status` / `error`.
