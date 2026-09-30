# 构建与标签

## Go

要求：**Go 1.26.6 及以上**（`go.mod`，`toolchain go1.26.6`）。镜像：`golang:1.26.6-bookworm`。

```bash
go build -o qllm ./cmd/qllm
go test ./...
```

不带标签的 `go test ./...` 不依赖 CGO。测试默认使用**纯 Go** 实现的本地引擎（`internal/duckdblocal`）。

## 内嵌 DuckDB（`-tags duckdb`）

要像镜像和生产环境那样**执行**目录 SQL（`ExecSQL`），必须启用此标签。

需要 CGO 和 C 编译器。

```bash
# Linux/macOS
CGO_ENABLED=1 go test -tags duckdb ./internal/duckdblocal/
CGO_ENABLED=1 go build -tags duckdb -o qllm ./cmd/qllm
```

Windows：

```powershell
.\scripts\dev-shell.ps1
go test -tags duckdb ./internal/duckdblocal/
go build -tags duckdb -o qllm.exe ./cmd/qllm
go run -tags duckdb .\scripts\duckdb_smoke.go
```

[`Dockerfile`](../../Dockerfile) 和 [`Dockerfile.dev`](../../Dockerfile.dev) 都会执行 `go build -tags duckdb`。产品镜像复制 `deploy/prd`；Compose 使用 `.dev` 以及 `deploy/image/config`。

## Docker / nerdctl

```bash
nerdctl build -t qllm .
# Rancher Desktop 上的 Kubernetes：
nerdctl --namespace k8s.io build -t qllm:local .
```

如果只在 nerdctl 的默认命名空间中构建过：

```bash
nerdctl save qllm:local -o qllm-local.tar
nerdctl --namespace k8s.io load -i qllm-local.tar
```

## 协议依赖

`mcp-go` 已锁定版本（见 `go.mod` 和 README）。请勿盲目升级 MCP 库：工具契约必须保持稳定。

## 配置时无需重新构建的内容

preset、catalog、access、env 和运行时配置都是**文件**。重启进程或对 Deployment 执行 rollout 即可。只有 Go 代码发生变化时才需要重新编译。
