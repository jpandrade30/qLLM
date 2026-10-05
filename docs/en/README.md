# qLLM implementer manual

**Writing every `qllm.*` file by hand from zero?** Start with [from-scratch.md](from-scratch.md) and [field-reference.md](field-reference.md). A sample folder lives in [`deploy/prd/`](../../deploy/prd); see also [point-your-folder.md](point-your-folder.md). The Kubernetes simulation is in `deploy/prd-tst`.

The normative contract (JSON Schema, Query IR, errors) stays in [`planning/`](../../planning/) and [`planning/schemas/`](../../planning/schemas/). This manual is the operational guide. If the two disagree, **`planning/` wins**. Locked product rules labeled `D17`–`D22` are defined in [`planning/01-decisions.md`](../../planning/01-decisions.md) (plain-language index on the product site: [Decisions](https://jpandrade30.github.io/qLLM/decisions.html)) — a bare `D##` tag is not enough context by itself.

| Document | Contents |
|----------|----------|
| [from-scratch.md](from-scratch.md) | Your own folder, a Postgres example, `validate`, and how to **prove** the right YAML is loaded |
| [field-reference.md](field-reference.md) | Every field, enum, `*Env` key, and REST option |
| [point-your-folder.md](point-your-folder.md) | CLI, `Dockerfile` vs `.dev`, `deploy/prd` vs `prd-tst` |
| [scope.md](scope.md) | What the product is and is not |
| [project-files.md](project-files.md) | File discovery and precedence |
| [cli.md](cli.md) | Every command and flag |
| [http-mcp.md](http-mcp.md) | HTTP `/v1`, MCP, auth, bind, CORS |
| [queries.md](queries.md) | Catalog SQL vs Query IR: what is accepted and refused |
| [responses.md](responses.md) | Output envelope, column types, json cells, `shape` |
| [connectors.md](connectors.md) | Source types, bindings, pushdown |
| [errors.md](errors.md) | Typed error codes |
| [multi-user-safety.md](multi-user-safety.md) | Row scope on the credential (decision D21) |
| [environments.md](environments.md) | Compose, images, PRD simulation, scripts |
| [install.md](install.md) | Every install option: image, Go, DuckDB (Windows `duckdblib`), standalone |
| [build.md](build.md) | Go, `-tags duckdb`, Docker |

Read [scope.md](scope.md) and [project-files.md](project-files.md) first. Without a valid preset and catalog the binary fails with `CONFIG_ERROR`; there is no fallback to `fixtures/`.

Other languages: [Português](../pt/README.md) · [Español](../es/README.md) · [中文](../zh/README.md)
