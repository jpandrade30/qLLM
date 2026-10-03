# scripts/

Operator helpers, grouped by job. Paths below are from the repo root.

| Folder | What it is |
|--------|------------|
| [`dev/`](dev/) | Local CGO/DuckDB shell, harness seed, DuckDB smoke |
| [`prd-tst/`](prd-tst/) | Kubernetes fleet-ops sim (up, down, port-forward, optional Argo CD) |
| [`standalone/`](standalone/) | Slim copy of the runtime for GitHub/GitLab |

```powershell
.\scripts\dev\dev-shell.ps1
.\scripts\dev\dev-seed-fake.ps1
.\scripts\dev\check-live.ps1
.\scripts\dev\check-live.ps1 -Filter rest_json
.\scripts\prd-tst\prd-tst-up.ps1
python .\scripts\standalone\init-standalone.py --user Alice --out ..
```

```bash
source ./scripts/dev/dev-shell.sh
./scripts/dev/dev-seed-fake.sh
./scripts/dev/check-live.sh
./scripts/dev/check-live.sh --filter rest_json
./scripts/prd-tst/prd-tst-up.sh
./scripts/standalone/init-standalone.sh --user Alice --out ..
```
