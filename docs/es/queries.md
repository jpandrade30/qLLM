# Consultas: SQL de catálogo y Query IR

Hay dos rutas. MCP tiene **solo** SQL. HTTP tiene ambas.

Schemas: [`sql-request.schema.json`](../../planning/schemas/sql-request.schema.json), [`query-ir.schema.json`](../../planning/schemas/query-ir.schema.json), [`query-response.schema.json`](../../planning/schemas/query-response.schema.json). Dialecto: [`planning/07-sql-dialect.md`](../../planning/07-sql-dialect.md).

## SQL de catálogo (`execute_sql` / `POST /v1/sql` / `qllm sql`)

Cuerpo: `{ "sql": "SELECT …", "version": "1"|"2" }`. Un `version` omitido significa **`"2"`**. Un valor desconocido devuelve `UNSUPPORTED_VERSION`.

### Aceptado (resumen)

- Una única sentencia `SELECT` (opcionalmente con `WITH`).
- Las tablas son **nombres de entidades** (más alias de CTE, que no son entidades).
- `LIMIT` es obligatorio o se inyecta (`defaultLimit`). Un `OFFSET` aislado también inyecta un limit.
- Dialecto `"1"`: `WHERE`, `HAVING`, `DISTINCT`, `CASE`, `LIKE`, `ILIKE`, `BETWEEN`, `IN`, `IS NULL`, CTE, subconsultas en `FROM`, joins, agregaciones básicas, funciones de cadena/número/cast y funciones de JSON/array/datetime **con los nombres de DuckDB**.
- Dialecto `"2"`: todo lo de `"1"` más `UNION` / `UNION ALL` / `INTERSECT` / `EXCEPT`, `QUALIFY`, funciones de ventana (`ROW_NUMBER`, …) y `XOR`.

Las funciones de Databricks con otro nombre (`GET_JSON_OBJECT`, `DATEADD`, …) no se rechazan si DuckDB las tiene; la guía pide la grafía de DuckDB.

### Rechazado (`INVALID_SQL` o equivalente)

- `INSERT` `UPDATE` `DELETE` `MERGE` `REPLACE`
- `CREATE` `DROP` `ALTER` `TRUNCATE` `COPY` `ATTACH` `DETACH`
- `INSTALL` `LOAD` `PRAGMA` `SET` `CALL` `GRANT`
- `read_csv` `read_parquet` `read_json` `postgres_scan` `httpfs` `glob` `read_text` `read_blob`
- Varias sentencias (`;`)
- `schema.tabla` (`public.customers`)
- Funciones de tabla arbitrarias
- `LIMIT` mayor que `maxLimit` devuelve `LIMIT_EXCEEDED`
- Una entidad o campo inexistente devuelve `UNKNOWN_ENTITY` / `UNKNOWN_FIELD`
- Una entidad fuera de la ACL devuelve `FORBIDDEN`

### Ejecución (importante si modelas el planner mentalmente)

1. Parseo y, después, validación de nombres y ACL.
2. **Obtención** de las tablas referenciadas (solo las columnas necesarias). En esta ruta **no** hay pushdown de `WHERE`.
3. DuckDB ejecuta el `SELECT` (`enable_external_access=false`).
4. Una compilación **sin** `-tags duckdb` no puede ejecutar esta ruta.

En fuentes KV y de stream, el `WHERE` debe incluir una igualdad sobre el `accessPath`, o la obtención falla con `UNSUPPORTED`. El SQL "parece" válido, pero la fuente lo rechaza.

## Query IR (`POST /v1/queries` / `qllm query`)

JSON con `additionalProperties: false`. Obligatorios: `from`, `select`.

### Aceptado

- `from` y el `from` de los joins: un nombre de entidad o alias del catálogo (`[a-z][a-z0-9_]*`).
- `as` en la consulta: un alias local único.
- `select`: referencias de field (`entity.field` o `alias.field`) **o** `{ "agg": "count|sum|avg|min|max", "field"?, "as" }`. `count` puede omitir `field`.
- `joins[]`: solo `inner` y `left`; `on` usa pares `{left, right}`.
- `where`: `{ "op", "args" }` para `and` / `or` / `not`; las comparaciones usan `{ "field", "op", "value"? }`.
- `op` de comparación: `eq` `neq` `gt` `gte` `lt` `lte` `in` `nin` `contains` `is_null` `not_null`. **No** uses `=`, `LIKE` ni `{and:[…]}` en lugar de `op` con `args`.
- `groupBy` cuando mezclas columnas agregadas y no agregadas.
- `orderBy`: `{ "field", "dir": "asc"|"desc" }`.
- `limit` ≥ 1 (o el valor por defecto del preset); `offset` ≥ 0.
- `mode`: `sync` o `async`.
- Con más de una entidad, califica los fields o recibirás `AMBIGUOUS_FIELD`.

### Rechazado o inexistente en el IR

- Joins `full` y `cross` en el IR (el SQL sobre DuckDB puede aceptarlos en la ruta SQL).
- `having`, `union`, `case`, `like` y `xor` en el IR; usa SQL.
- Mutaciones.
- Inventar entidades o claves de join.

`GET /v1/howtouseme` describe `never`, las formas y `invalidExamples`. El agente debe leerlo **antes** de inventar un IR.

## Respuesta

Un envoltorio con `protocolVersion`, `queryId`, `status` (`succeeded` / `failed` / `accepted`), un `result` tabular, `meta` (`elapsedMs`, `app`, `plan.usedDuckDB`, steps) o un `error` tipado. Ejemplos y tabla de tipos: [responses.md](responses.md).

No trates HTTP 200 como éxito sin comprobar `status` y `error`.
