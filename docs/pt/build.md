# Build e tags

## Go

Requisito: **Go 1.26.6+** (`go.mod`, `toolchain go1.26.6`). Imagem: `golang:1.26.6-bookworm`.

```bash
go build -o qllm ./cmd/qllm
go test ./...
```

`go test ./...` sem tag não usa CGO. Por padrão, os testes usam o motor local em **Go puro** (`internal/duckdblocal`).

## DuckDB embutido (`-tags duckdb`)

Obrigatório para **executar** SQL de catálogo (`ExecSQL`), como fazem a imagem e a produção.

Exige CGO e um compilador C.

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

O [`Dockerfile`](../../Dockerfile) e o [`Dockerfile.dev`](../../Dockerfile.dev) executam `go build -tags duckdb`. A imagem de produto copia `deploy/prd`; o Compose usa `.dev` com `deploy/image/config`.

## Testes unitários no container (`Dockerfile.test`)

Mesma suíte da CI (Go puro + `-tags duckdb` em `duckdblocal` / `executor`). Sem compose. Útil no Windows sem CGO/duckdblib local:

```bash
nerdctl build -f Dockerfile.test --target test .
# ou build + run (TTY para cores):
nerdctl build -f Dockerfile.test -t qllm-test .
nerdctl run --rm -t qllm-test
```

Helpers: `scripts/dev/test-in-container.ps1` / `test-in-container.sh`. Env: `QLLM_TEST_V=0` (quiet), `QLLM_TEST_COLOR=0` (sem ANSI).

## Docker / nerdctl

```bash
nerdctl build -t qllm .
# Kubernetes no Rancher Desktop:
nerdctl --namespace k8s.io build -t qllm:local .
```

Se você construiu apenas no namespace padrão do nerdctl:

```bash
nerdctl save qllm:local -o qllm-local.tar
nerdctl --namespace k8s.io load -i qllm-local.tar
```

## Dependências de protocolo

O `mcp-go` está fixado em uma versão (veja `go.mod` e o README). Não atualize a biblioteca MCP sem cuidado: o contrato das tools precisa continuar estável.

## O que não exige rebuild para configurar

Preset, catálogo, access, env e configuração de runtime são **arquivos**. Reinicie o processo ou faça o rollout do Deployment. Só recompile se o código Go mudar.
