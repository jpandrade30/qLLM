# Cómo apuntar qLLM a **tu** carpeta

El binario solo ve el YAML que le des.

| Imagen | Embebido en `/config` | Uso |
|--------|-----------------------|-----|
| [`Dockerfile`](../../Dockerfile) | [`deploy/prd/`](../../deploy/prd) | Producto / ejemplo (`docker build`) |
| [`Dockerfile.dev`](../../Dockerfile.dev) | [`deploy/image/config`](../../deploy/image/config) | Compose y binario de la simulación K8s |

## En tu PC (binario)

```text
qllm serve --http --mcp-http --config-dir RUTA_ABSOLUTA_DE_TU_CARPETA
```

Ejemplos:

```powershell
.\qllm.exe serve --http --mcp-http --config-dir C:\datos\mi-qllm
.\qllm.exe serve --http --mcp-http --config-dir .\mi-qllm
```

Equivalente:

```powershell
.\qllm.exe serve --http --preset C:\datos\mi-qllm\qllm.preset.yaml --catalog C:\datos\mi-qllm\qllm.catalog.yaml
```

(`--preset` y `--catalog` son obligatorios **juntos**.)

O usa un archivo puntero, `C:\datos\mi-qllm\qllm.project.yaml`:

```yaml
protocolVersion: "0.2.0"
preset: qllm.preset.yaml
catalog: qllm.catalog.yaml
```

```powershell
.\qllm.exe serve --http --project C:\datos\mi-qllm\qllm.project.yaml
```

`preset` y `catalog` son relativos a **esa** carpeta; no puedes usar `..\fuera`.

Sin `--config-dir`, `--project` ni rutas explícitas, qLLM usa el **directorio de trabajo actual**. Si ejecutaste el comando en `C:\codes\qLLM` y allí **no** hay `qllm.preset.yaml`, falla. Si hay un archivo olvidado, estarás sirviendo ese archivo.

`qllm.config.yaml`, `qllm.access.yaml` y `qllm.env.yaml` se leen del **mismo** `--config-dir` (o del directorio actual). `--runtime-config` solo sustituye la ruta de `qllm.config`.

## Docker / nerdctl (no reconstruyas el catálogo en el Dockerfile)

La imagen ya tiene `CMD ["serve", "--http", "--mcp-http", "--config-dir", "/config"]`.

Monta **tu** carpeta en `/config` (con los mismos nombres `qllm.*.yaml`):

```powershell
nerdctl run --rm -p 8088:8088 -p 8089:8089 `
  -v C:\datos\mi-qllm:/config:ro `
  -e QLLM_AUTH_TOKEN=un-token-largo `
  -e QLLM_CRM_PG_HOST=host.docker.internal `
  -e QLLM_CRM_PG_USER=app `
  -e QLLM_CRM_PG_PASSWORD=secreto `
  qllm
```

`host.docker.internal` es un Postgres que corre en el host Windows. Si la base de datos está en otro contenedor, usa el **nombre del servicio** en la misma red, no `127.0.0.1` (esa dirección es el propio contenedor de qLLM).

Comprobación: `GET /v1/catalog` devuelve tu `project`. Si sigue apareciendo el demo, el `-v` no se aplicó (ruta incorrecta en Windows, o montaste solo un archivo).

## Cambiar lo que la imagen **embebe** (recompilar)

El [`Dockerfile`](../../Dockerfile) de producto hace:

```text
COPY deploy/prd/qllm.preset.yaml …
COPY deploy/prd/qllm.access.yaml …
CMD serve --http --mcp-http --config-dir /config
```

Edita [`deploy/prd/`](../../deploy/prd) y ejecuta `docker build -t qllm .`. Guía: [`deploy/prd/README.md`](../../deploy/prd/README.md).

Harness: edita `deploy/image/config/*` y ejecuta `nerdctl compose build` o `nerdctl build -f Dockerfile.dev`.

Montar `-v tu-carpeta:/config` sigue sobrescribiendo lo embebido.

## Simulación Kubernetes (`deploy/prd-tst`)

El Deployment **no** usa el catálogo embebido (`Dockerfile.dev`). Monta el ConfigMap `qllm-config` en `/config`.

Los YAML que usa el clúster están en:

```text
deploy/prd-tst/config/qllm.preset.yaml
deploy/prd-tst/config/qllm.catalog.yaml
deploy/prd-tst/config/qllm.config.yaml
deploy/prd-tst/config/qllm.env.yaml
deploy/prd-tst/config/qllm.access.yaml
```

Conexión en [`deploy/prd-tst/kustomization.yaml`](../../deploy/prd-tst/kustomization.yaml):

```yaml
configMapGenerator:
  - name: qllm-config
    files:
      - qllm.preset.yaml=config/qllm.preset.yaml
      - qllm.catalog.yaml=config/qllm.catalog.yaml
      …
```

Qué hacer:

1. Sustituye `deploy/prd-tst/config/*.yaml` **o** cambia las rutas en kustomize.
2. Secretos: [`deploy/prd-tst/k8s/secret.yaml`](../../deploy/prd-tst/k8s/secret.yaml).
3. Ejecuta `.\scripts\prd-tst-up.ps1` (o `./scripts/prd-tst-up.sh`). Para omitir el compose down al reconstruir, usa `-SkipComposeDown`.
4. Comprobación: `GET /v1/catalog` muestra el proyecto `fleet-ops` y `vehicles`.

Argumentos del pod: `serve --http --mcp-http --config-dir /config` ([`k8s/qllm.yaml`](../../deploy/prd-tst/k8s/qllm.yaml)).

## Compose del harness

`nerdctl compose` levanta el **demo**. El servicio qLLM usa la imagen con `/config` embebido más las variables de entorno de Compose. Para **tu** mundo, usa otro Compose o `--config-dir`, o el overlay PRD. No los mezcles.

## Lista de verificación: "¿cargó lo que escribí?"

| Comprobar | Esperado cuando es tu carpeta |
|-----------|-------------------------------|
| `validate` en stderr `preset=` / `catalog=` | Rutas de **tus** archivos |
| `entities=` | El número de tu `entities:` |
| `GET /v1/catalog` → `project` | Igual que `project:` en ambos YAML (preset y catálogo deben coincidir) |
| Nombres en `entities[].name` | Solo los que escribiste |
| `SELECT * FROM customers` en PRD fleet-ops | `UNKNOWN_ENTITY` (allí no existe `customers`) |
| Logs `---- execute_sql ----` | El SQL que el agente envió **ahora** |

Si el health está bien pero el catálogo es el del demo: la **carpeta, el mount o el ConfigMap es incorrecto**. qLLM no mezcló los dos mundos por sí solo.
