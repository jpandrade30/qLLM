# 05 — Dev harness (Rancher Desktop + nerdctl compose)

## Premissas

- Rancher Desktop with **containerd** (nerdctl). Kubernetes **not** required for the default path.
- Official DB images; **test API in Python** (FastAPI)
- Fixtures need not be Go

## Objetivos

1. Subir Postgres, MySQL, MongoDB, API REST e qLLM com `nerdctl compose`. Tipos 0.2.0 experimentais (`mssql`, `sqlite`, `clickhouse`, `dynamodb`, `cassandra`, `ksql`) **não** entram no compose.
2. Seed correlacionado (`customer_id` cruzável) a partir de `fixtures/datasets/v1` via `load_dataset.py` nas portas publicadas (Faker só em `generate_dataset.py --regenerate`)
3. Queries: container (MCP/HTTP) **ou** host `qllm --config-dir deploy/image/config` com `QLLM_*` em `127.0.0.1` (ver `qllm.env.host.yaml`)
4. Fail-fast timeout (budget do preset)

## Layout

```text
docker-compose.yml          # postgres, mysql, mongodb, test-api, qllm
Dockerfile.dev              # compose: COPY deploy/image/config → /config
Dockerfile                  # product: COPY deploy/prd → /config
deploy/image/config/
  qllm.preset.yaml          # sources / *Env
  qllm.catalog.yaml         # entidades / fields / bindings
  qllm.config.yaml          # bind 0.0.0.0
  qllm.env.yaml             # compose DNS; secrets as ${VAR} (container)
  qllm.env.host.yaml        # documentação: localhost no host (não é qllm.env.yaml)
fixtures/
  datasets/v1/              # JSON lógico congelado + manifest hashes
  goldens/sql-v1/           # cases.yaml + expected/
  sqlcheck/                 # oracle DuckDB-python + MCP compare
  seed/generate_dataset.py  # Faker → datasets/v1
  seed/load_dataset.py      # datasets/v1 → DBs + test-api (sem Faker)
  test-api/                 # compose build context
  queries/                  # Query IR golden (testes, não schema)
  sql/manual-examples.md
  openapi/                  # spec mínima para CLI from-openapi
scripts/dev-seed-fake.ps1 / .sh
```

`fixtures/` = satélites de teste (processos + dados fake + queries de regressão). **Não** descreve formato das APIs/DBs. Isso é só `deploy/image/config`. Sem `deploy/dev` K8s no path default.

Exemplo só YAML: [`deploy/prd/`](../deploy/prd/). Simulação Kubernetes (namespace `qllm-prd`, projeto `fleet-ops`): [`deploy/prd-tst/`](../deploy/prd-tst/). Não altera goldens nem o compose. Não rode junto com `nerdctl compose`. Argo: `deploy/prd-tst/argocd/application.yaml`.

## Comandos

```bash
nerdctl compose up --build
# DBs: localhost 5432/3306/27017; test-api 18080; qllm 8088/8089

# seed from frozen fixtures/datasets/v1 (host → published ports)
.\scripts\dev-seed-fake.ps1
# only when the generator changed:
.\scripts\dev-seed-fake.ps1 --regenerate
# se data.json mudou: nerdctl compose up --build -d test-api

# SQL goldens (oracle, no compose):
python fixtures/sqlcheck/__main__.py oracle          # regenerate expected/ (do not run in CI)
python fixtures/sqlcheck/__main__.py coverage
python fixtures/sqlcheck/__main__.py check-oracle
# after compose + seed — log per query (SQL, columns, rows, OK/FAIL):
$env:QLLM_SQLCHECK_REQUIRE_MCP = "1"
python fixtures/sqlcheck/__main__.py mcp
# or pytest -v -s (each case is its own test node)
pytest fixtures/sqlcheck
```

MCP gate: Streamable HTTP `http://127.0.0.1:8089/mcp` Bearer `change-me`. Compare `execute_sql` rows to `fixtures/goldens/sql-v1/expected`. Matrix tags in `cases.yaml` must cover every connector and every dialect-07 family (including composed statements); CI fails if a required tag is missing. CI must not regenerate goldens.


HTTP: `Authorization: Bearer change-me` (compose injeta `QLLM_AUTH_TOKEN`; `qllm.env.yaml` só tem `${QLLM_AUTH_TOKEN}`).

Host binário (sem container qllm): `--config-dir deploy/image/config` e env de [`qllm.env.host.yaml`](../deploy/image/config/qllm.env.host.yaml) (`127.0.0.1`). Process env ganha de `qllm.env.yaml` (DNS compose).

## Serviços (compose DNS)

| Hostname | Porta no container | Host |
|----------|--------------------|------|
| `postgres` | 5432 | 5432 |
| `mysql` | 3306 | 3306 |
| `mongodb` | 27017 | 27017 |
| `test-api` | 8080 | 18080 |
| `qllm` | 8088 / 8089 | 8088 / 8089 |

Credenciais de dev **fixas só no harness**.

## Critérios de aceite

- [x] Um comando sobe backends + qllm (`compose up --build`)
- [x] Seed via frozen `fixtures/datasets/v1` (`load_dataset.py`; Faker only on regenerate)
- [x] IRs em `fixtures/queries/` + SQL em `fixtures/sql/manual-examples.md`
- [x] MCP `execute_sql` vs DuckDB-python goldens (`fixtures/sqlcheck`)
- [ ] TIMEOUT forçado (API sleep > budget) — ainda desejável
