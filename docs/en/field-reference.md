# Field reference (everything the schema accepts)

Source: [`planning/schemas/`](../../planning/schemas/). Objects with `additionalProperties: false` **reject** extra keys.

`*Env` convention: the string is the **name** of an environment variable (`QLLM_FOO`), never the secret itself.

Logical identifiers (`sources[].id`, `entities[].name`, aliases, field `name`, IR `from`/`as`): `^[a-z][a-z0-9_]*$`.

---

## `qllm.project.yaml` (optional)

Required: `protocolVersion`, `preset`, `catalog`.

| Field | Type | Notes |
|-------|------|-------|
| `protocolVersion` | semver string | `0.1.0` / `0.2.0` |
| `preset` | string | Path relative to **this** file |
| `catalog` | string | Same |

---

## `qllm.preset.yaml`

Required at the root: `protocolVersion`, `project`, `limits`, `sources` (at least 1).

| Field | Type | Notes |
|-------|------|-------|
| `protocolVersion` | semver | |
| `project` | non-empty string | Logical project name; must match the catalog |
| `limits` | object | **All** 5 fields are required |
| `sources` | array | |

### `limits` (all required)

| Field | Type | Range |
|-------|------|-------|
| `maxSyncMs` | int | 100–60000; total query budget |
| `maxSourceMs` | int | 100–60000; per source round trip |
| `defaultLimit` | int | ≥ 1; used when the IR/SQL omits LIMIT |
| `maxLimit` | int | ≥ 1; hard ceiling |
| `readOnly` | bool | must be `true` in the product |

### `sources[]` (each item)

Required: `id`, `type`, `connection`.

| Field | Type | Values |
|-------|------|--------|
| `id` | string | `crm_pg`, `legacy_api`, … |
| `type` | enum | `postgres` `mysql` `mongodb` `rest` `mssql` `sqlite` `clickhouse` `dynamodb` `cassandra` `ksql` `redis` `kafka` plus MySQL-wire aliases (`mariadb` `tidb` `vitess` `aurora_mysql` `planetscale`) and Postgres-wire aliases (`cockroach` `yugabyte` `alloydb` `aurora_postgres` `neon` `supabase` `timescale` `redshift`) |
| `connection` | object | Shape depends on `type` (below). Extra keys are an error |
| `options` | object | Free-form in the JSON schema; the runtime reads only what it knows (below) |

### `connection` by `type`

**postgres**, **mysql**, and their wire aliases (`sqlConnection`): required `hostEnv`, `port`, `database`, `userEnv`, `passwordEnv`.

| Field | Type | Notes |
|-------|------|-------|
| `hostEnv` | string | |
| `port` | int | 1–65535 |
| `database` | string | |
| `userEnv` | string | |
| `passwordEnv` | string | |
| `sslMode` | optional enum | `disable` `require` `verify-ca` `verify-full` (postgres; mysql ignores it when unused) |

**mssql**: same required fields. Optional extra: `encrypt`: `true` \| `false` \| `disable`. No `sslMode`.

**clickhouse**: same required fields. Optional extra: `secure` (bool).

**sqlite**: only `pathEnv` (the env var that holds the `.db` file path).

**mongodb**: required `uriEnv`, `database`.

**rest** and **ksql**: required `baseUrlEnv`. Optional `auth` (below).

**dynamodb**: required `region` (literal, for example `us-east-1`). Optional `endpointEnv` (Dynamo Local). AWS credentials come from the process credential chain, not from YAML fields.

**cassandra**: the schema requires `keyspace`. In practice the code uses `hostsEnv` (a list) **or** `hostEnv`. Optional: `port`, `userEnv`, `passwordEnv`.

**redis**: `addrEnv` or `hostEnv`+`port`. Optional: `db`, `userEnv`, `passwordEnv`, `tls`, `readReplica` (documented hint; the driver does not send writes).

**kafka**: required `brokersEnv`. Optional: `tls`, `sasl` (`none`/`plain`/`scram`), `userEnv`, `passwordEnv`. Options: `timeoutMs`, `maxRecords`, `maxScanRecords`.

### `connection.auth` (REST / ksql)

Required: `type`.

| `type` | Fields that matter |
|--------|--------------------|
| `none` | none |
| `bearer` | `tokenEnv` |
| `header` | `name` (header name), `valueEnv` |
| `basic` | `userEnv`, `passwordEnv` |

### `options` the Go code uses (not enumerated in the schema)

| Key | Applies to | Default | Meaning |
|-----|------------|---------|---------|
| `statementTimeoutMs` | postgres, mysql, their aliases, mssql, clickhouse, sqlite | `limits.maxSourceMs` | Statement timeout. The effective value is the **smallest** of `statementTimeoutMs`, `timeoutMs`, and `maxSourceMs` |
| `timeoutMs` | SQL sources (same rule as above), REST, ksql, redis, kafka | REST 10000, ksql/kafka 12000, redis 10000 | Client timeout |
| `resources` | REST, **required** to query | none | Map of resource name to `list` / `getById` operations (see below) |

Every other source type (mongodb, dynamodb, cassandra) reads no `options` keys today. Unknown keys are accepted by the schema and ignored by the runtime, so a typo fails silently.

REST `options.resources` example (the catalog `binding.resource` must be a key of this map, for example `users`):

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

`from-openapi` generates this map. Without `resources`, the REST connector fails with `CONFIG_ERROR`.

### REST `resources` in detail

Each key under `resources` is a resource name. A catalog entity points at it with `binding: { kind: rest_resource, resource: <name> }`. A resource holds up to two operations, and both share the same shape.

| Operation | Required | Purpose |
|-----------|----------|---------|
| `list` | yes* | Used when `getById` cannot run (no matching path params). Required unless every query hits `getById` |
| `getById` | no | Used when every `{name}` in `path` has a matching `eq` filter. `from-openapi` generates it from paths that contain `{id}` |

Operation fields (`list` and `getById`):

| Field | Type | Default | Meaning |
|-------|------|---------|---------|
| `method` | string | `GET` | HTTP method. With `limits.readOnly: true` only `GET` and `HEAD` are allowed; any other method fails at startup with `CONFIG_ERROR` |
| `path` | string | none | Appended to the base URL from `baseUrlEnv` (a trailing `/` on the base is trimmed). Start it with `/`. `{name}` is replaced from `eq` filters when this is `getById` |
| `queryParams` | array of strings | none | The query parameters the API accepts. It documents which filters exist; `from-openapi` fills it. The runtime does **not** enforce it |
| `itemsKey` | string | `data`/`items`/`results`/`users` (list) or `data`/`item`/`result` (getById) | JSON key that holds the array or the single object. Also allowed on the resource itself |
| `maxPages` | int | 1 | Offset pages to fetch (`list` only). Capped at 20 |
| `pageSize` | int | the query `limit` | Page size sent as the limit parameter when `maxPages` > 1 |
| `limitParam` | string | `limit` | Query parameter name for the page size |
| `offsetParam` | string | `offset` | Query parameter name for the offset |

`getById` example: `WHERE id = '42'` plus `path: /users/{id}` becomes `GET /users/42`. Remaining `eq` filters stay as query parameters. If a path placeholder has no matching filter, the runtime uses `list`.

```sql
SELECT id, email FROM users WHERE id = '42' LIMIT 1
```

How a query becomes an HTTP request:

- **Columns:** each selected field is read from the response item using its `physical` name. Only top-level keys are read; a dotted `physical` such as `addr.city` returns nothing on REST.
- **`WHERE`:** only `eq` (and `and`-combined `eq`) is sent, as `?<field>=<value>` (or as a path param on `getById`). The parameter name is the **logical** field name as written in the query, so keep the logical and physical names equal for fields you filter on. Any other operator (`neq`, `gt`, `in`, `contains`, …) is not pushed down to the REST connector and returns `UNSUPPORTED` there.
- **`LIMIT` / `OFFSET`:** sent as `limitParam` / `offsetParam` (defaults `limit` and `offset`).
- **Pagination:** `maxPages: 1` (default) is one request. Higher values walk offset until a short page, the row limit, or 20 pages.
- **Aggregations:** never pushed down; they run in DuckDB.
- **Response shape:** a JSON array, or an object whose `itemsKey` (or the default keys) holds the array. `getById` also accepts a bare object.
- **Errors:** HTTP status 400 or above returns `SOURCE_ERROR`; a response larger than `serve.maxRestResponseBytes` is refused; exceeding the timeout returns `TIMEOUT`.

Full example with both operations:

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

Required: `protocolVersion`, `project`, `entities` (at least 1).

### `entities[]`

Required: `name`, `source`, `binding`, `fields` (at least 1 field).

| Field | Type | Notes |
|-------|------|-------|
| `name` | logical id | `FROM name` / IR `from` |
| `aliases` | array of ids, unique | Alternative names in the IR |
| `description` | string | Text for the agent |
| `source` | id | **Must** exist in `preset.sources[].id` |
| `binding` | object | Physical mapping |
| `primaryKey` | array of strings | Logical field names |
| `fields` | array | |
| `relations` | array | Hints only; they do not create foreign keys |

### `binding`

Required: `kind`.

| `kind` | Also required | Use |
|--------|---------------|-----|
| `table` | `schema`, `table` | postgres/mysql/mssql/sqlite (`schema: main`)/clickhouse/dynamodb/cassandra/ksql |
| `collection` | `collection` | mongodb |
| `rest_resource` | `resource` | rest; a key of `options.resources` |
| `key` | `keyPattern`, `accessPath.partition` | redis (`user:{id}`) |
| `topic` | `topic`, `accessPath` (partition / `key` / timestamp) | kafka |

`schema` / `table` / `collection` / `resource`: `^[A-Za-z_][A-Za-z0-9_]*$`.

`accessPath` (object, extra keys forbidden) for Dynamo, Cassandra, and ksql:

| Field | Type | Typical use |
|-------|------|-------------|
| `pk` / `partition` | array of strings | **Logical** field names (equality required in the query) |
| `sk` / `sort` | string | Dynamo sort key |
| `ksqlKey` | string | ksql pull query |

Without the right equality in the query, the result is `UNSUPPORTED`.

### `fields[]`

Required: `name`, `type`, `physical`.

| Field | Values |
|-------|--------|
| `name` | logical id (`email`) |
| `type` | `string` `number` `boolean` `timestamp` `json` |
| `physical` | column or key; dotted paths allowed (`addr.city`) |
| `description` | optional string |

### `relations[]`

Required: `name`, `to`, `type`, `on`.

| Field | Values |
|-------|--------|
| `name` | label (`customer`) |
| `to` | target `entities[].name` |
| `type` | `many_to_one` `one_to_many` `one_to_one` |
| `on` | array of `[local_field, remote_field]` pairs, at least 1 pair |

---

## `qllm.config.yaml` (serve)

Optional root; only the `serve` key. Everything inside `serve` is optional, but if you set `authTokenEnv`, that env var must exist and be non-empty.

| Field | Type | Runtime default |
|-------|------|-----------------|
| `addr` | string | `127.0.0.1:8088` |
| `mcpAddr` | string | `127.0.0.1:8089` |
| `authTokenEnv` | string | none (no Bearer) |
| `insecureBind` | bool | `false` |
| `maxBodyBytes` | int ≥ 1024 | 1048576 (1 MiB) |
| `maxRestResponseBytes` | int ≥ 1024 | 10485760 (10 MiB) |
| `cors.origins` | array of strings | `[]` = CORS off; no `*` |
| `cors.allowHeaders` | array | |
| `cors.allowMethods` | array | |

CLI flags that **override** the file: `--addr`, `--mcp-addr`, `--auth-token-env`, `--insecure-bind`, `--cors-origin`.

---

## `qllm.access.yaml`

Required: `apps` (at least 1). Each app needs `name`, `key`, and `tables` (at least 1).

| Field | Notes |
|-------|-------|
| `name` | App id (`--app` / `QLLM_APP`) |
| `key` | Literal **or** exactly `${ENV_NAME}` |
| `tables` | Allowed `entities[].name` values |

---

## `qllm.env.yaml`

Required: `env` (object with at least 1 key).

| | |
|--|--|
| key names | `^[A-Za-z_][A-Za-z0-9_]*$` |
| values | string ≥ 1; literal or `${OTHER_ENV}` |
| already set in the process | **not** overwritten |

Do not use this file for secrets in git. On Kubernetes, use a Secret.

---

## `POST /v1/sql` / `execute_sql` body

Required: `sql`. Optional: `version` (`"1"` frozen; omitted or `"2"` latest).

---

## Query IR (fields)

Required: `from`, `select`. See [queries.md](queries.md) and [`query-ir.schema.json`](../../planning/schemas/query-ir.schema.json).

| Field | Notes |
|-------|-------|
| `protocolVersion` | optional in the request |
| `from` | entity or alias |
| `as` | local alias |
| `joins[]` | `type`: `inner`\|`left`; `from`; `as?`; `on[]` `{left,right}` |
| `select[]` | field string **or** `{agg, field?, as}`; `agg`: `count` `sum` `avg` `min` `max` |
| `where` | `{op,args}` or compare `{field,op,value?}` |
| `groupBy` | field refs |
| `orderBy[]` | `{field, dir?}` with `asc`\|`desc` |
| `limit` / `offset` | ints |
| `mode` | `sync` \| `async` |

Compare `op`: `eq` `neq` `gt` `gte` `lt` `lte` `in` `nin` `contains` `is_null` `not_null`.

Logic `op`: `and` `or`. `not` takes exactly one element in `args`.

---

## CLI flags (summary)

Covered in [cli.md](cli.md). The catalog has no YAML "tags" beyond `protocolVersion` and the `type`/`kind` enums above. The **build** tag `-tags duckdb` belongs to Go ([build.md](build.md)), not to a qllm file.
