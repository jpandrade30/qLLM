# CLI (`qllm`)

Binario: `go build -o qllm ./cmd/qllm`. Producción y la imagen usan `-tags duckdb`; ver [build.md](build.md) e [install.md](install.md).

```text
qllm
├── validate                 comprueba preset + catálogo (+ IR opcional), sin abrir fuentes
├── query                    ejecuta un archivo Query IR
├── sql                      ejecuta un archivo de SQL de catálogo
├── serve                    HTTP /v1 y/o MCP
├── catalog
│   ├── introspect           borrador de catálogo desde una fuente postgres/mysql
│   └── from-openapi         borrador de entidades rest_resource desde un OpenAPI 3
├── help [comando]           ayuda integrada
└── completion               scripts de autocompletado del shell (integrado)
```

`qllm <comando> --help` muestra las flags de cualquier comando. Los nombres de abajo son exactamente los del binario: por ejemplo, la flag de bind es `--insecure-bind` (no `--bind-insecure`).

El código de salida es `0` si todo va bien y `1` ante cualquier error. Los errores tipados salen por **stderr** en JSON (`{"protocolVersion": …, "error": {…}}`); ver [errors.md](errors.md).

## Cómo se encuentran los archivos

`validate`, `query`, `sql`, `serve` y los dos comandos `catalog` comparten estas cuatro flags.

| Flag | Función |
|------|---------|
| `--config-dir DIR` | Carpeta donde se buscan `qllm.preset`, `qllm.catalog` y los opcionales `qllm.config`, `qllm.access`, `qllm.env`. Por defecto: el directorio actual |
| `--preset ARCH` / `--catalog ARCH` | Rutas explícitas. **Ambas** son obligatorias juntas; indicar solo una es `CONFIG_ERROR` |
| `--project ARCH` | Un `qllm.project.yaml` con `preset:` y `catalog:` relativos a ese archivo. Se rechazan rutas que salgan de su carpeta |

Precedencia: `--preset` + `--catalog`, luego `--project`, luego `--config-dir` (o el CWD). En una carpeta cada archivo se busca como `.yaml`, `.yml` y `.json`, en ese orden. Un preset o catálogo ausente es `CONFIG_ERROR`.

Los archivos opcionales (`qllm.config.*`, `qllm.access.*`, `qllm.env.*`) solo se buscan en `--config-dir` (o en el CWD). `--preset`, `--catalog` y `--project` no cambian esa búsqueda.

### Archivo de entorno (`qllm.env.yaml`)

`query`, `sql`, `serve` y los dos `catalog` aplican `qllm.env.yaml` antes de conectar. **`validate` no lo aplica**, porque no abre fuentes. Reglas:

- Solo se definen las variables **vacías o ausentes** en el proceso; el entorno real siempre gana.
- El valor es un literal o exactamente `${OTRO_NOMBRE}`, leído del proceso. Un `${OTRO_NOMBRE}` ausente se omite, no se escribe como texto. Cualquier otro uso de `${` es `CONFIG_ERROR`.

### Variables de entorno que lee el propio binario

| Variable | La usan | Significado |
|----------|---------|-------------|
| `QLLM_APP` | `query`, `sql`, `serve` | Igual que `--app` (la flag gana) |
| `QLLM_SCOPE` | `query`, `sql`, `serve` | Igual que `--scope` (la flag gana) |
| el nombre en `serve.authTokenEnv` | `serve` | Contiene el Bearer token compartido |
| cada clave `*Env` del preset | quien abre fuentes | Secretos de conexión, p. ej. `QLLM_CRM_PG_PASSWORD` |

## `qllm validate`

Valida preset y catálogo sin abrir ninguna fuente.

| Flag | Significado |
|------|-------------|
| flags de config | ver arriba |
| `--ir ARCH` | Valida también este Query IR contra el catálogo |

```bash
./qllm validate --config-dir ./my-project
./qllm validate --config-dir ./my-project --ir ./query.json
```

Si todo va bien, stderr muestra `ok preset=… catalog=… entities=N` (y `ok ir=…`). Los fallos son JSON tipado en stderr.

## `qllm query`

Ejecuta un archivo Query IR e imprime la respuesta JSON en stdout. Abre las fuentes.

| Flag | Significado |
|------|-------------|
| flags de config | ver arriba |
| `-f`, `--file ARCH` | **Obligatoria.** IR en JSON o YAML |
| `--app NOMBRE` | App de `qllm.access.yaml` (o `QLLM_APP`) |
| `--scope VALOR` | Valor del alcance por fila para una app plantilla (o `QLLM_SCOPE`) |

Cuando existe `qllm.access.yaml`, las tablas se comprueban contra la app. Una app plantilla (`keySecret`) también exige `--scope`. Este comando **no** lee `qllm.config.yaml`, así que las respuestas REST usan el límite por defecto de 10 MiB.

```bash
./qllm query --config-dir ./my-project -f ./query.json
```

## `qllm sql`

Ejecuta un archivo de SQL de catálogo e imprime la respuesta JSON. Requiere un build con DuckDB embebido (`-tags duckdb`).

| Flag | Significado |
|------|-------------|
| flags de config | ver arriba |
| `-f`, `--file ARCH` | **Obligatoria.** Archivo de texto con el SQL |
| `--version V` | Dialecto SQL. Omitida = el más reciente (`"2"`). `"1"` está congelado (sin operaciones de conjunto, sin `QUALIFY`) |
| `--app`, `--scope` | Igual que en `query` |

```bash
./qllm sql --config-dir ./my-project -f ./q.sql
./qllm sql --config-dir ./my-project -f ./q.sql --version 1
```

## `qllm serve`

Inicia uno o más listeners. Sin ninguna de `--http`, `--mcp`, `--mcp-http`, el **HTTP queda activado**.

### Modos

| Flag | Efecto |
|------|--------|
| `--http` | REST `/v1` (`howtouseme`, `catalog`, `queries`, `sql`, `health`) |
| `--mcp-http` | MCP Streamable HTTP en `/mcp`, SSE en `/sse` y `/message` |
| `--mcp` | MCP por **stdio** para clientes locales (Inspector). Exclusiva: combinarla con `--http` o `--mcp-http` es un error. No abre listener de red, así que las reglas de bind y token de abajo no se aplican |

`--http` y `--mcp-http` pueden correr juntos en un mismo proceso.

### Flags de escucha y seguridad

| Flag | Por defecto | Significado |
|------|-------------|-------------|
| `--addr HOST:PUERTO` | `127.0.0.1:8088` | Dirección del HTTP `/v1` |
| `--mcp-addr HOST:PUERTO` | `127.0.0.1:8089` | Dirección del MCP HTTP |
| `--runtime-config ARCH` | `qllm.config.*` en `--config-dir` | Archivo de runtime explícito |
| `--auth-token-env NOMBRE` | ninguno | Nombre de la variable con el Bearer token compartido. Si se define, esa variable **debe estar no vacía** o `serve` falla con `CONFIG_ERROR` |
| `--insecure-bind` | `false` | Permite una dirección **no loopback** **sin** auth. Ver abajo |
| `--cors-origin ORIGEN` | ninguno (CORS apagado) | Origen de navegador permitido en MCP HTTP. Repetible. `*` se rechaza |
| `--app NOMBRE` | ninguno | App para MCP stdio cuando existe `qllm.access.yaml` (o `QLLM_APP`) |
| `--scope VALOR` | ninguno | Alcance por fila de una app plantilla en stdio (o `QLLM_SCOPE`) |

### Precedencia

Valores por defecto integrados → `qllm.config.yaml` → flags. Una flag solo cuenta si realmente la pasas; así `--insecure-bind=false` puede anular `insecureBind: true` del archivo, y `--cors-origin` reemplaza los orígenes del archivo.

### La regla de bind y `--insecure-bind`

qLLM se niega a escuchar en una dirección no loopback salvo que se cumpla una de estas condiciones:

1. Hay un token configurado (`serve.authTokenEnv` / `--auth-token-env`, no vacío), **o**
2. Existe `qllm.access.yaml` (sus claves son la autenticación), **o**
3. `--insecure-bind` / `serve.insecureBind: true` está activo.

Loopback significa `127.0.0.1`, `::1` o `localhost`. **No** son loopback y activan la regla: `0.0.0.0:8088`, `[::]:8088`, `:8088`, cualquier IP de LAN y cualquier hostname distinto de `localhost`. Los contenedores lo necesitan porque deben escuchar en `0.0.0.0`.

`--insecure-bind` **no** apaga la autenticación. Solo quita el rechazo al arrancar. Si hay token o archivo de acceso, las peticiones se siguen comprobando. Sirve para una red de confianza que proteges de otra forma (red privada de compose, service mesh, proxy inverso que autentica). Sin ninguna auth, quien alcance el puerto consulta todo lo que expone el catálogo. La comprobación se hace por listener, para `--http` y para `--mcp-http`.

Sin token y con dirección loopback, la API queda abierta a procesos locales. Es la postura por defecto.

### Opciones solo en `qllm.config.yaml`

No tienen flag. Schema: [planning/schemas/runtime-config.schema.json](../../planning/schemas/runtime-config.schema.json).

| Clave | Por defecto | Significado |
|-------|-------------|-------------|
| `serve.maxBodyBytes` | 1048576 (1 MiB) | Cuerpo máximo de la petición, HTTP y MCP HTTP |
| `serve.maxRestResponseBytes` | 10485760 (10 MiB) | Cuerpo máximo leído de una fuente `rest` |
| `serve.cors.allowHeaders` / `allowMethods` | integrados | Reemplazan las listas de CORS |

### Ejemplos

```bash
./qllm serve --http --mcp-http --config-dir ./my-project
./qllm serve --http --addr 0.0.0.0:8088 --auth-token-env QLLM_AUTH_TOKEN --config-dir ./my-project
./qllm serve --mcp --config-dir ./my-project --app crm-agent
./qllm serve --mcp --config-dir ./my-project --app crm-agent --scope 42
```

Stdio con `qllm.access.yaml` y sin `--app` / `QLLM_APP` devuelve `CONFIG_ERROR`. Una app plantilla también exige `--scope` / `QLLM_SCOPE`.

Logs: cada `execute_sql` imprime un bloque `---- execute_sql ----` en stderr; las otras dos tools imprimen `---- mcp_tool ----`.

## `qllm catalog introspect`

Lee `information_schema` de una fuente **postgres o mysql** del preset y escribe el YAML del catálogo. No sirve.

| Flag | Significado |
|------|-------------|
| flags de config | ver arriba |
| `--source ID` | **Obligatoria.** `sources[].id` |
| `--out ARCH` | Archivo de salida (por defecto: stdout) |
| `--merge` | Conserva las entidades de otras fuentes del catálogo ya cargado y reemplaza solo las de esta fuente |

Caduca a los 15 segundos. Revisa el YAML (relaciones, descripciones) antes de `serve`.

```bash
./qllm catalog introspect --source crm_pg --config-dir ./my-project --out ./my-project/qllm.catalog.yaml
```

## `qllm catalog from-openapi`

Genera entidades `rest_resource` y un fragmento `options.resources` desde un OpenAPI 3. `--source` debe ser una fuente `type: rest` del preset.

| Flag | Significado |
|------|-------------|
| flags de config | ver arriba |
| `-f`, `--file ARCH` | **Obligatoria.** OpenAPI 3 en YAML o JSON |
| `--source ID` | **Obligatoria.** Id de la fuente REST |
| `--out ARCH` | Salida del catálogo (por defecto: stdout) |
| `--resources-out ARCH` | Escribe el fragmento de resources (si no, sale por stderr) |
| `--merge` | Igual que en `introspect` |

El conector REST lee **solo** lo que ya está en el preset. Pega el fragmento en `sources[].options.resources`.
