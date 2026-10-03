# qLLM 能做什么，不能做什么

一个**只读查询运行时**（Go）。它读取 **preset**（数据源、限制、`*Env`）和**逻辑 catalog**（agent 可以引用的实体和字段）。它执行 Query IR（JSON）或目录 SQL；无法下推的 join 和聚合会交给本地计算（镜像构建中的 DuckDB）。

响应中声明的协议版本：**0.2.0**。**0.1.0** 的 preset、catalog 和 IR 文件仍然有效。

## 它能做什么

- 提供 **HTTP** `/v1` 和/或 **MCP**（stdio，或位于 `/mcp` 的 Streamable HTTP，外加 `/sse` 的 SSE）。
- 在不对数据源做任何 I/O 的情况下校验配置（`qllm validate`）。
- 针对 preset 中的数据源执行一个 IR（`qllm query`）或一个 SQL 文件（`qllm sql`）。
- 生成 catalog **草稿**：`introspect`（postgres/mysql）和 `from-openapi`（REST）。在对外提供服务之前，你必须审阅 relations 和别名。
- 使用 `qllm.access.yaml` 隔离应用（每个应用一个 Bearer key，外加实体允许列表）。
- 快速失败：典型的同步预算约为 **15 秒**（`limits.maxSyncMs`），每个数据源有独立超时，然后返回 `TIMEOUT` 并取消。

## 它不能做什么（这不是伪装起来的待办清单）

| 不在范围内 | 原因 |
|------------|------|
| GraphQL | 永远不会成为 qLLM 的 API（D17）。 |
| agent 直接对 `public.table` 或物理 schema 执行原始 SQL | 表是 catalog 中的**实体名称**。 |
| 每张表一个 MCP 工具，或 MCP 上的 `execute_query` | 只有 `how_to_use_me`、`describe_catalog`、`execute_sql`。IR 只在 HTTP 和 CLI 上提供。 |
| 写入（`INSERT`/`UPDATE`/…）、DDL、`PRAGMA`、`read_csv`、多条语句 | 只读，外加拒绝列表。 |
| 数据仓库 / Spark / Databricks（`PIVOT`、Unity、`ai_*`、Delta 历史） | 只是名称清单，不是克隆。 |
| Dynamo 扫描、Cassandra `ALLOW FILTERING`、ksql `EMIT CHANGES` | 没有对 `accessPath` 的等值条件，结果为 `UNSUPPORTED`。 |
| 为慢速数据源等待数分钟 | 快速失败；异步 HTTP 不是长时间运行的作业。 |
| MVP 中的 Python/Node SDK | HTTP 和 MCP 就是 API（第二阶段）。 |
| Mongo 内省或实验性数据源的内省 | `introspect` 仅支持 postgres 和 mysql。 |
| 不重启就重新加载 YAML | `serve` 在启动时一次性加载。 |
| HTTP `/v1` 监听器上的 CORS | CORS 位于 **MCP HTTP**。REST `/v1` 不会发送 CORS 头。 |
| 通配符 CORS `*` | 会被拒绝。 |
| 在 YAML 中放置 Bearer 令牌 | 只能通过环境变量（`authTokenEnv`、`key: ${VAR}`、`qllm.env.yaml`）。 |

## agent 的接口

1. `how_to_use_me` / `GET /v1/howtouseme`：使用指南，以及绝不能编造的内容。
2. `describe_catalog` / `GET /v1/catalog`：实体（存在 ACL 时会被过滤）。
3. 使用 **`execute_sql` / `POST /v1/sql`**（MCP 和 HTTP）查询，**或者**使用 **`POST /v1/queries`** / `qllm query` 上的 Query IR（它不是 MCP 工具）。

## 两个数据世界

| 世界 | 位置 | 典型实体 |
|------|------|----------|
| 演示 / goldens | `nerdctl compose` 加 `deploy/image/config` | `customers`、`invoices`、… |
| fleet-ops 模拟环境 | `scripts/prd-tst/prd-tst-up`（`deploy/prd-tst`） | `vehicles`、`depots`、`gps_samples`、… |

不要同时运行两者。进程**只能看到** `--config-dir`（或当前目录 / `--project`）。Compose 不会把 catalog “注入”到二进制文件中。
