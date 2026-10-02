# 数据源（连接器）

规范性矩阵：[`planning/04-connectors.md`](../../planning/04-connectors.md)。连接格式：[`planning/03-protocol-schemas.md`](../../planning/03-protocol-schemas.md) 和 [`preset.schema.json`](../../planning/schemas/preset.schema.json)。

## 类型

| `type` | Harness/CI | 说明 |
|--------|------------|------|
| `postgres` | 是 | `sslMode`、`statementTimeoutMs`。别名：`cockroach` `yugabyte` `alloydb` `aurora_postgres` `neon` `supabase` `timescale` `redshift` |
| `mysql` | 是 | 别名：`mariadb` `tidb` `vitess` `aurora_mysql` `planetscale` |
| `mongodb` | 是 | `uriEnv`、`database`；同源 join 通过 DuckDB 完成 |
| `rest` | 是 | `baseUrlEnv`，认证 `none`\|`bearer`\|`header`\|`basic`；聚合在 DuckDB 中执行；`options.resources` |
| `mssql` | 实验性 | `encrypt` |
| `sqlite` | 实验性 | `pathEnv`；`binding.schema: main` |
| `clickhouse` | 实验性 | 原生端口通常为 9000 |
| `dynamodb` | 实验性 | AWS 凭证与 `region`；`endpointEnv`（Local）；`accessPath` pk/sk |
| `cassandra` | 实验性 | `hostEnv`/`hostsEnv`、keyspace；`accessPath.partition` |
| `ksql` | 实验性 | **仅支持 pull 查询**；`baseUrlEnv`；`accessPath.ksqlKey` |

“实验性”表示该类型已包含在二进制文件中，但本仓库**没有**对应的 Compose 或 golden 测试。只有在你自己的实例上测试通过之后，才应认为它可用。

不是独立类型：Oracle、BigQuery、Snowflake、Elasticsearch、GraphQL、将 S3 当作表。许多 HTTP JSON API 使用 `rest`。

## catalog 中的 binding

| 类型 | `binding.kind` | 字段 |
|------|----------------|------|
| 表格型 SQL | `table` | `schema`、`table` |
| mongodb | `collection` | `collection` |
| rest | `rest_resource` | `resource` |
| dynamodb | `table` | `table` 以及 `accessPath.pk`/`partition`，可选 `sk`/`sort` |
| cassandra | `table` | `table` 以及 `accessPath.partition`（**逻辑**名称） |
| ksql | `table` | `table` 以及 `accessPath.ksqlKey` |

如果 Query IR **没有**对 KV 或流的键使用等值条件，则返回 `UNSUPPORTED`；运行时绝不会做全量扫描。

## 能力（IR 与下推）

- **写入：**永不支持。
- **同源 join：**SQL 引擎会下推；mongo、rest 和 KV 会先取回数据，再在 DuckDB 中 join。
- **跨源 join：**始终在 DuckDB 中执行。
- **REST：**`eq` 过滤、`limit` 和 `offset` 会到达 API；路径参数齐全时走 `getById`；`maxPages` 按 offset 翻页；聚合在 DuckDB 中执行。详见 [REST `resources` 详解](field-reference.md#rest-resources-详解)。
- **超时：**`min(options.timeoutMs|statementTimeoutMs, limits.maxSourceMs)`，外加 context 取消。

在**目录 SQL 路径**上，运行时会取回范围更大的数据，再由 DuckDB 完成计算。IR 的下推矩阵**不**适用于 SQL 的 `WHERE`。

## 连接认证（不要与 serve 的 Bearer 混淆）

- SQL：`userEnv` / `passwordEnv`。
- SQLite：文件路径来自环境变量。
- Mongo：`uriEnv`。
- REST 和 ksql：`none` / `bearer` / `header` / `basic`。
- Dynamo：AWS 凭证链；`region` 为字面值。
- Cassandra：主机，以及可选的用户名和密码。

## 创作 catalog

- `qllm catalog introspect`：postgres、mysql 及其线协议别名。
- `qllm catalog from-openapi`：生成 REST 草稿；你必须把 `resources` 粘贴到 preset 中。

Mongo 的自动“采样 collection”发现不在本 MVP 的范围内。
