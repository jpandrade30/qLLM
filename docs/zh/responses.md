# 查询响应格式

HTTP `/v1/queries`、`/v1/sql`、CLI `query`/`sql` 和 MCP `execute_sql` 都返回同一套信封。契约见 [`planning/schemas/query-response.schema.json`](../../planning/schemas/query-response.schema.json)（D22）。

运行时用 Go 结构体**组装**信封，服务时不读取该 schema。测试会用示例响应对照 schema 校验。

## 信封

| 字段 | 何时出现 | 含义 |
|------|----------|------|
| `protocolVersion` | 始终 | 运行时协议（`0.2.0`） |
| `queryId` | 始终 | 本次运行的 id |
| `status` | 始终 | `accepted` `running` `succeeded` `failed` `canceled` |
| `result` | 成功 | 表格：`columns`、`rows`、`rowCount`、`truncated` |
| `meta` | 通常 | `elapsedMs`、`mode`（`sync`/`async`）、`app`、`plan` |
| `error` | 失败 | `code`、`message`、可选 `details` |

不要仅凭 HTTP 200 判断成功，必须看 `status` 和 `error`。POST 可能返回 **202** 且 `status: accepted` 带 `queryId`；结果在内存中保留约两分钟。见 [http-mcp.md](http-mcp.md)。

## 成功

`result.rows` 是**数组的列表**，顺序与 `result.columns` 一致，不是对象列表。

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

`truncated: true` 表示 `limit`（或数据源上限）截断了结果。没有下一页游标。

## 逻辑类型 → JSON

| `columns[].type` | JSON 值 | 说明 |
|------------------|---------|------|
| `string` | 字符串 | 大于 2^53 的 id 必须用 string。`number` 在 DuckDB 中按 DOUBLE 物化，会丢失这些整数 |
| `number` | JSON 数字 | 聚合（`count`/`sum`/`avg`/`min`/`max`）一律为 `number` |
| `boolean` | true/false | |
| `timestamp` | RFC3339 字符串 | UTC |
| `json` | **对象或数组**（已解析） | 源数据是合法 JSON 时，不会再是序列化字符串 |

目录中的 `json` 字段可设 `shape`（自由文本），供 LLM 了解内部键。运行时不校验 `shape`。REST **只读顶层键**：`physical: addr.city` 在 REST 上为空。把对象放在 `addr` 上，由客户端再取子字段。

## 错误

```json
{
  "protocolVersion": "0.2.0",
  "queryId": "…",
  "status": "failed",
  "error": {"code": "INVALID_IR", "message": "…"}
}
```

错误码见 [errors.md](errors.md)。
