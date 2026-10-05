# Guía de instalación

Esta página cubre todas las formas de ejecutar qLLM: qué hace cada opción, qué requiere y cómo comprobar que funcionó. Las Releases de GitHub incluyen un zip slim (`qllm-standalone-<ver>.zip`); sigues compilando el binario o la imagen.

## 1. Elige una opción

| Opción | Requiere | Obtienes | Úsala cuando |
|--------|----------|----------|--------------|
| **A. Imagen de contenedor** (`Dockerfile`) | Docker, nerdctl o podman | Build completo (DuckDB embebido) con el YAML de `deploy/prd` dentro | Producción, o si no quieres toolchain C |
| **B. Build Go, Go puro** | Go 1.26.6+ | `qllm` sin CGO. Sin SQL de catálogo | `validate` rápido, Query IR, CI sin CGO |
| **C. Build Go, DuckDB embebido** (`-tags duckdb`) | Go 1.26.6+, CGO, compilador C (+ `duckdblib` en Windows) | Todo, incluido `execute_sql` / `qllm sql` | Desarrollar el producto completo en local |
| **D. Zip standalone** (preferir el asset de la Release; o `init-standalone.*`) | Docker (para ejecutar); Python 3 solo si regeneras desde un clone | Una carpeta pequeña para alojar en GitHub/GitLab | **Sugerencia por defecto:** ejecutar sin clonar este monorepo |
| **E. Harness Compose** (`docker-compose.yml`) | nerdctl compose (Rancher Desktop) | qLLM + Postgres, MySQL, MongoDB, API falsa | Ejecutar los goldens y probar el demo |
| **F. Simulación Kubernetes** (`deploy/prd-tst`) | Rancher Desktop con Kubernetes | Un clúster estilo fleet-ops | Probar rollouts. Mundo aparte, ver [environments.md](environments.md) |
| **G. Demo de claves con alcance** (`docker-compose.enforced.yml`) | nerdctl/docker compose | qLLM + Postgres + agente LangGraph | Ver el alcance por fila (D21) de punta a punta |

Si dudas: **D** (zip de la Release) o **A** para ejecutar; **C** para desarrollar este repo.

## 2. Los dos motores: Go puro vs DuckDB embebido

qLLM tiene un motor local de join detrás de una única API `Engine`. Cuál obtienes depende de una build tag.

| | Go puro (por defecto) | DuckDB embebido (`-tags duckdb`) |
|---|---|---|
| Comando | `go build ./cmd/qllm` | `go build -tags duckdb ./cmd/qllm` |
| CGO / compilador C | No hace falta | Obligatorio |
| Query IR (`/v1/queries`, `qllm query`) | Sí, incluidos joins entre fuentes, agregaciones REST, `where`, offset | Sí |
| SQL de catálogo (`/v1/sql`, MCP `execute_sql`, `qllm sql`) | **No**. No puede ejecutar el SQL | **Sí** |
| `go test ./...` | Sin CGO | Solo los paquetes marcados necesitan la tag |

Las imágenes de contenedor y producción usan `-tags duckdb`. Si una petición SQL falla porque el build no tiene DuckDB, estás ejecutando un binario Go puro.

## 3. Opción A: imagen de contenedor

El [`Dockerfile`](../../Dockerfile) de la raíz tiene dos etapas.

1. **Build** (`golang:1.26.6-bookworm`): instala `gcc` y `libc6-dev`, descarga módulos Go, define `CGO_ENABLED=1` y ejecuta `go build -tags duckdb`.
2. **Runtime** (`debian:bookworm-slim`): añade `ca-certificates`, copia el binario a `/usr/local/bin/qllm` e incluye en `/config` estos archivos de `deploy/prd/`: `qllm.preset.yaml`, `qllm.catalog.yaml`, `qllm.config.yaml`, `qllm.env.yaml`, `qllm.access.yaml`.

El contenedor arranca con `qllm serve --http --mcp-http --config-dir /config` y expone **8088** (HTTP `/v1`) y **8089** (MCP HTTP).

```bash
nerdctl build -t qllm .          # o: docker build -t qllm .
nerdctl run --rm -p 8088:8088 -p 8089:8089 qllm
```

Para usar tu propio YAML, edita `deploy/prd/default/*.yaml` y reconstruye, o monta tu carpeta sobre la incluida y evita el rebuild:

```bash
docker run --rm -p 8088:8088 -p 8089:8089 -v "$PWD/my-project:/config" qllm
```

Los secretos son variables de entorno nombradas por las claves `*Env` del preset, pasadas con `-e NOMBRE=valor` o `--env-file`. Nunca van en el YAML.

Dentro de un contenedor el servidor debe escuchar en una dirección no loopback, lo que `qllm` solo permite con auth configurada (o `serve.insecureBind: true`). Mantén coherentes `serve.addr` y `authTokenEnv` en el `qllm.config.yaml` incluido.

Los tres Dockerfiles solo se diferencian en lo que incluyen:

| Archivo | Incluye en `/config` | Lo usa |
|---------|----------------------|--------|
| `Dockerfile` | `deploy/prd/default/` | Imagen de producto |
| `Dockerfile.dev` | `deploy/image/config/` | Harness Compose y `prd-tst` (Kubernetes monta un ConfigMap sobre `/config`) |
| `Dockerfile.enforced` | `deploy/prd/enforced/config/` | Demo de claves con alcance |

### Rancher Desktop y Kubernetes

El Kubernetes de Rancher Desktop lee imágenes del namespace containerd `k8s.io`:

```bash
nerdctl --namespace k8s.io build -t qllm:local .
```

Si construiste en el namespace por defecto, copia la imagen:

```bash
nerdctl save qllm:local -o qllm-local.tar
nerdctl --namespace k8s.io load -i qllm-local.tar
```

`ErrImagePull` en la simulación significa que la imagen no está en ese namespace (los manifests usan `imagePullPolicy: Never`).

## 4. Opción B: build Go, Go puro

Instala **Go 1.26.6 o superior** (`go.mod` fija `go 1.26.6`).

```bash
go build -o qllm ./cmd/qllm        # Windows: -o qllm.exe
go test ./...
./qllm validate --config-dir ./my-project
```

No interviene ningún compilador C. Sirve para validar archivos y ejecutar Query IR. El SQL de catálogo no funciona (sección 2).

## 5. Opción C: build Go con DuckDB embebido

Añade `-tags duckdb` y activa CGO. El resto es el mismo comando.

### Linux y macOS

Instala un compilador C (`gcc`, o las herramientas de línea de comandos de Xcode: `xcode-select --install`). Los bindings Go (`duckdb-go-bindings`, ver `go.mod`) incluyen la biblioteca DuckDB para linux y darwin en amd64 y arm64, así que no descargas DuckDB a mano.

```bash
source ./scripts/dev/dev-shell.sh      # define CGO_ENABLED=1 y las variables QLLM_* de demo
go test -tags duckdb ./internal/duckdblocal/
go build -tags duckdb -o qllm ./cmd/qllm
```

`dev-shell.sh` debe cargarse con **`source`**, no ejecutarse, o las variables desaparecen al terminar. Sus variables de demo usan `${VAR:-valor}`, así que lo que ya exportaste prevalece. Si solo necesitas CGO:

```bash
CGO_ENABLED=1 go build -tags duckdb -o qllm ./cmd/qllm
```

### Windows

En Windows el repositorio enlaza con la biblioteca oficial de DuckDB mediante dos cosas: un gcc MinGW y la carpeta `duckdblib/`.

**Paso 1. gcc (MSYS2 UCRT64).** Instala [MSYS2](https://www.msys2.org/), abre la shell **UCRT64** y ejecuta:

```bash
pacman -S mingw-w64-ucrt-x86_64-gcc
```

`scripts/dev/dev-shell.ps1` busca `gcc.exe` en `C:\ghcup\msys64\ucrt64\bin`. Si tu MSYS2 está en otro sitio (por defecto `C:\msys64\ucrt64\bin`), edita la línea `$MsysGccBin` al inicio del script. Sin gcc el script muestra la pista de instalación y termina.

**Paso 2. Biblioteca DuckDB.** En la [página oficial de instalación de DuckDB](https://duckdb.org/docs/installation/), descarga la biblioteca C/C++ para Windows (`libduckdb-windows-amd64.zip`). Extrae y copia **`duckdb.dll`**, **`duckdb.lib`** y el header **`duckdb.h`** a la carpeta `duckdblib/` en la raíz del repo. Son binarios grandes; no los subas a tu fork.

**Paso 3. Carga el entorno.** En PowerShell:

```powershell
.\scripts\dev\dev-shell.ps1
```

o haz doble clic en `scripts\dev\dev-shell.cmd`, que abre un PowerShell con el script cargado (política de ejecución omitida solo en esa ventana). El script:

| Acción | Motivo |
|--------|--------|
| Pone el `bin` de MSYS2 y `duckdblib/` al inicio del `PATH` | El linker encuentra gcc y `qllm.exe` encuentra `duckdb.dll` **en tiempo de ejecución** |
| `CGO_ENABLED=1`, `CC=gcc` | Activa CGO |
| `CGO_CFLAGS=-I<duckdblib>` | El compilador encuentra `duckdb.h` |
| `CGO_LDFLAGS=-L<duckdblib> -lduckdb` | El linker encuentra la biblioteca |
| Define variables `QLLM_*` de demo (Postgres, MySQL, Mongo y API falsa locales) | Coincide con el harness Compose. Ignóralas en tu proyecto |
| Entra en la raíz del repo y cambia el prompt a `qLLM-dev` | Comodidad |

Solo cambia la sesión **actual**. En una terminal nueva debes ejecutarlo otra vez.

**Paso 4. Compila y comprueba.**

```powershell
go test -tags duckdb ./internal/duckdblocal/
go build -tags duckdb -o qllm.exe ./cmd/qllm
go run -tags duckdb .\scripts\dev\duckdb_smoke.go
```

`duckdb_smoke.go` abre el motor embebido y ejecuta una consulta mínima; es la forma más rápida de probar que CGO y la biblioteca están bien enlazados.

Para ejecutar `qllm.exe` después desde una terminal normal, deja `duckdb.dll` junto al ejecutable o en el `PATH`.

## 6. Opción D: zip standalone (preferible al clone)

No clones este monorepo solo para hospedar qLLM. Descarga **`qllm-standalone-<ver>.zip`** desde la [Release de GitHub](https://github.com/jpandrade30/qLLM/releases) (ignora los archivos automáticos **Source code**), descomprime y haz `docker build` como en el README de esa carpeta.

Solo si ya tienes un clone y necesitas una carpeta con nombre (sin docs, fixtures ni harness):

```bash
python scripts/standalone/init-standalone.py --user Alice --out ..
./scripts/standalone/init-standalone.sh  --user Alice --out ..      # Linux/macOS, llama a python3 o python
.\scripts\standalone\init-standalone.ps1 --user Alice --out ..      # Windows
```

| Flag | Significado |
|------|-------------|
| `--user` | Obligatorio. El nombre se convierte en slug: minúsculas, lo que no sea `a-z0-9` pasa a `-`. Un slug que no empieza por letra recibe el prefijo `user-` |
| `--out` | Directorio **padre** de la carpeta nueva. Por defecto, el directorio actual. `--out ..` la deja junto a este repo |
| `--force` | Borra y recrea la carpeta destino si existe. Sin él el script se niega |

El resultado es `<out>/qllm-<slug>/` y el script imprime esa ruta. Contiene:

| Ruta | Qué es |
|------|--------|
| `go.mod`, `go.sum`, `cmd/qllm/`, `internal/` | El runtime Go, sin tests |
| `config/` | `qllm.preset.yaml`, `qllm.catalog.yaml`, `qllm.config.yaml`, `qllm.env.yaml`, `qllm.access.yaml` para una fuente SQLite con una tabla, `items` |
| `data/app.db` | El archivo SQLite (`items`: `id=1`, `name=hello`) |
| `Dockerfile` | El mismo build en dos etapas con `-tags duckdb`, incluye `config/` y `data/` |
| `LICENSE.md`, `.env.example`, `.gitignore`, `README.md` | Licencia MIT, plantilla de token, ignores, instrucciones de ejecución |

Ejecútalo en la carpeta nueva:

```bash
cp .env.example .env          # define QLLM_AUTH_TOKEN
docker build -t qllm-alice .
docker run --rm -p 8088:8088 -p 8089:8089 --env-file .env qllm-alice
curl -s http://127.0.0.1:8088/v1/health
```

Sin Docker, un build Go puro puede ejecutar `validate` (sin SQL). Luego edita preset y catálogo para añadir tus fuentes.

## 7. Opción E: harness Compose

```bash
nerdctl compose up --build
```

Construye `Dockerfile.dev` y levanta qLLM, Postgres, MySQL, MongoDB y la API falsa. Bearer de demo: `change-me`. Puertos en el host: HTTP 8088, MCP 8089. Carga datos con `.\scripts\dev\dev-seed-fake.ps1` o `./scripts/dev/dev-seed-fake.sh`. Ejecuta los goldens SQL con `pytest fixtures/sqlcheck`. Las contraseñas son débiles y Mongo está abierto, así que úsalo solo en localhost. Detalles: [environments.md](environments.md).

## 8. Opciones F y G

- **F.** `.\scripts\prd-tst\prd-tst-up.ps1` (o `.sh`) construye `qllm:local` en `k8s.io` y aplica `deploy/prd-tst`. No lo ejecutes junto con Compose. Ver [environments.md](environments.md).
- **G.** Demo de claves con alcance, con su propio compose: `deploy/prd/enforced/README.md` y [multi-user-safety.md](multi-user-safety.md).

## 9. Después de instalar

La configuración son **archivos más variables de entorno**. Cambiar el YAML solo requiere reiniciar (o hacer rollout del Deployment). Recompila únicamente cuando cambie el código Go.

Lo mínimo para servir:

1. Una carpeta con `qllm.preset.yaml` y `qllm.catalog.yaml` ([from-scratch.md](from-scratch.md)). Sin ellos `serve` falla con `CONFIG_ERROR`; no hay fallback a `fixtures/`.
2. Las variables de entorno nombradas por las claves `*Env` del preset.
3. Valida y arranca:

```bash
./qllm validate --config-dir ./my-project
./qllm serve --http --mcp-http --config-dir ./my-project
curl -s http://127.0.0.1:8088/v1/health
```

Por defecto escucha en `127.0.0.1:8088` (HTTP) y `127.0.0.1:8089` (MCP). Todos los comandos y flags: [cli.md](cli.md). Auth, bind y CORS: [http-mcp.md](http-mcp.md).

## 10. Solución de problemas

| Síntoma | Causa y arreglo |
|---------|-----------------|
| SQL devuelve `UNSUPPORTED` mencionando DuckDB | Binario Go puro. Recompila con `-tags duckdb` y CGO |
| `gcc not found at …` en `dev-shell.ps1` | Instala el paquete gcc de MSYS2 o corrige `$MsysGccBin` |
| `Warning: duckdb.dll not found` | Pon `duckdb.dll`, `duckdb.lib`, `duckdb.h` en `duckdblib/` |
| El build falla con `cannot find -lduckdb` o `duckdb.h` | `duckdblib/` incompleta, o no ejecutaste `dev-shell.ps1` en esta terminal |
| `qllm.exe` termina al iniciar por DLL ausente | `duckdb.dll` no está en el `PATH` ni junto al exe |
| Linux/macOS: `gcc: command not found` | Instala un compilador C |
| `CONFIG_ERROR` en `serve` | No hay `qllm.preset.yaml` / `qllm.catalog.yaml` en `--config-dir` ni en el CWD |
| Se niega a enlazar una dirección no loopback | Define `serve.authTokenEnv` (y exporta la variable) o `--insecure-bind` solo para pruebas locales |
| Env de auth definida pero el servidor no arranca | La variable nombrada en `authTokenEnv` está vacía |
| `ErrImagePull` en la simulación | Construye o carga la imagen en el namespace `k8s.io` |
| `refusing to overwrite …` en init-standalone | La carpeta existe. Usa otro `--user` o `--force` |

Otros detalles de build (`mcp-go` fijado, cuándo recompilar): [build.md](build.md).
