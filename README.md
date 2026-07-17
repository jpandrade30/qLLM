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
```

Set env vars referenced by the preset (`QLLM_*`), then:

```bash
./qllm validate --config-dir ./my-project
./qllm serve --http --addr :8088 --config-dir ./my-project
```

### 3. Query

```bash
curl -s localhost:8088/v1/howtouseme | jq .
curl -s localhost:8088/v1/catalog | jq .
curl -s -X POST localhost:8088/v1/queries -d @fixtures/queries/customers_list.json
```

`GET /v1/howtouseme` is the closed Query IR contract for LLMs (`never`, where shapes, invalidExamples). Call it before inventing queries.

Or CLI:

```bash
./qllm query --config-dir ./fixtures/presets -f ./fixtures/queries/customers_list.json
```

### MCP

```bash
./qllm serve --mcp --config-dir ./fixtures/presets
```

Tools: `how_to_use_me`, `describe_catalog`, `execute_query`, `get_query`.

## Dev shell (gcc + CGO + duckdblib)

Se você usa MSYS2 do GHCup + libs em `duckdblib/`:

```powershell
.\scripts\dev-shell.ps1
```

Ou dê duplo clique em `scripts\dev-shell.cmd`. O prompt vira `qLLM-dev ...>` com `PATH`/`CGO_*` já configurados.

## Fake data (Faker)

With port-forwards up:

```powershell
.\scripts\dev-seed-fake.ps1 200
```

Uses [Faker](https://faker.readthedocs.io/) via `fixtures/seed/generate_and_load.py` — expands Postgres/MySQL/Mongo schemas and regenerates `test-api/data.json`.

## Dev harness (Rancher Desktop)

Requires Kubernetes enabled in Rancher Desktop and `kubectl` context set.

```powershell
.\scripts\dev-up.ps1
.\scripts\dev-seed.ps1
.\scripts\dev-port-forward.ps1
# then set the env vars printed, and:
.\qllm.exe query --config-dir .\fixtures\presets -f .\fixtures\queries\invoices_paid_agg.json
```

Bash equivalents: `scripts/dev-up.sh`, `dev-seed.sh`, `dev-port-forward.sh`.

## Fail-fast

Default budget ~15s (`limits.maxSyncMs`). Slow sources return typed `TIMEOUT` — not something qLLM tries to “fix” for you.

## Local join engine

Cross-source joins run in-process (pure Go local engine). Package path `internal/duckdblocal` is the extension point for embedded DuckDB when CGO is available.
