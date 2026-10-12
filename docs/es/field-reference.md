# Referencia de campos (todo lo que acepta el schema)

Fuente: [`planning/schemas/`](../../planning/schemas/). Los objetos con `additionalProperties: false` **rechazan** claves adicionales.

Convención `*Env`: la cadena es el **nombre** de una variable de entorno (`QLLM_FOO`), nunca el secreto en sí.

Identificadores lógicos (`sources[].id`, `entities[].name`, alias, `name` de field, `from`/`as` del IR): `^[a-z][a-z0-9_]*$`.

---

## `qllm.project.yaml` (opcional)

Obligatorios: `protocolVersion`, `preset`, `catalog`.

| Campo | Tipo | Notas |
|-------|------|-------|
| `protocolVersion` | No version | `0.1.0` / `0.2.0` |
| `preset` | cadena | Ruta relativa a **este** archivo |
| `catalog` | cadena | Ídem |

---

## `qllm.preset.yaml`

Obligatorios en la raíz: `protocolVersion`, `project`, `limits`, `sources` (mínimo 1).

| Campo | Tipo | Notas |
|-------|------|-------|
| `protocolVersion` | No version | |
| `project` | cadena no vacía | Nombre lógico del proyecto; debe coincidir con el del catálogo |
| `limits` | object | Los 5 campos son **obligatorios** |
| `sources` | array | |

### `limits` (todos obligatorios)

| Campo | Tipo | Rango |
|-------|------|-------|
| `maxSyncMs` | int | 100–60000; presupuesto total de la consulta |
| `maxSourceMs` | int | 100–60000; por llamada a la fuente |
| `defaultLimit` | int | ≥ 1; se usa cuando el IR/SQL omite LIMIT |
| `maxLimit` | int | ≥ 1; tope máximo |
| `readOnly` | bool | debe ser `true` en el producto |

### `sources[]` (cada elemento)

Obligatorios: `id`, `type`, `connection`.

| Campo | Tipo | Valores |
|-------|------|---------|
| `id` | cadena | `crm_pg`, `legacy_api`, … |
| `type` | enum | `postgres` `mysql` `mongodb` `rest` `mssql` `sqlite` `clickhouse` `dynamodb` `cassandra` `ksql` `redis` `kafka` `graphql` más alias de cable MySQL (`mariadb` `tidb` `vitess` `aurora_mysql` `planetscale`) y Postgres (`cockroach` `yugabyte` `alloydb` `aurora_postgres` `neon` `supabase` `timescale` `redshift`) |
| `connection` | object | La forma depende del `type` (abajo). Las claves adicionales son un error |
| `options` | object | Libre en el JSON schema; el runtime lee solo lo que conoce (abajo) |

### `connection` según `type`

**postgres**, **mysql** y sus alias de cable (`sqlConnection`): obligatorios `hostEnv`, `port`, `database`, `userEnv`, `passwordEnv`.

| Campo | Tipo | Notas |
|-------|------|-------|
| `hostEnv` | cadena | |
| `port` | int | 1–65535 |
| `database` | cadena | |
| `userEnv` | cadena | |
| `passwordEnv` | cadena | |
| `sslMode` | enum opcional | `disable` `require` `verify-ca` `verify-full` (postgres; mysql lo ignora si no se usa) |
| `maxOpenConns` | int opcional | `1`–`100`; tope del pool `database/sql` (por defecto **5**). Con muchos pods, bájalo por réplica (`pods × maxOpenConns ≲ max_connections` del DB) |
| `maxOpenConnsEnv` | string opcional | Nombre de la env con un entero; si está definida, gana a `maxOpenConns`. Vacía o no numérica → `CONFIG_ERROR` al abrir la source |

**mssql**: los mismos obligatorios. Extra opcional: `encrypt`: `true` \| `false` \| `disable`. Mismos `maxOpenConns` / `maxOpenConnsEnv` opcionales. No tiene `sslMode`.

**clickhouse**: los mismos obligatorios. Extra opcional: `secure` (bool). Mismos `maxOpenConns` / `maxOpenConnsEnv` opcionales.

**sqlite**: `pathEnv` (la variable de entorno con la ruta del archivo `.db`). Se aceptan `maxOpenConns` / `maxOpenConnsEnv`, pero el tope efectivo es siempre **1**.

**mongodb**: obligatorios `uriEnv`, `database`.

**rest** y **ksql**: obligatorio `baseUrlEnv`. `auth` es opcional (abajo).

**dynamodb**: obligatorio `region` (valor literal, por ejemplo `us-east-1`). Opcional `endpointEnv` (Dynamo Local). Las credenciales de AWS provienen de la cadena de credenciales del proceso, no de campos del YAML.

**cassandra**: el schema exige `keyspace`. En la práctica, el código usa `hostsEnv` (una lista) **o** `hostEnv`. Opcionales: `port`, `userEnv`, `passwordEnv`.

**redis**: `addrEnv` o `hostEnv`+`port`. Opcionales: `db`, `userEnv`, `passwordEnv`, `tls`, `readReplica`.

**kafka**: obligatorio `brokersEnv`. Opcionales: `tls`, `sasl` (`none`/`plain`/`scram`), `userEnv`, `passwordEnv`. Options: `timeoutMs`, `maxRecords`, `maxScanRecords`.

### `connection.auth` (REST / ksql)

Obligatorio: `type`.

| `type` | Campos relevantes |
|--------|-------------------|
| `none` | ninguno |
| `bearer` | `tokenEnv` |
| `header` | `name` (nombre del header), `valueEnv` |
| `basic` | `userEnv`, `passwordEnv` |

### `options` que usa el código Go (no están enumeradas en el schema)

| Clave | Se aplica a | Por defecto | Significado |
|-------|-------------|-------------|-------------|
| `statementTimeoutMs` | postgres, mysql, alias, mssql, clickhouse, sqlite | `limits.maxSourceMs` | Timeout del statement. Vale el **menor** entre `statementTimeoutMs`, `timeoutMs` y `maxSourceMs` |
| `timeoutMs` | SQL (misma regla), REST, ksql | REST 10000, ksql 12000 | Timeout del cliente HTTP en REST y ksql. En SQL es un segundo tope, como `statementTimeoutMs` |
| `resources` | REST, **obligatorio** para consultar | ninguno | Mapa de nombre de recurso a operaciones `list` / `getById` (ver abajo) |

Los demás tipos (mongodb, dynamodb, cassandra) no leen ninguna clave de `options` hoy. Las claves desconocidas las acepta el schema y el runtime las ignora, así que un error de tipeo pasa en silencio.

Ejemplo de `options.resources` para REST (el `binding.resource` del catálogo debe ser una clave de este mapa, por ejemplo `users`):

```yaml
options:
  timeoutMs: 10000
  resources:
    users:
      list:
        method: GET
        path: /users
        queryParams: [email, limit, offset]
      getById:
        method: GET
        path: /users/{id}
```

`from-openapi` genera este mapa. Sin `resources`, el conector REST falla con `CONFIG_ERROR`.

### `resources` de REST en detalle

Cada clave de `resources` es el nombre de un recurso. Una entidad del catálogo apunta a él con `binding: { kind: rest_resource, resource: <nombre> }`. Un recurso tiene hasta dos operaciones, con la misma forma.

| Operación | Obligatoria | Para qué sirve |
|-----------|-------------|----------------|
| `list` | sí* | Se usa cuando `getById` no puede ejecutarse (faltan path params). Obligatoria si alguna consulta no es por id |
| `getById` | no | Se usa cuando cada `{nombre}` de `path` tiene un filtro `eq`. `from-openapi` la genera a partir de rutas con `{id}` |

Campos de cada operación (`list` y `getById`):

| Campo | Tipo | Por defecto | Significado |
|-------|------|-------------|-------------|
| `method` | string | `GET` | Método HTTP. Con `limits.readOnly: true` solo se aceptan `GET` y `HEAD`; otro método falla al iniciar con `CONFIG_ERROR` |
| `path` | string | ninguno | Se concatena a la URL base de `baseUrlEnv` (se quita la `/` final de la base). Empieza con `/`. `{nombre}` se sustituye con filtros `eq` en `getById` |
| `queryParams` | lista de strings | ninguno | Parámetros de query que acepta la API. Documenta qué filtros existen; `from-openapi` lo rellena. El runtime **no** lo valida |
| `itemsKey` | string | `data`/`items`/`results`/`users` (list) o `data`/`item`/`result` (getById) | Clave JSON del array o del objeto. También vale en el recurso |
| `maxPages` | int | 1 | Páginas por offset (`list`). Tope 20 |
| `pageSize` | int | el `limit` de la consulta | Tamaño de página enviado en el parámetro de limit cuando `maxPages` > 1 |
| `limitParam` | string | `limit` | Nombre del parámetro de tamaño de página |
| `offsetParam` | string | `offset` | Nombre del parámetro de offset |

Ejemplo de `getById`: `WHERE id = '42'` con `path: /users/{id}` se convierte en `GET /users/42`. Los demás `eq` quedan como query params. Si falta un placeholder, el runtime usa `list`.

```sql
SELECT id, email FROM users WHERE id = '42' LIMIT 1
```

Cómo una consulta se convierte en petición HTTP:

- **Columnas:** cada campo seleccionado se lee del elemento de la respuesta por su nombre `physical`. Solo se leen claves de primer nivel; un `physical` con punto, como `addr.city`, no devuelve nada en REST. Un campo con `fromFilter: true` se rellena desde un `eq` de primer nivel (o `and` de `eq`) si el cuerpo lo omite; sin ese `eq` la consulta devuelve `INVALID_IR`; un valor distinto en el cuerpo devuelve `SOURCE_ERROR`.
- **`WHERE`:** solo se envía `eq` (y `eq` combinados con `and`), como `?<campo>=<valor>` (o path param en `getById`). El nombre del parámetro es el nombre **lógico** del campo; mantén iguales el nombre lógico y el físico en los campos que filtras. Otro operador (`neq`, `gt`, `in`, `contains`, …) no se empuja al conector REST y allí devuelve `UNSUPPORTED`.
- **`LIMIT` / `OFFSET`:** se envían como `limitParam` / `offsetParam` (por defecto `limit` y `offset`).
- **Paginación:** `maxPages: 1` (por defecto) es una petición. Valores mayores recorren el offset hasta una página corta, el límite de filas o 20 páginas.
- **Agregaciones:** nunca se empujan; se ejecutan en DuckDB.
- **Forma de la respuesta:** un array JSON, o un objeto cuya `itemsKey` (o las claves por defecto) contiene el array. `getById` también acepta un objeto suelto.
- **Errores:** un estado HTTP 400 o mayor devuelve `SOURCE_ERROR`; una respuesta mayor que `serve.maxRestResponseBytes` se rechaza; superar el timeout devuelve `TIMEOUT`.

Ejemplo completo con las dos operaciones:

```yaml
sources:
  - id: legacy_api
    type: rest
    connection:
      baseUrlEnv: QLLM_LEGACY_API_URL
      auth:
        type: bearer
        tokenEnv: QLLM_LEGACY_API_TOKEN
    options:
      timeoutMs: 8000
      resources:
        users:
          list:
            method: GET
            path: /users
            queryParams: [id, email, status]
          getById:
            method: GET
            path: /users/{id}
```

---

## `qllm.catalog.yaml`

Obligatorios: `protocolVersion`, `project`, `entities` (mínimo 1).

### `entities[]`

Obligatorios: `name`, `source`, `binding`, `fields` (mínimo 1 field).

| Campo | Tipo | Notas |
|-------|------|-------|
| `name` | id lógico | `FROM name` / `from` del IR |
| `aliases` | array de ids, únicos | Nombres alternativos en el IR |
| `description` | cadena | Texto para el agente |
| `source` | id | **Debe** existir en `preset.sources[].id` |
| `binding` | object | Mapeo físico |
| `primaryKey` | array de cadenas | Nombres lógicos de field |
| `fields` | array | |
| `relations` | array | Solo orientativas; no crean claves foráneas |
| `scope` | `{ field, column? }` | D21: fuerza `eq` en `column` (o `field`) desde la credencial |

### `binding`

Obligatorio: `kind`.

| `kind` | También obligatorio | Uso |
|--------|---------------------|-----|
| `table` | `schema`, `table` | postgres/mysql/mssql/sqlite (`schema: main`)/clickhouse/dynamodb/cassandra/ksql |
| `collection` | `collection` | mongodb |
| `rest_resource` | `resource` | rest; una clave de `options.resources` |
| `graphql_operation` | `resource` | graphql; una clave de `options.operations` |
| `key` | `keyPattern`, `accessPath.partition` | redis (`user:{id}`) |
| `topic` | `topic`, `accessPath` (partition / `key` / timestamp) | kafka |

`schema` / `table` / `collection` / `resource`: `^[A-Za-z_][A-Za-z0-9_]*$`.

`accessPath` (object, claves adicionales prohibidas) para Dynamo, Cassandra y ksql:

| Campo | Tipo | Uso habitual |
|-------|------|--------------|
| `pk` / `partition` | array de cadenas | Nombres **lógicos** de field (igualdad obligatoria en la consulta) |
| `sk` / `sort` | cadena | Sort key de Dynamo |
| `ksqlKey` | cadena | Pull query de ksql |

Sin la igualdad correcta en la consulta, el resultado es `UNSUPPORTED`.

### `fields[]`

Obligatorios: `name`, `type`, `physical`.

| Campo | Valores |
|-------|---------|
| `name` | id lógico (`email`) |
| `type` | `string` `number` `boolean` `timestamp` `json`. `number` es DOUBLE en DuckDB: los ids > 2^53 deben ser `string` |
| `physical` | columna o clave; se permiten rutas con puntos (`addr.city`) excepto en REST (solo claves de primer nivel) |
| `description` | cadena opcional |
| `fromFilter` | bool opcional; solo REST. La API no devuelve el campo; qLLM copia el valor de un `eq` de primer nivel (D20) |
| `shape` | texto libre opcional; solo `type: json`. Estructura interna para el LLM (`{street, city}`, `string[]`). Sale en `describe_catalog`. No se valida (D22) |

### `relations[]`

Obligatorios: `name`, `to`, `type`, `on`.

| Campo | Valores |
|-------|---------|
| `name` | etiqueta (`customer`) |
| `to` | `entities[].name` de destino |
| `type` | `many_to_one` `one_to_many` `one_to_one` |
| `on` | array de pares `[campo_local, campo_remoto]`, mínimo 1 par |

---

## `qllm.config.yaml` (serve)

La raíz es opcional y solo admite la clave `serve`. Todo lo que hay dentro de `serve` es opcional, pero si defines `authTokenEnv`, esa variable de entorno debe existir y no estar vacía.

| Campo | Tipo | Valor por defecto del runtime |
|-------|------|-------------------------------|
| `addr` | cadena | `127.0.0.1:8088` |
| `mcpAddr` | cadena | `127.0.0.1:8089` |
| `authTokenEnv` | cadena | ninguno (sin Bearer) |
| `insecureBind` | bool | `false` |
| `maxBodyBytes` | int ≥ 1024 | 1048576 (1 MiB) |
| `maxRestResponseBytes` | int ≥ 1024 | 10485760 (10 MiB) |
| `cors.origins` | array de cadenas | `[]` = CORS desactivado; sin `*` |
| `cors.allowHeaders` | array | |
| `cors.allowMethods` | array | |

Flags de la CLI que **prevalecen** sobre el archivo: `--addr`, `--mcp-addr`, `--auth-token-env`, `--insecure-bind`, `--cors-origin`.

---

## `qllm.access.yaml`

Obligatorio: `apps` (mínimo 1). Cada app necesita `name`, `tables` y exactamente uno de `key` o `keySecret`. Una entrada por **tipo** de app, no por usuario.

| Campo | Notas |
|-------|-------|
| `scopeMode` | En el archivo: `reject` (por defecto) o `inject` |
| `name` | ID de la app (`--app` / `QLLM_APP`). Con `keySecret`, `[a-z][a-z0-9_-]*` |
| `key` | Bearer estático; literal **o** exactamente `${ENV_NAME}` |
| `keySecret` | Verifica claves derivadas `app.scopeValue.expiry.hmac` |
| `scope` | Plantilla `{ field: user_id }` o estática `{ user_id: "acme" }` |
| `tables` | `entities[].name` permitidos, o `*` |
| `unscopedTables` | Tablas compartidas exigidas cuando la app tiene `scope` |

---

## `qllm.env.yaml`

Obligatorio: `env` (object con al menos 1 clave).

| | |
|--|--|
| nombres de las claves | `^[A-Za-z_][A-Za-z0-9_]*$` |
| valores | cadena ≥ 1; literal o `${OTRA_ENV}` |
| ya definida en el proceso | **no** se sobrescribe |

No uses este archivo para secretos versionados en git. En Kubernetes, usa un Secret.

---

## Cuerpo de `POST /v1/sql` / `execute_sql`

Obligatorio: `sql`. Opcional: `version` (`"1"` congelado; omitido o `"2"` es el más reciente).

Opcional (D23): `constraints` (objeto campo→string/número/boolean) y `constraintMode` (`validate` | `inject`). Por defecto con mapa no vacío: `validate`. Preferir mapa fijado en el host; ver [multi-user-safety.md](multi-user-safety.md).

---

## Query IR (campos)

Obligatorios: `from`, `select`. Consulta [queries.md](queries.md) y [`query-ir.schema.json`](../../planning/schemas/query-ir.schema.json).

| Campo | Notas |
|-------|-------|
| `protocolVersion` | opcional en la petición |
| `from` | entidad o alias |
| `as` | alias local |
| `joins[]` | `type`: `inner`\|`left`; `from`; `as?`; `on[]` `{left,right}` |
| `select[]` | cadena de field **o** `{agg, field?, as}`; `agg`: `count` `sum` `avg` `min` `max` |
| `where` | `{op,args}` o comparación `{field,op,value?}` |
| `groupBy` | referencias de field |
| `orderBy[]` | `{field, dir?}` con `asc`\|`desc` |
| `limit` / `offset` | enteros |
| `mode` | `sync` \| `async` |

`op` de comparación: `eq` `neq` `gt` `gte` `lt` `lte` `in` `nin` `contains` `is_null` `not_null`.

`op` lógico: `and` `or`. `not` recibe exactamente un elemento en `args`.

---

## Flags de la CLI (resumen)

Descritos en [cli.md](cli.md). El catálogo no tiene "tags" en YAML aparte de `protocolVersion` y los enums `type`/`kind` de arriba. El tag de **compilación** `-tags duckdb` pertenece a Go ([build.md](build.md)), no a un archivo de qllm.
