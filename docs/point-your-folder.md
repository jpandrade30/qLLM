# Como apontar o qLLM para **a tua** pasta

O binário só vê YAML se lhos deres. A imagem deste repo **copia** `deploy/image/config` para `/config` no build — isso é o demo. Produção / o teu projecto = **outra pasta** montada ou outro ConfigMap.

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

O [`Dockerfile`](../Dockerfile) faz:

```text
COPY deploy/image/config/qllm.preset.yaml …
COPY deploy/image/config/qllm.catalog.yaml …
COPY deploy/image/config/qllm.config.yaml deploy/image/config/qllm.env.yaml …
ENTRYPOINT qllm
CMD serve --http --mcp-http --config-dir /config
```

Para o **harness deste repo**, edita `deploy/image/config/*` e `nerdctl build`. **Não** é o sítio do projecto do cliente.

Para uma imagem **tua**: muda os `COPY` para a tua pasta, ou deixa o `COPY` do demo e **monta sempre** `/config` em runtime (preferível).

A imagem **não** copia `qllm.access.yaml` no Dockerfile actual. ACL no contentor = monta o ficheiro em `/config/qllm.access.yaml` ou acrescenta um `COPY` e rebuild.

## Kubernetes neste repo (`deploy/prd`)

O Deployment **não** usa o catalog bakeado. Monta o ConfigMap `qllm-config` em `/config`.

Os YAML que o cluster usa estão em:

```text
deploy/prd/config/qllm.preset.yaml
deploy/prd/config/qllm.catalog.yaml
deploy/prd/config/qllm.config.yaml
deploy/prd/config/qllm.env.yaml
deploy/prd/config/qllm.access.yaml
```

Ligação no [`deploy/prd/kustomization.yaml`](../deploy/prd/kustomization.yaml):

```yaml
configMapGenerator:
  - name: qllm-config
    files:
      - qllm.preset.yaml=config/qllm.preset.yaml
      - qllm.catalog.yaml=config/qllm.catalog.yaml
      …
```

O que fazes:

1. Substitui o conteúdo de `deploy/prd/config/*.yaml` **ou** muda os paths `config/…` para a tua pasta (ex. `../../meu-qllm/qllm.preset.yaml` — kustomize resolve relativamente a `deploy/prd`).
2. Segredos: [`deploy/prd/k8s/secret.yaml`](../deploy/prd/k8s/secret.yaml) (`envFrom` no pod). Os `*Env` do preset têm de existir neste Secret ou no `qllm.env.yaml` do ConfigMap.
3. `kubectl apply -k deploy/prd` e `rollout restart deploy/qllm` (ConfigMap com `disableNameSuffixHash` precisa restart para o pod reler).
4. Prova: `kubectl logs` + port-forward + `GET /v1/catalog` → `project` e entidades **tuas**.

Args fixos no pod: `serve --http --mcp-http --config-dir /config` ([`k8s/qllm.yaml`](../deploy/prd/k8s/qllm.yaml)). Não apontam para `deploy/image/config`.

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
