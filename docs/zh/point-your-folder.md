# 如何让 qLLM 指向**你的**目录

二进制文件只会看到你提供给它的 YAML。

| 镜像 | 内嵌到 `/config` 的内容 | 用途 |
|------|--------------------------|------|
| [`Dockerfile`](../../Dockerfile) | [`deploy/prd/`](../../deploy/prd) | 产品 / 示例（`docker build`） |
| [`Dockerfile.dev`](../../Dockerfile.dev) | [`deploy/image/config`](../../deploy/image/config) | Compose 和 K8s 模拟环境所用的二进制 |

## 在你的电脑上（二进制）

```text
qllm serve --http --mcp-http --config-dir 你的目录的绝对路径
```

示例：

```powershell
.\qllm.exe serve --http --mcp-http --config-dir C:\data\my-qllm
.\qllm.exe serve --http --mcp-http --config-dir .\my-qllm
```

等价写法：

```powershell
.\qllm.exe serve --http --preset C:\data\my-qllm\qllm.preset.yaml --catalog C:\data\my-qllm\qllm.catalog.yaml
```

（`--preset` 和 `--catalog` 必须**同时**提供。）

或者使用指针文件 `C:\data\my-qllm\qllm.project.yaml`：

```yaml
protocolVersion: "0.2.0"
preset: qllm.preset.yaml
catalog: qllm.catalog.yaml
```

```powershell
.\qllm.exe serve --http --project C:\data\my-qllm\qllm.project.yaml
```

`preset` 和 `catalog` 是相对于**该**目录的路径；不能使用 `..\outside` 跳出目录。

如果没有 `--config-dir`、`--project` 或显式路径，qLLM 会使用**当前工作目录**。如果你在 `C:\codes\qLLM` 中运行命令，而那里**没有** `qllm.preset.yaml`，就会失败。如果那里恰好有遗留文件，你提供服务的就是这份遗留文件。

`qllm.config.yaml`、`qllm.access.yaml` 和 `qllm.env.yaml` 从**同一个** `--config-dir`（或当前目录）读取。`--runtime-config` 只替换 `qllm.config` 的路径。

## Docker / nerdctl（不要在 Dockerfile 中重新构建 catalog）

镜像已经包含 `CMD ["serve", "--http", "--mcp-http", "--config-dir", "/config"]`。

把**你的**目录挂载到 `/config`（文件名保持 `qllm.*.yaml` 不变）：

```powershell
nerdctl run --rm -p 8088:8088 -p 8089:8089 `
  -v C:\data\my-qllm:/config:ro `
  -e QLLM_AUTH_TOKEN=a-long-token `
  -e QLLM_CRM_PG_HOST=host.docker.internal `
  -e QLLM_CRM_PG_USER=app `
  -e QLLM_CRM_PG_PASSWORD=secret `
  qllm
```

`host.docker.internal` 指的是运行在 Windows 主机上的 Postgres。如果数据库在另一个容器中，请在同一个网络中使用**服务名**，而不是 `127.0.0.1`（这个地址指的是 qLLM 容器自身）。

确认方法：`GET /v1/catalog` 返回你的 `project`。如果仍然显示演示项目，说明 `-v` 挂载没有生效（Windows 路径有误，或者你只挂载了单个文件）。

## 修改镜像**内嵌**的内容（需要重新构建）

产品的 [`Dockerfile`](../../Dockerfile) 会执行：

```text
COPY deploy/prd/default/ /config/
CMD serve --http --mcp-http --config-dir /config
```

编辑 [`deploy/prd/`](../../deploy/prd)，然后运行 `docker build -t qllm .`。指南：[`deploy/prd/README.md`](../../deploy/prd/README.md)。

Harness：编辑 `deploy/image/config/*`，然后运行 `nerdctl compose build` 或 `nerdctl build -f Dockerfile.dev`。

使用 `-v 你的目录:/config` 挂载仍会覆盖内嵌内容。

## Kubernetes 模拟环境（`deploy/prd-tst`）

Deployment **不**使用内嵌的 catalog（`Dockerfile.dev`）。它把 ConfigMap `qllm-config` 挂载到 `/config`。

集群使用的 YAML 位于：

```text
deploy/prd-tst/config/qllm.preset.yaml
deploy/prd-tst/config/qllm.catalog.yaml
deploy/prd-tst/config/qllm.config.yaml
deploy/prd-tst/config/qllm.env.yaml
deploy/prd-tst/config/qllm.access.yaml
```

在 [`deploy/prd-tst/kustomization.yaml`](../../deploy/prd-tst/kustomization.yaml) 中的关联方式：

```yaml
configMapGenerator:
  - name: qllm-config
    files:
      - qllm.preset.yaml=config/qllm.preset.yaml
      - qllm.catalog.yaml=config/qllm.catalog.yaml
      …
```

操作步骤：

1. 替换 `deploy/prd-tst/config/*.yaml`，**或者**修改 kustomize 中的路径。
2. 密钥：[`deploy/prd-tst/k8s/secret.yaml`](../../deploy/prd-tst/k8s/secret.yaml)。
3. 运行 `.\scripts\prd-tst\prd-tst-up.ps1`（或 `./scripts/prd-tst/prd-tst-up.sh`）。重新构建时如需跳过 compose down，请使用 `-SkipComposeDown`。
4. 确认：`GET /v1/catalog` 显示项目 `fleet-ops` 和 `vehicles`。

Pod 参数：`serve --http --mcp-http --config-dir /config`（[`k8s/qllm.yaml`](../../deploy/prd-tst/k8s/qllm.yaml)）。

## Harness Compose

`nerdctl compose` 启动的是**演示**环境。qLLM 服务使用内嵌了 `/config` 的镜像，加上 Compose 的环境变量。要运行**你自己的**环境，请使用另一个 Compose 文件或 `--config-dir`，或者使用 PRD overlay。不要混用。

## 检查清单：“加载的是我写的内容吗？”

| 检查项 | 如果是你的目录，应当看到 |
|--------|--------------------------|
| `validate` 的 stderr 中的 `preset=` / `catalog=` | **你自己的**文件路径 |
| `entities=` | 你的 `entities:` 的数量 |
| `GET /v1/catalog` → `project` | 与两个 YAML 中的 `project:` 相同（preset 和 catalog 必须一致） |
| `entities[].name` 中的名称 | 只有你写的那些 |
| PRD fleet-ops 中的 `SELECT * FROM customers` | `UNKNOWN_ENTITY`（那里没有 `customers`） |
| 日志中的 `---- execute_sql ----` | agent **刚刚**发送的 SQL |

如果 health 正常，但 catalog 是演示的那份：说明**目录、挂载或 ConfigMap 有误**。并不是 qLLM 自己把两个世界混在了一起。
