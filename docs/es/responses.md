# Formato de la respuesta

HTTP `/v1/queries`, `/v1/sql`, CLI `query`/`sql` y MCP `execute_sql` devuelven el mismo sobre. El contrato es [`planning/schemas/query-response.schema.json`](../../planning/schemas/query-response.schema.json) (D22).

El runtime **arma** el sobre desde structs de Go. No lee el schema al servir. Los tests validan respuestas de ejemplo contra el schema.

## Sobre

| Campo | Cuándo | Significado |
|-------|--------|-------------|
| `protocolVersion` | siempre | Protocolo del runtime (`0.2.0`) |
| `queryId` | siempre | Id de esta ejecución |
| `status` | siempre | `accepted` `running` `succeeded` `failed` `canceled` |
| `result` | éxito | Tabla: `columns`, `rows`, `rowCount`, `truncated` |
| `meta` | casi siempre | `elapsedMs`, `mode` (`sync`/`async`), `app`, `plan` |
| `error` | fallo | `code`, `message`, `details` opcional |

No trates HTTP 200 como éxito sin mirar `status` y `error`. Un POST puede devolver **202** con `status: accepted` y un `queryId`; el resultado queda en memoria unos dos minutos. Ver [http-mcp.md](http-mcp.md).

## Éxito

`result.rows` es una **lista de arrays**, en el mismo orden que `result.columns`. No es una lista de objetos.

```json
{
  "protocolVersion": "0.2.0",
  "queryId": "…",
  "status": "succeeded",
  "result": {
    "columns": [
      {"name": "id", "type": "string"},
      {"name": "price", "type": "number"},
      {"name": "addr", "type": "json"},
      {"name": "tags", "type": "json"}
    ],
    "rows": [
      ["1", 12.5, {"city": "SP"}, ["a", "b"]]
    ],
    "rowCount": 1,
    "truncated": false
  },
  "meta": {
    "elapsedMs": 12,
    "mode": "sync",
    "plan": {"usedDuckDB": true, "steps": [{"source": "shop", "pushdown": false, "elapsedMs": 4}]}
  }
}
```

`truncated: true` significa que el `limit` (o el tope de la fuente) cortó el resultado. No hay cursor de página siguiente.

## Tipos lógicos → JSON

| `columns[].type` | Valor JSON | Notas |
|------------------|------------|-------|
| `string` | string | Úsalo para ids mayores que 2^53. `number` se materializa como DOUBLE en DuckDB y pierde esos enteros |
| `number` | número JSON | Las agregaciones (`count`/`sum`/`avg`/`min`/`max`) son siempre `number` |
| `boolean` | true/false | |
| `timestamp` | string RFC3339 | UTC |
| `json` | **objeto o array** (parseado) | Nunca un string serializado si la fuente era JSON válido |

Un campo `json` del catálogo puede llevar `shape` (texto libre) para que el LLM conozca las claves internas. El runtime no valida `shape`. REST solo lee claves de **primer nivel**: `physical: addr.city` queda vacío en REST. Pon el objeto en `addr` y que el cliente lo recorra.

## Error

```json
{
  "protocolVersion": "0.2.0",
  "queryId": "…",
  "status": "failed",
  "error": {"code": "INVALID_IR", "message": "…"}
}
```

Códigos: [errors.md](errors.md).
