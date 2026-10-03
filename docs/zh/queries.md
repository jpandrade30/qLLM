# 查询：目录 SQL 与 Query IR

有两条路径。MCP **只有** SQL。HTTP 两种都有。

Schemas：[`sql-request.schema.json`](../../planning/schemas/sql-request.schema.json)、[`query-ir.schema.json`](../../planning/schemas/query-ir.schema.json)、[`query-response.schema.json`](../../planning/schemas/query-response.schema.json)。方言：[`planning/07-sql-dialect.md`](../../planning/07-sql-dialect.md)。

## 目录 SQL（`execute_sql` / `POST /v1/sql` / `qllm sql`）

请求体：`{ "sql": "SELECT …", "version": "1"|"2" }`。省略 `version` 表示 **`"2"`**。未知的值返回 `UNSUPPORTED_VERSION`。

### 接受的内容（摘要）

- 单条 `SELECT` 语句（可带 `WITH`）。
- 表是**实体名称**（以及 CTE 别名，CTE 别名不是实体）。
- `LIMIT` 必须提供，或由系统注入（`defaultLimit`）。单独的 `OFFSET` 也会注入 limit。
- 方言 `"1"`：`WHERE`、`HAVING`、`DISTINCT`、`CASE`、`LIKE`、`ILIKE`、`BETWEEN`、`IN`、`IS NULL`、CTE、`FROM` 子查询、join、基础聚合、字符串/数值/类型转换函数，以及**使用 DuckDB 名称**的 JSON/数组/日期时间函数。
- 方言 `"2"`：包含 `"1"` 的全部内容，另加 `UNION` / `UNION ALL` / `INTERSECT` / `EXCEPT`、`QUALIFY`、窗口函数（`ROW_NUMBER` 等）以及 `XOR`。

名称不同的 Databricks 函数（`GET_JSON_OBJECT`、`DATEADD` 等）只要 DuckDB 支持就不会被拒绝；指南要求使用 DuckDB 的拼写。

### 被拒绝的内容（`INVALID_SQL` 或同类错误）

- `INSERT` `UPDATE` `DELETE` `MERGE` `REPLACE`
- `CREATE` `DROP` `ALTER` `TRUNCATE` `COPY` `ATTACH` `DETACH`
- `INSTALL` `LOAD` `PRAGMA` `SET` `CALL` `GRANT`
- `read_csv` `read_parquet` `read_json` `postgres_scan` `httpfs` `glob` `read_text` `read_blob`
- 多条语句（`;`）
- `schema.table`（`public.customers`）
- 任意的表函数
- `LIMIT` 大于 `maxLimit` 时返回 `LIMIT_EXCEEDED`
- 实体或字段不存在时返回 `UNKNOWN_ENTITY` / `UNKNOWN_FIELD`
- 实体不在 ACL 范围内时返回 `FORBIDDEN`

### 执行过程（如果你要在脑中模拟 planner，这一点很重要）

1. 解析，然后校验名称和 ACL。
2. **取回**被引用的表（只取所需的列）。这条路径上**没有** `WHERE` 下推。
3. DuckDB 执行 `SELECT`（`enable_external_access=false`）。
4. **不带** `-tags duckdb` 的构建无法运行这条路径。

对于 KV 和流类数据源，`WHERE` 必须包含对 `accessPath` 的等值条件，否则取数会以 `UNSUPPORTED` 失败。SQL “看起来”有效，但数据源会拒绝它。

## Query IR（`POST /v1/queries` / `qllm query`）

JSON，`additionalProperties: false`。必填：`from`、`select`。

### 接受的内容

- `from` 以及 join 的 `from`：实体名或 catalog 别名（`[a-z][a-z0-9_]*`）。
- 查询中的 `as`：唯一的本地别名。
- `select`：字段引用（`entity.field` 或 `alias.field`）**或** `{ "agg": "count|sum|avg|min|max", "field"?, "as" }`。`count` 可以省略 `field`。
- `joins[]`：只支持 `inner` 和 `left`；`on` 使用 `{left, right}` 对。
- `where`：`and` / `or` / `not` 使用 `{ "op", "args" }`；比较使用 `{ "field", "op", "value"? }`。
- 比较 `op`：`eq` `neq` `gt` `gte` `lt` `lte` `in` `nin` `contains` `is_null` `not_null`。**不要**用 `=`、`LIKE` 或 `{and:[…]}` 来代替 `op` 加 `args`。
- 当聚合列与非聚合列混用时，需要 `groupBy`。
- `orderBy`：`{ "field", "dir": "asc"|"desc" }`。
- `limit` ≥ 1（或使用 preset 的默认值）；`offset` ≥ 0。
- `mode`：`sync` 或 `async`。
- 涉及多个实体时，请对字段加限定，否则会得到 `AMBIGUOUS_FIELD`。

### IR 中被拒绝或不存在的内容

- IR 中的 `full` 和 `cross` join（在 SQL 路径上，DuckDB 上的 SQL 可能接受它们）。
- IR 中的 `having`、`union`、`case`、`like` 和 `xor`；请改用 SQL。
- 任何修改操作。
- 自行编造实体或 join 键。

`GET /v1/howtouseme` 描述了 `never`、各种结构以及 `invalidExamples`。agent 应当在编造 IR **之前**先阅读它。

## 响应

一个信封，包含 `protocolVersion`、`queryId`、`status`（`succeeded` / `failed` / `accepted`）、表格形式的 `result`、`meta`（`elapsedMs`、`app`、`plan.usedDuckDB`、steps），或者类型化的 `error`。完整示例与类型表见 [responses.md](responses.md)。

不要在没有检查 `status` 和 `error` 的情况下，就把 HTTP 200 当作成功。
