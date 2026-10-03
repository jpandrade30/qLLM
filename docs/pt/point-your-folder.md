# Como apontar o qLLM para a **sua** pasta

O binário só enxerga o YAML que você fornecer.

| Imagem | Embutido em `/config` | Uso |
|--------|-----------------------|-----|
| [`Dockerfile`](../../Dockerfile) | [`deploy/prd/`](../../deploy/prd) | Produto / exemplo (`docker build`) |
| [`Dockerfile.dev`](../../Dockerfile.dev) | [`deploy/image/config`](../../deploy/image/config) | Compose e binário da simulação K8s |

## No seu PC (binário)

```text
qllm serve --http --mcp-http --config-dir CAMINHO_ABSOLUTO_DA_SUA_PASTA
```

Exemplos:

```powershell
.\qllm.exe serve --http --mcp-http --config-dir C:\dados\meu-qllm
.\qllm.exe serve --http --mcp-http --config-dir .\meu-qllm
```

Equivalente:

```powershell
.\qllm.exe serve --http --preset C:\dados\meu-qllm\qllm.preset.yaml --catalog C:\dados\meu-qllm\qllm.catalog.yaml
```

(`--preset` e `--catalog` são obrigatórios **juntos**.)

Ou use um arquivo ponteiro, `C:\dados\meu-qllm\qllm.project.yaml`:

```yaml
protocolVersion: "0.2.0"
preset: qllm.preset.yaml
catalog: qllm.catalog.yaml
```

```powershell
.\qllm.exe serve --http --project C:\dados\meu-qllm\qllm.project.yaml
```

`preset` e `catalog` são relativos a **essa** pasta; não é possível usar `..\fora`.

Sem `--config-dir`, `--project` ou caminhos explícitos, o qLLM usa o **diretório de trabalho atual**. Se você rodou o comando em `C:\codes\qLLM` e **não** há `qllm.preset.yaml` lá, ele falha. Se houver um arquivo esquecido, você estará servindo esse arquivo.

`qllm.config.yaml`, `qllm.access.yaml` e `qllm.env.yaml` são lidos do **mesmo** `--config-dir` (ou do diretório atual). O `--runtime-config` só substitui o caminho do `qllm.config`.

## Docker / nerdctl (não reconstrua o catálogo no Dockerfile)

A imagem já tem `CMD ["serve", "--http", "--mcp-http", "--config-dir", "/config"]`.

Monte a **sua** pasta em `/config` (com os mesmos nomes `qllm.*.yaml`):

```powershell
nerdctl run --rm -p 8088:8088 -p 8089:8089 `
  -v C:\dados\meu-qllm:/config:ro `
  -e QLLM_AUTH_TOKEN=um-token-longo `
  -e QLLM_CRM_PG_HOST=host.docker.internal `
  -e QLLM_CRM_PG_USER=app `
  -e QLLM_CRM_PG_PASSWORD=segredo `
  qllm
```

`host.docker.internal` é um Postgres rodando no host Windows. Se o banco estiver em outro contêiner, use o **nome do serviço** na mesma rede, não `127.0.0.1` (esse endereço é o próprio contêiner do qLLM).

Prova: `GET /v1/catalog` retorna o seu `project`. Se ainda aparecer o demo, o `-v` não foi aplicado (caminho errado no Windows, ou você montou só um arquivo).

## Alterar o que a imagem **embute** (rebuild)

O [`Dockerfile`](../../Dockerfile) de produto faz:

```text
COPY deploy/prd/default/ /config/
CMD serve --http --mcp-http --config-dir /config
```

Edite [`deploy/prd/`](../../deploy/prd) e execute `docker build -t qllm .`. Guia: [`deploy/prd/README.md`](../../deploy/prd/README.md).

Harness: edite `deploy/image/config/*` e rode `nerdctl compose build` ou `nerdctl build -f Dockerfile.dev`.

Montar `-v sua-pasta:/config` continua sobrepondo o que foi embutido.

## Simulação Kubernetes (`deploy/prd-tst`)

O Deployment **não** usa o catálogo embutido (`Dockerfile.dev`). Ele monta o ConfigMap `qllm-config` em `/config`.

Os YAML que o cluster usa ficam em:

```text
deploy/prd-tst/config/qllm.preset.yaml
deploy/prd-tst/config/qllm.catalog.yaml
deploy/prd-tst/config/qllm.config.yaml
deploy/prd-tst/config/qllm.env.yaml
deploy/prd-tst/config/qllm.access.yaml
```

Ligação em [`deploy/prd-tst/kustomization.yaml`](../../deploy/prd-tst/kustomization.yaml):

```yaml
configMapGenerator:
  - name: qllm-config
    files:
      - qllm.preset.yaml=config/qllm.preset.yaml
      - qllm.catalog.yaml=config/qllm.catalog.yaml
      …
```

O que fazer:

1. Substitua `deploy/prd-tst/config/*.yaml` **ou** altere os caminhos no kustomize.
2. Segredos: [`deploy/prd-tst/k8s/secret.yaml`](../../deploy/prd-tst/k8s/secret.yaml).
3. Rode `.\scripts\prd-tst\prd-tst-up.ps1` (ou `./scripts/prd-tst/prd-tst-up.sh`). Para pular o compose down em um rebuild, use `-SkipComposeDown`.
4. Prova: `GET /v1/catalog` mostra o projeto `fleet-ops` e `vehicles`.

Argumentos do pod: `serve --http --mcp-http --config-dir /config` ([`k8s/qllm.yaml`](../../deploy/prd-tst/k8s/qllm.yaml)).

## Compose do harness

O `nerdctl compose` sobe o **demo**. O serviço qLLM usa a imagem com `/config` embutido mais as variáveis de ambiente do Compose. Para o **seu** mundo, use outro Compose ou `--config-dir`, ou o overlay PRD. Não misture os dois.

## Checklist: "subiu o que eu escrevi?"

| Verificar | Esperado quando é a sua pasta |
|-----------|-------------------------------|
| `validate` no stderr `preset=` / `catalog=` | Caminhos dos **seus** arquivos |
| `entities=` | A contagem do seu `entities:` |
| `GET /v1/catalog` → `project` | Igual ao `project:` nos dois YAML (preset e catálogo precisam coincidir) |
| Nomes em `entities[].name` | Somente os que você escreveu |
| `SELECT * FROM customers` no PRD fleet-ops | `UNKNOWN_ENTITY` (lá não existe `customers`) |
| Logs `---- execute_sql ----` | O SQL que o agente enviou **agora** |

Se o health está ok, mas o catálogo é o do demo: a **pasta, o mount ou o ConfigMap está errado**. O qLLM não misturou os dois mundos sozinho.
