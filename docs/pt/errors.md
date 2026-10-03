# Erros tipados

Formato da resposta: `{ "protocolVersion", "error": { "code", "message", "details"? } }`. A CLI imprime isso no **stderr**.

| `code` | Causa típica |
|--------|--------------|
| `INVALID_IR` | IR malformado ou falha na validação da consulta |
| `INVALID_SQL` | SQL recusado, ou erro de parse/DuckDB |
| `UNKNOWN_ENTITY` | Tabela ou entidade não existe no catálogo |
| `UNKNOWN_FIELD` | Campo lógico inexistente |
| `AMBIGUOUS_FIELD` | Campo sem qualificação com mais de uma entidade |
| `AMBIGUOUS_ALIAS` | Nomes `as` ou aliases em conflito |
| `LIMIT_EXCEEDED` | `limit` maior que `maxLimit` |
| `FORBIDDEN` | ACL: a entidade não está nas `tables` do app |
| `FORBIDDEN_SCOPE` | Credencial com escopo; filtro de outro sujeito, ou entidade escopada sem valor na chave |
| `UNAUTHORIZED` | Token Bearer ausente ou incorreto |
| `UNSUPPORTED` | Capacidade ausente na fonte (por exemplo, KV sem igualdade na chave) |
| `UNSUPPORTED_VERSION` | `version` de SQL desconhecida |
| `CONFIG_ERROR` | YAML ausente, bind inseguro, variável do token vazia, stdio sem `--app` |
| `TIMEOUT` | Orçamento de tempo ou timeout da fonte |
| `SOURCE_ERROR` | Falha de I/O na fonte |
| `NOT_READY` | O resultado assíncrono ainda não está pronto |
| `NOT_FOUND` | `queryId` desconhecido |
| `INTERNAL` | Bug ou falha local (por exemplo, ao abrir o DuckDB) |

Não invente códigos no cliente. O status HTTP acompanha o código (4xx vs 5xx), mas o contrato útil é o `code`.

No caminho feliz, `GET /v1/health` não usa este envelope de erro; ele retorna `{ "ok": true, "protocolVersion" }`.
