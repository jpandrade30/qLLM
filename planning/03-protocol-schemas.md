# 03 — Protocol Schemas (fonte da verdade)

`protocolVersion`: **0.2.0** (additive; **0.1.0** preset/catalog/IR files remain valid). Experimental source types: `mssql`, `sqlite`, `clickhouse`, `dynamodb`, `cassandra`, `ksql` (pull), `redis`, `kafka`, plus MySQL-wire aliases (`mariadb`, `tidb`, `vitess`, `aurora_mysql`, `planetscale`) and Postgres-wire aliases (`cockroach`, `yugabyte`, `alloydb`, `aurora_postgres`, `neon`, `supabase`, `timescale`, `redshift`). No harness coverage for experimental types.

Todos os exemplos abaixo são normativos para o MVP. JSON Schema máquina-legível: [`schemas/`](schemas/).

---

## 0. Project layout — como o executável lê as specs

As specs de **onde estão os bancos/APIs** e **o que se pode consultar** são YAML no `--config-dir` (não hardcode no binário). `fixtures/` deste repo (seed, test-api, queries) **não** é catalog. Neste repo o config-dir da imagem é [`deploy/image/config`](../deploy/image/config/).

### Arquivos

| Arquivo | Obrigatório | Conteúdo |
|---------|-------------|----------|
| `qllm.preset.yaml` (ou `.json`) | sim | sources, connection/`*Env`, limits |
| `qllm.catalog.yaml` (ou `.json`) | sim | entidades lógicas → source + binding + fields |
| `qllm.project.yaml` (ou `.json`) | não | ponteiro para preset/catalog |
| `qllm.config.yaml` (ou `.json`) | não | serve: bind, authTokenEnv, CORS, body caps (D14) |
| `qllm.access.yaml` (ou `.json`) | não | apps, keys, tabelas (D16); ausente = Bearer único / catalog inteiro |
| `qllm.env.yaml` (ou `.json`) | não | mapa `env:` nome→valor ou `${VAR}`; preenche variáveis **vazias** no processo. Env já setado (K8s Secret) ganha; `${MISSING}` não grava o placeholder. |

### `qllm.project.yaml` (opcional)

```yaml
protocolVersion: "0.1.0"
preset: ./qllm.preset.yaml
catalog: ./qllm.catalog.yaml
# paths relativos a este arquivo
```

JSON Schema: [`schemas/project.schema.json`](schemas/project.schema.json).

### Como o binário resolve

Ordem de precedência:

1. `--preset PATH` **e** `--catalog PATH`
2. `--project PATH` → lê `preset`/`catalog` do project file
3. `--config-dir DIR` → `DIR/qllm.preset.{yaml,yml,json}` + `DIR/qllm.catalog.{yaml,yml,json}` (+ opcional `qllm.config.*`, `qllm.access.*`, `qllm.env.*`)
4. Working directory atual com os nomes default `qllm.preset.*` + `qllm.catalog.*`

```bash
qllm serve --http --config-dir ./config
qllm validate --project ./qllm.project.yaml
qllm query --preset ./p.yaml --catalog ./c.yaml -f ./ir.json
```

- `serve`: carrega preset+catalog **no startup**; falha rápido se inválidos (`CONFIG_ERROR`). Sem arquivos, **não** usa `fixtures/` embutido. Runtime serve settings: defaults → `qllm.config.*` → flags CLI.
- Segredos continuam só via env (`passwordEnv`, `uriEnv`, `authTokenEnv`, …).
- Neste repo, o config-dir da imagem/harness é `deploy/image/config/` (preset, catalog, access, serve, env). `fixtures/` não entra. `go run` sem YAML → `CONFIG_ERROR`.

### Authoring CLI (não é runtime de query)

Rascunhos de catalog (humano revisa relations/aliases):

```bash
./qllm catalog introspect --source crm_pg --config-dir ./deploy/image/config --out ./deploy/image/config/qllm.catalog.yaml
qllm catalog from-openapi -f other-team.yaml --source legacy_api --config-dir ./config --out ./generated.catalog.yaml --resources-out ./generated.resources.yaml
```

- `introspect`: Postgres/MySQL via `information_schema` usando `*Env` do preset. Nomes lógicos = nomes de tabela; `physical` = colunas; PK se visível.
- `from-openapi`: paths GET listáveis → entities `rest_resource` + fragmento `options.resources`. Não substitui o connector REST.
- Credenciais iguais ao serve. Hosts vêm do env do **projeto alvo**, não de DNS de compose hardcoded no binário.

### `qllm.config.yaml` (opcional — serve/runtime)

Não faz parte do data-plane Query IR. JSON Schema: [`schemas/runtime-config.schema.json`](schemas/runtime-config.schema.json).

```yaml
serve:
  addr: "127.0.0.1:8088"
  mcpAddr: "127.0.0.1:8089"
  authTokenEnv: "QLLM_AUTH_TOKEN"
  insecureBind: false
  maxBodyBytes: 1048576
  maxRestResponseBytes: 10485760
  cors:
    origins: []   # vazio = CORS desligado; "*" rejeitado
```

Defaults seguros: loopback, CORS off. Se `authTokenEnv` estiver definido, o env **deve** resolver token não-vazio (senão `CONFIG_ERROR` no startup). Bind não-loopback sem auth exige `insecureBind: true` / `--insecure-bind`.

### `qllm.access.yaml` (opcional — ACL por app)

JSON Schema: [`schemas/access.schema.json`](schemas/access.schema.json). Arquivo presente **substitui** o Bearer único.

```yaml
scopeMode: reject   # reject | inject (default reject)
apps:
  - name: mobile
    keySecret: ${QLLM_MOBILE_SECRET}
    tables: [orders, profile, products]
    unscopedTables: [products]
    scope:
      field: user_id
  - name: partner-acme
    key: ${QLLM_ACME_KEY}
    tables: [orders]
    scope:
      user_id: "acme"
  - name: crm-agent
    key: ${QLLM_CRM_AGENT_KEY}
    tables: [customers, addresses, invoices]
```

- Uma entrada por **tipo de app**, não por usuário (D21). `key` **ou** `keySecret` (não os dois). `key` / `keySecret`: literal ou `${ENV_NAME}`.
- Chave derivada (template com `keySecret`): `app.scopeValue.expiryUnix.hmac` onde `hmac` é Base64URL(HMAC-SHA256(keySecret, `app.scopeValue.expiryUnix`)). `scopeValue` ∈ `[A-Za-z0-9_-]`, ≤128. O backend mint; o qLLM verifica HMAC e expiração.
- `scope.field` no app = nome do valor na chave. `entities[].scope.field` (e `column` se o nome físico/lógico diferir) marca a coluna filtrada.
- `tables: ["*"]` = todas as entidades do catalog. `unscopedTables` = tabelas compartilhadas exigidas quando o app tem `scope`.
- HTTP/MCP HTTP: `Authorization: Bearer` (key estática ou derivada). MCP stdio / CLI: `--app` / `QLLM_APP` e, se o app for template, `--scope` / `QLLM_SCOPE` (`42` ou `user_id=42`).
- `execute_sql` e o IR **não** ganham campo de constraint.

### `qllm.env.yaml` (opcional — seed de env)

JSON Schema: [`schemas/env-file.schema.json`](schemas/env-file.schema.json). Valores para `*Env` do preset / `authTokenEnv` sem `ENV` no Dockerfile. Secrets: exatamente `${ENV_NAME}` (mesmo token que access `key`); ApplyEnvFile lê `os.Getenv(NAME)` e **não** grava a string `${NAME}` se estiver vazia. Literais continuam válidos (hosts DNS). Placeholder malformado (`${}`, `${A}suffix`) → `CONFIG_ERROR`.

```yaml
env:
  QLLM_AUTH_TOKEN: ${QLLM_AUTH_TOKEN}
  QLLM_CRM_PG_HOST: postgres
  QLLM_CRM_PG_USER: qllm
  QLLM_CRM_PG_PASSWORD: ${QLLM_CRM_PG_PASSWORD}
```

Processo já tem a variável não-vazia (Secret / compose `environment:`) → YAML **não** sobrescreve. Valores nunca vão para log. `protocolVersion` inalterado.

---

## 0.1 Naming & aliases — 10 REST com o mesmo `email`

**Problema:** várias APIs/tabelas expõem `id`, `email`, `name`.  
**Solução:** nunca citar só o campo físico. Sempre há três camadas:

| Camada | Exemplo | Função |
|--------|---------|--------|
| `sources[].id` | `rest_crm`, `rest_erp` | conexão/auth distintas |
| `entities[].name` | `crm_users`, `erp_users` | o que o IR consulta (único no catalog) |
| alias de query `as` | `cu`, `eu` | nome curto **nessa** query |

Campo físico pode ser idêntico (`physical: email`); o FieldRef no IR usa **entity ou alias**:

- `crm_users.email` vs `erp_users.email`
- ou, com alias: `cu.email` vs `eu.email`

### Catalog: nomes únicos (+ aliases opcionais de entidade)

```yaml
entities:
  - name: crm_users          # único — é o identificador canônico
    aliases: [users_crm]      # opcional: nomes alternativos no IR
    source: rest_crm
    binding: { kind: rest_resource, resource: users }
    fields:
      - { name: id, type: string, physical: id }
      - { name: email, type: string, physical: email }

  - name: erp_users
    source: rest_erp
    binding: { kind: rest_resource, resource: users }
    fields:
      - { name: id, type: string, physical: id }
      - { name: email, type: string, physical: email }
```

Regras:

- `entities[].name` **globalmente único** no catalog.
- `aliases[]` também únicos e não podem colidir com outro `name`/alias.
- Dois resources “users” em APIs diferentes ⇒ **dois entity names** (não reutilizar `users`).
- `describe_catalog` devolve `name`, `aliases`, `source`, fields — o agente escolhe o nome certo.

### IR: alias de query (`as`)

```json
{
  "protocolVersion": "0.1.0",
  "from": "crm_users",
  "as": "cu",
  "joins": [
    {
      "type": "left",
      "from": "erp_users",
      "as": "eu",
      "on": [{ "left": "cu.email", "right": "eu.email" }]
    }
  ],
  "select": ["cu.id", "cu.email", "eu.id"],
  "limit": 50
}
```

Regras de binding na query:

1. Cada `from`/`joins[]` introduz um **binding name** = `as` se presente, senão `entity.name` (ou alias de catalog usado no `from`).
2. Binding names únicos na query (`AMBIGUOUS_ALIAS` se repetir).
3. Com **mais de uma** entidade no escopo, FieldRef **deve** ser qualificado (`binding.field`). Não qualificado ⇒ `AMBIGUOUS_FIELD`.
4. Com uma entidade só, `email` curto continua válido.
5. `as` no agg (`{ "agg": "sum", "as": "revenue" }`) é alias de **coluna de saída**, não de entidade.

---

## 1. Project Preset

Arquivo típico: `qllm.preset.yaml` ou `qllm.preset.json` (YAML vira JSON na carga). Ver § 0 para discovery.

### Semântica

- Um preset = um projeto/ambiente.
- `sources[]`: N fontes; `id` único e estável (referenciado pelo catalog). Várias REST ⇒ vários `id` (`rest_crm`, `rest_erp`, …).
- Segredos: preferir env (`passwordEnv`) — não commit de senha.

### Exemplo

```yaml
protocolVersion: "0.1.0"
project: acme-billing
limits:
  maxSyncMs: 15000
  maxSourceMs: 12000
  defaultLimit: 100
  maxLimit: 1000
  readOnly: true
sources:
  - id: crm_pg
    type: postgres
    connection:
      hostEnv: QLLM_CRM_PG_HOST
      port: 5432
      database: crm
      userEnv: QLLM_CRM_PG_USER
      passwordEnv: QLLM_CRM_PG_PASSWORD
      sslMode: disable
    options:
      statementTimeoutMs: 12000

  - id: billing_mysql
    type: mysql
    connection:
      hostEnv: QLLM_BILLING_MYSQL_HOST
      port: 3306
      database: billing
      userEnv: QLLM_BILLING_MYSQL_USER
      passwordEnv: QLLM_BILLING_MYSQL_PASSWORD

  - id: events_mongo
    type: mongodb
    connection:
      uriEnv: QLLM_EVENTS_MONGO_URI
      database: events

  - id: legacy_api
    type: rest
    connection:
      baseUrlEnv: QLLM_LEGACY_API_BASE_URL
      auth:
        type: bearer
        tokenEnv: QLLM_LEGACY_API_TOKEN
    options:
      timeoutMs: 10000

  # Experimental (0.2.0) — no compose/goldens in this repo
  - id: local_sqlite
    type: sqlite
    connection:
      pathEnv: QLLM_SQLITE_PATH
  - id: warehouse_ch
    type: clickhouse
    connection:
      hostEnv: QLLM_CH_HOST
      port: 9000
      database: default
      userEnv: QLLM_CH_USER
      passwordEnv: QLLM_CH_PASSWORD
  - id: app_mssql
    type: mssql
    connection:
      hostEnv: QLLM_MSSQL_HOST
      port: 1433
      database: app
      userEnv: QLLM_MSSQL_USER
      passwordEnv: QLLM_MSSQL_PASSWORD
      encrypt: "true"
  - id: items_ddb
    type: dynamodb
    connection:
      region: us-east-1
      endpointEnv: QLLM_DDB_ENDPOINT
  - id: events_cql
    type: cassandra
    connection:
      hostEnv: QLLM_CASSANDRA_HOST
      keyspace: events
      userEnv: QLLM_CASSANDRA_USER
      passwordEnv: QLLM_CASSANDRA_PASSWORD
  - id: ksql_pull
    type: ksql
    connection:
      baseUrlEnv: QLLM_KSQL_URL
```

### Campos obrigatórios

| Campo | Tipo | Notas |
|-------|------|-------|
| `protocolVersion` | string | semver |
| `project` | string | nome lógico |
| `limits` | object | ver abaixo |
| `sources` | array | min 1 |
| `sources[].id` | string | `[a-z][a-z0-9_]*` |
| `sources[].type` | enum | `postgres` \| `mysql` \| `mongodb` \| `rest` \| `mssql` \| `sqlite` \| `clickhouse` \| `dynamodb` \| `cassandra` \| `ksql` \| `redis` \| `kafka` \| MySQL-wire aliases (`mariadb`, `tidb`, `vitess`, `aurora_mysql`, `planetscale`) \| Postgres-wire aliases (`cockroach`, `yugabyte`, `alloydb`, `aurora_postgres`, `neon`, `supabase`, `timescale`, `redshift`) |
| `sources[].connection` | object | por tipo (ver schemas) |

### `limits`

| Campo | Default | Significado |
|-------|---------|-------------|
| `maxSyncMs` | 15000 | budget total |
| `maxSourceMs` | 12000 | por round-trip de fonte |
| `defaultLimit` | 100 | se IR omitir limit |
| `maxLimit` | 1000 | teto absoluto |
| `readOnly` | true | bloqueia writes |

---

## 2. Catalog

Arquivo: `qllm.catalog.yaml` / `.json`. Descreve **entidades lógicas** que o agente pode consultar.

### Exemplo

```yaml
protocolVersion: "0.1.0"
project: acme-billing
entities:
  - name: customers
    description: Clientes do CRM
    source: crm_pg
    binding:
      kind: table
      schema: public
      table: customers
    primaryKey: [id]
    fields:
      - name: id
        type: string
        physical: id
      - name: email
        type: string
        physical: email
      - name: created_at
        type: timestamp
        physical: created_at

  - name: invoices
    description: Faturas
    source: billing_mysql
    binding:
      kind: table
      schema: billing
      table: invoices
    primaryKey: [id]
    fields:
      - name: id
        type: string
        physical: id
      - name: customer_id
        type: string
        physical: customer_id
      - name: total
        type: number
        physical: total_cents
        transform: cents_to_decimal   # opcional v1.1; v0.1 pode omitir
      - name: status
        type: string
        physical: status
    relations:
      - name: customer
        to: customers
        type: many_to_one
        on: [[customer_id, id]]

  - name: kv_items
    source: items_ddb
    binding:
      kind: table
      schema: main
      table: items
      accessPath:
        pk: [pk]
        sk: sk
    fields:
      - name: pk
        type: string
        physical: pk
      - name: sk
        type: string
        physical: sk

  - name: events
    source: events_mongo
    binding:
      kind: collection
      collection: app_events
    fields:
      - name: id
        type: string
        physical: _id
      - name: customer_id
        type: string
        physical: customerId
      - name: type
        type: string
        physical: type
      - name: ts
        type: timestamp
        physical: ts

  - name: legacy_users
    source: legacy_api
    binding:
      kind: rest_resource
      resource: users
    fields:
      - name: id
        type: string
        physical: id
      - name: email
        type: string
        physical: email
```

### Regras

- `entities[].name` único; identificador canônico em `IR.from` / joins (ver § 0.1).
- `entities[].aliases` opcional; cada alias único no catalog; também resolvível no `from`.
- `source` deve existir no preset (`sources[].id`).
- `fields[].name` = nome lógico na entidade; `physical` = coluna/path na fonte (pode repetir entre entidades).
- `fields[].fromFilter` (bool, opcional, D20): só em entidades cuja fonte é `rest`. A API não devolve o campo; o runtime preenche com o `eq` de topo do WHERE. Sem `eq` → `INVALID_IR`. Corpo com valor diferente → `SOURCE_ERROR`. `or`/`not` não alimentam o valor.
- `entities[].scope` (D21): `{ field, column? }`. O runtime força `eq` na coluna (`column` ou `field`) com o valor da credencial. Sem `scope` a entidade não é protegida.
- Tipos lógicos v0.1: `string` | `number` | `boolean` | `timestamp` | `json`
- `relations` são **hints** para o agente e para joins no IR; não criam FK automática no banco.

### REST no catalog

Resources REST detalhados ficam no preset ou em `fixtures` OpenAPI referenciado:

```yaml
# trecho no preset sources[].options.resources (REST)
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

IR para REST no MVP: filter/project/limit mapeáveis a query params; agg/join → DuckDB local após fetch limitado.

Quando a API recebe a chave no path/query e omite no JSON (`{"saldo":5300}`), marque o campo com `fromFilter: true` para o runtime ecoar o `eq` na linha (necessário para `GROUP BY` / join nessa chave).

---

## 3. Query IR

Contrato de `POST /v1/queries` e CLI `qllm query` (não é tool MCP).

### Exemplo single-source

```json
{
  "protocolVersion": "0.1.0",
  "from": "invoices",
  "select": [
    "customer_id",
    { "agg": "sum", "field": "total", "as": "revenue" },
    { "agg": "count", "as": "n" }
  ],
  "where": {
    "op": "and",
    "args": [
      { "field": "status", "op": "eq", "value": "paid" },
      { "field": "total", "op": "gte", "value": 10 }
    ]
  },
  "groupBy": ["customer_id"],
  "orderBy": [{ "field": "revenue", "dir": "desc" }],
  "limit": 50
}
```

### Exemplo join (local ou pushdown se mesma fonte)

```json
{
  "protocolVersion": "0.1.0",
  "from": "invoices",
  "as": "inv",
  "joins": [
    {
      "type": "left",
      "from": "customers",
      "as": "c",
      "on": [{ "left": "inv.customer_id", "right": "c.id" }]
    }
  ],
  "select": ["inv.id", "c.email", "inv.total"],
  "where": {
    "field": "inv.status",
    "op": "eq",
    "value": "paid"
  },
  "limit": 100
}
```

### Forma gramatical (v0.1)

```text
Query =
  protocolVersion?
  from: EntityRef          # name canônico ou alias de catalog
  as?: BindingName         # alias só desta query
  joins?: Join[]
  select: (FieldRef | AggExpr)[]
  where?: BoolExpr
  groupBy?: FieldRef[]
  orderBy?: OrderExpr[]
  limit: number            # se omitido, defaultLimit do preset
  offset?: number
  mode?: "sync" | "async"  # default sync

Join = { type: "inner"|"left", from: EntityRef, as?: BindingName, on: EqualOn[] }
EqualOn = { left: FieldRef, right: FieldRef }

EntityRef = string         # entities[].name ou entities[].aliases[]
BindingName = string       # [a-z][a-z0-9_]*
FieldRef = string          # "field" | "binding.field" | "entity.field"

AggExpr = { agg: "count"|"sum"|"avg"|"min"|"max", field?: FieldRef, as: string }

BoolExpr =
  | { field, op, value }
  | { op: "and"|"or", args: BoolExpr[] }
  | { op: "not", args: [BoolExpr] }

CompareOp = eq|neq|gt|gte|lt|lte|in|nin|contains|is_null|not_null
```

### Regras de validação

1. Toda entidade em `from`/`joins` resolve via `name` ou `aliases` do catalog e está allowlisted.
2. Binding names (`as` ou entity name) únicos na query.
3. Todo campo referenciado ∈ fields da entidade do binding; com >1 entidade, FieldRef deve ser qualificado.
4. `limit` ∈ (0, maxLimit].
5. Se houver `agg` sem `groupBy`, só aggs globais permitidas (uma linha).
6. `select` com mistura de field cru + agg exige `groupBy` contendo os fields crus.
7. Ops não suportadas pela fonte → planner marca local ou rejeita com `UNSUPPORTED`.
8. `readOnly: true` → IR não tem mutação (v0.1 nem define mutate).

### Agent contract (LLM)

Runtime fonte: **`GET /v1/howtouseme`** (HTTP) ou tool MCP **`how_to_use_me`**. Agentes devem chamar isso **antes** de montar IR.

**Never**

- Gerar Mongo / URLs REST como API do agente.
- Inventar entity/field/FK — só o catalog carregado (vazio se o YAML não tiver entidades).
- Escrever `where` como `{"and":[...]}` — forma canônica: `{"op":"and","args":[...]}`.
- Usar operadores SQL (`=`, `>=`, `LIKE`) — usar `eq`, `gte`, `contains`, …
- Colocar HAVING/UNION/CASE/subquery no Query IR — usar `execute_sql` (ver [`07-sql-dialect.md`](07-sql-dialect.md)).

**Where — válido vs inválido**

```json
// válido
{ "op": "and", "args": [
  { "field": "status", "op": "eq", "value": "paid" },
  { "field": "total", "op": "gte", "value": 10 }
]}

// inválido (shape Mongo) — o runtime rejeita com mensagem prescritiva
{ "and": [ { "field": "status", "op": "eq", "value": "paid" } ] }
```

**FieldRef**

- 1 entidade: bare OK (`email`).
- Com joins / `as`: preferir **sempre** `binding.field` (`c.id`). Não misturar `customers.id` e `c.id` na mesma query.

**groupBy:** todo campo bare no `select` que não é agg **deve** aparecer em `groupBy`.

---

## 4. API HTTP

Base path: `/v1`

### 4.1 `GET /v1/health`

```json
{ "ok": true, "protocolVersion": "0.2.0" }
```

### 4.2 `GET /v1/howtouseme`

Contrato fechado para LLM/agente (mesmo payload da tool MCP `how_to_use_me`):

- `workflow`, `never`, `notSupported`
- `where` (shapes canônicas + anti-exemplo `{and:[…]}` → fix `{op,args}`)
- `fieldRefRules`, `joinRules`, `aggregateRules`, `orderByRules`, `grammar`
- `queryIR` + **`sql`** (dialeto latest `"2"`: rules, supported, dialect2Only, reject, examples)
- `examples` (Query IR) + `invalidExamples`
- `project` (entityNames, limits)

Chamar **antes** de inventar queries; depois `GET /v1/catalog` para fields/relations. SQL rico → `execute_sql` / `POST /v1/sql` (ver `sql.examples`). Inventário: [`07-sql-dialect.md`](07-sql-dialect.md).

### 4.3 `GET /v1/catalog`

Resposta: catalog efetivo (entities + fields + relations + capabilities resumidas por source).

```json
{
  "protocolVersion": "0.1.0",
  "project": "acme-billing",
  "entities": [ /* igual ao catalog, possivelmente enriquecido */ ],
  "sources": [
    {
      "id": "crm_pg",
      "type": "postgres",
      "capabilities": {
        "filter": true,
        "project": true,
        "agg": true,
        "groupBy": true,
        "joinSameSource": true,
        "orderBy": true,
        "limit": true
      }
    }
  ]
}
```

### 4.4 `POST /v1/queries`

Request body = Query IR.

**Sync success `200`:**

```json
{
  "protocolVersion": "0.1.0",
  "queryId": "01HZX…",
  "status": "succeeded",
  "result": {
    "columns": [
      { "name": "customer_id", "type": "string" },
      { "name": "revenue", "type": "number" }
    ],
    "rows": [
      ["cust_1", 199.5],
      ["cust_2", 50]
    ],
    "rowCount": 2,
    "truncated": false
  },
  "meta": {
    "elapsedMs": 42,
    "mode": "sync",
    "plan": {
      "usedDuckDB": false,
      "steps": [
        { "source": "billing_mysql", "pushdown": true, "elapsedMs": 40 }
      ]
    }
  }
}
```

**Async accepted `202`:**

```json
{
  "protocolVersion": "0.1.0",
  "queryId": "01HZX…",
  "status": "accepted"
}
```

Nota: mesmo em async, o job respeita `maxSyncMs` / budget; status final será `succeeded` ou `failed` rapidamente.

### 4.4b `POST /v1/sql`

JSON Schema: [`schemas/sql-request.schema.json`](schemas/sql-request.schema.json).

```json
{ "version": "1", "sql": "SELECT c.id, i.total FROM customers c INNER JOIN invoices i ON i.customer_id = c.id LIMIT 100" }
```

- `version` opcional; omitido = dialeto mais novo (`"2"`). `"1"` e `"2"` suportados. Desconhecido = `UNSUPPORTED_VERSION`. Ver [`07-sql-dialect.md`](07-sql-dialect.md).
- Resposta tabular igual a `POST /v1/queries` (`query-response.schema.json`). `meta.app` = nome do app quando ACL está ativo.
- Dialeto `"1"`: uma statement `SELECT` (WITH, HAVING, DISTINCT, CASE, LIKE, BETWEEN, subquery, aritmética). Sem `UNION`/`INTERSECT`/`EXCEPT`/`QUALIFY`.
- Dialeto `"2"`: inclui set ops, `QUALIFY`, windows/`XOR`/`COUNT(DISTINCT …)` quando DuckDB aceita. Ambos recusam DML/DDL, multi-statement, schema físico, `read_csv` / `read_parquet` / `read_json` / `postgres_scan` / `httpfs` / `glob` e afins.
- Runtime: parser lista tabelas/colunas; connectors fazem scan (sem `WHERE` pushdown); DuckDB materializa nomes lógicos e executa o SQL com `enable_external_access=false`. Sem `LIMIT` no SELECT externo, aplica `defaultLimit`. `LIMIT` > `maxLimit` = `LIMIT_EXCEEDED`.
- Sem build `-tags duckdb`: `UNSUPPORTED`.

CLI: `qllm sql --config-dir … -f query.sql` (`--version` opcional).

### 4.5 `GET /v1/queries/{queryId}`

```json
{
  "protocolVersion": "0.1.0",
  "queryId": "01HZX…",
  "status": "running",
  "meta": { "elapsedMs": 800 }
}
```

Statuses: `accepted` | `running` | `succeeded` | `failed` | `canceled`

### 4.6 `GET /v1/queries/{queryId}/result`

- `200` + mesmo shape de `result` + `meta` se `succeeded`
- `409` se ainda não pronto
- `404` se expirado/desconhecido

### Representação tabular

- `columns[].name` / `columns[].type` (tipos lógicos)
- `rows` = array de arrays na ordem das columns (JSON scalars; timestamp = RFC3339 string)
- `null` JSON permitido
- `truncated: true` se bateu em maxLimit do runtime

---

## 5. Erros tipados

Todo erro de API:

```json
{
  "protocolVersion": "0.1.0",
  "error": {
    "code": "TIMEOUT",
    "message": "source billing_mysql exceeded 12000ms",
    "details": {
      "source": "billing_mysql",
      "elapsedMs": 12001
    }
  }
}
```

| code | HTTP | Quando |
|------|------|--------|
| `INVALID_IR` | 400 | schema/gramática |
| `UNKNOWN_ENTITY` | 400 | entity/alias fora do catalog |
| `UNKNOWN_FIELD` | 400 | field inexistente no binding |
| `AMBIGUOUS_FIELD` | 400 | field sem qualificar com >1 entidade |
| `AMBIGUOUS_ALIAS` | 400 | `as`/binding repetido na query |
| `LIMIT_EXCEEDED` | 400 | limit > maxLimit |
| `FORBIDDEN` | 403 | `readOnly` / método REST mutável / tabela fora da allowlist do app |
| `FORBIDDEN_SCOPE` | 403 | credencial escopada; filtro de outro sujeito, ou entidade escopada sem valor na chave |
| `UNAUTHORIZED` | 401 | Bearer token ausente ou inválido (HTTP/MCP HTTP) |
| `UNSUPPORTED` | 400 | op não suportada e não degradável |
| `UNSUPPORTED_VERSION` | 400 | `version` SQL desconhecida |
| `INVALID_SQL` | 400 | SQL fora do dialeto solicitado (`"1"` ou `"2"`) |
| `CONFIG_ERROR` | 500 | preset/catalog ausente ou inválido no startup |
| `TIMEOUT` | 504 | budget/fonte |
| `SOURCE_ERROR` | 502 | erro da fonte (msg sanitizada) |
| `NOT_READY` | 409 | result antes da hora |
| `NOT_FOUND` | 404 | queryId |
| `INTERNAL` | 500 | bug |

---

## 6. MCP tool mapping (MVP)

| Tool | Input | Output |
|------|-------|--------|
| `how_to_use_me` | `{}` | body de `GET /v1/howtouseme` (contrato LLM) |
| `describe_catalog` | `{}` | body de `GET /v1/catalog` |
| `execute_sql` | `{ sql, version? }` | body de `POST /v1/sql` |

Descriptions MCP no `serve` interpolam nomes do **catalog carregado** (teto ~4k chars). Sem entidades no YAML, as descriptions não citam tabelas de demo. Sem N tools por tabela (D06).

### Transportes

| Modo | CLI | Endpoint |
|------|-----|----------|
| stdio | `qllm serve --mcp` | processo (Inspector STDIO) |
| Streamable HTTP | `qllm serve --mcp-http` (default `127.0.0.1:8089`) | `POST/GET http://127.0.0.1:8089/mcp` |
| SSE (legado) | mesmo `--mcp-http` | `GET http://127.0.0.1:8089/sse` + `/message` |

`--mcp` (stdio) é exclusivo; `--http` e `--mcp-http` podem coexistir em portas distintas.

**Segurança (D14/D16):** Bearer opcional via `authTokenEnv`, **ou** keys em `qllm.access.yaml`. Compare HMAC-SHA256 + `hmac.Equal` (pepper `qllm-bearer-compare-v1`; sem short-circuit de `len`). CORS allowlist só no MCP HTTP. Defaults loopback; bind não-loopback sem token/key exige `insecureBind`. Stdio MCP é process-local; com access file exige `--app`.

---

## 7. Compatibilidade e evolução

- Additive (novos ops/campos opcionais) → bump **minor**
- Remoção/renomeação/semântica breaking → bump **major**
- Preset/catalog/IR/API compartilham o mesmo `protocolVersion` major.minor; patch só docs/bugfix de validação. **0.2.0** é additive (novos types); **0.1.0** continua válido.

## 8. Checklist antes de implementar código

- [ ] JSON Schemas em `planning/schemas/` batem com este doc (incl. `project`, `as`, `aliases`)
- [ ] Discovery `--config-dir` / `--project` / flags explícitas documentada na CLI
- [ ] Exemplos do harness usam entity names distintos por fonte REST
- [ ] Capability matrix em `04-connectors.md` alinhada ao IR
- [ ] Nenhuma tool de agente expõe SQL cru fora de `execute_sql` / `POST /v1/sql` (D15)
