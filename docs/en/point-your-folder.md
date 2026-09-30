# Point qLLM at **your** folder

The binary only sees YAML you give it.

| Image | Baked into `/config` | Use |
|-------|----------------------|-----|
| [`Dockerfile`](../../Dockerfile) | [`deploy/prd/`](../../deploy/prd) | Product / example (`docker build`) |
| [`Dockerfile.dev`](../../Dockerfile.dev) | [`deploy/image/config`](../../deploy/image/config) | Compose and the K8s simulation binary |

## On your PC (binary)

```text
qllm serve --http --mcp-http --config-dir ABSOLUTE_PATH_TO_YOUR_FOLDER
```

Examples:

```powershell
.\qllm.exe serve --http --mcp-http --config-dir C:\data\my-qllm
.\qllm.exe serve --http --mcp-http --config-dir .\my-qllm
```

Equivalent:

```powershell
.\qllm.exe serve --http --preset C:\data\my-qllm\qllm.preset.yaml --catalog C:\data\my-qllm\qllm.catalog.yaml
```

(`--preset` and `--catalog` are required **together**.)

Or use a pointer file, `C:\data\my-qllm\qllm.project.yaml`:

```yaml
protocolVersion: "0.2.0"
preset: qllm.preset.yaml
catalog: qllm.catalog.yaml
```

```powershell
.\qllm.exe serve --http --project C:\data\my-qllm\qllm.project.yaml
```

`preset` and `catalog` are relative to **that** folder; you cannot use `..\outside`.

Without `--config-dir`, `--project`, or explicit paths, qLLM uses the **current working directory**. If you ran the command in `C:\codes\qLLM` and there is **no** `qllm.preset.yaml` there, it fails. If a leftover file exists, you serve the leftover.

`qllm.config.yaml`, `qllm.access.yaml`, and `qllm.env.yaml` are read from the **same** `--config-dir` (or CWD). `--runtime-config` only replaces the path of `qllm.config`.

## Docker / nerdctl (do not rebuild the catalog in the Dockerfile)

The image already has `CMD ["serve", "--http", "--mcp-http", "--config-dir", "/config"]`.

Mount **your** folder at `/config` (same `qllm.*.yaml` names):

```powershell
nerdctl run --rm -p 8088:8088 -p 8089:8089 `
  -v C:\data\my-qllm:/config:ro `
  -e QLLM_AUTH_TOKEN=a-long-token `
  -e QLLM_CRM_PG_HOST=host.docker.internal `
  -e QLLM_CRM_PG_USER=app `
  -e QLLM_CRM_PG_PASSWORD=secret `
  qllm
```

`host.docker.internal` is a Postgres on your Windows host. If the database is in another container, use the **service name** on the same network, not `127.0.0.1` (that address is the qLLM container itself).

Proof: `GET /v1/catalog` returns your `project`. If it still shows the demo, the `-v` mount did not apply (wrong Windows path, or you mounted a single file).

## Change what the image **bakes** (rebuild)

The product [`Dockerfile`](../../Dockerfile) does:

```text
COPY deploy/prd/qllm.preset.yaml …
COPY deploy/prd/qllm.access.yaml …
CMD serve --http --mcp-http --config-dir /config
```

Edit [`deploy/prd/`](../../deploy/prd) and run `docker build -t qllm .`. Guide: [`deploy/prd/README.md`](../../deploy/prd/README.md).

Harness: edit `deploy/image/config/*` and run `nerdctl compose build` or `nerdctl build -f Dockerfile.dev`.

Mounting `-v your-folder:/config` still overrides the bake.

## Kubernetes simulation (`deploy/prd-tst`)

The Deployment does **not** use the baked catalog (`Dockerfile.dev`). It mounts the `qllm-config` ConfigMap at `/config`.

The YAML the cluster uses lives in:

```text
deploy/prd-tst/config/qllm.preset.yaml
deploy/prd-tst/config/qllm.catalog.yaml
deploy/prd-tst/config/qllm.config.yaml
deploy/prd-tst/config/qllm.env.yaml
deploy/prd-tst/config/qllm.access.yaml
```

Wiring in [`deploy/prd-tst/kustomization.yaml`](../../deploy/prd-tst/kustomization.yaml):

```yaml
configMapGenerator:
  - name: qllm-config
    files:
      - qllm.preset.yaml=config/qllm.preset.yaml
      - qllm.catalog.yaml=config/qllm.catalog.yaml
      …
```

What to do:

1. Replace `deploy/prd-tst/config/*.yaml`, **or** change the paths in kustomize.
2. Secrets: [`deploy/prd-tst/k8s/secret.yaml`](../../deploy/prd-tst/k8s/secret.yaml).
3. Run `.\scripts\prd-tst-up.ps1` (or `./scripts/prd-tst-up.sh`). To skip the compose teardown on a rebuild, use `-SkipComposeDown`.
4. Proof: `GET /v1/catalog` shows project `fleet-ops` and `vehicles`.

Pod args: `serve --http --mcp-http --config-dir /config` ([`k8s/qllm.yaml`](../../deploy/prd-tst/k8s/qllm.yaml)).

## Harness Compose

`nerdctl compose` brings up the **demo**. The qLLM service uses the image with `/config` baked in plus the Compose env. For **your** world, use another Compose file or `--config-dir`, or the PRD overlay. Do not mix them.

## Checklist: "did it load what I wrote?"

| Check | Expected when it is your folder |
|-------|---------------------------------|
| `validate` stderr `preset=` / `catalog=` | Paths of **your** files |
| `entities=` | The count of your `entities:` |
| `GET /v1/catalog` → `project` | Same as `project:` in both YAML files (preset and catalog must match) |
| Names in `entities[].name` | Only the ones you wrote |
| `SELECT * FROM customers` in PRD fleet-ops | `UNKNOWN_ENTITY` (there is no `customers` there) |
| Logs `---- execute_sql ----` | The SQL the agent sent **just now** |

If health is fine but the catalog is the demo: the **folder, mount, or ConfigMap is wrong**. qLLM did not mix the two worlds by itself.
