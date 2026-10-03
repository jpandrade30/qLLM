# Compilación y tags

## Go

Requisito: **Go 1.26.6+** (`go.mod`, `toolchain go1.26.6`). Imagen: `golang:1.26.6-bookworm`.

```bash
go build -o qllm ./cmd/qllm
go test ./...
```

`go test ./...` sin tag no usa CGO. Por defecto, las pruebas usan el motor local en **Go puro** (`internal/duckdblocal`).

## DuckDB embebido (`-tags duckdb`)

Es obligatorio para **ejecutar** SQL de catálogo (`ExecSQL`), como hacen la imagen y producción.

Requiere CGO y un compilador de C.

```bash
# Linux/macOS
CGO_ENABLED=1 go test -tags duckdb ./internal/duckdblocal/
CGO_ENABLED=1 go build -tags duckdb -o qllm ./cmd/qllm
```

Windows:

```powershell
.\scripts\dev\dev-shell.ps1
go test -tags duckdb ./internal/duckdblocal/
go build -tags duckdb -o qllm.exe ./cmd/qllm
go run -tags duckdb .\scripts\dev\duckdb_smoke.go
```

El [`Dockerfile`](../../Dockerfile) y el [`Dockerfile.dev`](../../Dockerfile.dev) ejecutan `go build -tags duckdb`. La imagen de producto copia `deploy/prd`; Compose usa `.dev` con `deploy/image/config`.

## Docker / nerdctl

```bash
nerdctl build -t qllm .
# Kubernetes en Rancher Desktop:
nerdctl --namespace k8s.io build -t qllm:local .
```

Si compilaste solo en el namespace predeterminado de nerdctl:

```bash
nerdctl save qllm:local -o qllm-local.tar
nerdctl --namespace k8s.io load -i qllm-local.tar
```

## Dependencias de protocolo

`mcp-go` está fijado a una versión (consulta `go.mod` y el README). No actualices la biblioteca MCP a ciegas: el contrato de las tools debe mantenerse estable.

## Qué no requiere recompilar para configurar

El preset, el catálogo, access, env y la configuración de runtime son **archivos**. Reinicia el proceso o haz el rollout del Deployment. Recompila solo cuando cambie el código Go.
