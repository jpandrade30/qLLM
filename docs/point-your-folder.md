# Como apontar o qLLM para **a tua** pasta

O binário só vê YAML se lhos deres.

| Imagem | Bake em `/config` | Uso |
|--------|-------------------|-----|
| [`Dockerfile`](../Dockerfile) | [`deploy/prd/`](../deploy/prd) | Produto / exemplo (`docker build`) |
| [`Dockerfile.dev`](../Dockerfile.dev) | [`deploy/image/config`](../deploy/image/config) | Compose + binário do sim K8s |

## No teu PC (binário)

```text
qllm serve --http --mcp-http --config-dir CAMINHO_ABSOLUTO_DA_TUA_PASTA
```

Exemplos:

```powershell
.\qllm.exe serve --http --mcp-http --config-dir C:\dados\meu-qllm
.\qllm.exe serve --http --mcp-http --config-dir .\meu-qllm
```

Equivalentes:

```powershell
.\qllm.exe serve --http --preset C:\dados\meu-qllm\qllm.preset.yaml --catalog C:\dados\meu-qllm\qllm.catalog.yaml
```

(`--preset` e `--catalog` **os dois**.)

Ou um ponteiro:

`C:\dados\meu-qllm\qllm.project.yaml`:

```yaml
protocolVersion: "0.2.0"
preset: qllm.preset.yaml
catalog: qllm.catalog.yaml
```

```powershell
.\qllm.exe serve --http --project C:\dados\meu-qllm\qllm.project.yaml
```

`preset`/`catalog` são relativos a **essa** pasta; não podes `..\fora`.

Sem `--config-dir` / `--project` / paths: usa o **diretório de trabalho actual**. Se correste o comando em `C:\codes\qLLM` e lá **não** tens `qllm.preset.yaml`, falha. Se tens um leftover, serves o leftover.

`qllm.config.yaml`, `qllm.access.yaml`, `qllm.env.yaml` são lidos do **mesmo** `--config-dir` (ou CWD). `--runtime-config` só substitui o path do `qllm.config`.

## Docker / nerdctl (não rebuildar o catalog no Dockerfile)

A imagem já tem `CMD ["serve", "--http", "--mcp-http", "--config-dir", "/config"]`.

Monta a **tua** pasta em `/config` (nomes `qllm.*.yaml` iguais):

```powershell
nerdctl run --rm -p 8088:8088 -p 8089:8089 `
  -v C:\dados\meu-qllm:/config:ro `
  -e QLLM_AUTH_TOKEN=um-token-longo `
  -e QLLM_CRM_PG_HOST=host.docker.internal `
  -e QLLM_CRM_PG_USER=app `
  -e QLLM_CRM_PG_PASSWORD=segredo `
  qllm
```

`host.docker.internal` = Postgres no Windows host. Se a base está noutro contentor, usa o **nome do serviço** na mesma rede, não `127.0.0.1` (isso é o contentor qLLM).

Prova: `GET /v1/catalog` → `project` = o teu. Se ainda é o demo, o `-v` não montou (path errado no Windows, ou só montaste um ficheiro).

## Mudar o que a imagem **bakeia** (rebuild)

O [`Dockerfile`](../Dockerfile) (produto) faz:

```text
COPY deploy/prd/qllm.preset.yaml …
COPY deploy/prd/qllm.access.yaml …
CMD serve --http --mcp-http --config-dir /config
```

Edita [`deploy/prd/`](../deploy/prd) e `docker build -t qllm .`. Guia: [`deploy/prd/README.md`](../deploy/prd/README.md).

Harness: edita `deploy/image/config/*` e `nerdctl compose build` / `nerdctl build -f Dockerfile.dev`.

Montar `-v tua-pasta:/config` continua a tapar o bake.

## Kubernetes sim (`deploy/prd-tst`)

O Deployment **não** usa o catalog bakeado (`Dockerfile.dev`). Monta o ConfigMap `qllm-config` em `/config`.

Os YAML que o cluster usa estão em:

```text
deploy/prd-tst/config/qllm.preset.yaml
deploy/prd-tst/config/qllm.catalog.yaml
deploy/prd-tst/config/qllm.config.yaml
deploy/prd-tst/config/qllm.env.yaml
deploy/prd-tst/config/qllm.access.yaml
```

Ligação no [`deploy/prd-tst/kustomization.yaml`](../deploy/prd-tst/kustomization.yaml):

```yaml
configMapGenerator:
  - name: qllm-config
    files:
      - qllm.preset.yaml=config/qllm.preset.yaml
      - qllm.catalog.yaml=config/qllm.catalog.yaml
      …
```

O que fazes:

1. Substitui `deploy/prd-tst/config/*.yaml` **ou** muda os paths no kustomize.
2. Segredos: [`deploy/prd-tst/k8s/secret.yaml`](../deploy/prd-tst/k8s/secret.yaml).
3. `nerdctl --namespace k8s.io build -f Dockerfile.dev -t qllm:local .` então `kubectl apply -k deploy/prd-tst` e `rollout restart deploy/qllm`.
4. Prova: `GET /v1/catalog` → project `fleet-ops` / `vehicles`.

Args do pod: `serve --http --mcp-http --config-dir /config` ([`k8s/qllm.yaml`](../deploy/prd-tst/k8s/qllm.yaml)).

## Compose do harness

`nerdctl compose` sobe o **demo**. O serviço qLLM usa a imagem com `/config` bakeado + env do compose. Para o **teu** mundo: outro compose/`--config-dir`, ou o overlay PRD. Não mistures.

## Checklist “subiu o que eu gerei”

| Ver | Esperado se for a tua pasta |
|-----|-----------------------------|
| `validate` stderr `preset=` / `catalog=` | Paths dos **teus** ficheiros |
| `entities=` | Contagem do teu `entities:` |
| `GET /v1/catalog` → `project` | Igual a `project:` nos dois YAML (preset e catalog devem coincidir) |
| Nomes em `entities[].name` | Só os que escreveste |
| `SELECT * FROM customers` no PRD fleet-ops | `UNKNOWN_ENTITY` (lá não existe `customers`) |
| Logs `---- execute_sql ----` | O SQL que o agente mandou **agora** |

Se health está ok mas o catalog é o demo: **pasta/mount/ConfigMap errados**, não “o qLLM misturou os mundos sozinho”.
