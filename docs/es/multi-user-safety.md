# Seguridad multi-usuario (claves con alcance)

El acceso por fila va en la **credencial**, no en un campo de `execute_sql` ni del Query IR. El modelo no escribe el código de usuario.

`qllm.access.yaml` tiene **una entrada por tipo de app**. Añadir el usuario 43 no cambia el YAML.

## Plantilla (por defecto)

```yaml
apps:
  - name: mobile
    keySecret: ${QLLM_MOBILE_SECRET}
    tables: [orders, profile, products]
    unscopedTables: [products]
    scope:
      field: user_id
```

El backend emite `mobile.42.<expiryUnix>.<hmac>` en `Authorization: Bearer`. qLLM verifica HMAC y caducidad y fuerza `user_id = '42'` en entidades con `scope`.

MCP stdio: `--app mobile` y `--scope 42` (o `QLLM_SCOPE`).

## Excepción estática

Unos pocos principals fijos pueden usar `key` + `scope: { user_id: "acme" }`.

Filtro de otro usuario en Query IR: `FORBIDDEN_SCOPE` (`scopeMode: reject`). En SQL de catálogo el alcance se **inyecta** en el fetch (resultado vacío). Una app sin `scope` (admin) no inyecta. Una entidad sin `scope` no está protegida; declara `unscopedTables`. Sigue siendo recomendable RLS en la base.

Grafo LangGraph: compile una vez; el Bearer va en `config["configurable"]["qllm_token"]`; cada `execute_sql` abre una sesión MCP corta. Ejemplo: [`deploy/prd/enforced/`](../../deploy/prd/enforced/README.md).

## Constraints en el request (D23)

Opcional en `execute_sql` / `POST /v1/sql`: `constraints` + `constraintMode` (`validate` | `inject`). Preferir fijar el mapa en el host. No sustituye el Bearer derivado (D21). Detalle: [en/multi-user-safety.md](../en/multi-user-safety.md).
