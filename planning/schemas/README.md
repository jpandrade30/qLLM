# Protocol JSON Schemas

Machine-readable contracts for protocol **0.1.0**.

| File | Validates |
|------|-----------|
| [project.schema.json](project.schema.json) | Optional `qllm.project.yaml` pointer |
| [preset.schema.json](preset.schema.json) | Project preset |
| [catalog.schema.json](catalog.schema.json) | Logical catalog (incl. entity `aliases`) |
| [query-ir.schema.json](query-ir.schema.json) | Query IR request (incl. query `as`) |
| [query-response.schema.json](query-response.schema.json) | Query API responses / errors |

Human-readable rules and examples: [../03-protocol-schemas.md](../03-protocol-schemas.md) (§ 0 layout, § 0.1 naming).

**Rule:** change the `.md` narrative and these schemas in the same PR/change set. Code generators and Go structs must follow these files.
