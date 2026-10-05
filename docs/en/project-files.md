# Project files

Tutorial from zero and proof that you loaded the right folder: [from-scratch.md](from-scratch.md). Field-by-field tables: [field-reference.md](field-reference.md). Where to point in a deployment: [point-your-folder.md](point-your-folder.md).

A qLLM project is a directory of YAML or JSON files. Without them, `serve` does **not** fall back to `fixtures/` or demo defaults.

## Discovery

In order (mutually exclusive in practice):

1. `--preset` **and** `--catalog` (both; one alone returns `CONFIG_ERROR`).
2. `--project`: a `qllm.project.yaml` whose `preset` and `catalog` paths are relative to it (they cannot leave the project file's directory).
3. `--config-dir DIR`: `DIR/qllm.preset.{yaml|yml|json}` plus `DIR/qllm.catalog.{yaml|yml|json}`.
4. No `--config-dir`: the **current working directory**.

Optional files in the same directory (or an explicit path where a flag exists):

| File | Required | Purpose |
|------|----------|---------|
| `qllm.preset.*` | yes | Sources, `limits`, `connection.*Env` |
| `qllm.catalog.*` | yes | Logical entities, fields, relations, `binding` |
| `qllm.config.*` | no | Bind, `authTokenEnv`, CORS (MCP HTTP), caps |
| `qllm.access.*` | no | Apps, keys, `tables`; **replaces** the single Bearer token |
| `qllm.env.*` | no | Seeds env vars when the process variable is empty |
| `qllm.project.yaml` | no | Pointer to `preset` and `catalog` |

Schemas: [`planning/schemas/`](../../planning/schemas/). Prose: [`planning/03-protocol-schemas.md`](../../planning/03-protocol-schemas.md).

## Serve precedence

Secure defaults (loopback, CORS off), then `qllm.config.yaml`, then CLI flags.

`qllm.env.yaml`: a **non-empty** process or Secret value wins. A value can be a literal or exactly `${NAME}`. Do not create an empty placeholder. **Do not log** these values.

## Preset: what you must fill in

Required: `protocolVersion`, `project`, `limits`, `sources[]` (`id`, `type`, `connection`).

`limits` (spec defaults): `maxSyncMs` 15000 **ms**, `maxSourceMs` 12000 **ms**, `defaultLimit` 100 **rows**, `maxLimit` 1000 **rows**, `readOnly` true.

`sources[].id`: `[a-z][a-z0-9_]*`. `type`: see [connectors.md](connectors.md).

Secrets only through `*Env` (or a URI env var). Never commit passwords in the preset.

`connection` examples by type: section 1 of `03-protocol-schemas.md`.

## Catalog: what the agent sees

- `entities[].name` (and `aliases`) are the table names in SQL and the IR `from`.
- `fields[].name` are the logical columns. `physical` maps to the real column or document key.
- `binding` points to the physical object (`table` / `collection` / `rest_resource`, plus `accessPath` for KV and stream sources).
- `relations` document joins; the SQL or IR must still cite them correctly.

The same physical field across 10 APIs means **10 entities** (`crm_users` vs `erp_users`), not one shared `users`.

## Access (`qllm.access.yaml`)

```yaml
apps:
  - name: crm-agent
    key: ${QLLM_CRM_AGENT_KEY}
    tables: [customers, invoices]
```

- `tables` are catalog entity names.
- HTTP and MCP HTTP: `Authorization: Bearer <key>`.
- MCP stdio, `qllm query`, and `qllm sql`: `--app` or `QLLM_APP` (the app name, not the key).
- File present: the catalog and `howtouseme` are filtered; an entity outside the list returns `FORBIDDEN`.
- File absent: a single token (`authTokenEnv`), or no auth on loopback; the full catalog is exposed.

## Runtime (`qllm.config.yaml`)

Fields are defined in [`runtime-config.schema.json`](../../planning/schemas/runtime-config.schema.json). `additionalProperties: false`.

| Field | Default / rule |
|-------|----------------|
| `serve.addr` | `127.0.0.1:8088` |
| `serve.mcpAddr` | `127.0.0.1:8089` |
| `serve.authTokenEnv` | Name of the Bearer env var; if the name is set, that env var **must** be non-empty |
| `serve.insecureBind` | `false`; a non-loopback bind without auth requires `true` or `--insecure-bind` |
| `serve.maxBodyBytes` | POST body cap in **bytes** (schema minimum 1024 bytes; default 1048576 bytes = 1 MiB) |
| `serve.maxRestResponseBytes` | REST response cap in **bytes** (default 10485760 bytes = 10 MiB) |
| `serve.cors.origins` | Empty means CORS off; `*` is rejected |

## Minimal layout to implement

```text
my-project/
  qllm.preset.yaml
  qllm.catalog.yaml
  qllm.config.yaml      # recommended for any exposure
  qllm.access.yaml      # if you have more than one agent
  qllm.env.yaml         # local only; on K8s use a Secret
```

```bash
export QLLM_…   # everything the preset references
./qllm validate --config-dir ./my-project
./qllm serve --http --mcp-http --config-dir ./my-project
```
