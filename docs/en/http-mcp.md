# HTTP and MCP

## HTTP `/v1`

Listener: `--addr` / `serve.addr` (default `127.0.0.1:8088`).

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| GET | `/v1/health` | **no** | Probes |
| GET | `/v1/howtouseme` | yes* | Closed guide for IR and SQL |
| GET | `/v1/catalog` | yes* | Catalog (filtered by ACL) |
| POST | `/v1/queries` | yes* | Body is a Query IR |
| POST | `/v1/sql` | yes* | `{ "sql", "version"? }` |
| GET | `/v1/queries/{id}` | yes* | Async status |
| GET | `/v1/queries/{id}/result` | yes* | Result; `NOT_READY` if not done yet |

`*` When `authTokenEnv` or `qllm.access.yaml` is active. Otherwise, on loopback, the API is open.

A `POST` may return **202** with a `queryId` (async). The job still honors the ~15 s budget. The in-memory store keeps results for about 2 minutes.

Logs: `http` (method/path), `http_execute_sql` (summary), and the `---- execute_sql ----` block from the executor (full SQL).

This listener has **no** CORS.

## MCP

Tools (these only):

| Tool | Args | Effect |
|------|------|--------|
| `how_to_use_me` | none | Same purpose as `/v1/howtouseme` |
| `describe_catalog` | none | Catalog JSON (ACL applied) |
| `execute_sql` | `sql` (required), `version` optional | Same as `POST /v1/sql` |

There is no Query IR tool.

### Stdio

```bash
./qllm serve --mcp --config-dir ./my-project
```

Local Inspector. No Bearer; ACL comes from `--app` / `QLLM_APP`.

### HTTP

```bash
./qllm serve --mcp-http --config-dir ./my-project
```

| Path | Transport |
|------|-----------|
| `/mcp` | Streamable HTTP |
| `/sse` + `/message` | SSE (older Inspector) |

Auth: the same Bearer and ACL as `/v1`. CORS applies only here (`serve.cors` / `--cors-origin`). Empty origins mean CORS is off, so the browser in **Direct** mode fails; use **Via Proxy** in the Inspector.

Logs: `---- mcp_tool ----` and the same `execute_sql` block.

### Inspector (PRD simulation)

URL `http://127.0.0.1:18089/mcp` (port-forward). Send `Authorization: Bearer …` with the header **toggle on**. See [`deploy/prd-tst/README.md`](../../deploy/prd-tst/README.md).

## Auth

1. **None**: safe only on loopback (or with `--insecure-bind` / `insecureBind`).
2. **One token**: `serve.authTokenEnv` names a non-empty env var. Send `Authorization: Bearer <value>`.
3. **Apps**: when `qllm.access.yaml` is present, the token selects the app, and `authTokenEnv` stops being the model.

Bearer comparison uses HMAC-SHA256 and `hmac.Equal`, so it does not branch on `len`.

A `0.0.0.0` bind without a token and without `insecureBind` is refused at startup.

## LangChain client (MCP HTTP)

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

Do not invent tools. After `get_tools()`, the model should follow `how_to_use_me`.
