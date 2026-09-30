# Crear un proyecto qLLM desde cero

qLLM **no** adivina tus tablas. Tú escribes el YAML y el proceso lee **una sola carpeta**. Si esa carpeta no es la que crees, verás el catálogo del demo o un `CONFIG_ERROR`.

## 1. Qué vas a crear

Una carpeta **tuya** (no uses `fixtures/` como producto). Usa estos nombres exactos; el binario busca estos prefijos:

```text
C:\datos\mi-qllm\           (ejemplo)
  qllm.preset.yaml          OBLIGATORIO: bases de datos/APIs y límites
  qllm.catalog.yaml         OBLIGATORIO: nombres que el agente puede consultar con SELECT
  qllm.config.yaml          recomendado: puerto, token, CORS
  qllm.env.yaml             opcional: rellena variables de entorno vacías en tu PC
  qllm.access.yaml          opcional: varios agentes / listas de permitidos
```

Extensiones admitidas: `.yaml`, `.yml`, `.json`. No puedes usar `preset.yaml` sin el prefijo `qllm.`.

`protocolVersion` en los YAML: `"0.1.0"` o `"0.2.0"` (semver `N.N.N`). El runtime **responde** con `0.2.0`.

Todos los campos: [field-reference.md](field-reference.md). Carpeta de ejemplo (copia del demo, embebida por el `Dockerfile`): [`deploy/prd/`](../../deploy/prd). Docker y Kubernetes: [point-your-folder.md](point-your-folder.md).

## 2. Orden de trabajo

1. Lista tus fuentes reales (host, puerto, usuario, database **o** URI **o** URL).
2. Escribe el **preset**: un `sources[].id` por conexión. El `id` debe cumplir `[a-z][a-z0-9_]*` (por ejemplo `crm_pg`, no `CRM-PG`).
3. Crea las variables de entorno con los **nombres** que usaste en `hostEnv`, `passwordEnv`, etc. El YAML nunca contiene la contraseña; contiene el **nombre** de la variable (`QLLM_CRM_PG_PASSWORD`).
4. Escribe el **catálogo**: una entidad por tabla lógica. El `name` es lo que va en el `FROM`. El `source` es un `id` del preset. El `binding` es el schema y la tabla **físicos** (o collection/resource).
5. Ejecuta `qllm validate --config-dir …` hasta que muestre `ok` y `entities=N` con la **N que escribiste**.
6. Ejecuta `qllm serve --http --mcp-http --config-dir …`.
7. Confirma con health y catalog (sección 5). Sin esto no puedes saber si cargaste el YAML equivocado.

## 3. Ejemplo mínimo (un Postgres)

`qllm.preset.yaml`: toda clave de `connection` que termina en `Env` es el **nombre** de una variable, no su valor:

```yaml
protocolVersion: "0.2.0"
project: mi-empresa
limits:
  maxSyncMs: 15000
  maxSourceMs: 12000
  defaultLimit: 100
  maxLimit: 1000
  readOnly: true
sources:
  - id: crm_pg
    type: postgres
    connection:
      hostEnv: QLLM_CRM_PG_HOST
      port: 5432
      database: crm
      userEnv: QLLM_CRM_PG_USER
      passwordEnv: QLLM_CRM_PG_PASSWORD
      sslMode: disable
    options:
      statementTimeoutMs: 12000
```

En PowerShell, **antes** de levantar el servidor:

```powershell
$env:QLLM_CRM_PG_HOST = "127.0.0.1"
$env:QLLM_CRM_PG_USER = "app"
$env:QLLM_CRM_PG_PASSWORD = "secreto"
```

`qllm.catalog.yaml`: el agente **nunca** escribe `public.customers`; escribe `customers`:

```yaml
protocolVersion: "0.2.0"
project: mi-empresa
entities:
  - name: customers
    description: Clientes del CRM
    source: crm_pg
    binding:
      kind: table
      schema: public
      table: customers
    primaryKey: [id]
    fields:
      - name: id
        type: string
        physical: id
      - name: email
        type: string
        physical: email
        description: Correo electrónico único
```

`qllm.config.yaml`:

```yaml
serve:
  addr: "127.0.0.1:8088"
  mcpAddr: "127.0.0.1:8089"
  authTokenEnv: QLLM_AUTH_TOKEN
  insecureBind: false
  maxBodyBytes: 1048576
  maxRestResponseBytes: 10485760
  cors:
    origins: []
```

```powershell
$env:QLLM_AUTH_TOKEN = "un-token-largo"
```

Más fuentes, REST y Dynamo: [field-reference.md](field-reference.md).

## 4. Valida lo que escribiste (todavía sin servidor)

En la carpeta del **código fuente** de qLLM (donde está el binario), apunta a **tu** carpeta:

```powershell
cd C:\codes\qLLM
go build -tags duckdb -o qllm.exe .\cmd\qllm
.\qllm.exe validate --config-dir C:\datos\mi-qllm
```

Deberías ver:

```text
ok preset=C:\datos\mi-qllm\qllm.preset.yaml catalog=C:\datos\mi-qllm\qllm.catalog.yaml entities=1
```

- Las rutas deben ser de **tus** archivos (no de `deploy\image\config`).
- `entities=` es el número de entradas de `entities:` en el catálogo.
- JSON en stderr con `"code":"CONFIG_ERROR"` indica YAML ausente, un campo adicional (`additionalProperties: false`), un `id` inválido o `--preset` sin `--catalog`.

Si la validación pasa y tu catálogo tiene `customers`, un IR contra `invoices` debe fallar:

```powershell
# archivo tmp.json: { "from": "invoices", "select": ["id"], "limit": 1 }
.\qllm.exe validate --config-dir C:\datos\mi-qllm --ir C:\datos\tmp.json
```

Esperado: `UNKNOWN_ENTITY`. Si pasa, el `--config-dir` **no** es la carpeta que crees.

## 5. Levanta el servidor y comprueba que es **tu** proyecto

```powershell
.\qllm.exe serve --http --mcp-http --config-dir C:\datos\mi-qllm
```

Stderr debe mostrar:

```text
qllm http listening on 127.0.0.1:8088
qllm mcp-http listening on 127.0.0.1:8089 (/mcp streamable, /sse SSE)
```

En otra terminal:

```powershell
curl.exe -s -H "Authorization: Bearer un-token-largo" http://127.0.0.1:8088/v1/health
curl.exe -s -H "Authorization: Bearer un-token-largo" http://127.0.0.1:8088/v1/catalog
```

Health: `"ok":true` y `"protocolVersion":"0.2.0"`.

Catalog: `"project":"mi-empresa"` (la cadena de **tu** YAML) y `entities` con `customers`. Si ves `qllm-demo` con `customers` e `invoices` del harness, el proceso **no** está usando `C:\datos\mi-qllm`. Olvidaste el `--config-dir`, Docker montó otra carpeta o Kubernetes montó un ConfigMap antiguo.

SQL de prueba rápida (sobre **tu** tabla):

```powershell
curl.exe -s -H "Authorization: Bearer un-token-largo" -H "Content-Type: application/json" `
  -d "{\"sql\":\"SELECT id, email FROM customers LIMIT 5\"}" `
  http://127.0.0.1:8088/v1/sql
```

- `"status":"succeeded"` con filas: la base de datos conecta y el catálogo es correcto.
- `UNKNOWN_ENTITY`: el SQL usa un `name` que no está en el catálogo cargado.
- `SOURCE_ERROR` / `TIMEOUT`: el YAML está bien; la red, las credenciales o el host son incorrectos.
- `UNAUTHORIZED`: el token es distinto de `QLLM_AUTH_TOKEN` o el header está mal formado (`Bearer ` seguido de un espacio).

MCP: las mismas comprobaciones con las tools `describe_catalog` y `execute_sql`. El log del proceso muestra `---- execute_sql ----` con el SQL.

## 6. Genera un borrador en lugar de escribir el catálogo a mano

Con Postgres o MySQL ya en el preset y las variables de entorno definidas:

```powershell
.\qllm.exe catalog introspect --source crm_pg --config-dir C:\datos\mi-qllm --out C:\datos\mi-qllm\qllm.catalog.yaml
```

Abre el archivo y confirma `source`, `binding.schema`/`table` y `fields`. Añade `relations` y `description`. Después vuelve a ejecutar `validate`.

REST: `catalog from-openapi` y pega `options.resources` en el preset ([cli.md](cli.md)).

## 7. Qué **no** puedes poner en los YAML

- Campos que no están en el schema: validate y serve los rechazan (`additionalProperties: false` en preset, catalog, config, access, env y project).
- Una contraseña en texto plano en el preset (usa `passwordEnv`).
- `FROM public.customers` en el SQL del agente.
- `type: oracle` (no existe).
- CORS con `origins: ["*"]`.
- `qllm.access.yaml` con `tables: [customers]` cuando el catálogo no tiene esa entidad.

Sigue [field-reference.md](field-reference.md) campo por campo.
