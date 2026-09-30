# qLLM PRD simulation (fleet-ops)

This is **not** the compose harness (`qllm-demo` / `customers` / `invoices`). It is a second world: Kubernetes namespace `qllm-prd`, project **`fleet-ops`**. Existing pytest/goldens/`docker-compose.yml` are unchanged.

**Do not run at the same time as compose.** From the repo root (Rancher Desktop Kubernetes on):

```powershell
.\scripts\prd-tst-up.ps1
.\scripts\prd-tst-port-forward.ps1
```

```bash
./scripts/prd-tst-up.sh
./scripts/prd-tst-port-forward.sh
```

`prd-tst-up` runs `nerdctl compose down -v`, builds `qllm:local` into the `k8s.io` namespace, applies this overlay, and waits for `deploy/qllm`. Skip those steps with `-SkipComposeDown` / `--skip-compose-down` and `-SkipBuild` / `--skip-build`.

## What you should see

- `describe_catalog` lists `vehicles`, `depots`, `gps_samples`, `shift_notes`, `stock_items`, `drivers` — never `customers` / `invoices`.
- `SELECT * FROM customers LIMIT 1` → `UNKNOWN_ENTITY` (isolation).
- Dynamo `stock_items` without `WHERE pk = ...` → `UNSUPPORTED`.

Connectors exercised here: postgres, clickhouse, sqlite, dynamodb (local), rest. **Not** in the default overlay: mssql, cassandra, ksql (Kafka). See `k8s/optional/`.

## Apply without Argo

Prefer `prd-tst-up`. Manual equivalent (harness binary via `Dockerfile.dev`; ConfigMap still mounts `deploy/prd-tst/config` over `/config`):

```bash
nerdctl --namespace k8s.io build -f Dockerfile.dev -t qllm:local .
kubectl apply -k deploy/prd-tst
kubectl -n qllm-prd rollout restart deploy/qllm
kubectl -n qllm-prd rollout status deploy/qllm --timeout=180s
```

HTTP and MCP together (one process, one shell):

```powershell
kubectl -n qllm-prd port-forward svc/qllm 18088:8088 18089:8089
```

All of this in **one** window (`Ctrl+C` stops every forward). Skips Argo if `argocd` is not installed:

```powershell
.\scripts\prd-tst-port-forward.ps1
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
| `http://127.0.0.1:18081` | Argo CD UI **after** `.\scripts\prd-tst-argocd-up.ps1` (plain HTTP) |

After adding ClickHouse HTTP on the Service, re-apply: `kubectl apply -k deploy/prd-tst`.

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

`kubectl apply -k deploy/prd-tst` does **not** install Argo. Stock `argocd-server` speaks **TLS** on the pod even if you forward Service port 80, so `http://127.0.0.1:18081` looks “dead” until `--insecure`.

```powershell
.\scripts\prd-tst-argocd-up.ps1
# Ctrl+C the old port-forward, then:
.\scripts\prd-tst-port-forward.ps1
```

Then register the app so it **appears in the UI** (this is a separate kubectl; kustomize does not create Applications):

```powershell
.\scripts\prd-tst-argocd-register-app.ps1
```

Refresh the Argo browser tab. You should see **qllm-prd-sim**. Do not use Git `HEAD` as revision.

**SSH “no key found”:** the key in git-gui lives on **your PC**. Argo CD runs **inside the cluster** and does not use `ssh-agent`. Register the OpenSSH **private** key as a Secret:

```powershell
.\scripts\prd-tst-argocd-add-ssh-repo.ps1
# or: .\scripts\prd-tst-argocd-add-ssh-repo.ps1 -KeyPath $env:USERPROFILE\.ssh\id_ed25519
```

Use the private key file, not `.pub`. PuTTY `.ppk` must be exported as OpenSSH in PuTTYgen. Then remove the broken repo entry in Argo **Settings → Repositories** (if you added SSH there without a key) and Refresh.

Alternatively connect **HTTPS + PAT** in Settings → Repositories (`https://github.com/jpandrade30/qLLM.git`) and keep `repoURL` as HTTPS — no SSH needed.

Password (`admin`):

```powershell
.\scripts\prd-tst-argocd-password.ps1
```

(`prd-tst-argocd-up.ps1` also prints it once.) If that Secret is gone, Argo was already reconfigured — reset with `argocd account update-password`.

Local-only without Argo: fleet-ops still works with `kubectl apply -k deploy/prd-tst`.

## MCP Inspector (Streamable HTTP)

URL: `http://127.0.0.1:18089/mcp` (keep `prd-tst-port-forward` running). Transport: Streamable HTTP. Prefer **Via Proxy**.

Custom header: name `Authorization`, value `Bearer fleet-prd-token`. **Enable the header toggle** — off means the token is not sent and the Inspector reports a generic proxy/token error.

CORS on this sim is empty; **Direct** from the browser can fail even with a valid token.

Watch agent SQL (needs an image rebuilt after this logging landed):

```powershell
kubectl logs -n qllm-prd deploy/qllm -f | Select-String "execute_sql|mcp_tool"
```

## Tear down PRD sim

Stop port-forward first (Ctrl+C), then:

```powershell
.\scripts\prd-tst-down.ps1
```

```bash
./scripts/prd-tst-down.sh
```

That deletes the overlay, Application `qllm-prd-sim` if present, and namespace `qllm-prd`. It does **not** uninstall Argo CD.

Then you can `nerdctl compose up --build` again for goldens.

## Image

`kustomization.yaml` expects `qllm:local` with `imagePullPolicy: Never`. Point Argo at a registry tag when you stop using a desktop cluster.
