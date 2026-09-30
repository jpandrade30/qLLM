# HTTP e MCP

## HTTP `/v1`

Listener: `--addr` / `serve.addr` (padrão `127.0.0.1:8088`).

| Método | Caminho | Auth | Observações |
|--------|---------|------|-------------|
| GET | `/v1/health` | **não** | Probes |
| GET | `/v1/howtouseme` | sim* | Guia fechado para IR e SQL |
| GET | `/v1/catalog` | sim* | Catálogo (filtrado pela ACL) |
| POST | `/v1/queries` | sim* | O corpo é um Query IR |
| POST | `/v1/sql` | sim* | `{ "sql", "version"? }` |
| GET | `/v1/queries/{id}` | sim* | Status assíncrono |
| GET | `/v1/queries/{id}/result` | sim* | Resultado; `NOT_READY` se ainda não terminou |

`*` Quando `authTokenEnv` ou `qllm.access.yaml` está ativo. Caso contrário, no loopback, a API fica aberta.

Um `POST` pode retornar **202** com um `queryId` (assíncrono). O job continua respeitando o orçamento de ~15 s. O armazenamento em memória guarda os resultados por cerca de 2 minutos.

Logs: `http` (método/caminho), `http_execute_sql` (resumo) e o bloco `---- execute_sql ----` do executor (SQL completo).

Este listener **não** tem CORS.

## MCP

Tools (somente estas):

| Tool | Argumentos | Efeito |
|------|------------|--------|
| `how_to_use_me` | nenhum | Mesma finalidade de `/v1/howtouseme` |
| `describe_catalog` | nenhum | JSON do catálogo (com ACL aplicada) |
| `execute_sql` | `sql` (obrigatório), `version` opcional | Igual a `POST /v1/sql` |

Não existe tool de Query IR.

### Stdio

```bash
./qllm serve --mcp --config-dir ./meu-projeto
```

Inspector local. Sem Bearer; a ACL vem de `--app` / `QLLM_APP`.

### HTTP

```bash
./qllm serve --mcp-http --config-dir ./meu-projeto
```

| Caminho | Transporte |
|---------|------------|
| `/mcp` | Streamable HTTP |
| `/sse` + `/message` | SSE (Inspector mais antigo) |

Autenticação: o mesmo Bearer e a mesma ACL de `/v1`. O CORS vale somente aqui (`serve.cors` / `--cors-origin`). Origins vazias significam CORS desativado, então o navegador no modo **Direct** falha; use **Via Proxy** no Inspector.

Logs: `---- mcp_tool ----` e o mesmo bloco `execute_sql`.

### Inspector (simulação PRD)

URL `http://127.0.0.1:18089/mcp` (port-forward). Envie `Authorization: Bearer …` com o **toggle do header ligado**. Veja [`deploy/prd-tst/README.md`](../../deploy/prd-tst/README.md).

## Autenticação

1. **Nenhuma**: segura apenas no loopback (ou com `--insecure-bind` / `insecureBind`).
2. **Um token**: `serve.authTokenEnv` aponta para uma variável de ambiente não vazia. Envie `Authorization: Bearer <valor>`.
3. **Apps**: quando `qllm.access.yaml` existe, o token seleciona o app e o `authTokenEnv` deixa de ser o modelo.

A comparação do Bearer usa HMAC-SHA256 e `hmac.Equal`, portanto não ramifica pelo `len`.

Um bind em `0.0.0.0` sem token e sem `insecureBind` é recusado na inicialização.

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

Não invente tools. Depois de `get_tools()`, o modelo deve seguir o `how_to_use_me`.
