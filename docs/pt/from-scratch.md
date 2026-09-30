# Criar um projeto qLLM do zero

O qLLM **não** adivinha as suas tabelas. Você escreve o YAML, e o processo lê **uma única pasta**. Se essa pasta não for a que você imagina, aparecerá o catálogo do demo ou um `CONFIG_ERROR`.

## 1. O que você vai criar

Uma pasta **sua** (não use `fixtures/` como produto). Use estes nomes exatos; o binário procura por estes prefixos:

```text
C:\dados\meu-qllm\          (exemplo)
  qllm.preset.yaml          OBRIGATÓRIO: bancos/APIs e limites
  qllm.catalog.yaml         OBRIGATÓRIO: nomes que o agente pode consultar com SELECT
  qllm.config.yaml          recomendado: porta, token, CORS
  qllm.env.yaml             opcional: preenche variáveis de ambiente vazias no seu PC
  qllm.access.yaml          opcional: vários agentes / allowlists
```

Extensões aceitas: `.yaml`, `.yml`, `.json`. Não é possível usar `preset.yaml` sem o prefixo `qllm.`.

`protocolVersion` nos YAML: `"0.1.0"` ou `"0.2.0"` (semver `N.N.N`). O runtime **responde** com `0.2.0`.

Todos os campos: [field-reference.md](field-reference.md). Pasta de exemplo (cópia do demo, embutida pelo `Dockerfile`): [`deploy/prd/`](../../deploy/prd). Docker e Kubernetes: [point-your-folder.md](point-your-folder.md).

## 2. Ordem de trabalho

1. Liste as suas fontes reais (host, porta, usuário, database **ou** URI **ou** URL).
2. Escreva o **preset**: um `sources[].id` por conexão. O `id` deve seguir `[a-z][a-z0-9_]*` (por exemplo `crm_pg`, não `CRM-PG`).
3. Crie as variáveis de ambiente com os **nomes** que você usou em `hostEnv`, `passwordEnv` etc. O YAML nunca contém a senha; ele contém o **nome** da variável (`QLLM_CRM_PG_PASSWORD`).
4. Escreva o **catálogo**: uma entidade por tabela lógica. O `name` é o que vai no `FROM`. O `source` é um `id` do preset. O `binding` é o schema e a tabela **físicos** (ou collection/resource).
5. Rode `qllm validate --config-dir …` até aparecer `ok` e `entities=N` com o **N que você escreveu**.
6. Rode `qllm serve --http --mcp-http --config-dir …`.
7. Confirme com health e catalog (seção 5). Sem isso, não dá para saber se você carregou o YAML errado.

## 3. Exemplo mínimo (um Postgres)

`qllm.preset.yaml`: toda chave de `connection` que termina em `Env` é o **nome** de uma variável, não o valor:

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

No PowerShell, **antes** de subir o servidor:

```powershell
$env:QLLM_CRM_PG_HOST = "127.0.0.1"
$env:QLLM_CRM_PG_USER = "app"
$env:QLLM_CRM_PG_PASSWORD = "segredo"
```

`qllm.catalog.yaml`: o agente **nunca** escreve `public.customers`; ele escreve `customers`:

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
        description: E-mail único
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

Mais fontes, REST e Dynamo: [field-reference.md](field-reference.md).

## 4. Valide o que você escreveu (ainda sem servidor)

Na pasta do **código-fonte** do qLLM (onde está o binário), aponte para a **sua** pasta:

```powershell
cd C:\codes\qLLM
go build -tags duckdb -o qllm.exe .\cmd\qllm
.\qllm.exe validate --config-dir C:\dados\meu-qllm
```

Você deve ver:

```text
ok preset=C:\dados\meu-qllm\qllm.preset.yaml catalog=C:\dados\meu-qllm\qllm.catalog.yaml entities=1
```

- Os caminhos precisam ser dos **seus** arquivos (não de `deploy\image\config`).
- `entities=` é o número de entradas em `entities:` no catálogo.
- JSON no stderr com `"code":"CONFIG_ERROR"` indica YAML ausente, campo extra (`additionalProperties: false`), `id` inválido ou `--preset` sem `--catalog`.

Se a validação passar e o seu catálogo tiver `customers`, um IR contra `invoices` precisa falhar:

```powershell
# arquivo tmp.json: { "from": "invoices", "select": ["id"], "limit": 1 }
.\qllm.exe validate --config-dir C:\dados\meu-qllm --ir C:\dados\tmp.json
```

Esperado: `UNKNOWN_ENTITY`. Se passar, o `--config-dir` **não** é a pasta que você imagina.

## 5. Suba o servidor e prove que é o **seu** projeto

```powershell
.\qllm.exe serve --http --mcp-http --config-dir C:\dados\meu-qllm
```

O stderr deve mostrar:

```text
qllm http listening on 127.0.0.1:8088
qllm mcp-http listening on 127.0.0.1:8089 (/mcp streamable, /sse SSE)
```

Em outro terminal:

```powershell
curl.exe -s -H "Authorization: Bearer um-token-longo" http://127.0.0.1:8088/v1/health
curl.exe -s -H "Authorization: Bearer um-token-longo" http://127.0.0.1:8088/v1/catalog
```

Health: `"ok":true` e `"protocolVersion":"0.2.0"`.

Catalog: `"project":"minha-empresa"` (a string do **seu** YAML) e `entities` contendo `customers`. Se aparecer `qllm-demo` com `customers` e `invoices` do harness, o processo **não** está usando `C:\dados\meu-qllm`. Você esqueceu o `--config-dir`, o Docker montou outra pasta ou o Kubernetes montou um ConfigMap antigo.

SQL de teste rápido (na **sua** tabela):

```powershell
curl.exe -s -H "Authorization: Bearer um-token-longo" -H "Content-Type: application/json" `
  -d "{\"sql\":\"SELECT id, email FROM customers LIMIT 5\"}" `
  http://127.0.0.1:8088/v1/sql
```

- `"status":"succeeded"` com linhas: o banco conectou e o catálogo está correto.
- `UNKNOWN_ENTITY`: o SQL usa um `name` que não está no catálogo carregado.
- `SOURCE_ERROR` / `TIMEOUT`: o YAML está certo; rede, credenciais ou host estão errados.
- `UNAUTHORIZED`: o token é diferente de `QLLM_AUTH_TOKEN` ou o header está malformado (`Bearer ` seguido de espaço).

MCP: as mesmas provas pelas tools `describe_catalog` e `execute_sql`. O log do processo mostra `---- execute_sql ----` com o SQL.

## 6. Gere um rascunho em vez de escrever o catálogo à mão

Com Postgres ou MySQL já no preset e as variáveis de ambiente definidas:

```powershell
.\qllm.exe catalog introspect --source crm_pg --config-dir C:\dados\meu-qllm --out C:\dados\meu-qllm\qllm.catalog.yaml
```

Abra o arquivo e confira `source`, `binding.schema`/`table` e `fields`. Acrescente `relations` e `description`. Depois rode o `validate` de novo.

REST: `catalog from-openapi` e cole `options.resources` no preset ([cli.md](cli.md)).

## 7. O que você **não** pode colocar nos YAML

- Campos que não estão no schema: validate e serve os recusam (`additionalProperties: false` em preset, catalog, config, access, env e project).
- Senha em texto puro no preset (use `passwordEnv`).
- `FROM public.customers` no SQL do agente.
- `type: oracle` (não existe).
- CORS com `origins: ["*"]`.
- `qllm.access.yaml` com `tables: [customers]` quando o catálogo não tem essa entidade.

Siga o [field-reference.md](field-reference.md) campo a campo.
