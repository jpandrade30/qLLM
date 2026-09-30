# Manual de implementação (qLLM)

**Não sabes nada e vais escrever todos os `qllm.*` à mão:** [from-scratch.md](from-scratch.md), [field-reference.md](field-reference.md), pasta de exemplo [`deploy/prd/`](../deploy/prd), [point-your-folder.md](point-your-folder.md). Sim K8s: `deploy/prd-tst`.

O contrato normativo (JSON Schema, Query IR, erros) continua em [`planning/`](../planning/) e [`planning/schemas/`](../planning/schemas/). Este `docs/` é o guia operacional. Se os dois divergirem, **vence o `planning/`**.

| Doc | Conteúdo |
|-----|----------|
| [from-scratch.md](from-scratch.md) | Pasta tua, exemplo Postgres, validate, como **provar** que subiu o YAML certo |
| [field-reference.md](field-reference.md) | Todos os campos, enums, `*Env`, options REST |
| [point-your-folder.md](point-your-folder.md) | CLI, `Dockerfile` vs `.dev`, `deploy/prd` vs `prd-tst` |
| [scope.md](scope.md) | O que o produto é / não é |
| [project-files.md](project-files.md) | Discovery e precedência |
| [cli.md](cli.md) | Todos os comandos e flags |
| [http-mcp.md](http-mcp.md) | HTTP `/v1`, MCP, auth, bind, CORS |
| [queries.md](queries.md) | Catalog SQL vs Query IR: aceite / recusado |
| [connectors.md](connectors.md) | Tipos de fonte, bindings, pushdown |
| [errors.md](errors.md) | Códigos tipados |
| [environments.md](environments.md) | Compose, imagem, sim PRD, scripts |
| [build.md](build.md) | Go, `-tags duckdb`, Docker |

Começa por [scope.md](scope.md) e [project-files.md](project-files.md). Sem preset+catalog válidos o binário falha com `CONFIG_ERROR` — não há fallback para `fixtures/`.
