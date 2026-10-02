# 字段参考（schema 接受的全部内容）

来源：[`planning/schemas/`](../../planning/schemas/)。设置了 `additionalProperties: false` 的对象会**拒绝**多余的键。

`*Env` 约定：该字符串是环境变量的**名称**（`QLLM_FOO`），绝不是密钥本身。

逻辑标识符（`sources[].id`、`entities[].name`、别名、字段 `name`、IR 的 `from`/`as`）：`^[a-z][a-z0-9_]*$`。

---

## `qllm.project.yaml`（可选）

必填：`protocolVersion`、`preset`、`catalog`。

| 字段 | 类型 | 说明 |
|------|------|------|
| `protocolVersion` | semver 字符串 | `0.1.0` / `0.2.0` |
| `preset` | 字符串 | 相对于**此**文件的路径 |
| `catalog` | 字符串 | 同上 |

---

## `qllm.preset.yaml`

根级必填：`protocolVersion`、`project`、`limits`、`sources`（至少 1 项）。

| 字段 | 类型 | 说明 |
|------|------|------|
| `protocolVersion` | semver | |
| `project` | 非空字符串 | 项目的逻辑名称；必须与 catalog 一致 |
| `limits` | object | 5 个字段**全部**必填 |
| `sources` | array | |

### `limits`（全部必填）

| 字段 | 类型 | 范围 |
|------|------|------|
| `maxSyncMs` | int | 100–60000；查询总预算 |
| `maxSourceMs` | int | 100–60000；单次访问数据源的预算 |
| `defaultLimit` | int | ≥ 1；IR/SQL 省略 LIMIT 时使用 |
| `maxLimit` | int | ≥ 1；硬性上限 |
| `readOnly` | bool | 在产品中必须为 `true` |

### `sources[]`（每一项）

必填：`id`、`type`、`connection`。

| 字段 | 类型 | 取值 |
|------|------|------|
| `id` | 字符串 | `crm_pg`、`legacy_api`、… |
| `type` | enum | `postgres` `mysql` `mongodb` `rest` `mssql` `sqlite` `clickhouse` `dynamodb` `cassandra` `ksql`，以及 MySQL 线协议别名（`mariadb` `tidb` `vitess` `aurora_mysql` `planetscale`）和 Postgres 线协议别名（`cockroach` `yugabyte` `alloydb` `aurora_postgres` `neon` `supabase` `timescale` `redshift`） |
| `connection` | object | 结构取决于 `type`（见下）。多余的键会报错 |
| `options` | object | 在 JSON schema 中是自由格式；运行时只读取它认识的键（见下） |

### 按 `type` 划分的 `connection`

**postgres**、**mysql** 及其线协议别名（`sqlConnection`）：必填 `hostEnv`、`port`、`database`、`userEnv`、`passwordEnv`。

| 字段 | 类型 | 说明 |
|------|------|------|
| `hostEnv` | 字符串 | |
| `port` | int | 1–65535 |
| `database` | 字符串 | |
| `userEnv` | 字符串 | |
| `passwordEnv` | 字符串 | |
| `sslMode` | 可选 enum | `disable` `require` `verify-ca` `verify-full`（postgres；mysql 不使用时会忽略） |

**mssql**：必填字段相同。可选附加字段：`encrypt`：`true` \| `false` \| `disable`。没有 `sslMode`。

**clickhouse**：必填字段相同。可选附加字段：`secure`（bool）。

**sqlite**：只有 `pathEnv`（保存 `.db` 文件路径的环境变量）。

**mongodb**：必填 `uriEnv`、`database`。

**rest** 和 **ksql**：必填 `baseUrlEnv`。`auth` 可选（见下）。

**dynamodb**：必填 `region`（字面值，例如 `us-east-1`）。可选 `endpointEnv`（Dynamo Local）。AWS 凭证来自进程的凭证链，而不是 YAML 字段。

**cassandra**：schema 要求 `keyspace`。实际上代码使用 `hostsEnv`（列表）**或** `hostEnv`。可选：`port`、`userEnv`、`passwordEnv`。

### `connection.auth`（REST / ksql）

必填：`type`。

| `type` | 相关字段 |
|--------|----------|
| `none` | 无 |
| `bearer` | `tokenEnv` |
| `header` | `name`（header 名称）、`valueEnv` |
| `basic` | `userEnv`、`passwordEnv` |

### Go 代码使用的 `options`（schema 中未枚举）

| 键 | 适用于 | 默认值 | 含义 |
|----|--------|--------|------|
| `statementTimeoutMs` | postgres、mysql、其别名、mssql、clickhouse、sqlite | `limits.maxSourceMs` | 语句超时。实际生效的是 `statementTimeoutMs`、`timeoutMs`、`maxSourceMs` 三者中**最小**的 |
| `timeoutMs` | SQL（同上规则）、REST、ksql | REST 10000，ksql 12000 | REST 和 ksql 的 HTTP 客户端超时。对 SQL 来说是第二个上限，与 `statementTimeoutMs` 相同 |
| `resources` | REST，查询时**必填** | 无 | 资源名到 `list` / `getById` 操作的映射（见下文） |

其他类型（mongodb、dynamodb、cassandra）目前不读取任何 `options` 键。schema 接受未知键，运行时会忽略它们，所以拼写错误不会有任何提示。

REST `options.resources` 示例（catalog 的 `binding.resource` 必须是该映射中的一个键，例如 `users`）：

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

`from-openapi` 会生成这个映射。没有 `resources` 时，REST 连接器会以 `CONFIG_ERROR` 失败。

### REST `resources` 详解

`resources` 下的每个键是一个资源名。catalog 实体通过 `binding: { kind: rest_resource, resource: <名称> }` 指向它。一个资源最多有两个操作，二者结构相同。

| 操作 | 是否必填 | 用途 |
|------|----------|------|
| `list` | 是* | 当 `getById` 无法运行（缺少路径参数）时使用。只要有非按 id 的查询就需要它 |
| `getById` | 否 | 当 `path` 中每个 `{name}` 都有对应的 `eq` 过滤时使用。`from-openapi` 会根据含 `{id}` 的路径生成它 |

每个操作（`list` 和 `getById`）的字段：

| 字段 | 类型 | 默认值 | 含义 |
|------|------|--------|------|
| `method` | string | `GET` | HTTP 方法。`limits.readOnly: true` 时只允许 `GET` 和 `HEAD`；其他方法会在启动时以 `CONFIG_ERROR` 失败 |
| `path` | string | 无 | 拼接到 `baseUrlEnv` 的基础 URL 之后（基础 URL 末尾的 `/` 会被去掉）。请以 `/` 开头。`getById` 会用 `eq` 过滤替换 `{name}` |
| `queryParams` | 字符串数组 | 无 | API 接受的查询参数。用来说明有哪些过滤条件；`from-openapi` 会填写。运行时**不会**校验它 |
| `itemsKey` | string | list 为 `data`/`items`/`results`/`users`，getById 为 `data`/`item`/`result` | 存放数组或单个对象的 JSON 键。也可写在资源上 |
| `maxPages` | int | 1 | 按 offset 翻页的页数（仅 `list`）。上限 20 |
| `pageSize` | int | 查询的 `limit` | 当 `maxPages` > 1 时作为 limit 参数发送的页大小 |
| `limitParam` | string | `limit` | 页大小查询参数名 |
| `offsetParam` | string | `offset` | offset 查询参数名 |

`getById` 示例：`WHERE id = '42'` 且 `path: /users/{id}` 会变成 `GET /users/42`。其余 `eq` 仍作为查询参数。若缺少某个占位符，运行时改用 `list`。

```sql
SELECT id, email FROM users WHERE id = '42' LIMIT 1
```

一次查询如何变成 HTTP 请求：

- **列：**每个被选中的字段按其 `physical` 名称从响应条目中读取。只读取顶层键；带点的 `physical`（如 `addr.city`）在 REST 上取不到值。
- **`WHERE`：**只发送 `eq`（以及用 `and` 组合的 `eq`），形式为 `?<字段>=<值>`（或作为 `getById` 的路径参数）。参数名是查询中书写的**逻辑**字段名，所以对需要过滤的字段，请让逻辑名和物理名保持一致。其他运算符（`neq`、`gt`、`in`、`contains` 等）不会下推到 REST 连接器，在那里返回 `UNSUPPORTED`。
- **`LIMIT` / `OFFSET`：**作为 `limitParam` / `offsetParam` 发送（默认 `limit` 和 `offset`）。
- **分页：**`maxPages: 1`（默认）只发一次请求。更大的值会按 offset 翻页，直到短页、行数上限或 20 页。
- **聚合：**从不下推，在 DuckDB 中执行。
- **响应结构：**JSON 数组，或 `itemsKey`（或默认键）中含有数组的对象。`getById` 也接受裸对象。
- **错误：**HTTP 状态码 400 及以上返回 `SOURCE_ERROR`；响应超过 `serve.maxRestResponseBytes` 会被拒绝；超时返回 `TIMEOUT`。

包含两个操作的完整示例：

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

必填：`protocolVersion`、`project`、`entities`（至少 1 项）。

### `entities[]`

必填：`name`、`source`、`binding`、`fields`（至少 1 个 field）。

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | 逻辑 id | `FROM name` / IR 的 `from` |
| `aliases` | id 数组，不可重复 | IR 中的备用名称 |
| `description` | 字符串 | 给 agent 看的文字 |
| `source` | id | **必须**存在于 `preset.sources[].id` 中 |
| `binding` | object | 物理映射 |
| `primaryKey` | 字符串数组 | 逻辑字段名 |
| `fields` | array | |
| `relations` | array | 仅作提示；不会创建外键 |

### `binding`

必填：`kind`。

| `kind` | 另外必填 | 用途 |
|--------|----------|------|
| `table` | `schema`、`table` | postgres/mysql/mssql/sqlite（`schema: main`）/clickhouse/dynamodb/cassandra/ksql |
| `collection` | `collection` | mongodb |
| `rest_resource` | `resource` | rest；`options.resources` 中的某个键 |

`schema` / `table` / `collection` / `resource`：`^[A-Za-z_][A-Za-z0-9_]*$`。

`accessPath`（object，禁止多余的键），用于 Dynamo、Cassandra 和 ksql：

| 字段 | 类型 | 典型用途 |
|------|------|----------|
| `pk` / `partition` | 字符串数组 | **逻辑**字段名（查询中必须使用等值条件） |
| `sk` / `sort` | 字符串 | Dynamo 的排序键 |
| `ksqlKey` | 字符串 | ksql pull 查询 |

如果查询中缺少正确的等值条件，结果为 `UNSUPPORTED`。

### `fields[]`

必填：`name`、`type`、`physical`。

| 字段 | 取值 |
|------|------|
| `name` | 逻辑 id（`email`） |
| `type` | `string` `number` `boolean` `timestamp` `json` |
| `physical` | 列名或键；允许使用点号路径（`addr.city`） |
| `description` | 可选字符串 |

### `relations[]`

必填：`name`、`to`、`type`、`on`。

| 字段 | 取值 |
|------|------|
| `name` | 标签（`customer`） |
| `to` | 目标 `entities[].name` |
| `type` | `many_to_one` `one_to_many` `one_to_one` |
| `on` | `[本地字段, 远端字段]` 对的数组，至少 1 对 |

---

## `qllm.config.yaml`（serve）

根级可选，且只允许 `serve` 这一个键。`serve` 内的所有内容都是可选的，但如果设置了 `authTokenEnv`，该环境变量必须存在且不为空。

| 字段 | 类型 | 运行时默认值 |
|------|------|--------------|
| `addr` | 字符串 | `127.0.0.1:8088` |
| `mcpAddr` | 字符串 | `127.0.0.1:8089` |
| `authTokenEnv` | 字符串 | 无（不使用 Bearer） |
| `insecureBind` | bool | `false` |
| `maxBodyBytes` | int ≥ 1024 | 1048576（1 MiB） |
| `maxRestResponseBytes` | int ≥ 1024 | 10485760（10 MiB） |
| `cors.origins` | 字符串数组 | `[]` = 关闭 CORS；不允许 `*` |
| `cors.allowHeaders` | array | |
| `cors.allowMethods` | array | |

会**覆盖**文件配置的 CLI 参数：`--addr`、`--mcp-addr`、`--auth-token-env`、`--insecure-bind`、`--cors-origin`。

---

## `qllm.access.yaml`

必填：`apps`（至少 1 项）。每个 app 需要 `name`、`key` 和 `tables`（至少 1 项）。

| 字段 | 说明 |
|------|------|
| `name` | 应用 id（`--app` / `QLLM_APP`） |
| `key` | 字面值，**或**严格写成 `${ENV_NAME}` |
| `tables` | 允许访问的 `entities[].name` |

---

## `qllm.env.yaml`

必填：`env`（至少包含 1 个键的 object）。

| | |
|--|--|
| 键名 | `^[A-Za-z_][A-Za-z0-9_]*$` |
| 值 | 长度 ≥ 1 的字符串；字面值或 `${OTHER_ENV}` |
| 进程中已设置的变量 | **不会**被覆盖 |

请勿用此文件保存提交到 git 的密钥。在 Kubernetes 上请使用 Secret。

---

## `POST /v1/sql` / `execute_sql` 请求体

必填：`sql`。可选：`version`（`"1"` 为冻结版本；省略或 `"2"` 为最新版本）。

---

## Query IR（字段）

必填：`from`、`select`。参见 [queries.md](queries.md) 和 [`query-ir.schema.json`](../../planning/schemas/query-ir.schema.json)。

| 字段 | 说明 |
|------|------|
| `protocolVersion` | 请求中可选 |
| `from` | 实体或别名 |
| `as` | 本地别名 |
| `joins[]` | `type`：`inner`\|`left`；`from`；`as?`；`on[]` `{left,right}` |
| `select[]` | 字段字符串**或** `{agg, field?, as}`；`agg`：`count` `sum` `avg` `min` `max` |
| `where` | `{op,args}` 或比较 `{field,op,value?}` |
| `groupBy` | 字段引用 |
| `orderBy[]` | `{field, dir?}`，取值 `asc`\|`desc` |
| `limit` / `offset` | 整数 |
| `mode` | `sync` \| `async` |

比较 `op`：`eq` `neq` `gt` `gte` `lt` `lte` `in` `nin` `contains` `is_null` `not_null`。

逻辑 `op`：`and` `or`。`not` 的 `args` 中恰好有一个元素。

---

## CLI 参数（摘要）

详见 [cli.md](cli.md)。除 `protocolVersion` 以及上述 `type`/`kind` 枚举之外，catalog 的 YAML 中没有其他“标签”。**构建**标签 `-tags duckdb` 属于 Go（[build.md](build.md)），不属于 qllm 文件。
