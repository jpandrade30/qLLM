# 05 — Dev Harness (Rancher Desktop)

## Premissas

- Rancher Desktop com **containerd** + **Kubernetes** habilitados
- Imagens oficiais dos bancos; **API de teste em Python** (FastAPI)
- Não precisa ser Go nas fixtures

## Objetivos do harness

1. Subir Postgres, MySQL, MongoDB, API REST com dados seed correlacionados (`customer_id` cruzável)
2. Aplicar preset + catalog de exemplo via `--config-dir fixtures/presets` (mesmo mecanismo de produção; ver `03` § 0)
3. Rodar queries IR de integração contra o binário `qllm`
4. Validar timeout fail-fast (endpoint/API lenta opcional)

## Layout alvo

```text
deploy/dev/
  namespace.yaml          # qllm-dev
  postgres.yaml
  mysql.yaml
  mongodb.yaml
  test-api.yaml           # FastAPI Deployment+Service
  kustomization.yaml      # opcional
fixtures/
  seed/
    postgres.sql
    mysql.sql
    mongo.js              # ou JSON import
  test-api/               # código Python da API
  presets/
    demo.preset.yaml      # ou qllm.preset.yaml
    demo.catalog.yaml
    # preferir nomes qllm.preset.yaml / qllm.catalog.yaml para --config-dir
    qllm.project.yaml     # opcional
  queries/                # IRs de golden test
```

Carregar:

```bash
qllm serve --http --config-dir fixtures/presets
# com port-forwards + env QLLM_* exportados
```

## Serviços (nomes estáveis)

| Service DNS (in-cluster) | Porta | Uso |
|--------------------------|-------|-----|
| `postgres.qllm-dev.svc` | 5432 | CRM tables |
| `mysql.qllm-dev.svc` | 3306 | billing |
| `mongodb.qllm-dev.svc` | 27017 | events |
| `test-api.qllm-dev.svc` | 8080 | REST legacy_users |

Credenciais de dev **fixas e documentadas** só no harness (nunca produção).

## Dados seed (mínimo correlacionado)

- `customers` (pg): id, email, created_at
- `invoices` (mysql): id, customer_id, total_cents, status
- `app_events` (mongo): _id, customerId, type, ts
- `GET /users` (api): id, email (subset alinhado a customers)

## Como o runtime acessa do host

Opções (escolher na implementação; documentar uma default):

1. `kubectl port-forward` + preset com `localhost`
2. Ingress local Rancher
3. Rodar `qllm` **dentro** do cluster (Job/Pod de integração)

Default recomendado para DX: scripts que sobem port-forwards + exportam env `QLLM_*`.

## Comandos alvo (a implementar)

```bash
# aplicar manifests
kubectl apply -k deploy/dev

# seeds
./scripts/dev-seed.sh

# testar
./scripts/dev-query.sh fixtures/queries/invoices_paid.json
```

## Critérios de aceite do harness

- [ ] Um comando sobe todos os backends healthy
- [ ] Seed idempotente
- [ ] Pelo menos 3 IRs golden: single pg, join pg+mysql via DuckDB, REST list+filter
- [ ] Um teste de `TIMEOUT` forçado (API sleep > budget)
