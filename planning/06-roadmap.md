# 06 — Roadmap

## Fase 0 — Spec lock

- [x] Overview, decisions, architecture
- [x] Protocol schemas documentados
- [x] JSON Schemas em `planning/schemas/`
- [x] Rules + skills Cursor
- [x] Review humano dos contratos (preset/catalog/IR/API) — baseline aceito para implementação

**Exit:** schemas revisados; implementação segue contratos 0.1.0.

## Fase 1 — Skeleton Go + validação

- [x] CLI `qllm` com `validate` / `query` / `serve`
- [x] Load YAML/JSON → structs → validação JSON Schema
- [x] `GET /health`, `GET /catalog`
- [x] Validação semântica (aliases, AMBIGUOUS_*)

**Exit:** IR inválido falha com `INVALID_IR` / `UNKNOWN_*`.

## Fase 2 — Connector Postgres + MySQL

- [x] Pushdown filter/project/agg/group/limit
- [x] Resultado tabular
- [x] Timeouts + cancel
- [x] Harness K8s manifests + seeds (aplicar com Rancher ligado)

**Exit:** golden queries SQL prontas em `fixtures/queries/`.

## Fase 3 — Mongo + REST + local join

- [x] Mongo aggregation básica
- [x] REST list/filter
- [x] Cross-source join via motor local (`internal/duckdblocal`, pure Go / sem CGO)
- [x] `meta.plan.usedDuckDB` (flag de compute local)

**Exit:** IRs de join e REST em fixtures.

## Fase 4 — Serve HTTP async shape + MCP

- [x] `POST /queries` sync default; `mode: async` → 202
- [x] `GET /queries/{id}` + result
- [x] MCP tools espelhando HTTP
- [x] Erros tipados

**Exit:** `qllm serve --http` / `--mcp`.

## Fase 5 — Hardening

- [x] Read-only default no preset; allowlist = catalog
- [x] Logs estruturados HTTP (`queryId`, `elapsedMs`, `error.code`)
- [x] README quickstart

## Fase 5b — SQL dialeto + ACL por app

- [x] `qllm.access.yaml` (`${ENV}` só em `key`)
- [x] Parser `SELECT` + `POST /v1/sql` + MCP `execute_sql`
- [x] Fetch colunas citadas → DuckDB `ExecSQL` (`enable_external_access=false`)
- [x] Dockerfile multi-stage (`-tags duckdb`), sem Compose
- [x] Dialeto `"2"` (default): set ops, `QUALIFY`, windows; `"1"` congelado
- [x] Inventário Databricks-like + testes parse/DuckDB/reject — [`07-sql-dialect.md`](07-sql-dialect.md)

**Exit:** SQL e IR respeitam a mesma allowlist; expressões ricas só no SQL (IR 0.1.0 unchanged); testes = contrato do dialeto.

## Fase 5c — SQL expressiveness (done)

- [x] Query IR **não** ganha HAVING/UNION/CASE/LIKE — agents usam `execute_sql`
- [x] Funções extras via DuckDB + denylist (não lista allow de centenas de nomes no schema IR)
- [x] Golden + table-driven tests em `internal/sqlparse`, `internal/duckdblocal` (`-tags duckdb`), `internal/validate` (llmlint)

## Fase 6 — Clients (depois)

- [ ] Python thin client
- [ ] Node thin client
- [ ] OpenAPI gerada a partir dos schemas

---

## Nota de implementação

Motor local de join: **default** pure Go em `internal/duckdblocal` (sem CGO). Interface `Engine` estável; build `go build -tags duckdb` (CGO) troca por DuckDB embutido (`engine_duckdb.go`). SQL gerado via `BuildDuckSQL` (parameterized). Caminho `execute_sql` exige DuckDB (`ExecSQL`). Ver README § Local join engine.
