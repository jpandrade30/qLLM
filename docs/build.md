# Build e tags

## Go

Requisito: **1.26+** (`go.mod`). Imagem: `golang:1.26-bookworm`.

```bash
go build -o qllm ./cmd/qllm
go test ./...
```

`go test ./...` **sem** tag é CGO-free. O motor local default em testes é a implementação **pure Go** (`internal/duckdblocal`).

## DuckDB embutido (`-tags duckdb`)

Obrigatório para **executar** catalog SQL (`ExecSQL`) como na imagem/produção.

Precisa CGO + compilador C.

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

O `Dockerfile` do repo faz `go build -tags duckdb`.

## Docker / nerdctl

```bash
nerdctl build -t qllm .
# Kubernetes no Rancher:
nerdctl --namespace k8s.io build -t qllm:local .
```

Se construíste só no namespace default do nerdctl:

```bash
nerdctl save qllm:local -o qllm-local.tar
nerdctl --namespace k8s.io load -i qllm-local.tar
```

## Dependências de protocolo

`mcp-go` está pinado (ver `go.mod` / README). Não actualizes MCP à cegas sem o contrato de tools.

## O que não recompilar para “configurar”

Preset, catalog, access, env, runtime config: **ficheiros**. Restart do processo / rollout do Deployment. Sem rebuild salvo mudares o Go.
