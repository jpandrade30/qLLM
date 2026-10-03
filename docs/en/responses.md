# Query response format

HTTP `/v1/queries`, `/v1/sql`, CLI `query`/`sql`, and MCP `execute_sql` all return the same envelope. The contract is [`planning/schemas/query-response.schema.json`](../../planning/schemas/query-response.schema.json) (D22).

The runtime **builds** the envelope from Go structs. It does not read the schema at serve time. Tests validate sample responses against the schema.

## Envelope

| Field | When | Meaning |
|-------|------|---------|
| `protocolVersion` | always | Runtime protocol (`0.2.0`) |
| `queryId` | always | Id of this run |
| `status` | always | `accepted` `running` `succeeded` `failed` `canceled` |
| `result` | success | Table: `columns`, `rows`, `rowCount`, `truncated` |
| `meta` | usually | `elapsedMs`, `mode` (`sync`/`async`), `app`, `plan` |
| `error` | failure | `code`, `message`, optional `details` |

Do not treat HTTP 200 as success without checking `status` and `error`. A POST may return **202** with `status: accepted` and a `queryId`; the in-memory store keeps the result for about two minutes. See [http-mcp.md](http-mcp.md).

## Success

`result.rows` is a **list of arrays**, in the same order as `result.columns`. It is not a list of objects.

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

`truncated: true` means the `limit` (or the source cap) cut the result. There is no next-page cursor.

## Logical types → JSON

| `columns[].type` | JSON value | Notes |
|------------------|------------|-------|
| `string` | string | Use this for ids larger than 2^53. `number` is stored as DOUBLE in DuckDB and loses those integers |
| `number` | JSON number | Aggregations (`count`/`sum`/`avg`/`min`/`max`) are always `number` |
| `boolean` | true/false | |
| `timestamp` | string RFC3339 | UTC |
| `json` | **object or array** (parsed) | Never a serialized string when the source was valid JSON |

A `json` field in the catalog may set `shape` (free text) so the LLM knows the inner keys. The runtime does not validate `shape`. REST still reads **only top-level** keys: `physical: addr.city` is empty on REST. Put the object on `addr` and let the client walk it.

Harness example: entity `api_profiles` (`address` / `tags` / `prefs`). SQL goldens `rest_json_*` in `fixtures/goldens/sql-v1`. Dataset: `fixtures/datasets/v1/api_profiles.json`. After seed, `GET /profiles` on the fake API. Compare with `python -m sqlcheck check-oracle` (or `mcp` against a running compose).

## Error

```json
{
  "protocolVersion": "0.2.0",
  "queryId": "…",
  "status": "failed",
  "error": {"code": "INVALID_IR", "message": "…"}
}
```

Codes: [errors.md](errors.md).
