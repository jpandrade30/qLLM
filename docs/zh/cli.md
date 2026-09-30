# CLI（`qllm`）

二进制文件：`go build -o qllm ./cmd/qllm`。生产环境和镜像使用 `-tags duckdb`；参见 [build.md](build.md)。

常用配置参数（几乎所有子命令都支持）：

| 参数 | 用途 |
|------|------|
| `--config-dir` | 包含 `qllm.preset` 和 `qllm.catalog` 的目录 |
| `--preset` / `--catalog` | 显式路径（两者必须同时提供） |
| `--project` | `qllm.project.yaml` 的路径 |

## `qllm validate`

在不打开任何数据源的情况下校验 preset 和 catalog。可选的 `--ir FILE` 会根据 catalog 校验 Query IR。

```bash
./qllm validate --config-dir ./my-project
./qllm validate --config-dir ./my-project --ir ./query.json
```

成功时，stderr 输出 `ok preset=… catalog=… entities=N`。错误以类型化 JSON 输出到 stderr。

## `qllm query`

执行一个 Query IR 文件。`--file` / `-f` 为必填。会打开数据源。

存在 `qllm.access.yaml` 时，请使用 `--app` 或 `QLLM_APP`。

连接之前会先应用配置目录中的 `qllm.env.yaml`。

```bash
./qllm query --config-dir ./my-project -f ./query.json
```

## `qllm sql`

执行包含目录 SQL 的文本文件。`--file` / `-f` 为必填。

省略 `--version` 时使用最新方言（`"2"`）。`"1"` 是冻结的方言（不支持集合运算，也不支持 `QUALIFY`）。

`--app` / `QLLM_APP` 用于应用 ACL。

`ExecSQL` 需要内嵌 DuckDB 的构建版本。

```bash
./qllm sql --config-dir ./my-project -f ./q.sql
./qllm sql --config-dir ./my-project -f ./q.sql --version 1
```

## `qllm serve`

如果没有指定 `--http`、`--mcp` 或 `--mcp-http` 中的任何一个，默认**启用 HTTP**。

| 参数 | 作用 |
|------|------|
| `--http` | REST `/v1` |
| `--mcp-http` | MCP 位于 `/mcp`，另有 `/sse` 和 `/message` |
| `--mcp` | 基于 **stdio** 的 MCP（本地 Inspector）。**互斥**：不能与 `--http` 或 `--mcp-http` 同时使用 |
| `--addr` | HTTP 监听地址（默认 `127.0.0.1:8088`） |
| `--mcp-addr` | MCP HTTP 监听地址（默认 `127.0.0.1:8089`） |
| `--runtime-config` | `qllm.config.yaml` 的路径 |
| `--auth-token-env` | 保存 Bearer 令牌的环境变量名 |
| `--insecure-bind` | 允许在没有认证的情况下绑定非回环地址 |
| `--cors-origin` | 可重复；MCP HTTP 的允许列表 |
| `--app` | 存在 `qllm.access.yaml` 时，stdio 使用的应用名 |

```bash
./qllm serve --http --mcp-http --config-dir ./my-project
./qllm serve --mcp --config-dir ./my-project --app crm-agent
```

使用 stdio、存在 `qllm.access.yaml` 且未提供 `--app` / `QLLM_APP` 时，返回 `CONFIG_ERROR`。

## `qllm catalog introspect`

从 preset 中的 **postgres 或 mysql** 数据源读取 `information_schema`，并写出 catalog YAML。它**不会**启动服务。

| 参数 | 含义 |
|------|------|
| `--source` | `sources[].id`（必填） |
| `--out` | 输出文件（默认：stdout） |
| `--merge` | 保留已加载 catalog 中其他数据源的实体 |

内省会在 15 秒后超时。运行 `serve` 之前，请先检查 YAML（relations、descriptions）。

```bash
./qllm catalog introspect --source crm_pg --config-dir ./my-project --out ./my-project/qllm.catalog.yaml
```

## `qllm catalog from-openapi`

根据 OpenAPI 3 规范生成 `rest_resource` 实体以及 `options.resources` 片段。`--source` 必须是 preset 中 `type: rest` 的数据源。

| 参数 | 含义 |
|------|------|
| `-f` / `--file` | OpenAPI 规范文件 |
| `--source` | REST 数据源 ID |
| `--out` | 输出的 catalog |
| `--resources-out` | resources 的 YAML 片段（否则输出到 stderr） |
| `--merge` | 与 `introspect` 相同 |

REST 连接器**只**读取 preset 中已有的内容。请把该片段粘贴到 `sources[].options.resources`。
