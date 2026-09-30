# Consultas: SQL de catálogo e Query IR

Há dois caminhos. O MCP tem **somente** SQL. O HTTP tem os dois.

Schemas: [`sql-request.schema.json`](../../planning/schemas/sql-request.schema.json), [`query-ir.schema.json`](../../planning/schemas/query-ir.schema.json), [`query-response.schema.json`](../../planning/schemas/query-response.schema.json). Dialeto: [`planning/07-sql-dialect.md`](../../planning/07-sql-dialect.md).

## SQL de catálogo (`execute_sql` / `POST /v1/sql` / `qllm sql`)

Corpo: `{ "sql": "SELECT …", "version": "1"|"2" }`. Um `version` omitido significa **`"2"`**. Um valor desconhecido retorna `UNSUPPORTED_VERSION`.

### Aceito (resumo)

- Uma única instrução `SELECT` (opcionalmente com `WITH`).
- As tabelas são **nomes de entidades** (mais aliases de CTE, que não são entidades).
- O `LIMIT` é obrigatório ou é injetado (`defaultLimit`). Um `OFFSET` isolado também injeta um limit.
- Dialeto `"1"`: `WHERE`, `HAVING`, `DISTINCT`, `CASE`, `LIKE`, `ILIKE`, `BETWEEN`, `IN`, `IS NULL`, CTEs, subqueries no `FROM`, joins, agregações básicas, funções de string/número/cast e funções de JSON/array/datetime **com os nomes do DuckDB**.
- Dialeto `"2"`: tudo do `"1"` mais `UNION` / `UNION ALL` / `INTERSECT` / `EXCEPT`, `QUALIFY`, funções de janela (`ROW_NUMBER`, …) e `XOR`.

Funções do Databricks com nome diferente (`GET_JSON_OBJECT`, `DATEADD`, …) não são rejeitadas se o DuckDB as tiver; o guia pede a grafia do DuckDB.

### Recusado (`INVALID_SQL` ou equivalente)

- `INSERT` `UPDATE` `DELETE` `MERGE` `REPLACE`
- `CREATE` `DROP` `ALTER` `TRUNCATE` `COPY` `ATTACH` `DETACH`
- `INSTALL` `LOAD` `PRAGMA` `SET` `CALL` `GRANT`
- `read_csv` `read_parquet` `read_json` `postgres_scan` `httpfs` `glob` `read_text` `read_blob`
- Várias instruções (`;`)
- `schema.tabela` (`public.customers`)
- Funções de tabela arbitrárias
- `LIMIT` maior que `maxLimit` retorna `LIMIT_EXCEEDED`
- Entidade ou campo inexistente retorna `UNKNOWN_ENTITY` / `UNKNOWN_FIELD`
- Entidade fora da ACL retorna `FORBIDDEN`

### Execução (importante para quem modela o planner mentalmente)

1. Parse e, em seguida, validação de nomes e ACL.
2. **Busca** das tabelas referenciadas (apenas as colunas necessárias). Neste caminho **não** há pushdown de `WHERE`.
3. O DuckDB executa o `SELECT` (`enable_external_access=false`).
4. Um build **sem** `-tags duckdb` não consegue executar este caminho.

Em fontes KV e de stream, o `WHERE` precisa incluir uma igualdade no `accessPath`, ou a busca falha com `UNSUPPORTED`. O SQL "parece" válido, mas a fonte o recusa.

## Query IR (`POST /v1/queries` / `qllm query`)

JSON com `additionalProperties: false`. Obrigatórios: `from`, `select`.

### Aceito

- `from` e o `from` dos joins: um nome de entidade ou alias do catálogo (`[a-z][a-z0-9_]*`).
- `as` na consulta: um alias local único.
- `select`: referências de field (`entity.field` ou `alias.field`) **ou** `{ "agg": "count|sum|avg|min|max", "field"?, "as" }`. O `count` pode omitir o `field`.
- `joins[]`: somente `inner` e `left`; `on` usa pares `{left, right}`.
- `where`: `{ "op", "args" }` para `and` / `or` / `not`; comparações usam `{ "field", "op", "value"? }`.
- `op` de comparação: `eq` `neq` `gt` `gte` `lt` `lte` `in` `nin` `contains` `is_null` `not_null`. **Não** use `=`, `LIKE` nem `{and:[…]}` no lugar de `op` com `args`.
- `groupBy` quando você mistura colunas agregadas e não agregadas.
- `orderBy`: `{ "field", "dir": "asc"|"desc" }`.
- `limit` ≥ 1 (ou o padrão do preset); `offset` ≥ 0.
- `mode`: `sync` ou `async`.
- Com mais de uma entidade, qualifique os fields, ou você receberá `AMBIGUOUS_FIELD`.

### Recusado ou inexistente no IR

- Joins `full` e `cross` no IR (o SQL no DuckDB pode aceitá-los no caminho de SQL).
- `having`, `union`, `case`, `like` e `xor` no IR; use SQL.
- Mutações.
- Inventar entidades ou chaves de join.

O `GET /v1/howtouseme` descreve `never`, os formatos e `invalidExamples`. O agente deve lê-lo **antes** de inventar um IR.

## Resposta

Um envelope com `protocolVersion`, `queryId`, `status` (`succeeded` / `failed` / `accepted`), um `result` tabular, `meta` (`elapsedMs`, `app`, `plan.usedDuckDB`, steps) ou um `error` tipado.

Não trate HTTP 200 como sucesso sem verificar `status` e `error`.
