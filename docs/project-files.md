# Ficheiros de projeto

Tutorial do zero + prova de que carregaste a pasta certa: [from-scratch.md](from-scratch.md). Tabelas campo a campo: [field-reference.md](field-reference.md). Onde apontar no deploy: [point-your-folder.md](point-your-folder.md).

Um projeto qLLM é um diretório de YAML/JSON. Sem estes ficheiros o serve **não** usa `fixtures/` nem defaults de demo.

## Discovery

Por ordem (mutuamente exclusiva no essencial):

1. `--preset` **e** `--catalog` (os dois; um sozinho → `CONFIG_ERROR`).
2. `--project` → `qllm.project.yaml` com paths relativos a `preset` e `catalog` (não saem do diretório do project file).
3. `--config-dir DIR` → `DIR/qllm.preset.{yaml\|yml\|json}` + `DIR/qllm.catalog.{yaml\|yml\|json}`.
4. Sem `--config-dir`: o **CWD**.

Opcionais no mesmo diretório (ou path explícito onde a flag existir):

| Ficheiro | Obrigatório | Função |
|----------|-------------|--------|
| `qllm.preset.*` | sim | Fontes, `limits`, `connection.*Env` |
| `qllm.catalog.*` | sim | Entidades lógicas, fields, relations, `binding` |
| `qllm.config.*` | não | Bind, `authTokenEnv`, CORS (MCP HTTP), caps |
| `qllm.access.*` | não | Apps, keys, `tables` — **substitui** o Bearer único |
| `qllm.env.*` | não | Semeia env se a variável de processo estiver vazia |
| `qllm.project.yaml` | não | Ponteiro `preset` + `catalog` |

Schemas: [`planning/schemas/`](../planning/schemas/). Prosa: [`planning/03-protocol-schemas.md`](../planning/03-protocol-schemas.md).

## Precedência de serve

Defaults seguros (loopback, CORS off) → `qllm.config.yaml` → flags CLI.

`qllm.env.yaml`: processo/Secret **não vazio** ganha. Valor pode ser literal ou exactamente `${NOME}`. Não cries placeholder vazio. **Não logues** estes valores.

## Preset — o que tens de preencher

Obrigatório: `protocolVersion`, `project`, `limits`, `sources[]` (`id`, `type`, `connection`).

`limits` (defaults de spec): `maxSyncMs` 15000, `maxSourceMs` 12000, `defaultLimit` 100, `maxLimit` 1000, `readOnly` true.

`sources[].id`: `[a-z][a-z0-9_]*`. `type`: ver [connectors.md](connectors.md).

Segredos só via `*Env` (ou URI env). Não commits senhas no preset.

Exemplos de `connection` por tipo: secção 1 de `03-protocol-schemas.md`.

## Catalog — o que o agente vê

- `entities[].name` (e `aliases`) são os nomes de tabela no SQL e o `from` do IR.
- `fields[].name` são as colunas lógicas. `physical` mapeia para a coluna/documento real.
- `binding` aponta para o objecto físico (`table`/`collection`/`rest_resource` + `accessPath` quando KV/stream).
- `relations` documentam joins; o SQL/IR ainda tem de os citar correctamente.

Mesmo campo físico em 10 APIs → **10 entidades** (`crm_users` vs `erp_users`), não um `users` partilhado.

## Access (`qllm.access.yaml`)

```yaml
apps:
  - name: crm-agent
    key: ${QLLM_CRM_AGENT_KEY}
    tables: [customers, invoices]
```

- `tables` = nomes de entidade do catalog.
- HTTP/MCP HTTP: `Authorization: Bearer <key>`.
- MCP stdio / `qllm query` / `qllm sql`: `--app` ou `QLLM_APP` (nome do app, não a key).
- Ficheiro presente: catalog e `howtouseme` filtrados; entidade fora da lista → `FORBIDDEN`.
- Ficheiro ausente: um token (`authTokenEnv`) ou sem auth em loopback; catalog completo.

## Runtime (`qllm.config.yaml`)

Campos em [`runtime-config.schema.json`](../planning/schemas/runtime-config.schema.json). `additionalProperties: false`.

| Campo | Default / regra |
|-------|-----------------|
| `serve.addr` | `127.0.0.1:8088` |
| `serve.mcpAddr` | `127.0.0.1:8089` |
| `serve.authTokenEnv` | nome da env do Bearer; se o nome está set, a env **tem** de ser não vazia |
| `serve.insecureBind` | `false` — bind não-loopback sem auth exige `true` ou `--insecure-bind` |
| `serve.maxBodyBytes` | POST body (mín. schema 1024) |
| `serve.maxRestResponseBytes` | teto do connector REST |
| `serve.cors.origins` | vazio = CORS off; `*` rejeitado |

## Layout mínimo para implementar

```text
meu-projeto/
  qllm.preset.yaml
  qllm.catalog.yaml
  qllm.config.yaml      # recomendado em qualquer exposição
  qllm.access.yaml      # se mais do que um agente
  qllm.env.yaml         # só local; em K8s usa Secret
```

```bash
export QLLM_…   # tudo o que o preset referencia
./qllm validate --config-dir ./meu-projeto
./qllm serve --http --mcp-http --config-dir ./meu-projeto
```
