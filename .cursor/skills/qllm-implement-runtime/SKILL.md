---
name: qllm-implement-runtime
description: >-
  Implement qLLM Go runtime pieces (CLI, validation, planner, connectors,
  DuckDB local join, HTTP/MCP serve) while keeping protocol contracts frozen
  unless explicitly changing specs. Use when writing Go code for qLLM execution
  paths or wiring serve modes.
---

# qLLM Implement Runtime

## When to use

- Building `cmd/qllm` or `internal/*`
- Adding a connector or planner rule
- Exposing HTTP `/v1` or MCP tools

## Order of implementation (respect roadmap)

Follow [planning/06-roadmap.md](../../../planning/06-roadmap.md):

1. Validate preset/catalog/IR (no I/O)
2. Postgres/MySQL pushdown + tabular result
3. Mongo + REST + DuckDB cross-join
4. HTTP async shape + MCP
5. Hardening

## Hard rules

- Context + timeouts on every source call
- Map errors to protocol codes (`TIMEOUT`, `INVALID_IR`, …)
- Never expose raw SQL as the default agent interface
- Secrets via preset `*Env` only
- If behavior needs a new IR field → stop and use skill `qllm-protocol` first

## Connector pattern

```text
Validate IR → Plan (pushdown|local) → Execute with cancel → Normalize rows → Meta.plan
```

Capability matrix: [planning/04-connectors.md](../../../planning/04-connectors.md)

## Done when

- Unit tests for validation
- Integration path documented against harness (when present)
- Response matches `query-response.schema.json`
