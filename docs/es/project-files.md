# Archivos del proyecto

Tutorial desde cero y comprobación de que cargaste la carpeta correcta: [from-scratch.md](from-scratch.md). Tablas campo por campo: [field-reference.md](field-reference.md). Dónde apuntar en un despliegue: [point-your-folder.md](point-your-folder.md).

Un proyecto qLLM es un directorio con archivos YAML o JSON. Sin ellos, `serve` **no** recurre a `fixtures/` ni a valores por defecto del demo.

## Descubrimiento

En orden (mutuamente excluyentes en la práctica):

1. `--preset` **y** `--catalog` (ambos; solo uno devuelve `CONFIG_ERROR`).
2. `--project`: un `qllm.project.yaml` cuyas rutas `preset` y `catalog` son relativas a él (no pueden salir del directorio del archivo de proyecto).
3. `--config-dir DIR`: `DIR/qllm.preset.{yaml|yml|json}` más `DIR/qllm.catalog.{yaml|yml|json}`.
4. Sin `--config-dir`: el **directorio de trabajo actual**.

Archivos opcionales en el mismo directorio (o en una ruta explícita cuando existe la flag):

| Archivo | Obligatorio | Función |
|---------|-------------|---------|
| `qllm.preset.*` | sí | Fuentes, `limits`, `connection.*Env` |
| `qllm.catalog.*` | sí | Entidades lógicas, fields, relations, `binding` |
| `qllm.config.*` | no | Bind, `authTokenEnv`, CORS (MCP HTTP), límites |
| `qllm.access.*` | no | Apps, keys, `tables`; **sustituye** al token Bearer único |
| `qllm.env.*` | no | Rellena variables de entorno cuando la variable del proceso está vacía |
| `qllm.project.yaml` | no | Puntero a `preset` y `catalog` |

Schemas: [`planning/schemas/`](../../planning/schemas/). Texto explicativo: [`planning/03-protocol-schemas.md`](../../planning/03-protocol-schemas.md).

## Precedencia de serve

Valores seguros por defecto (loopback, CORS desactivado), luego `qllm.config.yaml`, luego las flags de la CLI.

`qllm.env.yaml`: prevalece un valor **no vacío** del proceso o de un Secret. El valor puede ser literal o exactamente `${NOMBRE}`. No crees placeholders vacíos. **No registres en logs** estos valores.

## Preset: qué debes rellenar

Obligatorios: `protocolVersion`, `project`, `limits`, `sources[]` (`id`, `type`, `connection`).

`limits` (valores por defecto de la especificación): `maxSyncMs` 15000, `maxSourceMs` 12000, `defaultLimit` 100, `maxLimit` 1000, `readOnly` true.

`sources[].id`: `[a-z][a-z0-9_]*`. `type`: consulta [connectors.md](connectors.md).

Secretos solo mediante `*Env` (o una variable de entorno con URI). Nunca versiones contraseñas en el preset.

Ejemplos de `connection` por tipo: sección 1 de `03-protocol-schemas.md`.

## Catálogo: qué ve el agente

- `entities[].name` (y `aliases`) son los nombres de tabla en SQL y el `from` del IR.
- `fields[].name` son las columnas lógicas. `physical` se asigna a la columna o clave real del documento.
- `binding` apunta al objeto físico (`table` / `collection` / `rest_resource`, más `accessPath` para fuentes KV y de stream).
- `relations` documentan joins; el SQL o el IR aún debe citarlos correctamente.

El mismo campo físico en 10 APIs significa **10 entidades** (`crm_users` vs `erp_users`), no un único `users` compartido.

## Access (`qllm.access.yaml`)

```yaml
apps:
  - name: crm-agent
    key: ${QLLM_CRM_AGENT_KEY}
    tables: [customers, invoices]
```

- `tables` son nombres de entidades del catálogo.
- HTTP y MCP HTTP: `Authorization: Bearer <key>`.
- MCP stdio, `qllm query` y `qllm sql`: `--app` o `QLLM_APP` (el nombre de la app, no la key).
- Archivo presente: el catálogo y `howtouseme` se filtran; una entidad fuera de la lista devuelve `FORBIDDEN`.
- Archivo ausente: un único token (`authTokenEnv`) o ninguna autenticación en loopback; se expone el catálogo completo.

## Runtime (`qllm.config.yaml`)

Los campos están definidos en [`runtime-config.schema.json`](../../planning/schemas/runtime-config.schema.json). `additionalProperties: false`.

| Campo | Valor por defecto / regla |
|-------|---------------------------|
| `serve.addr` | `127.0.0.1:8088` |
| `serve.mcpAddr` | `127.0.0.1:8089` |
| `serve.authTokenEnv` | Nombre de la variable del Bearer; si se define el nombre, esa variable **debe** tener valor |
| `serve.insecureBind` | `false`; un bind fuera de loopback sin autenticación exige `true` o `--insecure-bind` |
| `serve.maxBodyBytes` | Límite del cuerpo del POST (mínimo del schema: 1024) |
| `serve.maxRestResponseBytes` | Límite del conector REST |
| `serve.cors.origins` | Vacío significa CORS desactivado; `*` se rechaza |

## Estructura mínima para implementar

```text
mi-proyecto/
  qllm.preset.yaml
  qllm.catalog.yaml
  qllm.config.yaml      # recomendado en cualquier exposición
  qllm.access.yaml      # si hay más de un agente
  qllm.env.yaml         # solo local; en K8s usa un Secret
```

```bash
export QLLM_…   # todo lo que el preset referencia
./qllm validate --config-dir ./mi-proyecto
./qllm serve --http --mcp-http --config-dir ./mi-proyecto
```
