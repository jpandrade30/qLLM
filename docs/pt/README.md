# Manual de implementação do qLLM

**Vai escrever todos os arquivos `qllm.*` à mão, do zero?** Comece por [from-scratch.md](from-scratch.md) e [field-reference.md](field-reference.md). Há uma pasta de exemplo em [`deploy/prd/`](../../deploy/prd); veja também [point-your-folder.md](point-your-folder.md). A simulação em Kubernetes está em `deploy/prd-tst`.

O contrato normativo (JSON Schema, Query IR, erros) continua em [`planning/`](../../planning/) e [`planning/schemas/`](../../planning/schemas/). Este manual é o guia operacional. Se os dois divergirem, **vale o `planning/`**.

| Documento | Conteúdo |
|-----------|----------|
| [from-scratch.md](from-scratch.md) | Sua própria pasta, exemplo com Postgres, `validate` e como **provar** que o YAML certo foi carregado |
| [field-reference.md](field-reference.md) | Todos os campos, enums, chaves `*Env` e opções REST |
| [point-your-folder.md](point-your-folder.md) | CLI, `Dockerfile` vs `.dev`, `deploy/prd` vs `prd-tst` |
| [scope.md](scope.md) | O que o produto é e o que não é |
| [project-files.md](project-files.md) | Descoberta de arquivos e precedência |
| [cli.md](cli.md) | Todos os comandos e flags |
| [http-mcp.md](http-mcp.md) | HTTP `/v1`, MCP, autenticação, bind e CORS |
| [queries.md](queries.md) | SQL de catálogo vs Query IR: o que é aceito e o que é recusado |
| [connectors.md](connectors.md) | Tipos de fonte, bindings e pushdown |
| [errors.md](errors.md) | Códigos de erro tipados |
| [environments.md](environments.md) | Compose, imagens, simulação PRD e scripts |
| [build.md](build.md) | Go, `-tags duckdb` e Docker |

Leia primeiro [scope.md](scope.md) e [project-files.md](project-files.md). Sem preset e catálogo válidos, o binário falha com `CONFIG_ERROR`; não há fallback para `fixtures/`.

Outros idiomas: [English](../en/README.md) · [Español](../es/README.md) · [中文](../zh/README.md)
