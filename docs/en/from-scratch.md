# Create a qLLM project from scratch

qLLM does **not** guess your tables. You write YAML, and the process reads **one folder**. If that folder is not the one you think it is, you will see the demo catalog or `CONFIG_ERROR`.

## 1. What you will create

A folder of **your own** (do not use `fixtures/` as your product). Use these exact names; the binary looks for these stems:

```text
C:\data\my-qllm\            (example)
  qllm.preset.yaml          REQUIRED: databases/APIs and limits
  qllm.catalog.yaml         REQUIRED: names the agent may SELECT
  qllm.config.yaml          recommended: port, token, CORS
  qllm.env.yaml             optional: fill empty env vars on your PC
  qllm.access.yaml          optional: several agents / allowlists
```

Accepted extensions: `.yaml`, `.yml`, `.json`. You cannot use `preset.yaml` without the `qllm.` prefix.

`protocolVersion` in YAML: `"0.1.0"` or `"0.2.0"` (No version `N.N.N`). The runtime **responds** with `0.2.0`.

All fields: [field-reference.md](field-reference.md). Sample folder (a copy of the demo, baked by the `Dockerfile`): [`deploy/prd/`](../../deploy/prd). Docker and Kubernetes: [point-your-folder.md](point-your-folder.md).

## 2. Order of work

1. List your real sources (host, port, user, database **or** URI **or** URL).
2. Write the **preset**: one `sources[].id` per connection. `id` must match `[a-z][a-z0-9_]*` (for example `crm_pg`, not `CRM-PG`).
3. Create environment variables with the **names** you used in `hostEnv`, `passwordEnv`, and so on. The YAML never holds the password; it holds the **name** of the env var (`QLLM_CRM_PG_PASSWORD`).
4. Write the **catalog**: one entity per logical table. `name` is what goes in `FROM`. `source` is an `id` from the preset. `binding` is the **physical** schema and table (or collection/resource).
5. Run `qllm validate --config-dir …` until it prints `ok` and `entities=N` with the **N you wrote**.
6. Run `qllm serve --http --mcp-http --config-dir …`.
7. Confirm with health and catalog (section 5). Without this you cannot tell whether you loaded the wrong YAML.

## 3. Minimal example (one Postgres)

`qllm.preset.yaml`: every key under `connection` that ends in `Env` is the **name** of a variable, not its value:

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

In PowerShell, **before** serving:

```powershell
$env:QLLM_CRM_PG_HOST = "127.0.0.1"
$env:QLLM_CRM_PG_USER = "app"
$env:QLLM_CRM_PG_PASSWORD = "secret"
```

`qllm.catalog.yaml`: the agent **never** writes `public.customers`; it writes `customers`:

```yaml
protocolVersion: "0.2.0"
project: my-company
entities:
  - name: customers
    description: CRM customers
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
        description: Unique email
```

`qllm.config.yaml`:

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

More sources, REST, and Dynamo: [field-reference.md](field-reference.md).

## 4. Validate what you wrote (no server yet)

In the qLLM **source** folder (where the binary is), point to **your** folder:

```powershell
cd C:\codes\qLLM
go build -tags duckdb -o qllm.exe .\cmd\qllm
.\qllm.exe validate --config-dir C:\data\my-qllm
```

You should see:

```text
ok preset=C:\data\my-qllm\qllm.preset.yaml catalog=C:\data\my-qllm\qllm.catalog.yaml entities=1
```

- The paths must be **your** files (not `deploy\image\config`).
- `entities=` is the number of entries under `entities:` in the catalog.
- JSON on stderr with `"code":"CONFIG_ERROR"` means missing YAML, an extra field (`additionalProperties: false`), an invalid `id`, or `--preset` without `--catalog`.

If validation passes and your catalog has `customers`, an IR against `invoices` must fail:

```powershell
# file tmp.json: { "from": "invoices", "select": ["id"], "limit": 1 }
.\qllm.exe validate --config-dir C:\data\my-qllm --ir C:\data\tmp.json
```

Expected: `UNKNOWN_ENTITY`. If it passes, `--config-dir` is **not** the folder you think it is.

## 5. Serve and prove it is **your** project

```powershell
.\qllm.exe serve --http --mcp-http --config-dir C:\data\my-qllm
```

Stderr must show:

```text
qllm http listening on 127.0.0.1:8088
qllm mcp-http listening on 127.0.0.1:8089 (/mcp streamable, /sse SSE)
```

In another terminal:

```powershell
curl.exe -s -H "Authorization: Bearer a-long-token" http://127.0.0.1:8088/v1/health
curl.exe -s -H "Authorization: Bearer a-long-token" http://127.0.0.1:8088/v1/catalog
```

Health: `"ok":true` and `"protocolVersion":"0.2.0"`.

Catalog: `"project":"my-company"` (the string from **your** YAML) and `entities` containing `customers`. If you see `qllm-demo` with `customers` and `invoices` from the harness, the process is **not** using `C:\data\my-qllm`. You forgot `--config-dir`, Docker mounted a different folder, or Kubernetes mounted an old ConfigMap.

Smoke SQL (against **your** table):

```powershell
curl.exe -s -H "Authorization: Bearer a-long-token" -H "Content-Type: application/json" `
  -d "{\"sql\":\"SELECT id, email FROM customers LIMIT 5\"}" `
  http://127.0.0.1:8088/v1/sql
```

- `"status":"succeeded"` with rows: the database connects and the catalog matches.
- `UNKNOWN_ENTITY`: the SQL uses a `name` that is not in the loaded catalog.
- `SOURCE_ERROR` / `TIMEOUT`: the YAML is fine; network, credentials, or host are wrong.
- `UNAUTHORIZED`: the token differs from `QLLM_AUTH_TOKEN`, or the header is malformed (`Bearer ` followed by a space).

MCP: the same proofs through the `describe_catalog` and `execute_sql` tools. The process log shows `---- execute_sql ----` with the SQL.

## 6. Generate a draft instead of writing the catalog by hand

With Postgres or MySQL already in the preset and the env vars set:

```powershell
.\qllm.exe catalog introspect --source crm_pg --config-dir C:\data\my-qllm --out C:\data\my-qllm\qllm.catalog.yaml
```

Open the file and confirm `source`, `binding.schema`/`table`, and `fields`. Add `relations` and `description`. Then run `validate` again.

REST: `catalog from-openapi` plus pasting `options.resources` into the preset ([cli.md](cli.md)).

## 7. What you must **not** put in the YAML

- Fields that are not in the schema: validate and serve refuse them (`additionalProperties: false` for preset, catalog, config, access, env, and project).
- A plaintext password in the preset (use `passwordEnv`).
- `FROM public.customers` in the agent's SQL.
- `type: oracle` (it does not exist).
- CORS `origins: ["*"]`.
- `qllm.access.yaml` with `tables: [customers]` when the catalog has no such entity.

Follow [field-reference.md](field-reference.md) field by field.
