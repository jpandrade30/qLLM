# qLLM 实施手册

**打算从零手写所有 `qllm.*` 文件？** 请先阅读 [from-scratch.md](from-scratch.md) 和 [field-reference.md](field-reference.md)。示例目录位于 [`deploy/prd/`](../../deploy/prd)，另请参阅 [point-your-folder.md](point-your-folder.md)。Kubernetes 模拟环境位于 `deploy/prd-tst`。

规范性契约（JSON Schema、Query IR、错误）仍保留在 [`planning/`](../../planning/) 和 [`planning/schemas/`](../../planning/schemas/) 中。本手册是操作指南。两者不一致时，**以 `planning/` 为准**。

| 文档 | 内容 |
|------|------|
| [from-scratch.md](from-scratch.md) | 创建自己的目录、Postgres 示例、`validate`，以及如何**确认**加载的是正确的 YAML |
| [field-reference.md](field-reference.md) | 所有字段、枚举、`*Env` 键和 REST 选项 |
| [point-your-folder.md](point-your-folder.md) | CLI、`Dockerfile` 与 `.dev` 的区别、`deploy/prd` 与 `prd-tst` 的区别 |
| [scope.md](scope.md) | 产品是什么、不是什么 |
| [project-files.md](project-files.md) | 文件发现与优先级 |
| [cli.md](cli.md) | 所有命令和参数 |
| [http-mcp.md](http-mcp.md) | HTTP `/v1`、MCP、认证、绑定与 CORS |
| [queries.md](queries.md) | 目录 SQL 与 Query IR：接受什么、拒绝什么 |
| [connectors.md](connectors.md) | 数据源类型、绑定与下推 |
| [errors.md](errors.md) | 类型化错误码 |
| [multi-user-safety.md](multi-user-safety.md) | 凭据上的行范围（D21） |
| [environments.md](environments.md) | Compose、镜像、PRD 模拟环境与脚本 |
| [build.md](build.md) | Go、`-tags duckdb` 与 Docker |

请先阅读 [scope.md](scope.md) 和 [project-files.md](project-files.md)。如果没有有效的 preset 和 catalog，二进制文件会以 `CONFIG_ERROR` 失败；不会回退到 `fixtures/`。

其他语言：[English](../en/README.md) · [Português](../pt/README.md) · [Español](../es/README.md)
