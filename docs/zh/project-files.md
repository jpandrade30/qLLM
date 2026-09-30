# 项目文件

从零开始的教程，以及确认你加载了正确目录的方法：[from-scratch.md](from-scratch.md)。逐字段的表格：[field-reference.md](field-reference.md)。部署时应指向哪里：[point-your-folder.md](point-your-folder.md)。

一个 qLLM 项目就是一个包含 YAML 或 JSON 文件的目录。没有这些文件时，`serve` **不会**回退到 `fixtures/` 或演示默认值。

## 发现机制

按顺序（实际上互斥）：

1. `--preset` **和** `--catalog`（两者都要；只给其中一个会返回 `CONFIG_ERROR`）。
2. `--project`：一个 `qllm.project.yaml`，其中的 `preset` 和 `catalog` 路径相对于它自身（不能离开项目文件所在的目录）。
3. `--config-dir DIR`：`DIR/qllm.preset.{yaml|yml|json}` 加上 `DIR/qllm.catalog.{yaml|yml|json}`。
4. 没有 `--config-dir`：使用**当前工作目录**。

同一目录中的可选文件（存在对应参数时也可以使用显式路径）：

| 文件 | 是否必需 | 作用 |
|------|----------|------|
| `qllm.preset.*` | 是 | 数据源、`limits`、`connection.*Env` |
| `qllm.catalog.*` | 是 | 逻辑实体、fields、relations、`binding` |
| `qllm.config.*` | 否 | 绑定、`authTokenEnv`、CORS（MCP HTTP）、上限 |
| `qllm.access.*` | 否 | 应用、keys、`tables`；**取代**单一 Bearer 令牌 |
| `qllm.env.*` | 否 | 当进程变量为空时，补全环境变量 |
| `qllm.project.yaml` | 否 | 指向 `preset` 和 `catalog` 的指针文件 |

Schemas：[`planning/schemas/`](../../planning/schemas/)。说明文字：[`planning/03-protocol-schemas.md`](../../planning/03-protocol-schemas.md)。

## serve 的优先级

安全默认值（回环地址、关闭 CORS），然后是 `qllm.config.yaml`，最后是 CLI 参数。

`qllm.env.yaml`：进程或 Secret 中**非空**的值优先。值可以是字面值，也可以严格写成 `${NAME}`。不要创建空的占位符。**不要把这些值写入日志**。

## Preset：你必须填写的内容

必填：`protocolVersion`、`project`、`limits`、`sources[]`（`id`、`type`、`connection`）。

`limits`（规范默认值）：`maxSyncMs` 15000、`maxSourceMs` 12000、`defaultLimit` 100、`maxLimit` 1000、`readOnly` true。

`sources[].id`：`[a-z][a-z0-9_]*`。`type`：参见 [connectors.md](connectors.md)。

密钥只能通过 `*Env`（或保存 URI 的环境变量）提供。切勿把密码提交到 preset 中。

各类型的 `connection` 示例：`03-protocol-schemas.md` 的第 1 节。

## Catalog：agent 能看到什么

- `entities[].name`（以及 `aliases`）是 SQL 中的表名，也是 IR 的 `from`。
- `fields[].name` 是逻辑列。`physical` 映射到真实的列或文档键。
- `binding` 指向物理对象（`table` / `collection` / `rest_resource`，KV 和流类数据源还需要 `accessPath`）。
- `relations` 用于说明 join；SQL 或 IR 仍然需要正确地引用它们。

同一个物理字段出现在 10 个 API 中，意味着有 **10 个实体**（`crm_users` 与 `erp_users`），而不是共用一个 `users`。

## Access（`qllm.access.yaml`）

```yaml
apps:
  - name: crm-agent
    key: ${QLLM_CRM_AGENT_KEY}
    tables: [customers, invoices]
```

- `tables` 是 catalog 中的实体名。
- HTTP 与 MCP HTTP：`Authorization: Bearer <key>`。
- MCP stdio、`qllm query` 和 `qllm sql`：`--app` 或 `QLLM_APP`（应用名称，而不是 key）。
- 文件存在时：catalog 和 `howtouseme` 会被过滤；列表之外的实体返回 `FORBIDDEN`。
- 文件不存在时：使用单一令牌（`authTokenEnv`），或者在回环地址上不认证；会暴露完整的 catalog。

## 运行时（`qllm.config.yaml`）

字段定义见 [`runtime-config.schema.json`](../../planning/schemas/runtime-config.schema.json)。`additionalProperties: false`。

| 字段 | 默认值 / 规则 |
|------|----------------|
| `serve.addr` | `127.0.0.1:8088` |
| `serve.mcpAddr` | `127.0.0.1:8089` |
| `serve.authTokenEnv` | Bearer 环境变量的名称；一旦设置了名称，该环境变量**必须**非空 |
| `serve.insecureBind` | `false`；在没有认证的情况下绑定非回环地址，需要 `true` 或 `--insecure-bind` |
| `serve.maxBodyBytes` | POST 请求体上限（schema 最小值 1024） |
| `serve.maxRestResponseBytes` | REST 连接器的上限 |
| `serve.cors.origins` | 为空表示关闭 CORS；`*` 会被拒绝 |

## 实施所需的最小目录结构

```text
my-project/
  qllm.preset.yaml
  qllm.catalog.yaml
  qllm.config.yaml      # 任何对外暴露的场景都建议提供
  qllm.access.yaml      # 存在多个 agent 时提供
  qllm.env.yaml         # 仅限本地；在 K8s 上请使用 Secret
```

```bash
export QLLM_…   # preset 引用的所有变量
./qllm validate --config-dir ./my-project
./qllm serve --http --mcp-http --config-dir ./my-project
```
