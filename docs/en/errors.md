# Typed errors

Response shape: `{ "protocolVersion", "error": { "code", "message", "details"? } }`. The CLI prints this on **stderr**.

| `code` | Typical cause |
|--------|---------------|
| `INVALID_IR` | Malformed IR or query validation failure |
| `INVALID_SQL` | SQL refused, or a parse/DuckDB error |
| `UNKNOWN_ENTITY` | Table or entity is not in the catalog |
| `UNKNOWN_FIELD` | Logical field does not exist |
| `AMBIGUOUS_FIELD` | Unqualified field with more than one entity |
| `AMBIGUOUS_ALIAS` | Conflicting `as` names or aliases |
| `LIMIT_EXCEEDED` | `limit` is greater than `maxLimit` |
| `FORBIDDEN` | ACL: entity is not in the app's `tables` |
| `FORBIDDEN_SCOPE` | Credential is scoped; filter for another subject, or a scoped entity with no value on the key |
| `UNAUTHORIZED` | Bearer token missing or wrong |
| `UNSUPPORTED` | Source capability missing (for example KV without a key equality) |
| `UNSUPPORTED_VERSION` | Unknown SQL `version` |
| `CONFIG_ERROR` | Missing YAML, insecure bind, empty token env, stdio without `--app` |
| `TIMEOUT` | Budget or source timeout |
| `SOURCE_ERROR` | Source I/O failure |
| `NOT_READY` | Async result is not ready yet |
| `NOT_FOUND` | Unknown `queryId` |
| `INTERNAL` | Bug or local failure (for example opening DuckDB) |

Do not invent codes on the client. The HTTP status follows the code (4xx vs 5xx), but the useful contract is `code`.

On the happy path, `GET /v1/health` does not use this error envelope; it returns `{ "ok": true, "protocolVersion" }`.
