---
name: qllm-dev-harness
description: >-
  Plan and operate the qLLM local test harness on Rancher Desktop (containerd):
  nerdctl compose (Postgres, MySQL, MongoDB, Python test API, qllm image),
  seeds, and golden Query IR. Use when creating compose/Dockerfile, fixtures,
  seed data, or integration test setup.
---

# qLLM Dev Harness

## When to use

- Adding/changing `docker-compose.yml`, `Dockerfile`, `fixtures/` (satellites), or `deploy/image/config`
- Wiring preset env vars to compose service DNS vs localhost
- Writing golden IR tests against live backends

## Read first

- [planning/05-dev-harness.md](../../../planning/05-dev-harness.md)
- Demo shapes in [planning/03-protocol-schemas.md](../../../planning/03-protocol-schemas.md)

## Target topology (compose)

| Service | Role |
|---------|------|
| `postgres` | CRM `customers` |
| `mysql` | billing `invoices` |
| `mongodb` | `app_events` |
| `test-api` | REST users (Python) |
| `qllm` | runtime (`/config` baked) |

No Kubernetes namespace required.

## Workflow

1. `nerdctl compose up --build`
2. Seed: `scripts/dev-seed-fake.ps1` (writes `fixtures/test-api/data.json`; rebuild test-api if JSON changed)
3. Preset/catalog/access/env: **only** `deploy/image/config/` (Dockerfile COPY). `fixtures/` = seed, test-api, golden IR — not schema.
4. Golden IRs under `fixtures/queries/`
5. Host reaches DBs via published ports; container qllm uses compose DNS from `qllm.env.yaml`

## Do not

- Put preset/catalog under `fixtures/` (that tree is not the data contract)
- Commit production secrets
- Use unbounded seed tables

## Acceptance

- Backends + qllm healthy with compose
- ≥3 golden queries (single-source, cross-source join, REST)
- Fail-fast timeout test proves typed `TIMEOUT` when added
