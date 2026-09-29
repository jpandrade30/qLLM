# 05 — Dev harness (Rancher Desktop + nerdctl compose)

## Premissas

- Rancher Desktop with **containerd** (nerdctl). Kubernetes **not** required for the default path.
- Official DB images; **test API in Python** (FastAPI)
- Fixtures need not be Go

## Objetivos

1. Subir Postgres, MySQL, MongoDB, API REST e qLLM com `nerdctl compose`
2. Seed correlacionado (`customer_id` cruzável) via `generate_and_load.py` nas portas publicadas
3. Queries: container (MCP/HTTP) **ou** host `qllm --config-dir deploy/image/config` com `QLLM_*` em `127.0.0.1` (ver `qllm.env.host.yaml`)
4. Fail-fast timeout (budget do preset)

## Layout

```text
docker-compose.yml          # postgres, mysql, mongodb, test-api, qllm
Dockerfile                  # COPY só deploy/image/config → /config
deploy/image/config/
  qllm.preset.yaml          # sources / *Env
  qllm.catalog.yaml         # entidades / fields / bindings
  qllm.config.yaml          # bind 0.0.0.0
  qllm.env.yaml             # compose DNS + token (container)
  qllm.env.host.yaml        # documentação: localhost no host (não é qllm.env.yaml)
fixtures/
  seed/generate_and_load.py
  test-api/                 # compose build context
  queries/                  # Query IR golden (testes, não schema)
  sql/manual-examples.md
  openapi/                  # spec mínima para CLI from-openapi
scripts/dev-seed-fake.ps1
```

`fixtures/` = satélites de teste (processos + dados fake + queries de regressão). **Não** descreve formato das APIs/DBs. Isso é só `deploy/image/config`. Sem `deploy/dev` K8s no path default. Produção: substitui o conteúdo de config (ConfigMap/Secret), não leva seed/compose.

## Comandos

```bash
nerdctl compose up --build
# DBs: localhost 5432/3306/27017; test-api 18080; qllm 8088/8089

# seed (host → published ports)
.\scripts\dev-seed-fake.ps1
# se data.json mudou: nerdctl compose up --build -d test-api
```

HTTP: `Authorization: Bearer change-me` (valor em `deploy/image/config/qllm.env.yaml`).

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
- [x] Seed via Python fake (idempotente no gerador)
- [x] IRs em `fixtures/queries/` + SQL em `fixtures/sql/manual-examples.md`
- [ ] TIMEOUT forçado (API sleep > budget) — ainda desejável
