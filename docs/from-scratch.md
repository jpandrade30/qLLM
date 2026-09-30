# Criar um projeto qLLM do zero (não sabes nada)

O qLLM **não** adivinha as tuas tabelas. Tu escreves YAML. O processo lê **uma pasta**. Se essa pasta não for a que pensas, vês o catalog do demo ou `CONFIG_ERROR`.

## 1. O que vais criar

Uma pasta **tua** (não uses `fixtures/` como produto). Nomes exactos — o binário procura estes stems:

```text
C:\dados\meu-qllm\          (exemplo)
  qllm.preset.yaml          OBRIGATÓRIO — bases/APIs + limites
  qllm.catalog.yaml         OBRIGATÓRIO — nomes que o agente pode SELECT
  qllm.config.yaml          recomendado — porta, token, CORS
  qllm.env.yaml             opcional — só para preencher env vazias no PC
  qllm.access.yaml          opcional — vários agentes / allowlist
```

Extensões aceites: `.yaml`, `.yml`, `.json`. **Não** podes inventar `preset.yaml` sem o prefixo `qllm.`.

`protocolVersion` nos YAML: `"0.1.0"` ou `"0.2.0"` (semver `N.N.N`). O runtime **responde** `0.2.0`.

Lista de **todos** os campos: [field-reference.md](field-reference.md). Como apontar a pasta no Docker/K8s: [point-your-folder.md](point-your-folder.md).

## 2. Ordem de trabalho

1. Lista fontes reais (host, porto, user, database **ou** URI **ou** URL).
2. Escreve o **preset**: um `sources[].id` por conexão. `id` só `[a-z][a-z0-9_]*` (ex. `crm_pg`, não `CRM-PG`).
3. Cria variáveis de ambiente com os **nomes** que puseste em `hostEnv` / `passwordEnv` / etc. O YAML **não** leva a senha; leva o **nome** da env (`QLLM_CRM_PG_PASSWORD`).
4. Escreve o **catalog**: uma entidade por “tabela lógica”. `name` = o que vais pôr no `FROM`. `source` = um `id` do preset. `binding` = schema+tabela **físicos** (ou collection/resource).
5. `qllm validate --config-dir …` até dizer `ok` e `entities=N` com o **N que escreveste**.
6. `qllm serve --http --mcp-http --config-dir …`
7. Confirma com health + catalog (passo 5 abaixo). Sem isto não sabes se subiste o YAML errado.

## 3. Exemplo mínimo (uma Postgres)

`qllm.preset.yaml` — cada chave em `connection` que acaba em `Env` é o **nome** de uma variável, não o valor:

```yaml
protocolVersion: "0.2.0"
project: minha-empresa
limits:
  maxSyncMs: 15000
  maxSourceMs: 12000
  defaultLimit: 100
  maxLimit: 1000
  readOnly: true
sources:
  - id: crm_pg
    type: postgres
    connection:
      hostEnv: QLLM_CRM_PG_HOST
      port: 5432
      database: crm
      userEnv: QLLM_CRM_PG_USER
      passwordEnv: QLLM_CRM_PG_PASSWORD
      sslMode: disable
    options:
      statementTimeoutMs: 12000
```

No PowerShell **antes** do serve:

```powershell
$env:QLLM_CRM_PG_HOST = "127.0.0.1"
$env:QLLM_CRM_PG_USER = "app"
$env:QLLM_CRM_PG_PASSWORD = "segredo"
```

`qllm.catalog.yaml` — o agente **nunca** escreve `public.customers`; escreve `customers`:

```yaml
protocolVersion: "0.2.0"
project: minha-empresa
entities:
  - name: customers
    description: Clientes do CRM
    source: crm_pg
    binding:
      kind: table
      schema: public
      table: customers
    primaryKey: [id]
    fields:
      - name: id
        type: string
        physical: id
      - name: email
        type: string
        physical: email
        description: Email único
```

`qllm.config.yaml`:

```yaml
serve:
  addr: "127.0.0.1:8088"
  mcpAddr: "127.0.0.1:8089"
  authTokenEnv: QLLM_AUTH_TOKEN
  insecureBind: false
  maxBodyBytes: 1048576
  maxRestResponseBytes: 10485760
  cors:
    origins: []
```

```powershell
$env:QLLM_AUTH_TOKEN = "um-token-longo"
```

Mais fontes / REST / Dynamo: [field-reference.md](field-reference.md).

## 4. Validar o que geraste (ainda sem servidor)

Na pasta do **código** do qLLM (onde está o binário), aponta para **a tua** pasta:

```powershell
cd C:\codes\qLLM
go build -tags duckdb -o qllm.exe .\cmd\qllm
.\qllm.exe validate --config-dir C:\dados\meu-qllm
```

Queres ver:

```text
ok preset=C:\dados\meu-qllm\qllm.preset.yaml catalog=C:\dados\meu-qllm\qllm.catalog.yaml entities=1
```

- Paths = **os teus** ficheiros (não `deploy\image\config`).
- `entities=` = número de entradas em `entities:` no catalog.
- JSON no stderr com `"code":"CONFIG_ERROR"` = YAML em falta, campo extra (`additionalProperties: false`), `id` inválido, ou `--preset` sem `--catalog`.

Se o validate passar e o catalog tiver `customers`, um IR contra `invoices` tem de falhar:

```powershell
# ficheiro tmp.json: { "from": "invoices", "select": ["id"], "limit": 1 }
.\qllm.exe validate --config-dir C:\dados\meu-qllm --ir C:\dados\tmp.json
```

Esperado: `UNKNOWN_ENTITY`. Se passar, o `--config-dir` **não** é a pasta que pensas.

## 5. Subir e provar que é o **teu** projeto

```powershell
.\qllm.exe serve --http --mcp-http --config-dir C:\dados\meu-qllm
```

Stderr tem de mostrar:

```text
qllm http listening on 127.0.0.1:8088
qllm mcp-http listening on 127.0.0.1:8089 (/mcp streamable, /sse SSE)
```

Noutro terminal:

```powershell
curl.exe -s -H "Authorization: Bearer um-token-longo" http://127.0.0.1:8088/v1/health
curl.exe -s -H "Authorization: Bearer um-token-longo" http://127.0.0.1:8088/v1/catalog
```

O health: `"ok":true` e `"protocolVersion":"0.2.0"`.

O catalog: `"project":"minha-empresa"` (o string do **teu** YAML) e `entities` com `customers`. Se vês `qllm-demo` / `customers`+`invoices` do harness, o processo **não** está a usar `C:\dados\meu-qllm` (esqueceste `--config-dir`, ou Docker montou outra pasta, ou K8s montou o ConfigMap velho).

SQL de fumo (tabela **tua**):

```powershell
curl.exe -s -H "Authorization: Bearer um-token-longo" -H "Content-Type: application/json" `
  -d "{\"sql\":\"SELECT id, email FROM customers LIMIT 5\"}" `
  http://127.0.0.1:8088/v1/sql
```

- `status":"succeeded"` + linhas → liga à base e o catalog bate certo.
- `UNKNOWN_ENTITY` → o SQL usa um `name` que não está no catalog carregado.
- `SOURCE_ERROR` / `TIMEOUT` → YAML ok, rede/credenciais/host errados.
- `UNAUTHORIZED` → token ≠ `QLLM_AUTH_TOKEN` ou header mal formado (`Bearer ` + espaço).

MCP: mesmas provas via tool `describe_catalog` e `execute_sql`. O log do processo mostra `---- execute_sql ----` com o SQL.

## 6. Gerar rascunho em vez de escrever o catalog à mão

Postgres/MySQL já no preset e a env preenchida:

```powershell
.\qllm.exe catalog introspect --source crm_pg --config-dir C:\dados\meu-qllm --out C:\dados\meu-qllm\qllm.catalog.yaml
```

Abre o ficheiro: confirma `source`, `binding.schema`/`table`, `fields`. Acrescenta `relations` e `description`. Volta ao `validate`.

REST: `catalog from-openapi` + colar `options.resources` no preset ([cli.md](cli.md)).

## 7. O que **não** podes pôr nos YAML

- Campos que não estão no schema → validate/serve recusam (`additionalProperties: false` no preset/catalog/config/access/env/project).
- Senha em claro no preset (usa `passwordEnv`).
- `FROM public.customers` no SQL do agente.
- `type: oracle` (não existe).
- CORS `origins: ["*"]`.
- `qllm.access.yaml` com `tables: [customers]` se o catalog não tiver essa entidade.

Segue [field-reference.md](field-reference.md) campo a campo.
