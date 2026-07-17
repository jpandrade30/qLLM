# 06 — Roadmap

## Fase 0 — Spec lock

- [x] Overview, decisions, architecture
- [x] Protocol schemas documentados
- [x] JSON Schemas em `planning/schemas/`
- [x] Rules + skills Cursor
- [x] Review humano dos contratos (preset/catalog/IR/API) — baseline aceito para implementação

**Exit:** schemas revisados; implementação segue contratos 0.1.0.

## Fase 1 — Skeleton Go + validação

- [ ] CLI `qllm` com `validate preset|catalog|ir`
- [ ] Load YAML/JSON → structs → validação JSON Schema
- [ ] `GET /health`, `GET /catalog` (static files)
- [ ] Sem connectors reais

**Exit:** IR inválido falha com `INVALID_IR` / `UNKNOWN_*`.

## Fase 2 — Connector Postgres + MySQL

- [ ] Pushdown filter/project/agg/group/limit
- [ ] Resultado tabular
- [ ] Timeouts + cancel
- [ ] Harness K8s com pg+mysql + seeds

**Exit:** golden queries SQL passam em <15s.

## Fase 3 — Mongo + REST + DuckDB join

- [ ] Mongo aggregation básica
- [ ] REST list/getById
- [ ] Cross-source join via DuckDB
- [ ] `meta.plan.usedDuckDB`

**Exit:** IR join invoices×customers e events×customers.

## Fase 4 — Serve HTTP async shape + MCP

- [ ] `POST /queries` sync default; 202 path disponível
- [ ] `GET /queries/{id}` + result
- [ ] MCP tools espelhando HTTP
- [ ] Erros tipados completos

**Exit:** agente local consegue `describe_catalog` + `execute_query`.

## Fase 5 — Hardening

- [ ] Allowlist, read-only enforcement tests
- [ ] Observabilidade mínima (log struct: queryId, elapsed, source)
- [ ] Docs de preset para “novo projeto em 5 minutos”

## Fase 6 — Clients (depois)

- [ ] Python thin client
- [ ] Node thin client
- [ ] Gerados ou manuais a partir OpenAPI

---

## Prioridade se o tempo apertar

1. Contratos estáveis  
2. Um SQL connector + catalog + execute sync  
3. Harness  
4. DuckDB cross-join  
5. MCP  
6. SDKs  
