# 从零创建 qLLM 项目

qLLM **不会**猜测你的表。你需要编写 YAML，进程只会读取**一个目录**。如果这个目录不是你以为的那个，你就会看到演示用的 catalog，或者收到 `CONFIG_ERROR`。

## 1. 你要创建什么

一个**属于你自己**的目录（不要把 `fixtures/` 当作产品）。请使用下面这些确切的名称；二进制文件会查找这些文件名前缀：

```text
C:\data\my-qllm\            （示例）
  qllm.preset.yaml          必需：数据库/API 与限制
  qllm.catalog.yaml         必需：agent 可以 SELECT 的名称
  qllm.config.yaml          推荐：端口、令牌、CORS
  qllm.env.yaml             可选：在你的电脑上补全为空的环境变量
  qllm.access.yaml          可选：多个 agent / 允许列表
```

支持的扩展名：`.yaml`、`.yml`、`.json`。不能使用没有 `qllm.` 前缀的 `preset.yaml`。

YAML 中的 `protocolVersion`：`"0.1.0"` 或 `"0.2.0"`（No version `N.N.N`）。运行时的**响应**为 `0.2.0`。

所有字段：[field-reference.md](field-reference.md)。示例目录（演示配置的副本，由 `Dockerfile` 内嵌）：[`deploy/prd/`](../../deploy/prd)。Docker 与 Kubernetes：[point-your-folder.md](point-your-folder.md)。

## 2. 工作顺序

1. 列出你的真实数据源（主机、端口、用户、database **或** URI **或** URL）。
2. 编写 **preset**：每个连接对应一个 `sources[].id`。`id` 必须匹配 `[a-z][a-z0-9_]*`（例如 `crm_pg`，而不是 `CRM-PG`）。
3. 按你在 `hostEnv`、`passwordEnv` 等字段中写的**名称**创建环境变量。YAML 中绝不保存密码，只保存环境变量的**名称**（`QLLM_CRM_PG_PASSWORD`）。
4. 编写 **catalog**：每张逻辑表对应一个实体。`name` 是 `FROM` 中使用的名称。`source` 是 preset 中的某个 `id`。`binding` 是**物理**的 schema 和表（或 collection/resource）。
5. 运行 `qllm validate --config-dir …`，直到输出 `ok`，并且 `entities=N` 中的 **N 与你写的数量一致**。
6. 运行 `qllm serve --http --mcp-http --config-dir …`。
7. 使用 health 和 catalog 进行确认（第 5 节）。没有这一步，你就无法判断自己是否加载了错误的 YAML。

## 3. 最小示例（一个 Postgres）

`qllm.preset.yaml`：`connection` 下每个以 `Env` 结尾的键，其值都是变量的**名称**，而不是变量的值：

```yaml
protocolVersion: "0.2.0"
project: my-company
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
```

在 PowerShell 中，启动服务**之前**：

```powershell
$env:QLLM_CRM_PG_HOST = "127.0.0.1"
$env:QLLM_CRM_PG_USER = "app"
$env:QLLM_CRM_PG_PASSWORD = "secret"
```

`qllm.catalog.yaml`：agent **永远不会**写 `public.customers`，而是写 `customers`：

```yaml
protocolVersion: "0.2.0"
project: my-company
entities:
  - name: customers
    description: CRM 客户
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
        description: 唯一的电子邮件
```

`qllm.config.yaml`：

```yaml
serve:
  addr: "127.0.0.1:8088"
  mcpAddr: "127.0.0.1:8089"
  authTokenEnv: QLLM_AUTH_TOKEN
  insecureBind: false
  maxBodyBytes: 1048576
  maxRestResponseBytes: 10485760
  cors:
    origins: []
```

```powershell
$env:QLLM_AUTH_TOKEN = "a-long-token"
```

更多数据源、REST 和 Dynamo：[field-reference.md](field-reference.md)。

## 4. 校验你写的内容（此时还没有服务）

在 qLLM 的**源码**目录（二进制文件所在位置），指向**你自己的**目录：

```powershell
cd C:\codes\qLLM
go build -tags duckdb -o qllm.exe .\cmd\qllm
.\qllm.exe validate --config-dir C:\data\my-qllm
```

你应该看到：

```text
ok preset=C:\data\my-qllm\qllm.preset.yaml catalog=C:\data\my-qllm\qllm.catalog.yaml entities=1
```

- 路径必须是**你自己的**文件（而不是 `deploy\image\config`）。
- `entities=` 是 catalog 中 `entities:` 下的条目数。
- stderr 中出现 `"code":"CONFIG_ERROR"` 的 JSON，表示 YAML 缺失、存在多余字段（`additionalProperties: false`）、`id` 无效，或者只给了 `--preset` 而没有 `--catalog`。

如果校验通过，并且你的 catalog 中有 `customers`，那么针对 `invoices` 的 IR 必须失败：

```powershell
# 文件 tmp.json：{ "from": "invoices", "select": ["id"], "limit": 1 }
.\qllm.exe validate --config-dir C:\data\my-qllm --ir C:\data\tmp.json
```

预期结果：`UNKNOWN_ENTITY`。如果通过了，说明 `--config-dir` **不是**你以为的那个目录。

## 5. 启动服务并确认这是**你的**项目

```powershell
.\qllm.exe serve --http --mcp-http --config-dir C:\data\my-qllm
```

stderr 必须显示：

```text
qllm http listening on 127.0.0.1:8088
qllm mcp-http listening on 127.0.0.1:8089 (/mcp streamable, /sse SSE)
```

在另一个终端中：

```powershell
curl.exe -s -H "Authorization: Bearer a-long-token" http://127.0.0.1:8088/v1/health
curl.exe -s -H "Authorization: Bearer a-long-token" http://127.0.0.1:8088/v1/catalog
```

Health：`"ok":true` 和 `"protocolVersion":"0.2.0"`。

Catalog：`"project":"my-company"`（来自**你的** YAML 的字符串），并且 `entities` 中包含 `customers`。如果你看到的是 harness 的 `qllm-demo`，以及其中的 `customers` 和 `invoices`，说明进程**没有**使用 `C:\data\my-qllm`。可能是你忘了写 `--config-dir`，Docker 挂载了另一个目录，或者 Kubernetes 挂载了旧的 ConfigMap。

冒烟测试 SQL（针对**你的**表）：

```powershell
curl.exe -s -H "Authorization: Bearer a-long-token" -H "Content-Type: application/json" `
  -d "{\"sql\":\"SELECT id, email FROM customers LIMIT 5\"}" `
  http://127.0.0.1:8088/v1/sql
```

- `"status":"succeeded"` 且有数据行：数据库连接正常，catalog 也匹配。
- `UNKNOWN_ENTITY`：SQL 使用了已加载 catalog 中不存在的 `name`。
- `SOURCE_ERROR` / `TIMEOUT`：YAML 没问题；网络、凭证或主机有误。
- `UNAUTHORIZED`：令牌与 `QLLM_AUTH_TOKEN` 不一致，或 header 格式有误（`Bearer ` 后面要有一个空格）。

MCP：通过 `describe_catalog` 和 `execute_sql` 工具做同样的确认。进程日志会显示带有 SQL 的 `---- execute_sql ----`。

## 6. 生成草稿，而不是手写 catalog

在 preset 中已经配置好 Postgres 或 MySQL，并且已设置环境变量的情况下：

```powershell
.\qllm.exe catalog introspect --source crm_pg --config-dir C:\data\my-qllm --out C:\data\my-qllm\qllm.catalog.yaml
```

打开生成的文件，确认 `source`、`binding.schema`/`table` 和 `fields`。补充 `relations` 和 `description`，然后再次运行 `validate`。

REST：使用 `catalog from-openapi`，并把 `options.resources` 粘贴到 preset 中（[cli.md](cli.md)）。

## 7. YAML 中**不能**出现的内容

- schema 中没有的字段：validate 和 serve 会拒绝它们（preset、catalog、config、access、env 和 project 都设置了 `additionalProperties: false`）。
- preset 中的明文密码（请使用 `passwordEnv`）。
- agent 的 SQL 中出现 `FROM public.customers`。
- `type: oracle`（不存在该类型）。
- CORS 设置为 `origins: ["*"]`。
- 当 catalog 中没有该实体时，在 `qllm.access.yaml` 中写 `tables: [customers]`。

请逐字段对照 [field-reference.md](field-reference.md)。
