# Entornos de este repositorio

## Demo con Compose (goldens)

Rancher Desktop con **nerdctl compose**. Especificación: [`planning/05-dev-harness.md`](../../planning/05-dev-harness.md).

```bash
nerdctl compose up --build
```

Compose monta [`deploy/image/config`](../../deploy/image/config) en `/config` y `fixtures/test-api/data.json` en `test-api`. Cambio de catálogo: `nerdctl compose up -d qllm` (sin rebuild). Bearer: `change-me`. Puertos: HTTP 8088, MCP 8089. Seed: `.\scripts\dev\dev-seed-fake.ps1`.

Seed de la API falsa:

```powershell
.\scripts\dev\dev-seed-fake.ps1
# --regenerate solo reescribe el JSON congelado
```

Goldens de SQL por MCP: `pytest fixtures/sqlcheck`.

Las contraseñas débiles y un Mongo abierto son aceptables **solo en localhost**.

**No** trates este Compose como tu producto. Copia únicamente el *formato* de los YAML a tu propio `--config-dir`.

Para usar tu carpeta en lugar del demo, consulta [point-your-folder.md](point-your-folder.md).

## Imagen standalone

```bash
nerdctl build -t qllm .
# bake: deploy/prd
nerdctl run --rm -p 8088:8088 -p 8089:8089 qllm
```

Compose usa `Dockerfile.dev` y `deploy/image/config`. En la simulación de Kubernetes, el ConfigMap de `deploy/prd-tst/config` sobrescribe lo embebido en la imagen.

Ejemplo solo con YAML y un `Dockerfile`: [`deploy/prd/README.md`](../../deploy/prd/README.md).

## Simulación Kubernetes fleet-ops (`deploy/prd-tst`)

Un mundo **separado** (D18), en el namespace `qllm-prd`. **No** lo ejecutes junto con Compose.

Guía: [`deploy/prd-tst/README.md`](../../deploy/prd-tst/README.md).

```powershell
.\scripts\prd-tst\prd-tst-up.ps1
.\scripts\prd-tst\prd-tst-port-forward.ps1
.\scripts\prd-tst\prd-tst-down.ps1
```

```bash
./scripts/prd-tst/prd-tst-up.sh
./scripts/prd-tst/prd-tst-port-forward.sh
./scripts/prd-tst/prd-tst-down.sh
```

`prd-tst-up` ejecuta `compose down`, construye `qllm:local` (namespace `k8s.io`) y aplica `kubectl apply -k deploy/prd-tst`. El port-forward corre en un solo proceso; Ctrl+C detiene todos los forwards:

```powershell
.\scripts\prd-tst\prd-tst-port-forward.ps1
# Unix: ./scripts/prd-tst/prd-tst-port-forward.sh
```

| Puerto en el host | Servicio |
|-------------------|----------|
| 18088 | qLLM HTTP |
| 18089 | qLLM MCP |
| 15432 | Postgres |
| 19000 / 18123 | ClickHouse |
| 18000 | DynamoDB Local |
| 18080 | API falsa crew |
| 18081 | Argo CD (tras instalarlo) |

Token Bearer de la simulación: `fleet-prd-token` (es un Secret; no lo uses en producción).

Argo CD es **opcional**. Scripts:

| Script | Función |
|--------|---------|
| `prd-tst-up.ps1` / `.sh` | Levanta fleet-ops (compose down, imagen, apply) |
| `prd-tst-down.ps1` / `.sh` | Elimina el overlay y la Application de Argo |
| `prd-tst-argocd-up.ps1` / `.sh` | Instala Argo con `--insecure` |
| `prd-tst-argocd-password.ps1` / `.sh` | Muestra la contraseña de `admin` |
| `prd-tst-argocd-register-app.ps1` / `.sh` | Crea el recurso Application |
| `prd-tst-argocd-add-ssh-repo.ps1` / `.sh` | Registra una clave SSH en el clúster |

La imagen usa `imagePullPolicy: Never` con `qllm:local` en el namespace `k8s.io`. `ErrImagePull` significa que la imagen no está en el containerd de Kubernetes.

## Scripts de desarrollo

| Script | Función |
|--------|---------|
| `scripts/dev/dev-shell.ps1` / `.cmd` / `.sh` | Configuración de CGO y gcc (Windows vs Unix) |
| `scripts/dev/dev-seed-fake.ps1` / `.sh` | Seed de `fixtures/datasets/v1` |
| `scripts/dev/duckdb_smoke.go` | Smoke test de DuckDB con CGO |
| `scripts/prd-tst/` | Simulación Kubernetes fleet-ops |
| `scripts/standalone/` | Generador de repo reducido |

## Logs de consultas

```powershell
kubectl logs -n qllm-prd deploy/qllm -f
```

Verás bloques `---- execute_sql ----` (SQL en varias líneas) y `---- mcp_tool ----`. Si quieres ver el SQL, no filtres solo por la primera línea.
