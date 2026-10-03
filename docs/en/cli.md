# CLI (`qllm`)

Binary: `go build -o qllm ./cmd/qllm`. Production and the image use `-tags duckdb`; see [build.md](build.md) and [install.md](install.md).

```text
qllm
├── validate                 check preset + catalog (+ optional IR), no sources opened
├── query                    run a Query IR file
├── sql                      run a catalog SQL file
├── serve                    HTTP /v1 and/or MCP
├── catalog
│   ├── introspect           draft catalog YAML from a postgres/mysql source
│   └── from-openapi         draft rest_resource entities from an OpenAPI 3 file
├── help [command]           built-in help
└── completion               shell completion scripts (built in)
```

`qllm <command> --help` prints the flags of any command. The flag names below are the exact ones in the binary: for example the bind override is `--insecure-bind` (not `--bind-insecure`).

Exit status is `0` on success and `1` on any error. Typed errors are written to **stderr** as JSON (`{"protocolVersion": …, "error": {…}}`); see [errors.md](errors.md).

## How files are found

`validate`, `query`, `sql`, `serve`, and both `catalog` commands share these four flags.

| Flag | Purpose |
|------|---------|
| `--config-dir DIR` | Folder searched for `qllm.preset`, `qllm.catalog`, and the optional `qllm.config`, `qllm.access`, `qllm.env`. Default: the current working directory |
| `--preset FILE` / `--catalog FILE` | Explicit paths. **Both** are required together; setting only one is `CONFIG_ERROR` |
| `--project FILE` | A `qllm.project.yaml` with `preset:` and `catalog:` paths relative to that file. Paths that escape the file's folder are refused |

Precedence: `--preset` + `--catalog` first, then `--project`, then `--config-dir` (or the CWD). In a directory each file is looked up as `.yaml`, then `.yml`, then `.json`. A missing preset or catalog is `CONFIG_ERROR`.

The optional siblings (`qllm.config.*`, `qllm.access.*`, `qllm.env.*`) are only searched in `--config-dir` (or the CWD). `--preset`, `--catalog`, and `--project` do not move that search.

### Environment file (`qllm.env.yaml`)

`query`, `sql`, `serve`, and both `catalog` commands apply `qllm.env.yaml` before connecting. **`validate` does not**, because it opens no source. Rules:

- Only variables that are **empty or unset** in the process are set; the real environment always wins.
- A value is either a literal or exactly `${OTHER_NAME}`, read from the process. A missing `${OTHER_NAME}` is skipped, not written as text. Anything else containing `${` is `CONFIG_ERROR`.

### Environment variables read by the binary itself

| Variable | Used by | Meaning |
|----------|---------|---------|
| `QLLM_APP` | `query`, `sql`, `serve` | Same as `--app` (the flag wins) |
| `QLLM_SCOPE` | `query`, `sql`, `serve` | Same as `--scope` (the flag wins) |
| the name in `serve.authTokenEnv` | `serve` | Holds the shared Bearer token |
| every `*Env` key in the preset | all that open sources | Connection secrets, e.g. `QLLM_CRM_PG_PASSWORD` |

## `qllm validate`

Validates preset and catalog without opening any source.

| Flag | Meaning |
|------|---------|
| config flags | see above |
| `--ir FILE` | Also validate this Query IR against the catalog |

```bash
./qllm validate --config-dir ./my-project
./qllm validate --config-dir ./my-project --ir ./query.json
```

On success stderr prints `ok preset=… catalog=… entities=N` (and `ok ir=…`). Failures are typed JSON on stderr.

## `qllm query`

Runs a Query IR file and prints the JSON response to stdout. Opens the sources.

| Flag | Meaning |
|------|---------|
| config flags | see above |
| `-f`, `--file FILE` | **Required.** IR as JSON or YAML |
| `--app NAME` | App from `qllm.access.yaml` (or `QLLM_APP`) |
| `--scope VALUE` | Row-scope value for a template app (or `QLLM_SCOPE`) |

When `qllm.access.yaml` exists, tables are checked against the app. A template app (`keySecret`) also needs `--scope`. This command does **not** read `qllm.config.yaml`, so REST responses use the default 10 MiB cap.

```bash
./qllm query --config-dir ./my-project -f ./query.json
```

## `qllm sql`

Runs a catalog SQL file and prints the JSON response. Requires a build with embedded DuckDB (`-tags duckdb`).

| Flag | Meaning |
|------|---------|
| config flags | see above |
| `-f`, `--file FILE` | **Required.** Text file with the SQL |
| `--version V` | SQL dialect. Omitted = latest (`"2"`). `"1"` is frozen (no set operations, no `QUALIFY`) |
| `--app`, `--scope` | Same as for `query` |

```bash
./qllm sql --config-dir ./my-project -f ./q.sql
./qllm sql --config-dir ./my-project -f ./q.sql --version 1
```

## `qllm serve`

Starts one or more listeners. With none of `--http`, `--mcp`, `--mcp-http`, **HTTP is on**.

### Modes

| Flag | Effect |
|------|--------|
| `--http` | REST `/v1` (`howtouseme`, `catalog`, `queries`, `sql`, `health`) |
| `--mcp-http` | MCP Streamable HTTP at `/mcp`, SSE at `/sse` and `/message` |
| `--mcp` | MCP over **stdio** for local clients (Inspector). Exclusive: combining it with `--http` or `--mcp-http` is an error. No network listener, so the bind and token rules below do not apply |

`--http` and `--mcp-http` can run together in one process.

### Listening and security flags

| Flag | Default | Meaning |
|------|---------|---------|
| `--addr HOST:PORT` | `127.0.0.1:8088` | HTTP `/v1` address |
| `--mcp-addr HOST:PORT` | `127.0.0.1:8089` | MCP HTTP address |
| `--runtime-config FILE` | `qllm.config.*` in `--config-dir` | Explicit runtime config file |
| `--auth-token-env NAME` | none | Name of the env var holding the shared Bearer token. If set, that variable **must be non-empty** or `serve` fails with `CONFIG_ERROR` |
| `--insecure-bind` | `false` | Allow a **non-loopback** address with **no** auth. See below |
| `--cors-origin ORIGIN` | none (CORS off) | Allowed browser origin for MCP HTTP. Repeatable. `*` is refused |
| `--app NAME` | none | App for MCP stdio when `qllm.access.yaml` exists (or `QLLM_APP`) |
| `--scope VALUE` | none | Row scope for a template app on stdio (or `QLLM_SCOPE`) |

### Precedence

Built-in defaults → `qllm.config.yaml` → flags. A flag counts only if you actually pass it, so `--insecure-bind=false` can override `insecureBind: true` in the file, and `--cors-origin` replaces the file's origins.

### The bind rule and `--insecure-bind`

qLLM refuses to listen on a non-loopback address unless one of these holds:

1. A token is configured (`serve.authTokenEnv` / `--auth-token-env`, non-empty), **or**
2. `qllm.access.yaml` exists (its keys are the auth), **or**
3. `--insecure-bind` / `serve.insecureBind: true` is set.

Loopback means `127.0.0.1`, `::1`, or `localhost`. These are **not** loopback and trigger the rule: `0.0.0.0:8088`, `[::]:8088`, `:8088`, any LAN IP, and any hostname other than `localhost`. Containers need this because they must bind `0.0.0.0`.

`--insecure-bind` does **not** turn authentication off. It only removes the refusal at startup. If a token or access file is present, requests are still checked. It exists for a trusted network you protect another way (a private compose network, a service mesh, an authenticating reverse proxy). Without any auth, anyone who can reach the port can query everything the catalog exposes. The check runs per listener, for `--http` and for `--mcp-http`.

When there is no token and the address is loopback, the API is open to local processes. That is the default posture.

### Settings available only in `qllm.config.yaml`

These have no flag. Schema: [planning/schemas/runtime-config.schema.json](../../planning/schemas/runtime-config.schema.json).

| Key | Default | Meaning |
|-----|---------|---------|
| `serve.maxBodyBytes` | 1048576 (1 MiB) | Maximum request body, HTTP and MCP HTTP |
| `serve.maxRestResponseBytes` | 10485760 (10 MiB) | Maximum body read from a `rest` source |
| `serve.cors.allowHeaders` / `allowMethods` | built-in | Override the CORS allow lists |

### Examples

```bash
./qllm serve --http --mcp-http --config-dir ./my-project
./qllm serve --http --addr 0.0.0.0:8088 --auth-token-env QLLM_AUTH_TOKEN --config-dir ./my-project
./qllm serve --mcp --config-dir ./my-project --app crm-agent
./qllm serve --mcp --config-dir ./my-project --app crm-agent --scope 42
```

Stdio with `qllm.access.yaml` and no `--app` / `QLLM_APP` returns `CONFIG_ERROR`. A template app also needs `--scope` / `QLLM_SCOPE`.

Stdio logs: each `execute_sql` prints a `---- execute_sql ----` block on stderr; the other two tools print `---- mcp_tool ----`.

## `qllm catalog introspect`

Reads `information_schema` from a **postgres or mysql** source in the preset and writes catalog YAML. It does not serve.

| Flag | Meaning |
|------|---------|
| config flags | see above |
| `--source ID` | **Required.** `sources[].id` |
| `--out FILE` | Output file (default: stdout) |
| `--merge` | Keep entities of other sources from the already loaded catalog and replace only this source's |

Times out after 15 seconds. Review the YAML (relations, descriptions) before `serve`.

```bash
./qllm catalog introspect --source crm_pg --config-dir ./my-project --out ./my-project/qllm.catalog.yaml
```

## `qllm catalog from-openapi`

Generates `rest_resource` entities plus an `options.resources` fragment from an OpenAPI 3 spec. `--source` must be a `type: rest` source in the preset.

| Flag | Meaning |
|------|---------|
| config flags | see above |
| `-f`, `--file FILE` | **Required.** OpenAPI 3 YAML or JSON |
| `--source ID` | **Required.** REST source id |
| `--out FILE` | Catalog output (default: stdout) |
| `--resources-out FILE` | Write the resources fragment (otherwise printed to stderr) |
| `--merge` | Same as for `introspect` |

The REST connector reads **only** what is already in the preset. Paste the fragment into `sources[].options.resources`.
