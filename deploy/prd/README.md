# `deploy/prd` — product example stacks

Each subfolder is a complete `--config-dir`. Do not mix their YAML.

| Folder | What it is | Image |
|--------|------------|-------|
| [`default/`](default/) | Demo catalog (Postgres, MySQL, Mongo, REST). Baked by the product `Dockerfile` | `docker build -t qllm .` |
| [`enforced/`](enforced/) | Scoped keys (D21) + LangGraph agent. Own compose | `Dockerfile.enforced` + `docker-compose.enforced.yml` |

Harness / goldens: [`deploy/image/config`](../image/config) + [`Dockerfile.dev`](../../Dockerfile.dev). Kubernetes sim: [`deploy/prd-tst`](../prd-tst).

REST profiles with a nested `address` object (and goldens): entity `api_profiles` in `default/` and in the harness catalog. Dataset: `fixtures/datasets/v1/api_profiles.json`. Cases: `fixtures/goldens/sql-v1` ids `rest_json_*`.
