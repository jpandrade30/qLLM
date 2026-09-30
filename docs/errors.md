# Erros tipados

Resposta: `{ "protocolVersion", "error": { "code", "message", "details"? } }`. CLI imprime isto no **stderr**.

| `code` | Quando (típico) |
|--------|------------------|
| `INVALID_IR` | IR malformado / validação de query |
| `INVALID_SQL` | SQL recusado ou parse/DuckDB |
| `UNKNOWN_ENTITY` | Tabela/entidade não está no catalog |
| `UNKNOWN_FIELD` | Campo lógico inexistente |
| `AMBIGUOUS_FIELD` | Campo sem qualificar com >1 entidade |
| `AMBIGUOUS_ALIAS` | `as` / aliases em conflito |
| `LIMIT_EXCEEDED` | `limit` > `maxLimit` |
| `FORBIDDEN` | ACL: entidade fora de `tables` |
| `UNAUTHORIZED` | Bearer em falta ou errado |
| `UNSUPPORTED` | Capacidade da fonte (ex. KV sem key eq) |
| `UNSUPPORTED_VERSION` | `version` SQL desconhecido |
| `CONFIG_ERROR` | YAML em falta, bind inseguro, env do token vazia, stdio sem `--app` |
| `TIMEOUT` | Budget / fonte |
| `SOURCE_ERROR` | Falha I/O da fonte |
| `NOT_READY` | Resultado async ainda não |
| `NOT_FOUND` | `queryId` desconhecido |
| `INTERNAL` | Bug / falha local (ex. abrir DuckDB) |

Não inventes códigos no cliente. HTTP status segue o código (4xx vs 5xx); o contrato útil é o `code`.

`GET /v1/health` não usa este envelope de erro para o happy path (`{ "ok": true, "protocolVersion" }`).
