# HTTP y MCP

## HTTP `/v1`

Listener: `--addr` / `serve.addr` (por defecto `127.0.0.1:8088`).

| Método | Ruta | Auth | Notas |
|--------|------|------|-------|
| GET | `/v1/health` | **no** | Probes |
| GET | `/v1/howtouseme` | sí* | Guía cerrada para IR y SQL |
| GET | `/v1/catalog` | sí* | Catálogo (filtrado por ACL) |
| POST | `/v1/queries` | sí* | El cuerpo es un Query IR |
| POST | `/v1/sql` | sí* | `{ "sql", "version"?, "constraints"?, "constraintMode"? }` |
| GET | `/v1/queries/{id}` | sí* | Estado asíncrono |
| GET | `/v1/queries/{id}/result` | sí* | Resultado; `NOT_READY` si aún no terminó |

`*` Cuando `authTokenEnv` o `qllm.access.yaml` está activo. En caso contrario, en loopback, la API queda abierta.

Un `POST` puede devolver **202** con un `queryId` (asíncrono). El job sigue respetando el presupuesto de ~15 s. El almacén en memoria conserva los resultados unos 2 minutos.

Logs: `http` (método/ruta), `http_execute_sql` (resumen) y el bloque `---- execute_sql ----` del ejecutor (SQL completo).

Este listener **no** tiene CORS.

## MCP

Tools (solo estas):

| Tool | Argumentos | Efecto |
|------|------------|--------|
| `how_to_use_me` | ninguno | Mismo propósito que `/v1/howtouseme` |
| `describe_catalog` | ninguno | JSON del catálogo (con ACL aplicada) |
| `execute_sql` | `sql` (obligatorio), `version` opcional, `constraints` / `constraintMode` opcional (D23) | Igual que `POST /v1/sql` |

No existe ninguna tool de Query IR.

### Stdio

```bash
./qllm serve --mcp --config-dir ./mi-proyecto
```

Inspector local. Sin Bearer; la ACL proviene de `--app` / `QLLM_APP`.

### HTTP

```bash
./qllm serve --mcp-http --config-dir ./mi-proyecto
```

| Ruta | Transporte |
|------|------------|
| `/mcp` | Streamable HTTP |
| `/sse` + `/message` | SSE (Inspector antiguo) |

Autenticación: el mismo Bearer y la misma ACL que `/v1`. El CORS solo se aplica aquí (`serve.cors` / `--cors-origin`). Orígenes vacíos significan CORS desactivado, por lo que el navegador en modo **Direct** falla; usa **Via Proxy** en el Inspector.

Logs: `---- mcp_tool ----` y el mismo bloque `execute_sql`.

### Inspector (simulación PRD)

URL `http://127.0.0.1:18089/mcp` (port-forward). Envía `Authorization: Bearer …` con el **interruptor del header activado**. Consulta [`deploy/prd-tst/README.md`](../../deploy/prd-tst/README.md).

## Autenticación

1. **Ninguna**: segura solo en loopback (o con `--insecure-bind` / `insecureBind`).
2. **Un token**: `serve.authTokenEnv` apunta a una variable de entorno no vacía. Envía `Authorization: Bearer <valor>`.
3. **Apps**: cuando existe `qllm.access.yaml`, el token selecciona la app y `authTokenEnv` deja de ser el modelo.

La comparación del Bearer usa HMAC-SHA256 y `hmac.Equal`, por lo que no se ramifica según `len`.

Un bind en `0.0.0.0` sin token y sin `insecureBind` se rechaza al arrancar.

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

No inventes tools. Tras `get_tools()`, el modelo debe seguir `how_to_use_me`.
