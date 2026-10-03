# 类型化错误

响应格式：`{ "protocolVersion", "error": { "code", "message", "details"? } }`。CLI 会将其输出到 **stderr**。

| `code` | 典型原因 |
|--------|----------|
| `INVALID_IR` | IR 格式错误，或查询校验失败 |
| `INVALID_SQL` | SQL 被拒绝，或出现解析/DuckDB 错误 |
| `UNKNOWN_ENTITY` | 表或实体不在 catalog 中 |
| `UNKNOWN_FIELD` | 逻辑字段不存在 |
| `AMBIGUOUS_FIELD` | 涉及多个实体时，字段未加限定 |
| `AMBIGUOUS_ALIAS` | `as` 名称或别名冲突 |
| `LIMIT_EXCEEDED` | `limit` 大于 `maxLimit` |
| `FORBIDDEN` | ACL：该实体不在应用的 `tables` 中 |
| `FORBIDDEN_SCOPE` | 凭据带行范围；过滤了其他主体，或范围内实体在密钥上没有值 |
| `UNAUTHORIZED` | 缺少 Bearer 令牌或令牌错误 |
| `UNSUPPORTED` | 数据源缺少所需能力（例如 KV 没有键的等值条件） |
| `UNSUPPORTED_VERSION` | SQL `version` 未知 |
| `CONFIG_ERROR` | 缺少 YAML、不安全的绑定、令牌环境变量为空、stdio 缺少 `--app` |
| `TIMEOUT` | 预算或数据源超时 |
| `SOURCE_ERROR` | 数据源 I/O 失败 |
| `NOT_READY` | 异步结果尚未就绪 |
| `NOT_FOUND` | `queryId` 未知 |
| `INTERNAL` | 缺陷或本地故障（例如打开 DuckDB 失败） |

不要在客户端自行发明错误码。HTTP 状态码会跟随错误码（4xx 或 5xx），但真正有用的契约是 `code`。

在正常情况下，`GET /v1/health` 不使用此错误信封；它返回 `{ "ok": true, "protocolVersion" }`。
