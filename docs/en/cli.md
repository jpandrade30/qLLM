# CLI (`qllm`)

Binary: `go build -o qllm ./cmd/qllm`. Production and the image use `-tags duckdb`; see [build.md](build.md).

Common config flags (almost every subcommand):

| Flag | Purpose |
|------|---------|
| `--config-dir` | Folder with `qllm.preset` and `qllm.catalog` |
| `--preset` / `--catalog` | Explicit paths (both required together) |
| `--project` | Path to `qllm.project.yaml` |

## `qllm validate`

Validates preset and catalog without opening any source. Optional `--ir FILE` validates a Query IR against the catalog.

```bash
./qllm validate --config-dir ./my-project
./qllm validate --config-dir ./my-project --ir ./query.json
```

On success, stderr prints `ok preset=… catalog=… entities=N`. Errors are typed JSON on stderr.

## `qllm query`

Runs a Query IR file. `--file` / `-f` is required. Opens the sources.

Use `--app` or `QLLM_APP` when `qllm.access.yaml` exists. A template app also needs `--scope` / `QLLM_SCOPE` (the user code).

Applies `qllm.env.yaml` from the config directory before connecting.

```bash
./qllm query --config-dir ./my-project -f ./query.json
```

## `qllm sql`

Runs a text file containing catalog SQL. `--file` / `-f` is required.

Omitting `--version` uses the latest dialect (`"2"`). `"1"` is the frozen dialect (no set operations, no `QUALIFY`).

`--app` / `QLLM_APP` applies ACLs. `--scope` / `QLLM_SCOPE` supplies the row-scope value for a template app.

Requires a build with embedded DuckDB for `ExecSQL`.

```bash
./qllm sql --config-dir ./my-project -f ./q.sql
./qllm sql --config-dir ./my-project -f ./q.sql --version 1
```

## `qllm serve`

With none of `--http`, `--mcp`, or `--mcp-http`, **HTTP is enabled** by default.

| Flag | Effect |
|------|--------|
| `--http` | REST `/v1` |
| `--mcp-http` | MCP at `/mcp`, plus `/sse` and `/message` |
| `--mcp` | MCP over **stdio** (local Inspector). **Exclusive**: do not combine with `--http` or `--mcp-http` |
| `--addr` | HTTP listen address (default `127.0.0.1:8088`) |
| `--mcp-addr` | MCP HTTP listen address (default `127.0.0.1:8089`) |
| `--runtime-config` | Path to `qllm.config.yaml` |
| `--auth-token-env` | Name of the env var that holds the Bearer token |
| `--insecure-bind` | Allow a non-loopback bind without auth |
| `--cors-origin` | Repeatable; MCP HTTP allowlist |
| `--app` | App name for stdio when `qllm.access.yaml` exists |

```bash
./qllm serve --http --mcp-http --config-dir ./my-project
./qllm serve --mcp --config-dir ./my-project --app crm-agent
```

Stdio with `qllm.access.yaml` and no `--app` / `QLLM_APP` returns `CONFIG_ERROR`. Template apps also need `--scope` / `QLLM_SCOPE`.

## `qllm catalog introspect`

Reads `information_schema` from a **postgres or mysql** source in the preset and writes catalog YAML. It does **not** serve.

| Flag | Meaning |
|------|---------|
| `--source` | `sources[].id` (required) |
| `--out` | Output file (default: stdout) |
| `--merge` | Keep entities from other sources in the already loaded catalog |

Introspection times out after 15 seconds. Review the YAML (relations, descriptions) before `serve`.

```bash
./qllm catalog introspect --source crm_pg --config-dir ./my-project --out ./my-project/qllm.catalog.yaml
```

## `qllm catalog from-openapi`

Generates `rest_resource` entities plus an `options.resources` fragment from an OpenAPI 3 spec. `--source` must be a `type: rest` source in the preset.

| Flag | Meaning |
|------|---------|
| `-f` / `--file` | OpenAPI spec |
| `--source` | REST source id |
| `--out` | Catalog output |
| `--resources-out` | Resources YAML fragment (otherwise printed to stderr) |
| `--merge` | Same as for `introspect` |

The REST connector reads **only** what is already in the preset. Paste the fragment into `sources[].options.resources`.
