# Referência de campos (tudo o que o schema aceita)

Fonte: [`planning/schemas/`](../planning/schemas/). Objectos com `additionalProperties: false` **rejeitam** chaves extra.

Convenção `*Env`: string = **nome** da variável de ambiente (`QLLM_FOO`), nunca a senha.

Identificadores lógicos (`sources[].id`, `entities[].name`, aliases, field `name`, IR `from`/`as`): `^[a-z][a-z0-9_]*$`.

---

## `qllm.project.yaml` (opcional)

Obrigatório: `protocolVersion`, `preset`, `catalog`.

| Campo | Tipo | Notas |
|-------|------|--------|
| `protocolVersion` | string semver | `0.1.0` / `0.2.0` |
| `preset` | string | Path relativo a **este** ficheiro |
| `catalog` | string | Idem |

---

## `qllm.preset.yaml`

Obrigatório na raiz: `protocolVersion`, `project`, `limits`, `sources` (mín. 1).

| Campo | Tipo | Notas |
|-------|------|--------|
| `protocolVersion` | semver | |
| `project` | string não vazia | Nome lógico do projecto; deve bater com o catalog |
| `limits` | object | **Todos** os 5 campos obrigatórios |
| `sources` | array | |

### `limits` (todos obrigatórios)

| Campo | Tipo | Intervalo |
|-------|------|-----------|
| `maxSyncMs` | int | 100–60000 — budget total da query |
| `maxSourceMs` | int | 100–60000 — por ida à fonte |
| `defaultLimit` | int | ≥ 1 — se o IR/SQL omitir LIMIT |
| `maxLimit` | int | ≥ 1 — teto |
| `readOnly` | bool | tem de ser `true` no produto |

### `sources[]` (cada item)

Obrigatório: `id`, `type`, `connection`.

| Campo | Tipo | Valores |
|-------|------|---------|
| `id` | string | `crm_pg`, `legacy_api`, … |
| `type` | enum | `postgres` `mysql` `mongodb` `rest` `mssql` `sqlite` `clickhouse` `dynamodb` `cassandra` `ksql` |
| `connection` | object | forma **consoante** `type` (abaixo). Chaves a mais → erro |
| `options` | object | livre no schema JSON; o runtime lê só o que conhece (abaixo) |

### `connection` por `type`

**postgres** e **mysql** (`sqlConnection`) — obrigatório: `hostEnv`, `port`, `database`, `userEnv`, `passwordEnv`.

| Campo | Tipo | Notas |
|-------|------|--------|
| `hostEnv` | string | |
| `port` | int | 1–65535 |
| `database` | string | |
| `userEnv` | string | |
| `passwordEnv` | string | |
| `sslMode` | enum opcional | `disable` `require` `verify-ca` `verify-full` (postgres; mysql ignora se não usar) |

**mssql** — mesmos obrigatórios. Extra opcional: `encrypt`: `true` \| `false` \| `disable`. Sem `sslMode`.

**clickhouse** — mesmos obrigatórios. Extra opcional: `secure` (bool).

**sqlite** — só `pathEnv` (path do ficheiro `.db` na env).

**mongodb** — obrigatório: `uriEnv`, `database`.

**rest** e **ksql** — obrigatório: `baseUrlEnv`. Opcional `auth` (abaixo).

**dynamodb** — obrigatório: `region` (literal, ex. `us-east-1`). Opcional `endpointEnv` (Dynamo Local). Credenciais AWS = chain do processo, não campos no YAML.

**cassandra** — obrigatório no schema: `keyspace`. Na prática o código usa `hostsEnv` (lista) **ou** `hostEnv`. Opcionais: `port`, `userEnv`, `passwordEnv`.

### `connection.auth` (REST / ksql)

Obrigatório: `type`.

| `type` | Campos que importam |
|--------|---------------------|
| `none` | — |
| `bearer` | `tokenEnv` |
| `header` | `name` (nome do header), `valueEnv` |
| `basic` | `userEnv`, `passwordEnv` |

### `options` que o Go usa (não estão enum no schema)

| Chave | Tipos | Significado |
|-------|-------|-------------|
| `statementTimeoutMs` | SQL | timeout do statement; cap com `maxSourceMs` |
| `timeoutMs` | SQL (fallback), REST, ksql | timeout HTTP/cliente |
| `resources` | REST **obrigatório** para query | mapa nome → `list` / `getById` |

Exemplo REST `options.resources` (o catalog `binding.resource` tem de ser uma chave deste mapa, ex. `users`):

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

`from-openapi` gera este mapa. Sem `resources`, o connector REST falha com `CONFIG_ERROR`.

---

## `qllm.catalog.yaml`

Obrigatório: `protocolVersion`, `project`, `entities` (mín. 1).

### `entities[]`

Obrigatório: `name`, `source`, `binding`, `fields` (mín. 1 field).

| Campo | Tipo | Notas |
|-------|------|--------|
| `name` | id lógico | `FROM name` / IR `from` |
| `aliases` | array de ids, unique | nomes alternativos no IR |
| `description` | string | texto para o agente |
| `source` | id | **tem** de existir em `preset.sources[].id` |
| `binding` | object | físico |
| `primaryKey` | array de strings | nomes lógicos de field |
| `fields` | array | |
| `relations` | array | hints; não criam FK |

### `binding`

Obrigatório: `kind`.

| `kind` | Também obrigatório | Uso |
|--------|--------------------|-----|
| `table` | `schema`, `table` | postgres/mysql/mssql/sqlite (`schema: main`)/clickhouse/dynamodb/cassandra/ksql |
| `collection` | `collection` | mongodb |
| `rest_resource` | `resource` | rest — chave em `options.resources` |

`schema` / `table` / `collection` / `resource`: `^[A-Za-z_][A-Za-z0-9_]*$`.

`accessPath` (object, chaves extra proibidas) — Dynamo/Cassandra/ksql:

| Campo | Tipo | Uso típico |
|-------|------|------------|
| `pk` / `partition` | array de strings | nomes **lógicos** de field (eq obrigatória na query) |
| `sk` / `sort` | string | sort key Dynamo |
| `ksqlKey` | string | pull ksql |

Sem a igualdade certa na query → `UNSUPPORTED`.

### `fields[]`

Obrigatório: `name`, `type`, `physical`.

| Campo | Valores |
|-------|---------|
| `name` | id lógico (`email`) |
| `type` | `string` `number` `boolean` `timestamp` `json` |
| `physical` | coluna/chave; path com pontos permitido (`addr.city`) |
| `description` | string opcional |

### `relations[]`

Obrigatório: `name`, `to`, `type`, `on`.

| Campo | Valores |
|-------|---------|
| `name` | rótulo (`customer`) |
| `to` | `entities[].name` destino |
| `type` | `many_to_one` `one_to_many` `one_to_one` |
| `on` | array de pares `[campo_daqui, campo_de_la]`, mín. 1 par |

---

## `qllm.config.yaml` (serve)

Raiz opcional; só chave `serve`. Tudo opcional **dentro** de `serve`, mas se pões `authTokenEnv` a env tem de existir e ser não vazia.

| Campo | Tipo | Default runtime |
|-------|------|-----------------|
| `addr` | string | `127.0.0.1:8088` |
| `mcpAddr` | string | `127.0.0.1:8089` |
| `authTokenEnv` | string | — (sem Bearer) |
| `insecureBind` | bool | `false` |
| `maxBodyBytes` | int ≥ 1024 | 1048576 (1 MiB) |
| `maxRestResponseBytes` | int ≥ 1024 | 10485760 (10 MiB) |
| `cors.origins` | array de strings | `[]` = CORS off; sem `*` |
| `cors.allowHeaders` | array | |
| `cors.allowMethods` | array | |

Flags CLI que **ganham** ao ficheiro: `--addr`, `--mcp-addr`, `--auth-token-env`, `--insecure-bind`, `--cors-origin`.

---

## `qllm.access.yaml`

Obrigatório: `apps` (mín. 1). Cada app: `name`, `key`, `tables` (mín. 1).

| Campo | Notas |
|-------|--------|
| `name` | id do app (`--app` / `QLLM_APP`) |
| `key` | literal **ou** exactamente `${ENV_NAME}` |
| `tables` | `entities[].name` permitidos |

---

## `qllm.env.yaml`

Obrigatório: `env` (object, mín. 1 chave).

| | |
|--|--|
| nomes das keys | `^[A-Za-z_][A-Za-z0-9_]*$` |
| valores | string ≥ 1; literal ou `${OUTRA_ENV}` |
| processo já preenchido | **não** é sobrescrito |

Não uses isto para secrets em git. Em K8s: Secret.

---

## Body `POST /v1/sql` / `execute_sql`

Obrigatório: `sql`. Opcional: `version` (`"1"` congelado, omitido/`"2"` latest).

---

## Query IR (campos)

Obrigatório: `from`, `select`. Ver [queries.md](queries.md) e [`query-ir.schema.json`](../planning/schemas/query-ir.schema.json).

| Campo | |
|-------|--|
| `protocolVersion` | opcional no pedido |
| `from` | entity/alias |
| `as` | alias local |
| `joins[]` | `type`: `inner`\|`left`; `from`; `as?`; `on[]` `{left,right}` |
| `select[]` | string field **ou** `{agg, field?, as}` — `agg`: `count` `sum` `avg` `min` `max` |
| `where` | `{op,args}` / compare `{field,op,value?}` |
| `groupBy` | field refs |
| `orderBy[]` | `{field, dir?}` `asc`\|`desc` |
| `limit` / `offset` | ints |
| `mode` | `sync` \| `async` |

Compare `op`: `eq` `neq` `gt` `gte` `lt` `lte` `in` `nin` `contains` `is_null` `not_null`.

Logic `op`: `and` `or`. `not` tem `args` com 1 elemento.

---

## Flags CLI (resumo)

Já em [cli.md](cli.md). Não há “tags” de catalog no YAML além de `protocolVersion` e `type`/`kind` enums acima. A tag de **compilação** `-tags duckdb` é do Go ([build.md](build.md)), não de um ficheiro qllm.
