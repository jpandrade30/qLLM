# Protocol JSON Schemas

Machine-readable contracts for protocol **0.1.0**.

| File | Validates |
|------|-----------|
| [project.schema.json](project.schema.json) | Optional `qllm.project.yaml` pointer |
| [preset.schema.json](preset.schema.json) | Project preset |
| [catalog.schema.json](catalog.schema.json) | Logical catalog (incl. entity `aliases`) |
| [query-ir.schema.json](query-ir.schema.json) | Query IR request (incl. query `as`) |
| [query-response.schema.json](query-response.schema.json) | Query API responses / errors |
| [runtime-config.schema.json](runtime-config.schema.json) | Optional `qllm.config.yaml` serve/runtime settings (not Query IR) |
| [sql-request.schema.json](sql-request.schema.json) | `POST /v1/sql` / MCP `execute_sql` body |
| [access.schema.json](access.schema.json) | Optional `qllm.access.yaml` apps/keys/tables |
| [env-file.schema.json](env-file.schema.json) | Optional `qllm.env.yaml` — seed `os` env (process env wins) |

Human-readable rules and examples: [../03-protocol-schemas.md](../03-protocol-schemas.md) (§ 0 layout, § 0.1 naming). SQL dialect inventory: [../07-sql-dialect.md](../07-sql-dialect.md).

**Rule:** change the `.md` narrative and these schemas in the same PR/change set. Code generators and Go structs must follow these files.
