---
name: qllm-protocol
description: >-
  Author and review qLLM protocol contracts (preset, catalog, Query IR, HTTP/MCP
  responses, typed errors). Use when changing planning schemas, designing
  queries, validating IR examples, or aligning API shapes to protocol 0.1.0.
---

# qLLM Protocol

## When to use

- Editing `planning/03-protocol-schemas.md` or `planning/schemas/*`
- Designing a new entity, source connection, or query example
- Implementing validators or HTTP handlers that must match the wire format

## Mandatory reading order

1. `planning/01-decisions.md` (constraints)
2. `planning/03-protocol-schemas.md` (normative prose)
3. Matching file in `planning/schemas/`

## Workflow for contract changes

1. State whether change is **additive** or **breaking**
2. Update markdown examples
3. Update JSON Schema
4. Bump `protocolVersion` if needed (D11)
5. Note impact on connectors (`planning/04-connectors.md`)
6. Do not implement Go until schemas and md agree

## IR checklist

- Entities/fields exist in catalog (logical names or catalog aliases)
- Query `as` bindings unique; qualify fields when >1 entity (`AMBIGUOUS_FIELD` otherwise)
- Same physical name on 10 REST APIs ⇒ 10 entity names (e.g. `crm_users` vs `erp_users`), not one `users`
- `limit` within preset `maxLimit`
- Agg + non-agg select ⇒ valid `groupBy`
- Cross-source join ⇒ expect DuckDB in plan
- No mutation ops in v0.1

## Config checklist

- Preset + catalog on disk; discovery via `--config-dir` / `--project` / flags (see `03` § 0)
- Secrets only via `*Env` in preset

## Response checklist

- Success: `columns`, `rows`, `rowCount`, `truncated`
- Failure: `error.code` from the enum only
- Timestamps in rows: RFC3339 strings

## References

- Full examples: [planning/03-protocol-schemas.md](../../../planning/03-protocol-schemas.md)
- Schemas: [planning/schemas/](../../../planning/schemas/)
