# qLLM Planning

Índice do planejamento. **Contratos (schemas) são a fonte da verdade** — código segue estes docs, não o contrário.

| Doc | Conteúdo |
|-----|----------|
| [00-overview.md](00-overview.md) | Visão, problema, não-objetivos |
| [01-decisions.md](01-decisions.md) | Decisões fechadas (ADR curto) |
| [02-architecture.md](02-architecture.md) | Componentes, fluxo, limites |
| [03-protocol-schemas.md](03-protocol-schemas.md) | **Crítico:** layout de config, aliases, preset, catalog, IR, API I/O, erros |
| [04-connectors.md](04-connectors.md) | Capacidades por fonte (Postgres, MySQL, Mongo, REST) |
| [05-dev-harness.md](05-dev-harness.md) | Rancher Desktop / containerd / K8s para testes |
| [06-roadmap.md](06-roadmap.md) | Fases e critérios de pronto |

## Regra de ouro

Qualquer mudança em formato de conexão, schema de entidade, Query IR ou resposta HTTP/MCP **atualiza primeiro** `03-protocol-schemas.md` (e JSON Schema em `planning/schemas/` quando existir), depois o código.

## Cursor

- Rules: `.cursor/rules/` (`project-core`, `protocol-schemas`, `go-runtime`, `dev-harness`)
- Skills: `.cursor/skills/qllm-protocol`, `qllm-dev-harness`, `qllm-implement-runtime`, `qllm-spec-change`

## Status

- Fase 0 (specs/schemas/rules): **completa**.
- Fases 1–5 (runtime MVP): **implementadas** — CLI, connectors, harness, HTTP/MCP.
- Protocolo: **0.1.0** — ver `03-protocol-schemas.md` + `schemas/`.
- Cluster Rancher precisa estar ligado para exercitar `scripts/dev-up.*`.

