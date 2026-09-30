# What qLLM does and does not do

A **read-only query runtime** (Go). It reads a **preset** (sources, limits, `*Env`) and a **logical catalog** (the entities and fields an agent may cite). It runs Query IR (JSON) or catalog SQL; joins and aggregations that cannot be pushed down go to local compute (DuckDB in the image build).

Protocol advertised in responses: **0.2.0**. **0.1.0** preset, catalog, and IR files remain valid.

## What it can do

- Expose **HTTP** `/v1` and/or **MCP** (stdio or Streamable HTTP `/mcp` plus SSE `/sse`).
- Validate config (`qllm validate`) with no I/O to the sources.
- Run one IR (`qllm query`) or one SQL file (`qllm sql`) against the preset's sources.
- Generate a catalog **draft**: `introspect` (postgres/mysql) and `from-openapi` (REST). You must review relations and aliases before serving.
- Isolate apps with `qllm.access.yaml` (a Bearer key per app plus an entity allowlist).
- Fail fast: a typical sync budget of **~15 s** (`limits.maxSyncMs`), a per-source timeout, then `TIMEOUT` and cancellation.

## What it cannot do (and this is not a hidden backlog)

| Out of scope | Why |
|--------------|-----|
| GraphQL | Never a qLLM API (D17). |
| Raw agent SQL against `public.table` or the physical schema | Tables are catalog **entity names**. |
| An MCP tool per table, or `execute_query` on MCP | Only `how_to_use_me`, `describe_catalog`, `execute_sql`. IR stays on HTTP and the CLI. |
| Writes (`INSERT`/`UPDATE`/…), DDL, `PRAGMA`, `read_csv`, multi-statement | Read-only plus a denylist. |
| Warehouse / Spark / Databricks (`PIVOT`, Unity, `ai_*`, Delta history) | An inventory of names, not a clone. |
| Dynamo scan, Cassandra `ALLOW FILTERING`, ksql `EMIT CHANGES` | Without an equality on the `accessPath`, the result is `UNSUPPORTED`. |
| Waiting minutes for a slow source | Fail-fast; async HTTP is not a long-running job. |
| Python/Node SDK in the MVP | HTTP and MCP are the API (phase 2). |
| Mongo introspection or experimental sources | `introspect` supports postgres and mysql only. |
| Reloading YAML without a restart | `serve` loads at startup. |
| CORS on the HTTP `/v1` listener | CORS lives on **MCP HTTP**. REST `/v1` sends no CORS headers. |
| Wildcard CORS `*` | Rejected. |
| A Bearer token in YAML | Env only (`authTokenEnv`, `key: ${VAR}`, `qllm.env.yaml`). |

## Agent surface

1. `how_to_use_me` / `GET /v1/howtouseme`: the guide plus what never to invent.
2. `describe_catalog` / `GET /v1/catalog`: entities (filtered when an ACL exists).
3. Query with **`execute_sql` / `POST /v1/sql`** (MCP and HTTP), **or** Query IR on **`POST /v1/queries`** / `qllm query` (not an MCP tool).

## Two data worlds

| World | Where | Typical entities |
|-------|-------|------------------|
| Demo / goldens | `nerdctl compose` plus `deploy/image/config` | `customers`, `invoices`, … |
| fleet-ops simulation | `scripts/prd-tst-up` (`deploy/prd-tst`) | `vehicles`, `depots`, `gps_samples`, … |

Do not run both at the same time. The process **only sees** the `--config-dir` (or CWD / `--project`). Compose does not "inject" a catalog into the binary.
