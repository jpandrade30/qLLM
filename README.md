# qLLM

Multi-source query runtime (Go). Configure sources with YAML preset + logical catalog, query via JSON IR, serve HTTP `/v1` or MCP.

Protocol **0.2.0** (0.1.0 files remain valid) — see [planning/](planning/) (contracts) and [CHANGELOG.md](CHANGELOG.md). Hand-authoring YAML from zero: [docs/from-scratch.md](docs/from-scratch.md), field list [docs/field-reference.md](docs/field-reference.md).

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

Without YAML in `--config-dir` (or CWD), serve fails with `CONFIG_ERROR`. Compose/`fixtures/` only start test DBs and the fake API — they are not the catalog. Format, sources, ACL, and env names live in `deploy/image/config/` (this repo’s instance) or your own project dir.

Draft catalog from a live SQL source or an OpenAPI file (review before serve):

```bash
./qllm catalog introspect --source crm_pg --config-dir ./my-project --out ./my-project/qllm.catalog.yaml
./qllm catalog from-openapi -f ./other-team.yaml --source legacy_api --config-dir ./my-project --out ./draft.catalog.yaml --resources-out ./draft.resources.yaml
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
./qllm query --config-dir ./deploy/image/config -f ./fixtures/queries/customers_list.json
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

Image `/config` is copied **only** from [`deploy/image/config`](deploy/image/config) (preset, catalog, `qllm.config.yaml`, `qllm.env.yaml`). `fixtures/` is not copied into the image except as the `test-api` build context. Process env wins if already set.

```bash
nerdctl compose up --build
```

HTTP: `Authorization: Bearer change-me`. Seed: `.\scripts\dev-seed-fake.ps1` loads `fixtures/datasets/v1` (add `--regenerate` only to rewrite the frozen JSON). Rebuild `test-api` if `fixtures/test-api/data.json` changed. SQL MCP goldens: `pytest fixtures/sqlcheck`.

Optional **fleet-ops** Kubernetes sim: [`deploy/prd/README.md`](deploy/prd/README.md) (`kubectl apply -k deploy/prd`). Port-forward everything: `.\scripts\prd-port-forward.ps1`. Do not run with compose. Goldens unchanged.

Standalone image (same compose network / `--network`):

```bash
nerdctl build -t qllm .
nerdctl run --rm -p 8088:8088 -p 8089:8089 qllm
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
./qllm serve --mcp --config-dir ./deploy/image/config
```

**Streamable HTTP + SSE** (Jupyter / LangChain) — defaults to loopback; enable CORS allowlist only if a browser client needs it:

```bash
./qllm serve --mcp-http --config-dir ./deploy/image/config
# or together with REST:
./qllm serve --http --mcp-http --config-dir ./deploy/image/config
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

Tools: `how_to_use_me`, `describe_catalog`, `execute_sql`. Each `execute_sql` writes a multiline block to stderr (`---- execute_sql ----` plus the SQL). MCP logs `---- mcp_tool ----` for the other two. Example: `kubectl logs -n qllm-prd deploy/qllm | findstr execute_sql`.

Requires Go **1.26+** (`go.mod` and the image build stage `golang:1.26-bookworm`). `mcp-go` is pinned at **v0.48.0**.

### Experimental sources (0.2.0)

`mssql`, `sqlite`, `clickhouse`, `dynamodb`, `cassandra`, and `ksql` (pull) are implemented in the binary. They are **not** in `docker-compose.yml` or SQL goldens. Dynamo/Cassandra/ksql need `binding.accessPath` and equality on that key, or the query fails with `UNSUPPORTED`. Connection keys: [planning/04-connectors.md](planning/04-connectors.md).

Secrets in `qllm.env.yaml` should be `${QLLM_…}`; set the same names in the process or compose `environment:`.

## Dev harness notes

Compose DBs use weak passwords, open Mongo, and `sslMode: disable` for **local** use — do not publish those ports beyond localhost.

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
