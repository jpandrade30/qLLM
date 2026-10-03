# `deploy/prd` — exemplo de projecto (só YAML)

Pasta que o **`Dockerfile`** (não o `.dev`) copia para `/config`. É o caminho “gerar ficheiros de conexão e fazer build”. **Não** sobe Postgres/ClickHouse/Argo.

Cópia inicial de [`deploy/image/config`](../image/config) (projecto `qllm-demo`). Troca hosts/`*Env`/entidades pelos teus. Campos: [`docs/pt/field-reference.md`](../../docs/pt/field-reference.md) ([EN](../../docs/en/field-reference.md)). Do zero: [`docs/pt/from-scratch.md`](../../docs/pt/from-scratch.md) ([EN](../../docs/en/from-scratch.md)).

| Ficheiro | O que é |
|----------|---------|
| `qllm.preset.yaml` | Fontes (`type`, `connection.*Env`) e `limits` |
| `qllm.catalog.yaml` | Nomes lógicos que o agente pode `SELECT` / IR `from` |
| `qllm.config.yaml` | Bind HTTP/MCP, `authTokenEnv`, CORS, caps |
| `qllm.env.yaml` | Preenche env **vazias** (`${VAR}` ou literais). Processo ganha. |
| `qllm.access.yaml` | App `demo-agent`; `key` = `${QLLM_AUTH_TOKEN}`; `tables` = entidades deste catalog |

Harness compose: [`Dockerfile.dev`](../../Dockerfile.dev) + `deploy/image/config`. Sim K8s fleet-ops: [`deploy/prd-tst`](../prd-tst). Scoped-key + LangGraph demo: [`enforced/`](enforced/README.md) + [`Dockerfile.enforced`](../../Dockerfile.enforced).

## Build e run

```bash
docker build -t qllm .
# nerdctl build -t qllm .
docker run --rm -p 8088:8088 -p 8089:8089 \
  -e QLLM_AUTH_TOKEN=change-me \
  -e QLLM_CRM_PG_HOST=host.docker.internal \
  -e QLLM_CRM_PG_PASSWORD=qllm \
  -e QLLM_BILLING_MYSQL_PASSWORD=qllm \
  qllm
```

Os hosts em `qllm.env.yaml` (`postgres`, `mysql`, …) são DNS de **compose**. Fora da rede compose, exporta `QLLM_*_HOST` para o teu IP ou monta outro `qllm.env.yaml` em `/config`.

Prova: `GET /v1/catalog` com `Authorization: Bearer …` → `"project":"qllm-demo"` (até mudares o YAML). Depois rebuild.

Validar sem Docker: `./qllm validate --config-dir ./deploy/prd`
