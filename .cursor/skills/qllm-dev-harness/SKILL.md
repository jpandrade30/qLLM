---
name: qllm-dev-harness
description: >-
  Plan and operate the qLLM local Kubernetes test harness on Rancher Desktop
  (containerd): Postgres, MySQL, MongoDB, Python test API, seeds, and golden
  Query IR. Use when creating deploy/dev manifests, fixtures, seed data, or
  integration test setup.
---

# qLLM Dev Harness

## When to use

- Adding/changing `deploy/dev`, `fixtures/`, or seed scripts
- Wiring preset env vars to local Services / port-forwards
- Writing golden IR tests against live backends

## Read first

- [planning/05-dev-harness.md](../../../planning/05-dev-harness.md)
- Demo shapes in [planning/03-protocol-schemas.md](../../../planning/03-protocol-schemas.md)

## Target topology

| Service | Role |
|---------|------|
| `postgres` | CRM `customers` |
| `mysql` | billing `invoices` |
| `mongodb` | `app_events` |
| `test-api` | REST users (Python OK) |

Namespace: `qllm-dev`.

## Workflow

1. Manifests: Deployments + Services + Secrets/ConfigMaps for **dev-only** creds
2. Seeds: correlated `customer_id` across all stores
3. Preset/catalog demo files under `fixtures/presets/` (loaded with `qllm --config-dir fixtures/presets`)
4. Golden IRs under `fixtures/queries/` validated with `query-ir.schema.json` (use distinct entity names per REST source; qualify with `as` in joins)
5. Document how host reaches cluster (port-forward default)
6. Add one timeout fixture (API sleep > budget)

## Do not

- Commit production secrets
- Use unbounded seed tables
- Change stable DNS names without updating planning docs

## Acceptance

- Backends healthy with one apply path
- ≥3 golden queries (SQL single-source, cross-source join, REST)
- Fail-fast timeout test proves typed `TIMEOUT`
