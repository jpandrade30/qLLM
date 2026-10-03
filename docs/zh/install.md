# 安装指南

本页介绍运行 qLLM 的所有方式：每个选项做什么、需要什么，以及如何确认成功。目前尚未发布预编译二进制，所以你需要从源码编译，或构建容器镜像。

## 1. 选择方案

| 方案 | 需要 | 得到 | 适用场景 |
|------|------|------|----------|
| **A. 容器镜像**（`Dockerfile`） | Docker、nerdctl 或 podman | 完整构建（内嵌 DuckDB），已包含 `deploy/prd` 的 YAML | 生产环境，或不想装 C 工具链 |
| **B. Go 构建（纯 Go）** | Go 1.26.6+ | 不含 CGO 的 `qllm`，不支持目录 SQL | 快速 `validate`、Query IR、无 CGO 的 CI |
| **C. Go 构建（内嵌 DuckDB，`-tags duckdb`）** | Go 1.26.6+、CGO、C 编译器（Windows 还需 `duckdblib`） | 全部功能，含 `execute_sql` / `qllm sql` | 本地开发完整产品 |
| **D. 独立仓库**（`scripts/standalone/init-standalone.*`） | Python 3（生成）、Docker（运行） | 可托管到 GitHub/GitLab 的小型目录 | 不带文档、fixtures、harness 的自有副本 |
| **E. Compose harness**（`docker-compose.yml`） | nerdctl compose（Rancher Desktop） | qLLM + Postgres、MySQL、MongoDB、模拟 API | 运行 golden 测试、体验演示 |
| **F. Kubernetes 模拟**（`deploy/prd-tst`） | 启用 Kubernetes 的 Rancher Desktop | fleet-ops 风格集群 | 测试发布。独立环境，见 [environments.md](environments.md) |
| **G. 作用域密钥演示**（`docker-compose.enforced.yml`） | nerdctl/docker compose | qLLM + Postgres + LangGraph 代理 | 端到端查看行级作用域（D21） |

拿不准时：运行用 **A**，开发用 **C**。

## 2. 两种引擎：纯 Go 与内嵌 DuckDB

qLLM 在统一的 `Engine` API 后面有一个本地 join 引擎，具体用哪个取决于构建标签。

| | 纯 Go（默认） | 内嵌 DuckDB（`-tags duckdb`） |
|---|---|---|
| 命令 | `go build ./cmd/qllm` | `go build -tags duckdb ./cmd/qllm` |
| CGO / C 编译器 | 不需要 | 必需 |
| Query IR（`/v1/queries`、`qllm query`） | 支持，含跨源 join、REST 聚合、`where`、offset | 支持 |
| 目录 SQL（`/v1/sql`、MCP `execute_sql`、`qllm sql`） | **不支持**，无法执行 SQL | **支持** |
| `go test ./...` | 无需 CGO | 只有带标签的包需要该标签 |

容器镜像和生产环境都使用 `-tags duckdb`。如果 SQL 请求因构建缺少 DuckDB 而失败，说明你运行的是纯 Go 二进制。

## 3. 方案 A：容器镜像

根目录的 [`Dockerfile`](../../Dockerfile) 分两个阶段。

1. **构建阶段**（`golang:1.26.6-bookworm`）：安装 `gcc` 与 `libc6-dev`，下载 Go 模块，设置 `CGO_ENABLED=1`，执行 `go build -tags duckdb`。
2. **运行阶段**（`debian:bookworm-slim`）：添加 `ca-certificates`，把二进制复制到 `/usr/local/bin/qllm`，并把 `deploy/prd/` 中的这些文件放进 `/config`：`qllm.preset.yaml`、`qllm.catalog.yaml`、`qllm.config.yaml`、`qllm.env.yaml`、`qllm.access.yaml`。

容器以 `qllm serve --http --mcp-http --config-dir /config` 启动，暴露 **8088**（HTTP `/v1`）和 **8089**（MCP HTTP）。

```bash
nerdctl build -t qllm .          # 或：docker build -t qllm .
nerdctl run --rm -p 8088:8088 -p 8089:8089 qllm
```

使用自己的 YAML：编辑 `deploy/prd/default/*.yaml` 后重新构建，或把你的目录挂载到内置目录上，免去重建：

```bash
docker run --rm -p 8088:8088 -p 8089:8089 -v "$PWD/my-project:/config" qllm
```

密钥是由 preset 中 `*Env` 键命名的环境变量，用 `-e 名称=值` 或 `--env-file` 传入，绝不写进 YAML。

容器内服务必须监听非回环地址，`qllm` 仅在配置了鉴权（或 `serve.insecureBind: true`）时才允许。请保证内置 `qllm.config.yaml` 中 `serve.addr` 与 `authTokenEnv` 一致。

三个 Dockerfile 只在内置内容上不同：

| 文件 | 内置到 `/config` | 使用者 |
|------|------------------|--------|
| `Dockerfile` | `deploy/prd/default/` | 产品镜像 |
| `Dockerfile.dev` | `deploy/image/config/` | Compose harness 与 `prd-tst`（Kubernetes 用 ConfigMap 覆盖 `/config`） |
| `Dockerfile.enforced` | `deploy/prd/enforced/config/` | 作用域密钥演示 |

### Rancher Desktop 与 Kubernetes

Rancher Desktop 的 Kubernetes 从 containerd 的 `k8s.io` 命名空间读取镜像：

```bash
nerdctl --namespace k8s.io build -t qllm:local .
```

如果你在默认命名空间构建，需要复制镜像：

```bash
nerdctl save qllm:local -o qllm-local.tar
nerdctl --namespace k8s.io load -i qllm-local.tar
```

模拟环境中出现 `ErrImagePull`，说明镜像不在该命名空间（清单使用 `imagePullPolicy: Never`）。

## 4. 方案 B：Go 构建（纯 Go）

安装 **Go 1.26.6 或更高**（`go.mod` 声明 `go 1.26.6`）。

```bash
go build -o qllm ./cmd/qllm        # Windows：-o qllm.exe
go test ./...
./qllm validate --config-dir ./my-project
```

不涉及 C 编译器。可用于校验文件和运行 Query IR，目录 SQL 不可用（见第 2 节）。

## 5. 方案 C：内嵌 DuckDB 的 Go 构建

加上 `-tags duckdb` 并启用 CGO，其余命令不变。

### Linux 与 macOS

安装 C 编译器（`gcc`，或 Xcode 命令行工具：`xcode-select --install`）。Go 绑定（`duckdb-go-bindings`，见 `go.mod`）已包含 amd64 与 arm64 的 linux、darwin DuckDB 库，无需手动下载 DuckDB。

```bash
source ./scripts/dev/dev-shell.sh      # 设置 CGO_ENABLED=1 和演示用 QLLM_* 变量
go test -tags duckdb ./internal/duckdblocal/
go build -tags duckdb -o qllm ./cmd/qllm
```

`dev-shell.sh` 必须用 **`source`** 加载，不能直接执行，否则脚本结束后变量就消失了。其演示变量使用 `${VAR:-默认值}`，已导出的值优先。如果只需要 CGO：

```bash
CGO_ENABLED=1 go build -tags duckdb -o qllm ./cmd/qllm
```

### Windows

在 Windows 上，仓库通过两样东西链接官方 DuckDB 库：MinGW 的 gcc 和 `duckdblib/` 目录。

**步骤 1. gcc（MSYS2 UCRT64）。** 安装 [MSYS2](https://www.msys2.org/)，打开 **UCRT64** shell 并运行：

```bash
pacman -S mingw-w64-ucrt-x86_64-gcc
```

`scripts/dev/dev-shell.ps1` 在 `C:\ghcup\msys64\ucrt64\bin` 查找 `gcc.exe`。如果你的 MSYS2 在别处（默认是 `C:\msys64\ucrt64\bin`），请修改脚本开头的 `$MsysGccBin`。没有 gcc 时脚本会打印安装提示并退出。

**步骤 2. DuckDB 库。** 在 [DuckDB 官方安装页面](https://duckdb.org/docs/installation/) 下载 Windows 的 C/C++ 库（`libduckdb-windows-amd64.zip`）。解压后，把 **`duckdb.dll`**、**`duckdb.lib`** 和头文件 **`duckdb.h`** 复制到仓库根目录的 `duckdblib/` 中。这些文件很大，请不要提交到你自己的 fork。

**步骤 3. 加载环境。** 在 PowerShell 中：

```powershell
.\scripts\dev\dev-shell.ps1
```

或双击 `scripts\dev\dev-shell.cmd`，它会打开已加载该脚本的 PowerShell（仅该窗口绕过执行策略）。脚本会：

| 动作 | 原因 |
|------|------|
| 把 MSYS2 的 `bin` 和 `duckdblib/` 放到 `PATH` 最前 | 链接器找到 gcc，`qllm.exe` 在**运行时**找到 `duckdb.dll` |
| `CGO_ENABLED=1`、`CC=gcc` | 启用 CGO |
| `CGO_CFLAGS=-I<duckdblib>` | 编译器找到 `duckdb.h` |
| `CGO_LDFLAGS=-L<duckdblib> -lduckdb` | 链接器找到库 |
| 设置演示用 `QLLM_*` 变量（本地 Postgres、MySQL、Mongo、模拟 API） | 与 Compose harness 一致。自己的项目可忽略 |
| 切换到仓库根目录并把提示符改为 `qLLM-dev` | 方便使用 |

它只影响**当前**会话，新开终端需要重新运行。

**步骤 4. 构建并验证。**

```powershell
go test -tags duckdb ./internal/duckdblocal/
go build -tags duckdb -o qllm.exe ./cmd/qllm
go run -tags duckdb .\scripts\dev\duckdb_smoke.go
```

`duckdb_smoke.go` 会打开内嵌引擎并执行一个极小的查询，是确认 CGO 与库配置正确的最快方法。

之后要在普通终端运行 `qllm.exe`，请让 `duckdb.dll` 与可执行文件同目录，或位于 `PATH` 中。

## 6. 方案 D：独立仓库

生成一个小型项目目录，可托管到 GitHub 或 GitLab，不带本仓库的文档、fixtures 和 harness。

```bash
python scripts/standalone/init-standalone.py --user Alice --out ..
./scripts/standalone/init-standalone.sh  --user Alice --out ..      # Linux/macOS，调用 python3 或 python
.\scripts\standalone\init-standalone.ps1 --user Alice --out ..      # Windows
```

| 参数 | 含义 |
|------|------|
| `--user` | 必填。名称会变成 slug：转小写，`a-z0-9` 以外的字符变成 `-`。不以字母开头的 slug 会加 `user-` 前缀 |
| `--out` | 新目录的**父**目录，默认当前目录。`--out ..` 会把它放在本仓库旁边 |
| `--force` | 目标目录已存在时删除并重建。不加则脚本拒绝执行 |

结果是 `<out>/qllm-<slug>/`，脚本会打印该路径。内容：

| 路径 | 说明 |
|------|------|
| `go.mod`、`go.sum`、`cmd/qllm/`、`internal/` | Go 运行时（不含测试） |
| `config/` | 面向一个 SQLite 源（含一张表 `items`）的 `qllm.preset.yaml`、`qllm.catalog.yaml`、`qllm.config.yaml`、`qllm.env.yaml`、`qllm.access.yaml` |
| `data/app.db` | SQLite 文件（`items`：`id=1`、`name=hello`） |
| `Dockerfile` | 同样的两阶段 `-tags duckdb` 构建，内置 `config/` 和 `data/` |
| `.env.example`、`.gitignore`、`README.md` | 令牌模板、忽略规则、运行说明 |

在新目录中运行：

```bash
cp .env.example .env          # 设置 QLLM_AUTH_TOKEN
docker build -t qllm-alice .
docker run --rm -p 8088:8088 -p 8089:8089 --env-file .env qllm-alice
curl -s http://127.0.0.1:8088/v1/health
```

没有 Docker 时，纯 Go 构建可以运行 `validate`（无 SQL）。之后编辑 preset 与 catalog 添加你自己的数据源。

## 7. 方案 E：Compose harness

```bash
nerdctl compose up --build
```

构建 `Dockerfile.dev`，启动 qLLM、Postgres、MySQL、MongoDB 和模拟 API。演示 Bearer：`change-me`。宿主机端口：HTTP 8088、MCP 8089。用 `.\scripts\dev\dev-seed-fake.ps1` 或 `./scripts/dev/dev-seed-fake.sh` 导入数据，用 `pytest fixtures/sqlcheck` 运行 SQL golden 测试。密码很弱且 Mongo 未设防，仅限 localhost 使用。详情见 [environments.md](environments.md)。

## 8. 方案 F 与 G

- **F.** `.\scripts\prd-tst\prd-tst-up.ps1`（或 `.sh`）在 `k8s.io` 中构建 `qllm:local` 并应用 `deploy/prd-tst`。不要与 Compose 同时运行。见 [environments.md](environments.md)。
- **G.** 带独立 compose 的作用域密钥演示：`deploy/prd/enforced/README.md` 与 [multi-user-safety.md](multi-user-safety.md)。

## 9. 安装之后

配置是**文件加环境变量**。修改 YAML 只需重启（或滚动更新 Deployment），只有 Go 代码变化时才需要重新构建。

开始服务的最低要求：

1. 一个包含 `qllm.preset.yaml` 和 `qllm.catalog.yaml` 的目录（[from-scratch.md](from-scratch.md)）。没有它们 `serve` 会返回 `CONFIG_ERROR`，不会回退到 `fixtures/`。
2. preset 中 `*Env` 键所指的环境变量。
3. 校验并启动：

```bash
./qllm validate --config-dir ./my-project
./qllm serve --http --mcp-http --config-dir ./my-project
curl -s http://127.0.0.1:8088/v1/health
```

默认监听 `127.0.0.1:8088`（HTTP）和 `127.0.0.1:8089`（MCP）。全部命令与参数：[cli.md](cli.md)。鉴权、绑定与 CORS：[http-mcp.md](http-mcp.md)。

## 10. 故障排查

| 现象 | 原因与解决 |
|------|------------|
| SQL 返回提到 DuckDB 的 `UNSUPPORTED` | 纯 Go 二进制。用 `-tags duckdb` 和 CGO 重新构建 |
| `dev-shell.ps1` 报 `gcc not found at …` | 安装 MSYS2 的 gcc 包，或修正 `$MsysGccBin` |
| `Warning: duckdb.dll not found` | 把 `duckdb.dll`、`duckdb.lib`、`duckdb.h` 放入 `duckdblib/` |
| 构建失败：`cannot find -lduckdb` 或 `duckdb.h` | `duckdblib/` 不完整，或本终端未运行 `dev-shell.ps1` |
| `qllm.exe` 启动即退出，提示缺少 DLL | `duckdb.dll` 不在 `PATH` 也不在 exe 同目录 |
| Linux/macOS：`gcc: command not found` | 安装 C 编译器 |
| `serve` 报 `CONFIG_ERROR` | `--config-dir` 或当前目录下没有 `qllm.preset.yaml` / `qllm.catalog.yaml` |
| 拒绝绑定非回环地址 | 设置 `serve.authTokenEnv`（并导出该变量），仅本地测试时可用 `--insecure-bind` |
| 设置了鉴权环境变量但服务无法启动 | `authTokenEnv` 指向的变量为空 |
| 模拟环境出现 `ErrImagePull` | 在 `k8s.io` 命名空间构建或加载镜像 |
| init-standalone 报 `refusing to overwrite …` | 目录已存在。换一个 `--user` 或加 `--force` |

其他构建细节（固定的 `mcp-go`、何时重建）：[build.md](build.md)。
