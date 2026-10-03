# `deploy/prd/default` — example project (YAML only)

The product [`Dockerfile`](../../../Dockerfile) copies this folder to `/config`. It does **not** start Postgres/ClickHouse/Argo.

Copy of [`deploy/image/config`](../../image/config) (project `qllm-demo`). Swap hosts/`*Env`/entities for yours.

| File | Role |
|------|------|
| `qllm.preset.yaml` | Sources (`type`, `connection.*Env`) and `limits` |
| `qllm.catalog.yaml` | Logical names the agent may `SELECT` / IR `from` |
| `qllm.config.yaml` | HTTP/MCP bind, `authTokenEnv`, CORS, caps |
| `qllm.env.yaml` | Fills **empty** env (`${VAR}` or literals). Process wins |
| `qllm.access.yaml` | App `demo-agent`; `key` = `${QLLM_AUTH_TOKEN}` |

```bash
docker build -t qllm .
docker run --rm -p 8088:8088 -p 8089:8089 \
  -e QLLM_AUTH_TOKEN=change-me \
  -e QLLM_CRM_PG_HOST=host.docker.internal \
  qllm
```

`api_profiles` is the REST entity with a nested `address` object, a `tags` list, and a `prefs` object (`shape` on each json field). Frozen rows: `fixtures/datasets/v1/api_profiles.json`. SQL goldens: `rest_json_*` in `fixtures/goldens/sql-v1`.

Validate without Docker: `./qllm validate --config-dir ./deploy/prd/default`
