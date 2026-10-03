# Enforced scope example

Shows three roles without mixing them: the **user** (login + question), the **customer backend** (mints the key, compiles LangGraph once), and **qLLM** (verifies the key and forces `user_id`).

This stack is **not** the harness. Compose project name is `qllm-enforced` so it does not recreate `qllm-postgres-1` / `qllm-qllm-1`. Ports are offset (18000 / 18088 / 18089 / 15432).

## What is enforced

| Role | What it does |
|------|----------------|
| User | Calls `POST /login` and `POST /ask`. Never sees `keySecret`. |
| Backend (`agent/`) | Mints `mobile.<user>.<expiry>.<hmac>`. Graph is compiled once; the token is `config["configurable"]["qllm_token"]`. Each MCP call opens a short session with that Bearer. |
| qLLM | One `apps[]` entry for `mobile` (not per user). `orders` has `scope: { field: user_id }`. `execute_sql` fetches only that user's rows. |

`products` is in `unscopedTables` (shared). App `admin` has a static key and no scope.

Catalog SQL always **injects** the scope on fetch (a spoof `WHERE user_id = '7'` returns empty, not `FORBIDDEN_SCOPE`). Query IR still uses `scopeMode: reject` by default.

## Run

From the repo root (Rancher Desktop + nerdctl, or Docker Compose):

```bash
nerdctl compose -f docker-compose.enforced.yml up --build
```

```bash
cd deploy/prd/enforced/agent
pip install -r requirements.txt
python client_demo.py
```

Expected:

- user 42: two orders (`Notebook`, `Pen`)
- user 7: one order (`Headphones`)
- products: three rows for both users
- spoof as 42 with `WHERE user_id = '7'`: no rows (fetch already scoped)

Direct qLLM (after login you can mint with the same helper):

```bash
curl -sS http://127.0.0.1:18088/v1/catalog \
  -H "Authorization: Bearer <derived-key>"
```

Secrets in compose (`change-me-mobile`, `change-me-admin`) are fake demo values.

## Layout

```text
deploy/prd/enforced/
  config/     qLLM YAML (baked by Dockerfile.enforced)
  db/init.sql seed
  agent/      FastAPI + LangGraph
```

Validate config without compose: `./qllm validate --config-dir ./deploy/prd/enforced/config` (needs the `QLLM_*` env vars set).
