# CLI (`qllm`)

Binário: `go build -o qllm ./cmd/qllm`. Produção e a imagem usam `-tags duckdb`; veja [build.md](build.md) e [install.md](install.md).

```text
qllm
├── validate                 confere preset + catálogo (+ IR opcional), sem abrir fontes
├── query                    executa um arquivo Query IR
├── sql                      executa um arquivo de SQL de catálogo
├── serve                    HTTP /v1 e/ou MCP
├── catalog
│   ├── introspect           rascunho de catálogo a partir de uma fonte postgres/mysql
│   └── from-openapi         rascunho de entidades rest_resource a partir de um OpenAPI 3
├── help [comando]           ajuda embutida
└── completion               scripts de autocompletar do shell (embutido)
```

`qllm <comando> --help` mostra as flags de qualquer comando. Os nomes abaixo são exatamente os do binário: por exemplo, a flag de bind é `--insecure-bind` (não `--bind-insecure`).

O código de saída é `0` em sucesso e `1` em qualquer erro. Erros tipados vão para o **stderr** em JSON (`{"protocolVersion": …, "error": {…}}`); veja [errors.md](errors.md).

## Como os arquivos são encontrados

`validate`, `query`, `sql`, `serve` e os dois comandos `catalog` compartilham estas quatro flags.

| Flag | Função |
|------|--------|
| `--config-dir DIR` | Pasta onde se procuram `qllm.preset`, `qllm.catalog` e os opcionais `qllm.config`, `qllm.access`, `qllm.env`. Padrão: o diretório atual |
| `--preset ARQ` / `--catalog ARQ` | Caminhos explícitos. **Os dois** são obrigatórios juntos; informar só um é `CONFIG_ERROR` |
| `--project ARQ` | Um `qllm.project.yaml` com `preset:` e `catalog:` relativos a esse arquivo. Caminhos que saem da pasta dele são recusados |

Precedência: `--preset` + `--catalog`, depois `--project`, depois `--config-dir` (ou o CWD). Em uma pasta, cada arquivo é procurado como `.yaml`, `.yml` e `.json`, nessa ordem. Preset ou catálogo ausente é `CONFIG_ERROR`.

Os arquivos opcionais (`qllm.config.*`, `qllm.access.*`, `qllm.env.*`) só são procurados em `--config-dir` (ou no CWD). `--preset`, `--catalog` e `--project` não mudam essa busca.

### Arquivo de ambiente (`qllm.env.yaml`)

`query`, `sql`, `serve` e os dois `catalog` aplicam o `qllm.env.yaml` antes de conectar. **O `validate` não aplica**, pois não abre fontes. Regras:

- Só variáveis **vazias ou ausentes** no processo são definidas; o ambiente real sempre vence.
- O valor é um literal ou exatamente `${OUTRO_NOME}`, lido do processo. Um `${OUTRO_NOME}` ausente é ignorado, não gravado como texto. Qualquer outro uso de `${` é `CONFIG_ERROR`.

### Variáveis de ambiente lidas pelo próprio binário

| Variável | Usada por | Significado |
|----------|-----------|-------------|
| `QLLM_APP` | `query`, `sql`, `serve` | Igual a `--app` (a flag vence) |
| `QLLM_SCOPE` | `query`, `sql`, `serve` | Igual a `--scope` (a flag vence) |
| o nome em `serve.authTokenEnv` | `serve` | Guarda o Bearer token compartilhado |
| cada chave `*Env` do preset | quem abre fontes | Segredos de conexão, ex.: `QLLM_CRM_PG_PASSWORD` |

## `qllm validate`

Valida preset e catálogo sem abrir nenhuma fonte.

| Flag | Significado |
|------|-------------|
| flags de config | veja acima |
| `--ir ARQ` | Valida também este Query IR contra o catálogo |

```bash
./qllm validate --config-dir ./my-project
./qllm validate --config-dir ./my-project --ir ./query.json
```

Em sucesso o stderr mostra `ok preset=… catalog=… entities=N` (e `ok ir=…`). Falhas são JSON tipado no stderr.

## `qllm query`

Executa um arquivo Query IR e imprime a resposta JSON no stdout. Abre as fontes.

| Flag | Significado |
|------|-------------|
| flags de config | veja acima |
| `-f`, `--file ARQ` | **Obrigatória.** IR em JSON ou YAML |
| `--app NOME` | App do `qllm.access.yaml` (ou `QLLM_APP`) |
| `--scope VALOR` | Valor do escopo de linha para um app template (ou `QLLM_SCOPE`) |

Quando existe `qllm.access.yaml`, as tabelas são conferidas contra o app. Um app template (`keySecret`) também exige `--scope`. Este comando **não** lê o `qllm.config.yaml`, então as respostas REST usam o limite padrão de 10 MiB.

```bash
./qllm query --config-dir ./my-project -f ./query.json
```

## `qllm sql`

Executa um arquivo de SQL de catálogo e imprime a resposta JSON. Exige build com DuckDB embutido (`-tags duckdb`).

| Flag | Significado |
|------|-------------|
| flags de config | veja acima |
| `-f`, `--file ARQ` | **Obrigatória.** Arquivo de texto com o SQL |
| `--version V` | Dialeto SQL. Omitida = mais recente (`"2"`). `"1"` é congelado (sem operações de conjunto, sem `QUALIFY`) |
| `--app`, `--scope` | Iguais aos de `query` |

```bash
./qllm sql --config-dir ./my-project -f ./q.sql
./qllm sql --config-dir ./my-project -f ./q.sql --version 1
```

## `qllm serve`

Inicia um ou mais listeners. Sem nenhuma de `--http`, `--mcp`, `--mcp-http`, o **HTTP fica ligado**.

### Modos

| Flag | Efeito |
|------|--------|
| `--http` | REST `/v1` (`howtouseme`, `catalog`, `queries`, `sql`, `health`) |
| `--mcp-http` | MCP Streamable HTTP em `/mcp`, SSE em `/sse` e `/message` |
| `--mcp` | MCP via **stdio** para clientes locais (Inspector). Exclusiva: combinar com `--http` ou `--mcp-http` é erro. Não abre listener de rede, então as regras de bind e token abaixo não se aplicam |

`--http` e `--mcp-http` podem rodar juntos no mesmo processo.

### Flags de escuta e segurança

| Flag | Padrão | Significado |
|------|--------|-------------|
| `--addr HOST:PORTA` | `127.0.0.1:8088` | Endereço do HTTP `/v1` |
| `--mcp-addr HOST:PORTA` | `127.0.0.1:8089` | Endereço do MCP HTTP |
| `--runtime-config ARQ` | `qllm.config.*` em `--config-dir` | Arquivo de runtime explícito |
| `--auth-token-env NOME` | nenhum | Nome da variável com o Bearer token compartilhado. Se definida, essa variável **precisa estar não vazia** ou o `serve` falha com `CONFIG_ERROR` |
| `--insecure-bind` | `false` | Permite endereço **não loopback** **sem** auth. Veja abaixo |
| `--cors-origin ORIGEM` | nenhuma (CORS desligado) | Origem de navegador permitida no MCP HTTP. Repetível. `*` é recusado |
| `--app NOME` | nenhum | App para MCP stdio quando existe `qllm.access.yaml` (ou `QLLM_APP`) |
| `--scope VALOR` | nenhum | Escopo de linha de um app template no stdio (ou `QLLM_SCOPE`) |

### Precedência

Padrões embutidos → `qllm.config.yaml` → flags. Uma flag só conta se você de fato a passar; assim `--insecure-bind=false` pode sobrescrever `insecureBind: true` do arquivo, e `--cors-origin` substitui as origens do arquivo.

### A regra de bind e o `--insecure-bind`

O qLLM recusa escutar em endereço não loopback, a menos que uma destas condições valha:

1. Há token configurado (`serve.authTokenEnv` / `--auth-token-env`, não vazio), **ou**
2. Existe `qllm.access.yaml` (as chaves são a autenticação), **ou**
3. `--insecure-bind` / `serve.insecureBind: true` está ativo.

Loopback significa `127.0.0.1`, `::1` ou `localhost`. **Não** são loopback e ativam a regra: `0.0.0.0:8088`, `[::]:8088`, `:8088`, qualquer IP de LAN e qualquer hostname diferente de `localhost`. Containers precisam disso porque têm de escutar em `0.0.0.0`.

O `--insecure-bind` **não** desliga a autenticação. Ele só remove a recusa na inicialização. Se houver token ou arquivo de acesso, as requisições continuam sendo verificadas. Serve para uma rede confiável que você protege de outra forma (rede privada do compose, service mesh, proxy reverso que autentica). Sem nenhuma auth, quem alcançar a porta consulta tudo o que o catálogo expõe. A checagem roda por listener, para `--http` e para `--mcp-http`.

Sem token e com endereço loopback, a API fica aberta para processos locais. Essa é a postura padrão.

### Opções só no `qllm.config.yaml`

Não têm flag. Schema: [planning/schemas/runtime-config.schema.json](../../planning/schemas/runtime-config.schema.json).

| Chave | Padrão | Significado |
|-------|--------|-------------|
| `serve.maxBodyBytes` | 1048576 (1 MiB) | Corpo máximo da requisição, HTTP e MCP HTTP |
| `serve.maxRestResponseBytes` | 10485760 (10 MiB) | Corpo máximo lido de uma fonte `rest` |
| `serve.cors.allowHeaders` / `allowMethods` | embutidos | Substituem as listas de CORS |

### Exemplos

```bash
./qllm serve --http --mcp-http --config-dir ./my-project
./qllm serve --http --addr 0.0.0.0:8088 --auth-token-env QLLM_AUTH_TOKEN --config-dir ./my-project
./qllm serve --mcp --config-dir ./my-project --app crm-agent
./qllm serve --mcp --config-dir ./my-project --app crm-agent --scope 42
```

Stdio com `qllm.access.yaml` e sem `--app` / `QLLM_APP` retorna `CONFIG_ERROR`. Um app template também exige `--scope` / `QLLM_SCOPE`.

Logs: cada `execute_sql` imprime um bloco `---- execute_sql ----` no stderr; as outras duas tools imprimem `---- mcp_tool ----`.

## `qllm catalog introspect`

Lê o `information_schema` de uma fonte **postgres ou mysql** do preset e escreve o YAML do catálogo. Não serve.

| Flag | Significado |
|------|-------------|
| flags de config | veja acima |
| `--source ID` | **Obrigatória.** `sources[].id` |
| `--out ARQ` | Arquivo de saída (padrão: stdout) |
| `--merge` | Mantém as entidades de outras fontes do catálogo já carregado e troca só as desta fonte |

Expira em 15 segundos. Revise o YAML (relações, descrições) antes do `serve`.

```bash
./qllm catalog introspect --source crm_pg --config-dir ./my-project --out ./my-project/qllm.catalog.yaml
```

## `qllm catalog from-openapi`

Gera entidades `rest_resource` e um fragmento `options.resources` a partir de um OpenAPI 3. `--source` deve ser uma fonte `type: rest` do preset.

| Flag | Significado |
|------|-------------|
| flags de config | veja acima |
| `-f`, `--file ARQ` | **Obrigatória.** OpenAPI 3 em YAML ou JSON |
| `--source ID` | **Obrigatória.** Id da fonte REST |
| `--out ARQ` | Saída do catálogo (padrão: stdout) |
| `--resources-out ARQ` | Grava o fragmento de resources (senão sai no stderr) |
| `--merge` | Igual ao de `introspect` |

O conector REST lê **apenas** o que já está no preset. Cole o fragmento em `sources[].options.resources`.
