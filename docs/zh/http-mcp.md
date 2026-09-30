# HTTP 与 MCP

## HTTP `/v1`

监听器：`--addr` / `serve.addr`（默认 `127.0.0.1:8088`）。

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/v1/health` | **否** | 探针 |
| GET | `/v1/howtouseme` | 是* | 面向 IR 和 SQL 的封闭式指南 |
| GET | `/v1/catalog` | 是* | catalog（按 ACL 过滤） |
| POST | `/v1/queries` | 是* | 请求体为 Query IR |
| POST | `/v1/sql` | 是* | `{ "sql", "version"? }` |
| GET | `/v1/queries/{id}` | 是* | 异步状态 |
| GET | `/v1/queries/{id}/result` | 是* | 结果；尚未完成时返回 `NOT_READY` |

`*` 当 `authTokenEnv` 或 `qllm.access.yaml` 生效时。否则，在回环地址上 API 是开放的。

`POST` 可能返回 **202** 和一个 `queryId`（异步）。该任务仍然遵守约 15 秒的预算。内存存储会将结果保留约 2 分钟。

日志：`http`（方法/路径）、`http_execute_sql`（摘要），以及执行器输出的 `---- execute_sql ----` 块（完整 SQL）。

此监听器**没有** CORS。

## MCP

工具（仅有以下几个）：

| 工具 | 参数 | 作用 |
|------|------|------|
| `how_to_use_me` | 无 | 与 `/v1/howtouseme` 作用相同 |
| `describe_catalog` | 无 | catalog 的 JSON（已应用 ACL） |
| `execute_sql` | `sql`（必填）、`version`（可选） | 与 `POST /v1/sql` 相同 |

没有 Query IR 工具。

### Stdio

```bash
./qllm serve --mcp --config-dir ./my-project
```

本地 Inspector。无 Bearer；ACL 来自 `--app` / `QLLM_APP`。

### HTTP

```bash
./qllm serve --mcp-http --config-dir ./my-project
```

| 路径 | 传输方式 |
|------|----------|
| `/mcp` | Streamable HTTP |
| `/sse` + `/message` | SSE（旧版 Inspector） |

认证：与 `/v1` 使用相同的 Bearer 和 ACL。CORS 仅在此处生效（`serve.cors` / `--cors-origin`）。origins 为空表示关闭 CORS，因此浏览器在 **Direct** 模式下会失败；请在 Inspector 中使用 **Via Proxy**。

日志：`---- mcp_tool ----` 以及同样的 `execute_sql` 块。

### Inspector（PRD 模拟环境）

URL `http://127.0.0.1:18089/mcp`（端口转发）。发送 `Authorization: Bearer …`，并且**打开 header 的开关**。参见 [`deploy/prd-tst/README.md`](../../deploy/prd-tst/README.md)。

## 认证

1. **无**：只有在回环地址上才安全（或者使用 `--insecure-bind` / `insecureBind`）。
2. **单一令牌**：`serve.authTokenEnv` 指向一个非空环境变量。发送 `Authorization: Bearer <值>`。
3. **应用**：存在 `qllm.access.yaml` 时，由令牌决定是哪个应用，`authTokenEnv` 不再作为认证模型。

Bearer 比较使用 HMAC-SHA256 和 `hmac.Equal`，因此不会根据 `len` 产生分支。

如果绑定 `0.0.0.0` 却没有令牌，也没有 `insecureBind`，启动时会被拒绝。

## LangChain 客户端（MCP HTTP）

```python
from langchain_mcp_adapters.client import MultiServerMCPClient

client = MultiServerMCPClient({
    "qllm": {
        "transport": "streamable_http",
        "url": "http://127.0.0.1:8089/mcp",
        "headers": {"Authorization": "Bearer …"},
    }
})
```

不要自行发明工具。调用 `get_tools()` 之后，模型应当遵循 `how_to_use_me`。
