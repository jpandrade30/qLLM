# 命令行（`qllm`）

二进制：`go build -o qllm ./cmd/qllm`。生产环境和镜像使用 `-tags duckdb`；见 [build.md](build.md) 与 [install.md](install.md)。

```text
qllm
├── validate                 校验 preset + catalog（可选 IR），不打开任何数据源
├── query                    执行 Query IR 文件
├── sql                      执行目录 SQL 文件
├── serve                    HTTP /v1 和/或 MCP
├── catalog
│   ├── introspect           从 postgres/mysql 数据源生成 catalog 草稿
│   └── from-openapi         从 OpenAPI 3 文件生成 rest_resource 实体草稿
├── help [命令]              内置帮助
└── completion               shell 自动补全脚本（内置）
```

`qllm <命令> --help` 会显示任意命令的参数。下文的参数名与二进制完全一致：例如绑定开关是 `--insecure-bind`（不是 `--bind-insecure`）。

成功时退出码为 `0`，任何错误为 `1`。类型化错误以 JSON 写到 **stderr**（`{"protocolVersion": …, "error": {…}}`），见 [errors.md](errors.md)。

## 文件如何被找到

`validate`、`query`、`sql`、`serve` 以及两个 `catalog` 命令共用这四个参数。

| 参数 | 作用 |
|------|------|
| `--config-dir DIR` | 查找 `qllm.preset`、`qllm.catalog` 以及可选的 `qllm.config`、`qllm.access`、`qllm.env` 的目录。默认：当前工作目录 |
| `--preset 文件` / `--catalog 文件` | 显式路径。两者**必须同时**给出，只给一个会得到 `CONFIG_ERROR` |
| `--project 文件` | 一个 `qllm.project.yaml`，其中 `preset:` 和 `catalog:` 相对于该文件。超出其所在目录的路径会被拒绝 |

优先级：`--preset` + `--catalog`，其次 `--project`，最后 `--config-dir`（或 CWD）。在目录中，每个文件按 `.yaml`、`.yml`、`.json` 的顺序查找。缺少 preset 或 catalog 为 `CONFIG_ERROR`。

可选文件（`qllm.config.*`、`qllm.access.*`、`qllm.env.*`）只在 `--config-dir`（或 CWD）中查找。`--preset`、`--catalog`、`--project` 不会改变这一查找位置。

### 环境文件（`qllm.env.yaml`）

`query`、`sql`、`serve` 和两个 `catalog` 命令会在连接前应用 `qllm.env.yaml`。**`validate` 不会**，因为它不打开数据源。规则：

- 只设置进程中**为空或未设置**的变量，真实环境变量始终优先。
- 值是字面量，或恰好为 `${其他名称}`（从进程读取）。缺失的 `${其他名称}` 会被跳过，不会作为文本写入。其他任何含 `${` 的写法都是 `CONFIG_ERROR`。

### 二进制自身读取的环境变量

| 变量 | 使用者 | 含义 |
|------|--------|------|
| `QLLM_APP` | `query`、`sql`、`serve` | 等同 `--app`（参数优先） |
| `QLLM_SCOPE` | `query`、`sql`、`serve` | 等同 `--scope`（参数优先） |
| `serve.authTokenEnv` 中的名称 | `serve` | 保存共享 Bearer 令牌 |
| preset 中每个 `*Env` 键 | 需要打开数据源的命令 | 连接密钥，如 `QLLM_CRM_PG_PASSWORD` |

## `qllm validate`

校验 preset 和 catalog，不打开任何数据源。

| 参数 | 含义 |
|------|------|
| 配置参数 | 见上文 |
| `--ir 文件` | 同时按 catalog 校验该 Query IR |

```bash
./qllm validate --config-dir ./my-project
./qllm validate --config-dir ./my-project --ir ./query.json
```

成功时 stderr 输出 `ok preset=… catalog=… entities=N`（以及 `ok ir=…`）。失败为 stderr 上的类型化 JSON。

## `qllm query`

执行 Query IR 文件，并把 JSON 响应输出到 stdout。会打开数据源。

| 参数 | 含义 |
|------|------|
| 配置参数 | 见上文 |
| `-f`, `--file 文件` | **必填。** JSON 或 YAML 格式的 IR |
| `--app 名称` | `qllm.access.yaml` 中的应用（或 `QLLM_APP`） |
| `--scope 值` | 模板应用的行级作用域值（或 `QLLM_SCOPE`） |

存在 `qllm.access.yaml` 时，会按应用检查表权限。模板应用（`keySecret`）还需要 `--scope`。该命令**不会**读取 `qllm.config.yaml`，因此 REST 响应使用默认的 10 MiB 上限。

```bash
./qllm query --config-dir ./my-project -f ./query.json
```

## `qllm sql`

执行目录 SQL 文件并输出 JSON 响应。需要内嵌 DuckDB 的构建（`-tags duckdb`）。

| 参数 | 含义 |
|------|------|
| 配置参数 | 见上文 |
| `-f`, `--file 文件` | **必填。** 含 SQL 的文本文件 |
| `--version V` | SQL 方言。省略 = 最新（`"2"`）。`"1"` 已冻结（无集合运算，无 `QUALIFY`） |
| `--app`, `--scope` | 与 `query` 相同 |

```bash
./qllm sql --config-dir ./my-project -f ./q.sql
./qllm sql --config-dir ./my-project -f ./q.sql --version 1
```

## `qllm serve`

启动一个或多个监听器。`--http`、`--mcp`、`--mcp-http` 都未给出时，**默认启用 HTTP**。

### 模式

| 参数 | 效果 |
|------|------|
| `--http` | REST `/v1`（`howtouseme`、`catalog`、`queries`、`sql`、`health`） |
| `--mcp-http` | MCP Streamable HTTP 位于 `/mcp`，SSE 位于 `/sse` 与 `/message` |
| `--mcp` | 面向本地客户端（Inspector）的 **stdio** MCP。互斥：与 `--http` 或 `--mcp-http` 同时使用会报错。不开网络监听，因此下面的绑定和令牌规则不适用 |

`--http` 与 `--mcp-http` 可以在同一进程中同时运行。

### 监听与安全参数

| 参数 | 默认值 | 含义 |
|------|--------|------|
| `--addr HOST:PORT` | `127.0.0.1:8088` | HTTP `/v1` 地址 |
| `--mcp-addr HOST:PORT` | `127.0.0.1:8089` | MCP HTTP 地址 |
| `--runtime-config 文件` | `--config-dir` 中的 `qllm.config.*` | 显式指定运行时配置文件 |
| `--auth-token-env 名称` | 无 | 保存共享 Bearer 令牌的环境变量名。一旦设置，该变量**必须非空**，否则 `serve` 以 `CONFIG_ERROR` 失败 |
| `--insecure-bind` | `false` | 允许**非回环**地址且**无**鉴权。见下文 |
| `--cors-origin 来源` | 无（CORS 关闭） | MCP HTTP 允许的浏览器来源。可重复。`*` 会被拒绝 |
| `--app 名称` | 无 | 存在 `qllm.access.yaml` 时 MCP stdio 使用的应用（或 `QLLM_APP`） |
| `--scope 值` | 无 | stdio 下模板应用的行级作用域（或 `QLLM_SCOPE`） |

### 优先级

内置默认值 → `qllm.config.yaml` → 命令行参数。只有你实际传入的参数才算数，所以 `--insecure-bind=false` 可以覆盖文件中的 `insecureBind: true`，`--cors-origin` 会替换文件中的来源列表。

### 绑定规则与 `--insecure-bind`

除非满足以下任一条件，qLLM 拒绝监听非回环地址：

1. 已配置令牌（`serve.authTokenEnv` / `--auth-token-env`，且非空），**或**
2. 存在 `qllm.access.yaml`（其密钥即鉴权），**或**
3. 启用了 `--insecure-bind` / `serve.insecureBind: true`。

回环指 `127.0.0.1`、`::1` 或 `localhost`。以下**不是**回环，会触发该规则：`0.0.0.0:8088`、`[::]:8088`、`:8088`、任何局域网 IP，以及除 `localhost` 外的任何主机名。容器必须监听 `0.0.0.0`，所以需要这一点。

`--insecure-bind` **不会**关闭认证，它只是去掉启动时的拒绝。如果存在令牌或访问文件，请求仍会被校验。它适用于你用其他方式保护的可信网络（私有 compose 网络、服务网格、带认证的反向代理）。完全没有认证时，任何能访问该端口的人都能查询目录公开的全部数据。该检查按监听器分别执行，`--http` 和 `--mcp-http` 各一次。

没有令牌且地址为回环时，API 对本机进程开放，这是默认姿态。

### 仅在 `qllm.config.yaml` 中可用的设置

没有对应参数。Schema：[planning/schemas/runtime-config.schema.json](../../planning/schemas/runtime-config.schema.json)。

| 键 | 默认值 | 含义 |
|----|--------|------|
| `serve.maxBodyBytes` | 1048576（1 MiB） | 请求体上限，HTTP 与 MCP HTTP |
| `serve.maxRestResponseBytes` | 10485760（10 MiB） | 从 `rest` 数据源读取的响应体上限 |
| `serve.cors.allowHeaders` / `allowMethods` | 内置 | 覆盖 CORS 允许列表 |

### 示例

```bash
./qllm serve --http --mcp-http --config-dir ./my-project
./qllm serve --http --addr 0.0.0.0:8088 --auth-token-env QLLM_AUTH_TOKEN --config-dir ./my-project
./qllm serve --mcp --config-dir ./my-project --app crm-agent
./qllm serve --mcp --config-dir ./my-project --app crm-agent --scope 42
```

存在 `qllm.access.yaml` 而 stdio 未提供 `--app` / `QLLM_APP` 时返回 `CONFIG_ERROR`。模板应用还需要 `--scope` / `QLLM_SCOPE`。

日志：每次 `execute_sql` 会在 stderr 输出 `---- execute_sql ----` 块；另外两个工具输出 `---- mcp_tool ----`。

## `qllm catalog introspect`

读取 preset 中 **postgres 或 mysql** 数据源的 `information_schema`，并写出 catalog YAML。不提供服务。

| 参数 | 含义 |
|------|------|
| 配置参数 | 见上文 |
| `--source ID` | **必填。** `sources[].id` |
| `--out 文件` | 输出文件（默认：stdout） |
| `--merge` | 保留已加载 catalog 中其他数据源的实体，只替换该数据源的 |

15 秒超时。`serve` 前请先检查 YAML（关系、描述）。

```bash
./qllm catalog introspect --source crm_pg --config-dir ./my-project --out ./my-project/qllm.catalog.yaml
```

## `qllm catalog from-openapi`

从 OpenAPI 3 规范生成 `rest_resource` 实体和 `options.resources` 片段。`--source` 必须是 preset 中 `type: rest` 的数据源。

| 参数 | 含义 |
|------|------|
| 配置参数 | 见上文 |
| `-f`, `--file 文件` | **必填。** OpenAPI 3 的 YAML 或 JSON |
| `--source ID` | **必填。** REST 数据源 id |
| `--out 文件` | catalog 输出（默认：stdout） |
| `--resources-out 文件` | 写出 resources 片段（否则输出到 stderr） |
| `--merge` | 与 `introspect` 相同 |

REST 连接器**只**读取 preset 中已有的内容。请把该片段粘贴到 `sources[].options.resources`。
