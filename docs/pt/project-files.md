# Arquivos do projeto

Tutorial do zero e prova de que você carregou a pasta certa: [from-scratch.md](from-scratch.md). Tabelas campo a campo: [field-reference.md](field-reference.md). Onde apontar em um deploy: [point-your-folder.md](point-your-folder.md).

Um projeto qLLM é um diretório com arquivos YAML ou JSON. Sem eles, o `serve` **não** recorre a `fixtures/` nem a padrões do demo.

## Descoberta

Em ordem (mutuamente exclusivas na prática):

1. `--preset` **e** `--catalog` (os dois; só um retorna `CONFIG_ERROR`).
2. `--project`: um `qllm.project.yaml` cujos caminhos `preset` e `catalog` são relativos a ele (não podem sair do diretório do arquivo de projeto).
3. `--config-dir DIR`: `DIR/qllm.preset.{yaml|yml|json}` mais `DIR/qllm.catalog.{yaml|yml|json}`.
4. Sem `--config-dir`: o **diretório de trabalho atual**.

Arquivos opcionais no mesmo diretório (ou em caminho explícito quando existe flag):

| Arquivo | Obrigatório | Função |
|---------|-------------|--------|
| `qllm.preset.*` | sim | Fontes, `limits`, `connection.*Env` |
| `qllm.catalog.*` | sim | Entidades lógicas, fields, relations, `binding` |
| `qllm.config.*` | não | Bind, `authTokenEnv`, CORS (MCP HTTP), limites |
| `qllm.access.*` | não | Apps, keys, `tables`; **substitui** o token Bearer único |
| `qllm.env.*` | não | Preenche variáveis de ambiente quando a variável do processo está vazia |
| `qllm.project.yaml` | não | Ponteiro para `preset` e `catalog` |

Schemas: [`planning/schemas/`](../../planning/schemas/). Texto explicativo: [`planning/03-protocol-schemas.md`](../../planning/03-protocol-schemas.md).

## Precedência do serve

Padrões seguros (loopback, CORS desativado), depois `qllm.config.yaml`, depois as flags da CLI.

`qllm.env.yaml`: um valor **não vazio** do processo ou de um Secret prevalece. O valor pode ser literal ou exatamente `${NOME}`. Não crie placeholder vazio. **Não registre em log** esses valores.

## Preset: o que você precisa preencher

Obrigatórios: `protocolVersion`, `project`, `limits`, `sources[]` (`id`, `type`, `connection`).

`limits` (padrões da especificação): `maxSyncMs` 15000, `maxSourceMs` 12000, `defaultLimit` 100, `maxLimit` 1000, `readOnly` true.

`sources[].id`: `[a-z][a-z0-9_]*`. `type`: veja [connectors.md](connectors.md).

Segredos somente via `*Env` (ou variável de ambiente com URI). Nunca versione senhas no preset.

Exemplos de `connection` por tipo: seção 1 de `03-protocol-schemas.md`.

## Catálogo: o que o agente enxerga

- `entities[].name` (e `aliases`) são os nomes de tabela no SQL e o `from` do IR.
- `fields[].name` são as colunas lógicas. `physical` mapeia para a coluna ou chave real do documento.
- `binding` aponta para o objeto físico (`table` / `collection` / `rest_resource`, mais `accessPath` para fontes KV e de stream).
- `relations` documentam joins; o SQL ou o IR ainda precisa citá-los corretamente.

O mesmo campo físico em 10 APIs significa **10 entidades** (`crm_users` vs `erp_users`), não um único `users` compartilhado.

## Access (`qllm.access.yaml`)

```yaml
apps:
  - name: crm-agent
    key: ${QLLM_CRM_AGENT_KEY}
    tables: [customers, invoices]
```

- `tables` são nomes de entidades do catálogo.
- HTTP e MCP HTTP: `Authorization: Bearer <key>`.
- MCP stdio, `qllm query` e `qllm sql`: `--app` ou `QLLM_APP` (o nome do app, não a key).
- Arquivo presente: o catálogo e o `howtouseme` são filtrados; uma entidade fora da lista retorna `FORBIDDEN`.
- Arquivo ausente: um único token (`authTokenEnv`) ou nenhuma autenticação no loopback; o catálogo completo é exposto.

## Runtime (`qllm.config.yaml`)

Os campos estão definidos em [`runtime-config.schema.json`](../../planning/schemas/runtime-config.schema.json). `additionalProperties: false`.

| Campo | Padrão / regra |
|-------|----------------|
| `serve.addr` | `127.0.0.1:8088` |
| `serve.mcpAddr` | `127.0.0.1:8089` |
| `serve.authTokenEnv` | Nome da variável do Bearer; se o nome for definido, essa variável **precisa** estar preenchida |
| `serve.insecureBind` | `false`; bind fora do loopback sem autenticação exige `true` ou `--insecure-bind` |
| `serve.maxBodyBytes` | Limite do corpo do POST (mínimo do schema: 1024) |
| `serve.maxRestResponseBytes` | Limite do conector REST |
| `serve.cors.origins` | Vazio significa CORS desativado; `*` é rejeitado |

## Estrutura mínima para implementar

```text
meu-projeto/
  qllm.preset.yaml
  qllm.catalog.yaml
  qllm.config.yaml      # recomendado em qualquer exposição
  qllm.access.yaml      # se houver mais de um agente
  qllm.env.yaml         # apenas local; no K8s use um Secret
```

```bash
export QLLM_…   # tudo o que o preset referencia
./qllm validate --config-dir ./meu-projeto
./qllm serve --http --mcp-http --config-dir ./meu-projeto
```
