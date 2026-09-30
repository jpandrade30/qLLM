# Ambientes neste repositório

## Demo compose (goldens)

Rancher Desktop + **nerdctl compose**. Spec: [`planning/05-dev-harness.md`](../planning/05-dev-harness.md).

```bash
nerdctl compose up --build
```

Config bakeada no **compose**: [`Dockerfile.dev`](../Dockerfile.dev) + [`deploy/image/config`](../deploy/image/config). HTTP Bearer demo: `change-me`. Portas no host: HTTP 8088, MCP 8089 (ver compose). Seed: `.\scripts\dev-seed-fake.ps1` ou `./scripts/dev-seed-fake.sh`.

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
# bake: deploy/prd
nerdctl run --rm -p 8088:8088 -p 8089:8089 qllm
```

Compose: `Dockerfile.dev` + `deploy/image/config`. Sim K8s: ConfigMap `deploy/prd-tst/config` tapa o bake.

Exemplo **só YAML + `Dockerfile`**: [`deploy/prd/README.md`](../deploy/prd/README.md).

## Sim Kubernetes fleet-ops (`deploy/prd-tst`)

Mundo **separado** (D18). Namespace `qllm-prd`. **Não** com compose ao mesmo tempo.

Guia: [`deploy/prd-tst/README.md`](../deploy/prd-tst/README.md).

```bash
nerdctl compose down -v
nerdctl --namespace k8s.io build -f Dockerfile.dev -t qllm:local .
kubectl apply -k deploy/prd-tst
```

Port-forward (um processo, Ctrl+C mata todos):

```powershell
.\scripts\prd-tst-port-forward.ps1
# Unix: ./scripts/prd-tst-port-forward.sh
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
| `prd-tst-argocd-up.ps1` / `.sh` | Instala Argo `--insecure` |
| `prd-tst-argocd-password.ps1` / `.sh` | Password `admin` |
| `prd-tst-argocd-register-app.ps1` / `.sh` | Application CR |
| `prd-tst-argocd-add-ssh-repo.ps1` / `.sh` | Chave SSH in-cluster |

`imagePullPolicy: Never` + `qllm:local` no namespace `k8s.io`. `ErrImagePull` = a imagem não está no containerd do Kubernetes.

## Scripts de dev

| Script | Função |
|--------|--------|
| `dev-shell.ps1` / `dev-shell.cmd` / `dev-shell.sh` | CGO / gcc Windows vs Unix |
| `dev-seed-fake.ps1` / `.sh` | Seed `fixtures/datasets/v1` |
| `duckdb_smoke.go` | Smoke CGO DuckDB |

## Logs de queries

```powershell
kubectl logs -n qllm-prd deploy/qllm -f
```

Blocos `---- execute_sql ----` (SQL multilinha) e `---- mcp_tool ----`. Não filtres só a primeira linha se quiseres o SQL.
