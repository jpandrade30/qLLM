# Referência de campos (tudo o que o schema aceita)

Fonte: [`planning/schemas/`](../../planning/schemas/). Objetos com `additionalProperties: false` **rejeitam** chaves extras.

Convenção `*Env`: a string é o **nome** de uma variável de ambiente (`QLLM_FOO`), nunca o segredo em si.

Identificadores lógicos (`sources[].id`, `entities[].name`, aliases, `name` de field, `from`/`as` do IR): `^[a-z][a-z0-9_]*$`.

---

## `qllm.project.yaml` (opcional)

Obrigatórios: `protocolVersion`, `preset`, `catalog`.

| Campo | Tipo | Observações |
|-------|------|-------------|
| `protocolVersion` | string semver | `0.1.0` / `0.2.0` |
| `preset` | string | Caminho relativo a **este** arquivo |
| `catalog` | string | Idem |

---

## `qllm.preset.yaml`

Obrigatórios na raiz: `protocolVersion`, `project`, `limits`, `sources` (mínimo 1).

| Campo | Tipo | Observações |
|-------|------|-------------|
| `protocolVersion` | semver | |
| `project` | string não vazia | Nome lógico do projeto; deve ser igual ao do catálogo |
| `limits` | object | **Todos** os 5 campos são obrigatórios |
| `sources` | array | |

### `limits` (todos obrigatórios)

| Campo | Tipo | Intervalo |
|-------|------|-----------|
| `maxSyncMs` | int | 100–60000; orçamento total da consulta |
| `maxSourceMs` | int | 100–60000; por chamada à fonte |
| `defaultLimit` | int | ≥ 1; usado quando o IR/SQL não informa LIMIT |
| `maxLimit` | int | ≥ 1; teto máximo |
| `readOnly` | bool | deve ser `true` no produto |

### `sources[]` (cada item)

Obrigatórios: `id`, `type`, `connection`.

| Campo | Tipo | Valores |
|-------|------|---------|
| `id` | string | `crm_pg`, `legacy_api`, … |
| `type` | enum | `postgres` `mysql` `mongodb` `rest` `mssql` `sqlite` `clickhouse` `dynamodb` `cassandra` `ksql` `redis` `kafka` e aliases de fio MySQL (`mariadb` `tidb` `vitess` `aurora_mysql` `planetscale`) e Postgres (`cockroach` `yugabyte` `alloydb` `aurora_postgres` `neon` `supabase` `timescale` `redshift`) |
| `connection` | object | O formato depende do `type` (abaixo). Chaves extras são erro |
| `options` | object | Livre no JSON schema; o runtime lê apenas o que conhece (abaixo) |

### `connection` por `type`

**postgres**, **mysql** e os aliases de fio (`sqlConnection`): obrigatórios `hostEnv`, `port`, `database`, `userEnv`, `passwordEnv`.

| Campo | Tipo | Observações |
|-------|------|-------------|
| `hostEnv` | string | |
| `port` | int | 1–65535 |
| `database` | string | |
| `userEnv` | string | |
| `passwordEnv` | string | |
| `sslMode` | enum opcional | `disable` `require` `verify-ca` `verify-full` (postgres; o mysql ignora quando não é usado) |

**mssql**: mesmos obrigatórios. Extra opcional: `encrypt`: `true` \| `false` \| `disable`. Não tem `sslMode`.

**clickhouse**: mesmos obrigatórios. Extra opcional: `secure` (bool).

**sqlite**: apenas `pathEnv` (a variável de ambiente com o caminho do arquivo `.db`).

**mongodb**: obrigatórios `uriEnv`, `database`.

**rest** e **ksql**: obrigatório `baseUrlEnv`. `auth` é opcional (abaixo).

**dynamodb**: obrigatório `region` (valor literal, por exemplo `us-east-1`). Opcional `endpointEnv` (Dynamo Local). As credenciais da AWS vêm da cadeia de credenciais do processo, não de campos do YAML.

**cassandra**: o schema exige `keyspace`. Na prática, o código usa `hostsEnv` (uma lista) **ou** `hostEnv`. Opcionais: `port`, `userEnv`, `passwordEnv`.

**redis**: `addrEnv` ou `hostEnv`+`port`. Opcionais: `db`, `userEnv`, `passwordEnv`, `tls`, `readReplica`.

**kafka**: obrigatório `brokersEnv`. Opcionais: `tls`, `sasl` (`none`/`plain`/`scram`), `userEnv`, `passwordEnv`. Options: `timeoutMs`, `maxRecords`, `maxScanRecords`.

### `connection.auth` (REST / ksql)

Obrigatório: `type`.

| `type` | Campos relevantes |
|--------|-------------------|
| `none` | nenhum |
| `bearer` | `tokenEnv` |
| `header` | `name` (nome do header), `valueEnv` |
| `basic` | `userEnv`, `passwordEnv` |

### `options` que o código Go usa (não estão enumeradas no schema)

| Chave | Aplica-se a | Padrão | Significado |
|-------|-------------|--------|-------------|
| `statementTimeoutMs` | postgres, mysql, aliases, mssql, clickhouse, sqlite | `limits.maxSourceMs` | Timeout do statement. Vale o **menor** entre `statementTimeoutMs`, `timeoutMs` e `maxSourceMs` |
| `timeoutMs` | SQL (mesma regra), REST, ksql | REST 10000, ksql 12000 | Timeout do cliente HTTP em REST e ksql. Em SQL é um segundo teto, como `statementTimeoutMs` |
| `resources` | REST, **obrigatório** para consultar | nenhum | Mapa de nome do recurso para operações `list` / `getById` (veja abaixo) |

Os demais tipos (mongodb, dynamodb, cassandra) não leem nenhuma chave de `options` hoje. Chaves desconhecidas são aceitas pelo schema e ignoradas pelo runtime, então um erro de digitação passa em silêncio.

Exemplo de `options.resources` para REST (o `binding.resource` do catálogo precisa ser uma chave deste mapa, por exemplo `users`):

```yaml
options:
  timeoutMs: 10000
  resources:
    users:
      list:
        method: GET
        path: /users
        queryParams: [email, limit, offset]
      getById:
        method: GET
        path: /users/{id}
```

O `from-openapi` gera esse mapa. Sem `resources`, o conector REST falha com `CONFIG_ERROR`.

### `resources` do REST em detalhe

Cada chave de `resources` é o nome de um recurso. Uma entidade do catálogo aponta para ele com `binding: { kind: rest_resource, resource: <nome> }`. Um recurso tem até duas operações, com o mesmo formato.

| Operação | Obrigatória | Para que serve |
|----------|-------------|----------------|
| `list` | sim* | Usada quando o `getById` não pode rodar (faltam path params). Obrigatória se alguma consulta não for por id |
| `getById` | não | Usada quando todo `{nome}` do `path` tem um filtro `eq`. O `from-openapi` gera a partir de caminhos com `{id}` |

Campos de cada operação (`list` e `getById`):

| Campo | Tipo | Padrão | Significado |
|-------|------|--------|-------------|
| `method` | string | `GET` | Método HTTP. Com `limits.readOnly: true` só `GET` e `HEAD` são aceitos; outro método falha na inicialização com `CONFIG_ERROR` |
| `path` | string | nenhum | Concatenado à URL base de `baseUrlEnv` (a `/` final da base é removida). Comece com `/`. `{nome}` é substituído pelos filtros `eq` no `getById` |
| `queryParams` | lista de strings | nenhum | Parâmetros de query que a API aceita. Documenta quais filtros existem; o `from-openapi` preenche. O runtime **não** valida |
| `itemsKey` | string | `data`/`items`/`results`/`users` (list) ou `data`/`item`/`result` (getById) | Chave JSON do array ou do objeto. Também vale no recurso |
| `maxPages` | int | 1 | Páginas por offset (`list`). Teto 20 |
| `pageSize` | int | o `limit` da consulta | Tamanho da página enviado no parâmetro de limit quando `maxPages` > 1 |
| `limitParam` | string | `limit` | Nome do parâmetro de tamanho de página |
| `offsetParam` | string | `offset` | Nome do parâmetro de offset |

Exemplo de `getById`: `WHERE id = '42'` com `path: /users/{id}` vira `GET /users/42`. Os outros `eq` ficam como query params. Se faltar um placeholder, o runtime usa `list`.

```sql
SELECT id, email FROM users WHERE id = '42' LIMIT 1
```

Como uma consulta vira requisição HTTP:

- **Colunas:** cada campo selecionado é lido do item da resposta pelo nome `physical`. Só chaves de primeiro nível são lidas; um `physical` com ponto, como `addr.city`, não retorna nada em REST. Campo com `fromFilter: true` é preenchido a partir de um `eq` de topo (ou `and` de `eq`) quando o corpo omite a chave; sem esse `eq` a consulta retorna `INVALID_IR`; valor diferente no corpo retorna `SOURCE_ERROR`.
- **`WHERE`:** só `eq` (e `eq` combinados com `and`) é enviado, como `?<campo>=<valor>` (ou path param no `getById`). O nome do parâmetro é o nome **lógico** do campo; mantenha nome lógico e físico iguais nos campos que você filtra. Outro operador (`neq`, `gt`, `in`, `contains`, …) não é empurrado ao conector REST e retorna `UNSUPPORTED` ali.
- **`LIMIT` / `OFFSET`:** enviados como `limitParam` / `offsetParam` (padrões `limit` e `offset`).
- **Paginação:** `maxPages: 1` (padrão) é uma requisição. Valores maiores andam o offset até página curta, limite de linhas ou 20 páginas.
- **Agregações:** nunca empurradas; rodam no DuckDB.
- **Formato da resposta:** array JSON ou objeto cuja `itemsKey` (ou as chaves padrão) contém o array. `getById` também aceita um objeto puro.
- **Erros:** status HTTP 400 ou maior retorna `SOURCE_ERROR`; resposta maior que `serve.maxRestResponseBytes` é recusada; estourar o timeout retorna `TIMEOUT`.

Exemplo completo com as duas operações:

```yaml
sources:
  - id: legacy_api
    type: rest
    connection:
      baseUrlEnv: QLLM_LEGACY_API_URL
      auth:
        type: bearer
        tokenEnv: QLLM_LEGACY_API_TOKEN
    options:
      timeoutMs: 8000
      resources:
        users:
          list:
            method: GET
            path: /users
            queryParams: [id, email, status]
          getById:
            method: GET
            path: /users/{id}
```

---

## `qllm.catalog.yaml`

Obrigatórios: `protocolVersion`, `project`, `entities` (mínimo 1).

### `entities[]`

Obrigatórios: `name`, `source`, `binding`, `fields` (mínimo 1 field).

| Campo | Tipo | Observações |
|-------|------|-------------|
| `name` | id lógico | `FROM name` / `from` do IR |
| `aliases` | array de ids, únicos | Nomes alternativos no IR |
| `description` | string | Texto para o agente |
| `source` | id | **Precisa** existir em `preset.sources[].id` |
| `binding` | object | Mapeamento físico |
| `primaryKey` | array de strings | Nomes lógicos de field |
| `fields` | array | |
| `relations` | array | Apenas dicas; não criam chaves estrangeiras |
| `scope` | `{ field, column? }` | D21: força `eq` em `column` (ou `field`) a partir da credencial |

### `binding`

Obrigatório: `kind`.

| `kind` | Também obrigatório | Uso |
|--------|--------------------|-----|
| `table` | `schema`, `table` | postgres/mysql/mssql/sqlite (`schema: main`)/clickhouse/dynamodb/cassandra/ksql |
| `collection` | `collection` | mongodb |
| `rest_resource` | `resource` | rest; uma chave de `options.resources` |
| `key` | `keyPattern`, `accessPath.partition` | redis (`user:{id}`) |
| `topic` | `topic`, `accessPath` (partition / `key` / timestamp) | kafka |

`schema` / `table` / `collection` / `resource`: `^[A-Za-z_][A-Za-z0-9_]*$`.

`accessPath` (object, chaves extras proibidas) para Dynamo, Cassandra e ksql:

| Campo | Tipo | Uso típico |
|-------|------|------------|
| `pk` / `partition` | array de strings | Nomes **lógicos** de field (igualdade obrigatória na consulta) |
| `sk` / `sort` | string | Sort key do Dynamo |
| `ksqlKey` | string | Pull query do ksql |

Sem a igualdade correta na consulta, o resultado é `UNSUPPORTED`.

### `fields[]`

Obrigatórios: `name`, `type`, `physical`.

| Campo | Valores |
|-------|---------|
| `name` | id lógico (`email`) |
| `type` | `string` `number` `boolean` `timestamp` `json`. `number` vira DOUBLE no DuckDB: ids acima de 2^53 devem ser `string` |
| `physical` | coluna ou chave; caminhos com ponto são permitidos (`addr.city`), exceto no REST (só chaves de topo) |
| `description` | string opcional |
| `fromFilter` | bool opcional; só REST. A API não devolve o campo; o qLLM copia o valor de um `eq` de topo (D20) |
| `shape` | texto livre opcional; só para `type: json`. Estrutura interna para o LLM (`{street, city}`, `string[]`). Aparece no `describe_catalog`. Não é validado (D22) |

### `relations[]`

Obrigatórios: `name`, `to`, `type`, `on`.

| Campo | Valores |
|-------|---------|
| `name` | rótulo (`customer`) |
| `to` | `entities[].name` de destino |
| `type` | `many_to_one` `one_to_many` `one_to_one` |
| `on` | array de pares `[campo_local, campo_remoto]`, mínimo 1 par |

---

## `qllm.config.yaml` (serve)

A raiz é opcional e aceita somente a chave `serve`. Tudo dentro de `serve` é opcional, mas se você definir `authTokenEnv`, essa variável de ambiente precisa existir e não pode estar vazia.

| Campo | Tipo | Padrão do runtime |
|-------|------|-------------------|
| `addr` | string | `127.0.0.1:8088` |
| `mcpAddr` | string | `127.0.0.1:8089` |
| `authTokenEnv` | string | nenhum (sem Bearer) |
| `insecureBind` | bool | `false` |
| `maxBodyBytes` | int ≥ 1024 | 1048576 (1 MiB) |
| `maxRestResponseBytes` | int ≥ 1024 | 10485760 (10 MiB) |
| `cors.origins` | array de strings | `[]` = CORS desativado; sem `*` |
| `cors.allowHeaders` | array | |
| `cors.allowMethods` | array | |

Flags da CLI que **sobrescrevem** o arquivo: `--addr`, `--mcp-addr`, `--auth-token-env`, `--insecure-bind`, `--cors-origin`.

---

## `qllm.access.yaml`

Obrigatório: `apps` (mínimo 1). Cada app precisa de `name`, `tables` e exatamente um de `key` ou `keySecret`. Uma entrada por **tipo** de app, não por usuário.

| Campo | Observações |
|-------|-------------|
| `scopeMode` | No arquivo: `reject` (padrão) ou `inject` |
| `name` | ID do app (`--app` / `QLLM_APP`). Com `keySecret`, `[a-z][a-z0-9_-]*` |
| `key` | Bearer estático; literal **ou** exatamente `${ENV_NAME}` |
| `keySecret` | Valida chaves derivadas `app.scopeValue.expiry.hmac` |
| `scope` | Template `{ field: user_id }` ou estático `{ user_id: "acme" }` |
| `tables` | `entities[].name` permitidos, ou `*` |
| `unscopedTables` | Tabelas compartilhadas obrigatórias quando o app tem `scope` |

---

## `qllm.env.yaml`

Obrigatório: `env` (object com pelo menos 1 chave).

| | |
|--|--|
| nomes das chaves | `^[A-Za-z_][A-Za-z0-9_]*$` |
| valores | string ≥ 1; literal ou `${OUTRA_ENV}` |
| já definida no processo | **não** é sobrescrita |

Não use este arquivo para segredos versionados no git. No Kubernetes, use um Secret.

---

## Corpo de `POST /v1/sql` / `execute_sql`

Obrigatório: `sql`. Opcional: `version` (`"1"` congelado; omitido ou `"2"` é o mais recente).

---

## Query IR (campos)

Obrigatórios: `from`, `select`. Veja [queries.md](queries.md) e [`query-ir.schema.json`](../../planning/schemas/query-ir.schema.json).

| Campo | Observações |
|-------|-------------|
| `protocolVersion` | opcional na requisição |
| `from` | entidade ou alias |
| `as` | alias local |
| `joins[]` | `type`: `inner`\|`left`; `from`; `as?`; `on[]` `{left,right}` |
| `select[]` | string de field **ou** `{agg, field?, as}`; `agg`: `count` `sum` `avg` `min` `max` |
| `where` | `{op,args}` ou comparação `{field,op,value?}` |
| `groupBy` | referências de field |
| `orderBy[]` | `{field, dir?}` com `asc`\|`desc` |
| `limit` / `offset` | inteiros |
| `mode` | `sync` \| `async` |

`op` de comparação: `eq` `neq` `gt` `gte` `lt` `lte` `in` `nin` `contains` `is_null` `not_null`.

`op` lógico: `and` `or`. O `not` recebe exatamente um elemento em `args`.

---

## Flags da CLI (resumo)

Descritas em [cli.md](cli.md). O catálogo não tem "tags" em YAML além de `protocolVersion` e dos enums `type`/`kind` acima. A tag de **compilação** `-tags duckdb` pertence ao Go ([build.md](build.md)), não a um arquivo do qllm.
