# O que o qLLM faz e o que não faz

Runtime de **consulta só de leitura** (Go). Lê um **preset** (fontes + limites + `*Env`) e um **catálogo lógico** (entidades/campos que o agente pode citar). Executa Query IR (JSON) ou SQL de catálogo; joins/aggs que não fazem pushdown vão para compute local (DuckDB no build da imagem).

Protocolo anunciado nas respostas: **0.2.0**. Ficheiros preset/catalog/IR **0.1.0** continuam válidos.

## Consegue

- Expor **HTTP** `/v1` e/ou **MCP** (stdio ou Streamable HTTP `/mcp` + SSE `/sse`).
- Validar config (`qllm validate`) sem I/O às fontes.
- Correr um IR (`qllm query`) ou um ficheiro SQL (`qllm sql`) contra as fontes do preset.
- Gerar **rascunho** de catalog: `introspect` (postgres/mysql) e `from-openapi` (REST). Tens de rever relations/aliases antes de servir.
- Isolar apps com `qllm.access.yaml` (Bearer por app + allowlist de entidades).
- Falhar depressa: budget sync típico **~15s** (`limits.maxSyncMs`); timeout por fonte; `TIMEOUT` + cancel.

## Não consegue (e não é backlog disfarçado)

| Fora | Porquê |
|------|--------|
| GraphQL | Nunca é API qLLM (D17). |
| SQL cru do agente contra `public.tabela` / schema físico | Tabelas = **nomes de entidade** do catalog. |
| Tool MCP por tabela, `execute_query` no MCP | Só `how_to_use_me`, `describe_catalog`, `execute_sql`. IR fica em HTTP/CLI. |
| Writes (`INSERT`/`UPDATE`/…), DDL, `PRAGMA`, `read_csv`, multi-statement | Read-only + denylist. |
| Warehouse / Spark / Databricks (`PIVOT`, Unity, `ai_*`, Delta history) | Inventário de nomes, não clone. |
| Scan Dynamo / `ALLOW FILTERING` Cassandra / ksql `EMIT CHANGES` | Sem igualdade na `accessPath` → `UNSUPPORTED`. |
| Esperar minutos por fonte lenta | Fail-fast; async HTTP não é job longo. |
| SDK Python/Node no MVP | HTTP/MCP é a API (fase 2). |
| Introspect Mongo / fontes experimentais | `introspect` = postgres e mysql. |
| Recarregar YAML sem restart | Serve carrega no startup. |
| CORS no listener HTTP `/v1` | CORS está no **MCP HTTP**. REST `/v1` não envia CORS. |
| Wildcard CORS `*` | Rejeitado. |
| Token Bearer no YAML | Só env (`authTokenEnv`, `key: ${VAR}`, `qllm.env.yaml`). |

## Superfície do agente

1. `how_to_use_me` / `GET /v1/howtouseme` — guia + o que nunca inventar.
2. `describe_catalog` / `GET /v1/catalog` — entidades (filtradas se houver ACL).
3. Query: **`execute_sql` / `POST /v1/sql`** (MCP e HTTP) **ou** Query IR em **`POST /v1/queries`** / `qllm query` (não é tool MCP).

## Dois mundos de dados

| Mundo | Onde | Entidades típicas |
|-------|------|-------------------|
| Demo / goldens | `nerdctl compose` + `deploy/image/config` | `customers`, `invoices`, … |
| Sim fleet-ops | `scripts/prd-tst-up` (`deploy/prd-tst`) | `vehicles`, `depots`, `gps_samples`, … |

Não corras os dois ao mesmo tempo. O processo **só vê** o `--config-dir` (ou CWD / `--project`). Compose não “injeta” o catalog no binário.
