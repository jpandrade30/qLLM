# O que o qLLM faz e o que não faz

Um **runtime de consulta somente leitura** (Go). Ele lê um **preset** (fontes, limites, `*Env`) e um **catálogo lógico** (as entidades e os campos que um agente pode citar). Executa Query IR (JSON) ou SQL de catálogo; joins e agregações que não podem sofrer pushdown vão para o processamento local (DuckDB no build da imagem).

Protocolo anunciado nas respostas: **0.2.0**. Arquivos de preset, catálogo e IR da versão **0.1.0** continuam válidos.

## O que ele faz

- Expõe **HTTP** `/v1` e/ou **MCP** (stdio ou Streamable HTTP em `/mcp`, mais SSE em `/sse`).
- Valida a configuração (`qllm validate`) sem fazer I/O nas fontes.
- Executa um IR (`qllm query`) ou um arquivo SQL (`qllm sql`) contra as fontes do preset.
- Gera um **rascunho** de catálogo: `introspect` (postgres/mysql) e `from-openapi` (REST). Você precisa revisar relations e aliases antes de servir.
- Isola apps com `qllm.access.yaml` (uma chave Bearer por app mais uma allowlist de entidades).
- Falha rápido: orçamento síncrono típico de **~15 s** (`limits.maxSyncMs`), timeout por fonte, depois `TIMEOUT` e cancelamento.

## O que ele não faz (e isto não é um backlog disfarçado)

| Fora do escopo | Motivo |
|----------------|--------|
| GraphQL | Nunca será uma API do qLLM (D17). |
| SQL cru do agente contra `public.tabela` ou o schema físico | As tabelas são **nomes de entidades** do catálogo. |
| Uma tool MCP por tabela, ou `execute_query` no MCP | Apenas `how_to_use_me`, `describe_catalog` e `execute_sql`. O IR fica no HTTP e na CLI. |
| Escritas (`INSERT`/`UPDATE`/…), DDL, `PRAGMA`, `read_csv`, várias instruções | Somente leitura, mais uma denylist. |
| Warehouse / Spark / Databricks (`PIVOT`, Unity, `ai_*`, histórico Delta) | Um inventário de nomes, não um clone. |
| Scan no Dynamo, `ALLOW FILTERING` no Cassandra, `EMIT CHANGES` no ksql | Sem igualdade no `accessPath`, o resultado é `UNSUPPORTED`. |
| Esperar minutos por uma fonte lenta | Falha rápido; o HTTP assíncrono não é um job de longa duração. |
| SDK Python/Node no MVP | HTTP e MCP são a API (fase 2). |
| Introspecção de Mongo ou de fontes experimentais | O `introspect` só suporta postgres e mysql. |
| Recarregar o YAML sem reiniciar | O `serve` carrega tudo na inicialização. |
| CORS no listener HTTP `/v1` | O CORS fica no **MCP HTTP**. O REST `/v1` não envia headers de CORS. |
| CORS com curinga `*` | Rejeitado. |
| Token Bearer no YAML | Somente por variável de ambiente (`authTokenEnv`, `key: ${VAR}`, `qllm.env.yaml`). |

## Superfície do agente

1. `how_to_use_me` / `GET /v1/howtouseme`: o guia mais o que nunca inventar.
2. `describe_catalog` / `GET /v1/catalog`: as entidades (filtradas quando há ACL).
3. Consulte com **`execute_sql` / `POST /v1/sql`** (MCP e HTTP), **ou** com Query IR em **`POST /v1/queries`** / `qllm query` (não é uma tool MCP).

## Dois mundos de dados

| Mundo | Onde | Entidades típicas |
|-------|------|-------------------|
| Demo / goldens | `nerdctl compose` mais `deploy/image/config` | `customers`, `invoices`, … |
| Simulação fleet-ops | `scripts/prd-tst/prd-tst-up` (`deploy/prd-tst`) | `vehicles`, `depots`, `gps_samples`, … |

Não rode os dois ao mesmo tempo. O processo **só enxerga** o `--config-dir` (ou o diretório atual / `--project`). O Compose não "injeta" um catálogo no binário.
