# CLI (`qllm`)

Binário: `go build -o qllm ./cmd/qllm`. Produção e imagem usam `-tags duckdb`; veja [build.md](build.md).

Flags comuns de configuração (quase todos os subcomandos):

| Flag | Uso |
|------|-----|
| `--config-dir` | Pasta com `qllm.preset` e `qllm.catalog` |
| `--preset` / `--catalog` | Caminhos explícitos (os dois são obrigatórios juntos) |
| `--project` | Caminho do `qllm.project.yaml` |

## `qllm validate`

Valida preset e catálogo sem abrir nenhuma fonte. O `--ir FILE` opcional valida um Query IR contra o catálogo.

```bash
./qllm validate --config-dir ./meu-projeto
./qllm validate --config-dir ./meu-projeto --ir ./consulta.json
```

Em caso de sucesso, o stderr mostra `ok preset=… catalog=… entities=N`. Os erros saem como JSON tipado no stderr.

## `qllm query`

Executa um arquivo de Query IR. `--file` / `-f` é obrigatório. Abre as fontes.

Use `--app` ou `QLLM_APP` quando existir `qllm.access.yaml`. App template também precisa de `--scope` / `QLLM_SCOPE`.

Aplica o `qllm.env.yaml` do diretório de configuração antes de conectar.

```bash
./qllm query --config-dir ./meu-projeto -f ./consulta.json
```

## `qllm sql`

Executa um arquivo de texto com SQL de catálogo. `--file` / `-f` é obrigatório.

Sem `--version`, usa o dialeto mais recente (`"2"`). O `"1"` é o dialeto congelado (sem operações de conjunto e sem `QUALIFY`).

`--app` / `QLLM_APP` aplica as ACLs. `--scope` / `QLLM_SCOPE` envia o valor de escopo de um app template.

Exige um build com DuckDB embutido para o `ExecSQL`.

```bash
./qllm sql --config-dir ./meu-projeto -f ./q.sql
./qllm sql --config-dir ./meu-projeto -f ./q.sql --version 1
```

## `qllm serve`

Se nenhum de `--http`, `--mcp` ou `--mcp-http` for informado, o **HTTP fica ativo** por padrão.

| Flag | Efeito |
|------|--------|
| `--http` | REST `/v1` |
| `--mcp-http` | MCP em `/mcp`, além de `/sse` e `/message` |
| `--mcp` | MCP via **stdio** (Inspector local). **Exclusivo**: não combine com `--http` nem `--mcp-http` |
| `--addr` | Endereço de escuta do HTTP (padrão `127.0.0.1:8088`) |
| `--mcp-addr` | Endereço de escuta do MCP HTTP (padrão `127.0.0.1:8089`) |
| `--runtime-config` | Caminho do `qllm.config.yaml` |
| `--auth-token-env` | Nome da variável de ambiente que guarda o token Bearer |
| `--insecure-bind` | Permite bind fora do loopback sem autenticação |
| `--cors-origin` | Repetível; allowlist do MCP HTTP |
| `--app` | Nome do app no stdio quando existe `qllm.access.yaml` |

```bash
./qllm serve --http --mcp-http --config-dir ./meu-projeto
./qllm serve --mcp --config-dir ./meu-projeto --app crm-agent
```

Stdio com `qllm.access.yaml` e sem `--app` / `QLLM_APP` retorna `CONFIG_ERROR`. Apps template também precisam de `--scope` / `QLLM_SCOPE`.

## `qllm catalog introspect`

Lê o `information_schema` de uma fonte **postgres ou mysql** do preset e escreve o YAML do catálogo. **Não** sobe o servidor.

| Flag | Significado |
|------|-------------|
| `--source` | `sources[].id` (obrigatório) |
| `--out` | Arquivo de saída (padrão: stdout) |
| `--merge` | Mantém as entidades de outras fontes do catálogo já carregado |

A introspecção expira após 15 segundos. Revise o YAML (relations, descriptions) antes do `serve`.

```bash
./qllm catalog introspect --source crm_pg --config-dir ./meu-projeto --out ./meu-projeto/qllm.catalog.yaml
```

## `qllm catalog from-openapi`

Gera entidades `rest_resource` e um fragmento `options.resources` a partir de uma especificação OpenAPI 3. O `--source` precisa ser uma fonte `type: rest` no preset.

| Flag | Significado |
|------|-------------|
| `-f` / `--file` | Especificação OpenAPI |
| `--source` | ID da fonte REST |
| `--out` | Catálogo de saída |
| `--resources-out` | Fragmento YAML de resources (senão imprime no stderr) |
| `--merge` | Igual ao `introspect` |

O conector REST lê **apenas** o que já está no preset. Cole o fragmento em `sources[].options.resources`.
