# HTTP e MCP

## HTTP `/v1`

Listener: `--addr` / `serve.addr` (default `127.0.0.1:8088`).

| Método | Path | Auth | Notas |
|--------|------|------|--------|
| GET | `/v1/health` | **não** | Probes |
| GET | `/v1/howtouseme` | sim* | Guia fechado para IR/SQL |
| GET | `/v1/catalog` | sim* | Catalog (filtrado com ACL) |
| POST | `/v1/queries` | sim* | Body = Query IR |
| POST | `/v1/sql` | sim* | `{ "sql", "version"? }` |
| GET | `/v1/queries/{id}` | sim* | Estado async |
| GET | `/v1/queries/{id}/result` | sim* | Resultado; `NOT_READY` se ainda não |

`*` Se `authTokenEnv` ou `qllm.access.yaml` estiverem activos. Sem isso, em loopback, aberto.

`POST` pode devolver **202** + `queryId` (async). O job **ainda** respeita ~15s. Store em memória ~2 minutos.

Logs: `http` (método/path), `http_execute_sql` (resumo), e bloco `---- execute_sql ----` no executor (SQL completo).

**Não** há CORS neste listener.

## MCP

Tools (só estas):

| Tool | Args | Efeito |
|------|------|--------|
| `how_to_use_me` | — | Mesmo espírito que `/v1/howtouseme` |
| `describe_catalog` | — | Catalog JSON (ACL) |
| `execute_sql` | `sql` (obrigatório), `version` opcional | Igual a `POST /v1/sql` |

Não há tool de Query IR.

### Stdio

```bash
./qllm serve --mcp --config-dir ./meu-projeto
```

Inspector local. Sem Bearer; ACL via `--app` / `QLLM_APP`.

### HTTP

```bash
./qllm serve --mcp-http --config-dir ./meu-projeto
```

| Path | Transporte |
|------|------------|
| `/mcp` | Streamable HTTP |
| `/sse` + `/message` | SSE (Inspector antigo) |

Auth: mesmo Bearer / ACL que `/v1`. CORS: só aqui (`serve.cors` / `--cors-origin`). Origins vazias = CORS off — o browser em **Direct** falha; usa **Via Proxy** no Inspector.

Logs: `---- mcp_tool ----` e o mesmo bloco `execute_sql`.

### Inspector (sim PRD)

URL `http://127.0.0.1:18089/mcp` (port-forward). Header `Authorization: Bearer …` com o **toggle ligado**. Ver [`deploy/prd-tst/README.md`](../deploy/prd-tst/README.md).

## Auth

1. **Nada** — só seguro em loopback (ou `--insecure-bind` / `insecureBind`).
2. **Um token** — `serve.authTokenEnv` aponta para uma env não vazia. `Authorization: Bearer <valor>`.
3. **Apps** — `qllm.access.yaml` presente: o token escolhe o app; `authTokenEnv` deixa de ser o modelo.

Compare Bearer: HMAC-SHA256 + `hmac.Equal` (não ramifica em `len`).

Bind `0.0.0.0` sem token e sem `insecureBind` → recusa no start.

## Cliente LangChain (MCP HTTP)

```python
from langchain_mcp_adapters.client import MultiServerMCPClient

client = MultiServerMCPClient({
    "qllm": {
        "transport": "streamable_http",
        "url": "http://127.0.0.1:8089/mcp",
        "headers": {"Authorization": "Bearer …"},
    }
})
```

Não inventes tools. Depois de `get_tools()`, o modelo deve seguir `how_to_use_me`.
