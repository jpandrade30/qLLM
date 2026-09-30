# Build and tags

## Go

Requirement: **Go 1.26.6+** (`go.mod`, `toolchain go1.26.6`). Image: `golang:1.26.6-bookworm`.

```bash
go build -o qllm ./cmd/qllm
go test ./...
```

`go test ./...` without a tag is CGO-free. Tests use the **pure Go** local engine by default (`internal/duckdblocal`).

## Embedded DuckDB (`-tags duckdb`)

Required to **execute** catalog SQL (`ExecSQL`) as the image and production do.

It needs CGO and a C compiler.

```bash
# Linux/macOS
CGO_ENABLED=1 go test -tags duckdb ./internal/duckdblocal/
CGO_ENABLED=1 go build -tags duckdb -o qllm ./cmd/qllm
```

Windows:

```powershell
.\scripts\dev-shell.ps1
go test -tags duckdb ./internal/duckdblocal/
go build -tags duckdb -o qllm.exe ./cmd/qllm
go run -tags duckdb .\scripts\duckdb_smoke.go
```

[`Dockerfile`](../../Dockerfile) and [`Dockerfile.dev`](../../Dockerfile.dev) both run `go build -tags duckdb`. The product image copies `deploy/prd`; Compose uses `.dev` plus `deploy/image/config`.

## Docker / nerdctl

```bash
nerdctl build -t qllm .
# Kubernetes on Rancher Desktop:
nerdctl --namespace k8s.io build -t qllm:local .
```

If you built only in the default nerdctl namespace:

```bash
nerdctl save qllm:local -o qllm-local.tar
nerdctl --namespace k8s.io load -i qllm-local.tar
```

## Protocol dependencies

`mcp-go` is pinned (see `go.mod` and the README). Do not upgrade the MCP library blindly; the tool contract must stay stable.

## What you do not need to rebuild to configure

Preset, catalog, access, env, and runtime config are **files**. Restart the process or roll out the Deployment. Rebuild only when the Go code changes.
