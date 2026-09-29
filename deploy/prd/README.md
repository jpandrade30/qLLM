# qLLM PRD simulation (fleet-ops)

This is **not** the compose harness (`qllm-demo` / `customers` / `invoices`). It is a second world: Kubernetes namespace `qllm-prd`, project **`fleet-ops`**. Existing pytest/goldens/`docker-compose.yml` are unchanged.

**Do not run at the same time as compose.** Tear down test containers first:

```bash
nerdctl compose down -v
```

Then build the same image the harness uses and apply this overlay (Rancher Desktop Kubernetes on).

## What you should see

- `describe_catalog` lists `vehicles`, `depots`, `gps_samples`, `shift_notes`, `stock_items`, `drivers` — never `customers` / `invoices`.
- `SELECT * FROM customers LIMIT 1` → `UNKNOWN_ENTITY` (isolation).
- Dynamo `stock_items` without `WHERE pk = ...` → `UNSUPPORTED`.

Connectors exercised here: postgres, clickhouse, sqlite, dynamodb (local), rest. **Not** in the default overlay: mssql, cassandra, ksql (Kafka). See `k8s/optional/`.

## Apply without Argo

```bash
# Rancher Desktop: image must land in Kubernetes' containerd (k8s.io), not only compose.
nerdctl --namespace k8s.io build -t qllm:local .
kubectl apply -k deploy/prd
kubectl -n qllm-prd rollout restart deploy/qllm
kubectl -n qllm-prd rollout status deploy/qllm --timeout=180s
```

HTTP and MCP together (one process, one shell):

```powershell
kubectl -n qllm-prd port-forward svc/qllm 18088:8088 18089:8089
```

All of this in **one** window (`Ctrl+C` stops every forward). Skips Argo if `argocd` is not installed:

```powershell
.\scripts\prd-port-forward.ps1
```

| Host | What |
|------|------|
| `127.0.0.1:18088` | qLLM HTTP |
| `127.0.0.1:18089` | qLLM MCP |
| `127.0.0.1:15432` | fleet-pg (Postgres) |
| `127.0.0.1:19000` | fleet-ch native |
| `127.0.0.1:18123` | fleet-ch HTTP (`/ping`) |
| `127.0.0.1:18000` | fleet-ddb |
| `127.0.0.1:18080` | fleet-api |
| `http://127.0.0.1:18081` | Argo CD UI **after** `.\scripts\prd-argocd-up.ps1` (plain HTTP) |

After adding ClickHouse HTTP on the Service, re-apply: `kubectl apply -k deploy/prd`.

`ErrImagePull` on `qllm:local` means kubelet tried Docker Hub. Policy is `Never` (local only). If the image was built with plain `nerdctl build`, import it:

```bash
nerdctl save qllm:local -o qllm-local.tar
nerdctl --namespace k8s.io load -i qllm-local.tar
```

Bearer: `fleet-prd-token` (Secret `qllm-prd-secrets`; not for real production).

```bash
curl -s -H "Authorization: Bearer fleet-prd-token" http://127.0.0.1:18088/v1/catalog
curl -s -H "Authorization: Bearer fleet-prd-token" -H "Content-Type: application/json" ^
  -d "{\"sql\":\"SELECT vin FROM vehicles LIMIT 5\"}" http://127.0.0.1:18088/v1/sql
```

The Deployment **mounts** ConfigMap `qllm-config` at `/config`, so the catalog baked into the image (`deploy/image/config`) is **not** what serve uses.

## Argo CD

`kubectl apply -k deploy/prd` does **not** install Argo. Stock `argocd-server` speaks **TLS** on the pod even if you forward Service port 80, so `http://127.0.0.1:18081` looks “dead” until `--insecure`.

```powershell
.\scripts\prd-argocd-up.ps1
# Ctrl+C the old port-forward, then:
.\scripts\prd-port-forward.ps1
```

Then register the app so it **appears in the UI** (this is a separate kubectl; kustomize does not create Applications):

```powershell
.\scripts\prd-argocd-register-app.ps1
```

Refresh the Argo browser tab. You should see **qllm-prd-sim**. Do not use Git `HEAD` as revision (Argo error: *revision HEAD must be resolved*) — the register script sets `targetRevision` to your current branch. Push that branch (including `deploy/prd`) if it is only local. Private repo: Settings → Repositories.

Password (`admin`):

```powershell
.\scripts\prd-argocd-password.ps1
```

(`prd-argocd-up.ps1` also prints it once.) If that Secret is gone, Argo was already reconfigured — reset with `argocd account update-password`.

Local-only without Argo: fleet-ops still works with `kubectl apply -k deploy/prd`.

## Tear down PRD sim

```bash
kubectl delete -k deploy/prd
# if Argo created the app:
kubectl -n argocd delete application qllm-prd-sim --ignore-not-found
```

Then you can `nerdctl compose up --build` again for goldens.

## Image

`kustomization.yaml` expects `qllm:local` with `imagePullPolicy: Never`. Point Argo at a registry tag when you stop using a desktop cluster.
