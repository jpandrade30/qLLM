# Formato da resposta

HTTP `/v1/queries`, `/v1/sql`, CLI `query`/`sql` e MCP `execute_sql` devolvem o mesmo envelope. O contrato é [`planning/schemas/query-response.schema.json`](../../planning/schemas/query-response.schema.json) (D22).

O runtime **monta** o envelope a partir de structs Go. Ele não lê o schema ao servir. Os testes validam respostas de exemplo contra o schema.

## Envelope

| Campo | Quando | Significado |
|-------|--------|-------------|
| `protocolVersion` | sempre | Protocolo do runtime (`0.2.0`) |
| `queryId` | sempre | Id desta execução |
| `status` | sempre | `accepted` `running` `succeeded` `failed` `canceled` |
| `result` | sucesso | Tabela: `columns`, `rows`, `rowCount`, `truncated` |
| `meta` | em geral | `elapsedMs`, `mode` (`sync`/`async`), `app`, `plan` |
| `error` | falha | `code`, `message`, `details` opcional |

Não trate HTTP 200 como sucesso sem olhar `status` e `error`. Um POST pode devolver **202** com `status: accepted` e um `queryId`; o resultado fica na memória por cerca de dois minutos. Veja [http-mcp.md](http-mcp.md).

## Sucesso

`result.rows` é uma **lista de arrays**, na mesma ordem de `result.columns`. Não é uma lista de objetos.

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

`truncated: true` significa que o `limit` (ou o teto da fonte) cortou o resultado. Não há cursor de próxima página.

## Tipos lógicos → JSON

| `columns[].type` | Valor JSON | Notas |
|------------------|------------|-------|
| `string` | string | Use para ids maiores que 2^53. `number` vira DOUBLE no DuckDB e perde esses inteiros |
| `number` | número JSON | Agregações (`count`/`sum`/`avg`/`min`/`max`) são sempre `number` |
| `boolean` | true/false | |
| `timestamp` | string RFC3339 | UTC |
| `json` | **objeto ou array** (parseado) | Nunca uma string serializada quando a fonte era JSON válido |

Um campo `json` no catálogo pode ter `shape` (texto livre) para o LLM saber as chaves internas. O runtime não valida `shape`. REST lê **só chaves de topo**: `physical: addr.city` fica vazio no REST. Coloque o objeto em `addr` e deixe o cliente navegar.

## Erro

```json
{
  "protocolVersion": "0.2.0",
  "queryId": "…",
  "status": "failed",
  "error": {"code": "INVALID_IR", "message": "…"}
}
```

Códigos: [errors.md](errors.md).
