# Environments in this repository

## Compose demo (goldens)

Rancher Desktop with **nerdctl compose**. Spec: [`planning/05-dev-harness.md`](../../planning/05-dev-harness.md).

```bash
nerdctl compose up --build
```

The Compose stack bakes its config from [`Dockerfile.dev`](../../Dockerfile.dev) and [`deploy/image/config`](../../deploy/image/config). Demo HTTP Bearer: `change-me`. Host ports: HTTP 8088, MCP 8089 (see the Compose file). Seed with `.\scripts\dev-seed-fake.ps1` or `./scripts/dev-seed-fake.sh`.

Seed the fake API:

```powershell
.\scripts\dev-seed-fake.ps1
# --regenerate only rewrites the frozen JSON
```

SQL goldens over MCP: `pytest fixtures/sqlcheck`.

Weak passwords and an open Mongo are acceptable **on localhost only**.

Do **not** treat this Compose stack as your product. Copy the *shape* of the YAML into your own `--config-dir`.

To use your own folder instead of the demo, see [point-your-folder.md](point-your-folder.md).

## Standalone image

```bash
nerdctl build -t qllm .
# bake: deploy/prd
nerdctl run --rm -p 8088:8088 -p 8089:8089 qllm
```

Compose uses `Dockerfile.dev` and `deploy/image/config`. In the Kubernetes simulation, the ConfigMap from `deploy/prd-tst/config` overrides the bake.

YAML-only example with a `Dockerfile`: [`deploy/prd/README.md`](../../deploy/prd/README.md).

## Kubernetes fleet-ops simulation (`deploy/prd-tst`)

A **separate** world (D18), namespace `qllm-prd`. Do **not** run it together with Compose.

Guide: [`deploy/prd-tst/README.md`](../../deploy/prd-tst/README.md).

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

`prd-tst-up` runs `compose down`, builds `qllm:local` (namespace `k8s.io`), and applies `kubectl apply -k deploy/prd-tst`. Port-forward runs as one process; Ctrl+C stops every forward:

```powershell
.\scripts\prd-tst-port-forward.ps1
# Unix: ./scripts/prd-tst-port-forward.sh
```

| Host port | Service |
|-----------|---------|
| 18088 | qLLM HTTP |
| 18089 | qLLM MCP |
| 15432 | Postgres |
| 19000 / 18123 | ClickHouse |
| 18000 | DynamoDB Local |
| 18080 | fake crew API |
| 18081 | Argo CD (after install) |

Simulation Bearer token: `fleet-prd-token` (a Secret; not for production).

Argo CD is **optional**. Scripts:

| Script | Purpose |
|--------|---------|
| `prd-tst-up.ps1` / `.sh` | Bring up fleet-ops (compose down, image, apply) |
| `prd-tst-down.ps1` / `.sh` | Remove the overlay and the Argo Application |
| `prd-tst-argocd-up.ps1` / `.sh` | Install Argo with `--insecure` |
| `prd-tst-argocd-password.ps1` / `.sh` | Print the `admin` password |
| `prd-tst-argocd-register-app.ps1` / `.sh` | Create the Application resource |
| `prd-tst-argocd-add-ssh-repo.ps1` / `.sh` | Register an SSH key in the cluster |

The image uses `imagePullPolicy: Never` with `qllm:local` in the `k8s.io` namespace. `ErrImagePull` means the image is not in the Kubernetes containerd.

## Dev scripts

| Script | Purpose |
|--------|---------|
| `dev-shell.ps1` / `dev-shell.cmd` / `dev-shell.sh` | CGO and gcc setup (Windows vs Unix) |
| `dev-seed-fake.ps1` / `.sh` | Seed `fixtures/datasets/v1` |
| `duckdb_smoke.go` | CGO DuckDB smoke test |

## Query logs

```powershell
kubectl logs -n qllm-prd deploy/qllm -f
```

You will see `---- execute_sql ----` blocks (multi-line SQL) and `---- mcp_tool ----`. Do not filter on the first line only if you want the SQL.
