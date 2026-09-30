# Ambientes neste repositório

## Demo compose (goldens)

Rancher Desktop + **nerdctl compose**. Spec: [`planning/05-dev-harness.md`](../planning/05-dev-harness.md).

```bash
nerdctl compose up --build
```

Config bakeada na imagem: [`deploy/image/config`](../deploy/image/config). HTTP Bearer demo: `change-me`. Portas no host: HTTP 8088, MCP 8089 (ver compose).

Seed da fake API:

```powershell
.\scripts\dev-seed-fake.ps1
# --regenerate só reescreve JSON congelado
```

SQL goldens MCP: `pytest fixtures/sqlcheck`.

Senhas fracas e Mongo aberto: **só localhost**.

**Não** uses este compose como “o teu produto”. Copia o *formato* dos YAML para o teu `--config-dir`.

A tua pasta em vez do demo: [point-your-folder.md](point-your-folder.md).

## Imagem standalone

```bash
nerdctl build -t qllm .
nerdctl run --rm -p 8088:8088 -p 8089:8089 qllm
```

`/config` na imagem = só `deploy/image/config`. Em K8s PRD o Deployment **monta** um ConfigMap em `/config` e ignora o catalog bakeado.

## Sim Kubernetes fleet-ops (`deploy/prd`)

Mundo **separado** (D18). Namespace `qllm-prd`. **Não** com compose ao mesmo tempo.

Guia passo a passo: [`deploy/prd/README.md`](../deploy/prd/README.md).

```bash
nerdctl compose down -v
nerdctl --namespace k8s.io build -t qllm:local .
kubectl apply -k deploy/prd
```

Port-forward (um processo, Ctrl+C mata todos):

```powershell
.\scripts\prd-port-forward.ps1
```

| Host | Serviço |
|------|---------|
| 18088 | qLLM HTTP |
| 18089 | qLLM MCP |
| 15432 | Postgres |
| 19000 / 18123 | ClickHouse |
| 18000 | DynamoDB Local |
| 18080 | fake crew API |
| 18081 | Argo CD (depois de instalar) |

Bearer do sim: `fleet-prd-token` (Secret, não produção).

Argo é **opcional**. Scripts:

| Script | Função |
|--------|--------|
| `prd-argocd-up.ps1` | Instala Argo `--insecure` (HTTP real no :80) |
| `prd-argocd-password.ps1` | Password `admin` |
| `prd-argocd-register-app.ps1` | Application CR |
| `prd-argocd-add-ssh-repo.ps1` | Chave SSH **dentro** do cluster (git-gui local não conta) |

`imagePullPolicy: Never` + `qllm:local` no namespace `k8s.io`. `ErrImagePull` = a imagem não está no containerd do Kubernetes.

## Scripts de dev

| Script | Função |
|--------|--------|
| `dev-shell.ps1` / `dev-shell.cmd` | gcc + duckdblib no Windows para `-tags duckdb` |
| `dev-seed-fake.ps1` | Seed `fixtures/datasets/v1` |
| `duckdb_smoke.go` | Smoke CGO DuckDB |

## Logs de queries

```powershell
kubectl logs -n qllm-prd deploy/qllm -f
```

Blocos `---- execute_sql ----` (SQL multilinha) e `---- mcp_tool ----`. Não filtres só a primeira linha se quiseres o SQL.
