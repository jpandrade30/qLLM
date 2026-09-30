# CLI (`qllm`)

Binario: `go build -o qllm ./cmd/qllm`. Producción y la imagen usan `-tags duckdb`; consulta [build.md](build.md).

Flags comunes de configuración (casi todos los subcomandos):

| Flag | Uso |
|------|-----|
| `--config-dir` | Carpeta con `qllm.preset` y `qllm.catalog` |
| `--preset` / `--catalog` | Rutas explícitas (ambas son obligatorias juntas) |
| `--project` | Ruta de `qllm.project.yaml` |

## `qllm validate`

Valida el preset y el catálogo sin abrir ninguna fuente. El `--ir FILE` opcional valida un Query IR contra el catálogo.

```bash
./qllm validate --config-dir ./mi-proyecto
./qllm validate --config-dir ./mi-proyecto --ir ./consulta.json
```

Si todo va bien, stderr muestra `ok preset=… catalog=… entities=N`. Los errores salen como JSON tipado por stderr.

## `qllm query`

Ejecuta un archivo de Query IR. `--file` / `-f` es obligatorio. Abre las fuentes.

Usa `--app` o `QLLM_APP` cuando exista `qllm.access.yaml`.

Aplica el `qllm.env.yaml` del directorio de configuración antes de conectar.

```bash
./qllm query --config-dir ./mi-proyecto -f ./consulta.json
```

## `qllm sql`

Ejecuta un archivo de texto con SQL de catálogo. `--file` / `-f` es obligatorio.

Si omites `--version`, se usa el dialecto más reciente (`"2"`). `"1"` es el dialecto congelado (sin operaciones de conjunto ni `QUALIFY`).

`--app` / `QLLM_APP` aplica las ACL.

Requiere una compilación con DuckDB embebido para `ExecSQL`.

```bash
./qllm sql --config-dir ./mi-proyecto -f ./q.sql
./qllm sql --config-dir ./mi-proyecto -f ./q.sql --version 1
```

## `qllm serve`

Si no se indica ninguno de `--http`, `--mcp` o `--mcp-http`, **HTTP queda activado** por defecto.

| Flag | Efecto |
|------|--------|
| `--http` | REST `/v1` |
| `--mcp-http` | MCP en `/mcp`, además de `/sse` y `/message` |
| `--mcp` | MCP por **stdio** (Inspector local). **Exclusivo**: no lo combines con `--http` ni `--mcp-http` |
| `--addr` | Dirección de escucha de HTTP (por defecto `127.0.0.1:8088`) |
| `--mcp-addr` | Dirección de escucha de MCP HTTP (por defecto `127.0.0.1:8089`) |
| `--runtime-config` | Ruta de `qllm.config.yaml` |
| `--auth-token-env` | Nombre de la variable de entorno que contiene el token Bearer |
| `--insecure-bind` | Permite un bind fuera de loopback sin autenticación |
| `--cors-origin` | Repetible; lista de orígenes permitidos de MCP HTTP |
| `--app` | Nombre de la app en stdio cuando existe `qllm.access.yaml` |

```bash
./qllm serve --http --mcp-http --config-dir ./mi-proyecto
./qllm serve --mcp --config-dir ./mi-proyecto --app crm-agent
```

Stdio con `qllm.access.yaml` y sin `--app` / `QLLM_APP` devuelve `CONFIG_ERROR`.

## `qllm catalog introspect`

Lee `information_schema` de una fuente **postgres o mysql** del preset y escribe el YAML del catálogo. **No** levanta el servidor.

| Flag | Significado |
|------|-------------|
| `--source` | `sources[].id` (obligatorio) |
| `--out` | Archivo de salida (por defecto: stdout) |
| `--merge` | Conserva las entidades de otras fuentes del catálogo ya cargado |

La introspección caduca a los 15 segundos. Revisa el YAML (relations, descriptions) antes de `serve`.

```bash
./qllm catalog introspect --source crm_pg --config-dir ./mi-proyecto --out ./mi-proyecto/qllm.catalog.yaml
```

## `qllm catalog from-openapi`

Genera entidades `rest_resource` y un fragmento `options.resources` a partir de una especificación OpenAPI 3. `--source` debe ser una fuente `type: rest` del preset.

| Flag | Significado |
|------|-------------|
| `-f` / `--file` | Especificación OpenAPI |
| `--source` | ID de la fuente REST |
| `--out` | Catálogo de salida |
| `--resources-out` | Fragmento YAML de resources (si no, se imprime por stderr) |
| `--merge` | Igual que en `introspect` |

El conector REST lee **solo** lo que ya está en el preset. Pega el fragmento en `sources[].options.resources`.
