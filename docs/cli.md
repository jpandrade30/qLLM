# CLI (`qllm`)

Binário: `go build -o qllm ./cmd/qllm` (produção/imagem: `-tags duckdb` — ver [build.md](build.md)).

Flags comuns de config (quase todos os subcomandos):

| Flag | Uso |
|------|-----|
| `--config-dir` | Pasta com `qllm.preset` + `qllm.catalog` |
| `--preset` / `--catalog` | Paths explícitos (os dois) |
| `--project` | `qllm.project.yaml` |

## `qllm validate`

Valida preset+catalog (sem abrir fontes). Opcional `--ir FILE` valida um Query IR contra o catalog.

```bash
./qllm validate --config-dir ./meu-projeto
./qllm validate --config-dir ./meu-projeto --ir ./consulta.json
```

Stderr em sucesso: `ok preset=… catalog=… entities=N`. Erros tipados em JSON no stderr.

## `qllm query`

Executa um ficheiro Query IR. `--file` / `-f` obrigatório. Abre as fontes.

`--app` / `QLLM_APP` se existir `qllm.access.yaml`.

Aplica `qllm.env.yaml` do config-dir antes de ligar.

```bash
./qllm query --config-dir ./meu-projeto -f ./consulta.json
```

## `qllm sql`

Executa um ficheiro de texto SQL (dialeto de catálogo). `--file` / `-f` obrigatório.

`--version` omitido = latest (`"2"`). `"1"` = dialeto congelado (sem set ops / `QUALIFY`).

`--app` / `QLLM_APP` com ACL.

Requer build com DuckDB embutido para `ExecSQL`.

```bash
./qllm sql --config-dir ./meu-projeto -f ./q.sql
./qllm sql --config-dir ./meu-projeto -f ./q.sql --version 1
```

## `qllm serve`

Sem `--http`, `--mcp` nem `--mcp-http` → **HTTP ligado** (default).

| Flag | Efeito |
|------|--------|
| `--http` | REST `/v1` |
| `--mcp-http` | MCP `/mcp` + `/sse` + `/message` |
| `--mcp` | MCP **stdio** (Inspector local). **Exclusivo** — não mistures com `--http` / `--mcp-http` |
| `--addr` | Listen HTTP (default `127.0.0.1:8088`) |
| `--mcp-addr` | Listen MCP HTTP (default `127.0.0.1:8089`) |
| `--runtime-config` | Path de `qllm.config.yaml` |
| `--auth-token-env` | Nome da env do Bearer |
| `--insecure-bind` | Permite bind não-loopback sem auth |
| `--cors-origin` | Repeatable; allowlist MCP HTTP |
| `--app` | App stdio quando há `qllm.access.yaml` |

```bash
./qllm serve --http --mcp-http --config-dir ./meu-projeto
./qllm serve --mcp --config-dir ./meu-projeto --app crm-agent
```

Stdio + `qllm.access.yaml` sem `--app`/`QLLM_APP` → `CONFIG_ERROR`.

## `qllm catalog introspect`

Lê `information_schema` de uma fonte **postgres ou mysql** do preset. Escreve catalog YAML. **Não serve.**

| Flag | |
|------|--|
| `--source` | `sources[].id` (obrigatório) |
| `--out` | Ficheiro (omisso = stdout) |
| `--merge` | Mantém entidades de outras fontes do catalog já carregado |

Timeout de introspect: 15s. Rever o YAML (relations, descriptions) antes de `serve`.

```bash
./qllm catalog introspect --source crm_pg --config-dir ./meu-projeto --out ./meu-projeto/qllm.catalog.yaml
```

## `qllm catalog from-openapi`

Gera entidades `rest_resource` + fragmento `options.resources` a partir de OpenAPI 3. A fonte `--source` tem de ser `type: rest` no preset.

| Flag | |
|------|--|
| `-f` / `--file` | Spec OpenAPI |
| `--source` | id REST |
| `--out` | Catalog |
| `--resources-out` | Fragmento YAML de resources (senão imprime no stderr) |
| `--merge` | Como no introspect |

O connector REST **só** lê o que já está no preset; colar o fragmento em `sources[].options.resources`.
