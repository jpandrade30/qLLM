# Ambientes neste repositório

## Demo com Compose (goldens)

Rancher Desktop com **nerdctl compose**. Especificação: [`planning/05-dev-harness.md`](../../planning/05-dev-harness.md).

```bash
nerdctl compose up --build
```

O stack do Compose embute a configuração a partir do [`Dockerfile.dev`](../../Dockerfile.dev) e de [`deploy/image/config`](../../deploy/image/config). Bearer HTTP do demo: `change-me`. Portas no host: HTTP 8088, MCP 8089 (veja o arquivo do Compose). Faça o seed com `.\scripts\dev-seed-fake.ps1` ou `./scripts/dev-seed-fake.sh`.

Seed da API fake:

```powershell
.\scripts\dev-seed-fake.ps1
# --regenerate apenas reescreve o JSON congelado
```

Goldens de SQL via MCP: `pytest fixtures/sqlcheck`.

Senhas fracas e Mongo aberto são aceitáveis **somente em localhost**.

**Não** trate este Compose como o seu produto. Copie apenas o *formato* dos YAML para o seu próprio `--config-dir`.

Para usar a sua pasta no lugar do demo, veja [point-your-folder.md](point-your-folder.md).

## Imagem standalone

```bash
nerdctl build -t qllm .
# bake: deploy/prd
nerdctl run --rm -p 8088:8088 -p 8089:8089 qllm
```

O Compose usa `Dockerfile.dev` e `deploy/image/config`. Na simulação Kubernetes, o ConfigMap de `deploy/prd-tst/config` sobrepõe o que foi embutido na imagem.

Exemplo só com YAML e um `Dockerfile`: [`deploy/prd/README.md`](../../deploy/prd/README.md).

## Simulação Kubernetes fleet-ops (`deploy/prd-tst`)

Um mundo **separado** (D18), no namespace `qllm-prd`. **Não** rode junto com o Compose.

Guia: [`deploy/prd-tst/README.md`](../../deploy/prd-tst/README.md).

```powershell
.\scripts\prd-tst-up.ps1
.\scripts\prd-tst-port-forward.ps1
.\scripts\prd-tst-down.ps1
```

```bash
./scripts/prd-tst-up.sh
./scripts/prd-tst-port-forward.sh
./scripts/prd-tst-down.sh
```

O `prd-tst-up` executa `compose down`, constrói `qllm:local` (namespace `k8s.io`) e aplica `kubectl apply -k deploy/prd-tst`. O port-forward roda em um único processo; Ctrl+C encerra todos os forwards:

```powershell
.\scripts\prd-tst-port-forward.ps1
# Unix: ./scripts/prd-tst-port-forward.sh
```

| Porta no host | Serviço |
|---------------|---------|
| 18088 | qLLM HTTP |
| 18089 | qLLM MCP |
| 15432 | Postgres |
| 19000 / 18123 | ClickHouse |
| 18000 | DynamoDB Local |
| 18080 | API fake crew |
| 18081 | Argo CD (depois de instalar) |

Token Bearer da simulação: `fleet-prd-token` (é um Secret; não use em produção).

O Argo CD é **opcional**. Scripts:

| Script | Função |
|--------|--------|
| `prd-tst-up.ps1` / `.sh` | Sobe o fleet-ops (compose down, imagem, apply) |
| `prd-tst-down.ps1` / `.sh` | Remove o overlay e a Application do Argo |
| `prd-tst-argocd-up.ps1` / `.sh` | Instala o Argo com `--insecure` |
| `prd-tst-argocd-password.ps1` / `.sh` | Mostra a senha do `admin` |
| `prd-tst-argocd-register-app.ps1` / `.sh` | Cria o recurso Application |
| `prd-tst-argocd-add-ssh-repo.ps1` / `.sh` | Registra uma chave SSH no cluster |

A imagem usa `imagePullPolicy: Never` com `qllm:local` no namespace `k8s.io`. `ErrImagePull` significa que a imagem não está no containerd do Kubernetes.

## Scripts de desenvolvimento

| Script | Função |
|--------|--------|
| `dev-shell.ps1` / `dev-shell.cmd` / `dev-shell.sh` | Configuração de CGO e gcc (Windows vs Unix) |
| `dev-seed-fake.ps1` / `.sh` | Seed de `fixtures/datasets/v1` |
| `duckdb_smoke.go` | Smoke test do DuckDB com CGO |

## Logs de consultas

```powershell
kubectl logs -n qllm-prd deploy/qllm -f
```

Você verá blocos `---- execute_sql ----` (SQL em várias linhas) e `---- mcp_tool ----`. Se quiser ver o SQL, não filtre só pela primeira linha.
