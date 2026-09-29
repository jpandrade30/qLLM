# qLLM

Multi-source query runtime (Go). Configure sources with YAML preset + logical catalog, query via JSON IR, serve HTTP `/v1` or MCP.

Protocol **0.1.0** — see [planning/](planning/).

## Quick start (5 minutes)

### 1. Build

```bash
go build -o qllm ./cmd/qllm
```

### 2. Point at your project specs

```text
my-project/
  qllm.preset.yaml    # sources + *Env secrets
  qllm.catalog.yaml   # logical entities
  qllm.config.yaml    # optional: bind, auth, CORS, body caps
  qllm.access.yaml    # optional: apps, keys (${ENV} or literal), tables
```

Set env vars referenced by the preset (`QLLM_*`), then:

```bash
./qllm validate --config-dir ./my-project
./qllm serve --http --config-dir ./my-project
```

Defaults listen on **`127.0.0.1:8088`** (HTTP) and **`127.0.0.1:8089`** (MCP HTTP). Non-loopback bind without auth requires `--insecure-bind` or `serve.insecureBind: true`.

### 3. Query

```bash
curl -s http://127.0.0.1:8088/v1/howtouseme | jq .
curl -s http://127.0.0.1:8088/v1/catalog | jq .
curl -s -X POST http://127.0.0.1:8088/v1/queries -d @fixtures/queries/customers_list.json
```

`GET /v1/howtouseme` is the closed Query IR contract for LLMs (`never`, where shapes, invalidExamples). Call it before inventing queries.

Or CLI:

```bash
./qllm query --config-dir ./fixtures/presets -f ./fixtures/queries/customers_list.json
```

### Auth (optional)

Set `serve.authTokenEnv` in `qllm.config.yaml` (or `--auth-token-env`) to an env var that holds the shared Bearer token for HTTP `/v1` and MCP HTTP. If the env name is set, the variable **must** be non-empty or serve refuses to start. Stdio MCP is unaffected.

```bash
export QLLM_AUTH_TOKEN=dev-secret
# qllm.config.yaml: serve.authTokenEnv: QLLM_AUTH_TOKEN
curl -s -H "Authorization: Bearer $QLLM_AUTH_TOKEN" http://127.0.0.1:8088/v1/catalog
```

`GET /v1/health` stays unauthenticated for probes.

Optional `qllm.access.yaml` replaces the single Bearer token. Each app has a `key` (literal or `${ENV_NAME}`) and `tables` from the catalog. HTTP/MCP HTTP use `Authorization: Bearer <key>`. MCP stdio uses `--app` / `QLLM_APP`.

```yaml
apps:
  - name: crm-agent
    key: ${QLLM_CRM_AGENT_KEY}
    tables: [customers, invoices]
```

### Docker

Image bakes `deploy/image/config` at `/config` (`qllm.preset.yaml`, catalog, `qllm.config.yaml`, `qllm.env.yaml`). Connection/auth values live in `qllm.env.yaml`, not Dockerfile `ENV`. Process env / K8s Secret wins if already set.

```bash
nerdctl build -t qllm .
nerdctl run --rm -p 8088:8088 -p 8089:8089 qllm
```

Same network as the databases (compose or K8s namespace). HTTP: `Authorization: Bearer change-me`.

Satellites + app:

```bash
nerdctl compose up --build
```

### Runtime config (`qllm.config.yaml`)

Optional sibling file (schema: [planning/schemas/runtime-config.schema.json](planning/schemas/runtime-config.schema.json)). Precedence: secure defaults → file → CLI flags.

```yaml
serve:
  addr: "127.0.0.1:8088"
  mcpAddr: "127.0.0.1:8089"
  authTokenEnv: "QLLM_AUTH_TOKEN"
  insecureBind: false
  maxBodyBytes: 1048576
  maxRestResponseBytes: 10485760
  cors:
    origins: []   # empty = CORS off; wildcard * rejected
```

Flags: `--runtime-config`, `--addr`, `--mcp-addr`, `--auth-token-env`, `--insecure-bind`, `--cors-origin` (repeatable).

### MCP

**STDIO** (Inspector local):

```bash
./qllm serve --mcp --config-dir ./fixtures/presets
```

**Streamable HTTP + SSE** (Jupyter / LangChain) — defaults to loopback; enable CORS allowlist only if a browser client needs it:

```bash
./qllm serve --mcp-http --config-dir ./fixtures/presets
# or together with REST:
./qllm serve --http --mcp-http --config-dir ./fixtures/presets
```

| Path | Transport |
|------|-----------|
| `http://127.0.0.1:8089/mcp` | Streamable HTTP (LangChain) |
| `http://127.0.0.1:8089/sse` | SSE (legacy Inspector) |

```python
# pip install langchain-mcp-adapters
from langchain_mcp_adapters.client import MultiServerMCPClient

client = MultiServerMCPClient({
    "qllm": {
        "transport": "streamable_http",
        "url": "http://127.0.0.1:8089/mcp",
    }
})
tools = await client.get_tools()
```

Tools: `how_to_use_me`, `describe_catalog`, `execute_query`, `get_query`.

Requires Go **1.25+** toolchain (deps). `mcp-go` is pinned at **v0.48.0**; with Go 1.25.5+ you can later bump toward `v0.56`.

## Dev harness notes

`deploy/dev` fixtures use weak passwords, open Mongo, and `sslMode: disable` for local K8s only — do not expose those ports beyond localhost.

## Fail-fast

Default budget ~15s (`limits.maxSyncMs`). Slow sources return typed `TIMEOUT` — not something qLLM tries to “fix” for you.

## Local join engine

Default build uses the **pure Go** engine in `internal/duckdblocal` (no CGO). Cross-source joins, REST aggs, where (`and`/`or`/`not` + compare ops), offset, and composite join `on` run there.

### Optional embedded DuckDB (`-tags duckdb`)

Requires CGO + a C compiler. Same `Engine` API; `Open()` swaps implementation.

```bash
# Linux/macOS
CGO_ENABLED=1 go test -tags duckdb ./internal/duckdblocal/
CGO_ENABLED=1 go build -tags duckdb -o qllm ./cmd/qllm

# Windows — load gcc + duckdblib first
.\scripts\dev-shell.ps1
go test -tags duckdb ./internal/duckdblocal/
go build -tags duckdb -o qllm.exe ./cmd/qllm
go run -tags duckdb .\scripts\duckdb_smoke.go
```

Without `-tags duckdb`, `go test ./...` stays CGO-free.
