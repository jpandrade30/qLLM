# 本仓库中的环境

## Compose 演示（goldens）

使用 Rancher Desktop 和 **nerdctl compose**。规范：[`planning/05-dev-harness.md`](../../planning/05-dev-harness.md)。

```bash
nerdctl compose up --build
```

Compose 将 [`deploy/image/config`](../../deploy/image/config) 挂到 `/config`，并把 `fixtures/test-api/data.json` 挂到 `test-api`。改目录后执行 `nerdctl compose up -d qllm`（不必重建镜像）。Bearer：`change-me`。端口：HTTP 8088、MCP 8089。种子：`.\scripts\dev\dev-seed-fake.ps1`。

导入模拟 API 的种子数据：

```powershell
.\scripts\dev\dev-seed-fake.ps1
# --regenerate 仅会重写冻结的 JSON
```

通过 MCP 运行 SQL goldens：`pytest fixtures/sqlcheck`。

弱密码和开放的 Mongo **仅限 localhost** 使用。

**不要**把这个 Compose 当作你的产品。只需把 YAML 的*格式*复制到你自己的 `--config-dir` 中。

如需使用自己的目录而非演示目录，请参阅 [point-your-folder.md](point-your-folder.md)。

## 独立镜像

```bash
nerdctl build -t qllm .
# 内嵌：deploy/prd
nerdctl run --rm -p 8088:8088 -p 8089:8089 qllm
```

Compose 使用 `Dockerfile.dev` 和 `deploy/image/config`。在 Kubernetes 模拟环境中，`deploy/prd-tst/config` 生成的 ConfigMap 会覆盖镜像中内嵌的配置。

仅含 YAML 与 `Dockerfile` 的示例：[`deploy/prd/README.md`](../../deploy/prd/README.md)。

## Kubernetes fleet-ops 模拟环境（`deploy/prd-tst`）

这是一个**独立**的世界（D18），位于命名空间 `qllm-prd`。**不要**与 Compose 同时运行。

指南：[`deploy/prd-tst/README.md`](../../deploy/prd-tst/README.md)。

```powershell
.\scripts\prd-tst\prd-tst-up.ps1
.\scripts\prd-tst\prd-tst-port-forward.ps1
.\scripts\prd-tst\prd-tst-down.ps1
```

```bash
./scripts/prd-tst/prd-tst-up.sh
./scripts/prd-tst/prd-tst-port-forward.sh
./scripts/prd-tst/prd-tst-down.sh
```

`prd-tst-up` 会执行 `compose down`，构建 `qllm:local`（命名空间 `k8s.io`），并执行 `kubectl apply -k deploy/prd-tst`。端口转发以单个进程运行；按 Ctrl+C 会停止所有转发：

```powershell
.\scripts\prd-tst\prd-tst-port-forward.ps1
# Unix：./scripts/prd-tst/prd-tst-port-forward.sh
```

| 主机端口 | 服务 |
|----------|------|
| 18088 | qLLM HTTP |
| 18089 | qLLM MCP |
| 15432 | Postgres |
| 19000 / 18123 | ClickHouse |
| 18000 | DynamoDB Local |
| 18080 | 模拟 crew API |
| 18081 | Argo CD（安装后） |

模拟环境的 Bearer 令牌：`fleet-prd-token`（这是一个 Secret；不可用于生产）。

Argo CD 是**可选**的。脚本：

| 脚本 | 用途 |
|------|------|
| `prd-tst-up.ps1` / `.sh` | 启动 fleet-ops（compose down、镜像、apply） |
| `prd-tst-down.ps1` / `.sh` | 删除 overlay 和 Argo Application |
| `prd-tst-argocd-up.ps1` / `.sh` | 以 `--insecure` 安装 Argo |
| `prd-tst-argocd-password.ps1` / `.sh` | 显示 `admin` 密码 |
| `prd-tst-argocd-register-app.ps1` / `.sh` | 创建 Application 资源 |
| `prd-tst-argocd-add-ssh-repo.ps1` / `.sh` | 在集群中注册 SSH 密钥 |

镜像使用 `imagePullPolicy: Never`，并且 `qllm:local` 位于 `k8s.io` 命名空间。出现 `ErrImagePull` 表示该镜像不在 Kubernetes 的 containerd 中。

## 开发脚本

| 脚本 | 用途 |
|------|------|
| `scripts/dev/dev-shell.ps1` / `.cmd` / `.sh` | CGO 和 gcc 环境设置（Windows 与 Unix） |
| `scripts/dev/dev-seed-fake.ps1` / `.sh` | 导入 `fixtures/datasets/v1` 种子数据 |
| `scripts/dev/duckdb_smoke.go` | CGO DuckDB 冒烟测试 |
| `scripts/prd-tst/` | Kubernetes fleet-ops 模拟 |
| `scripts/standalone/` | 精简仓库生成器 |

## 查询日志

```powershell
kubectl logs -n qllm-prd deploy/qllm -f
```

你会看到 `---- execute_sql ----` 块（多行 SQL）和 `---- mcp_tool ----`。如果想查看 SQL，不要只按第一行过滤。
