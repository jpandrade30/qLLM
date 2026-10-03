# Errores tipados

Formato de la respuesta: `{ "protocolVersion", "error": { "code", "message", "details"? } }`. La CLI lo imprime por **stderr**.

| `code` | Causa típica |
|--------|--------------|
| `INVALID_IR` | IR mal formado o fallo en la validación de la consulta |
| `INVALID_SQL` | SQL rechazado, o error de parseo/DuckDB |
| `UNKNOWN_ENTITY` | La tabla o entidad no está en el catálogo |
| `UNKNOWN_FIELD` | El campo lógico no existe |
| `AMBIGUOUS_FIELD` | Campo sin calificar con más de una entidad |
| `AMBIGUOUS_ALIAS` | Nombres `as` o alias en conflicto |
| `LIMIT_EXCEEDED` | `limit` mayor que `maxLimit` |
| `FORBIDDEN` | ACL: la entidad no está en las `tables` de la app |
| `FORBIDDEN_SCOPE` | Credencial con alcance; filtro de otro sujeto, o entidad acotada sin valor en la clave |
| `UNAUTHORIZED` | Token Bearer ausente o incorrecto |
| `UNSUPPORTED` | Falta una capacidad de la fuente (por ejemplo, KV sin igualdad en la clave) |
| `UNSUPPORTED_VERSION` | `version` de SQL desconocida |
| `CONFIG_ERROR` | YAML ausente, bind inseguro, variable del token vacía, stdio sin `--app` |
| `TIMEOUT` | Presupuesto de tiempo o timeout de la fuente |
| `SOURCE_ERROR` | Fallo de E/S en la fuente |
| `NOT_READY` | El resultado asíncrono todavía no está listo |
| `NOT_FOUND` | `queryId` desconocido |
| `INTERNAL` | Error interno o fallo local (por ejemplo, al abrir DuckDB) |

No inventes códigos en el cliente. El estado HTTP sigue al código (4xx vs 5xx), pero el contrato útil es `code`.

En el caso normal, `GET /v1/health` no usa este envoltorio de error; devuelve `{ "ok": true, "protocolVersion" }`.
