# 04 — Connectors

## Capability matrix (v0.1)

| Capacidade | postgres | mysql | mongodb | rest |
|------------|----------|-------|---------|------|
| filter (eq/cmp/in/null) | yes | yes | yes | partial (params) |
| project | yes | yes | yes | yes (client-side ok) |
| orderBy | yes | yes | yes | partial |
| limit/offset | yes | yes | yes | partial |
| agg + groupBy | yes | yes | yes (pipeline) | **no** → DuckDB |
| join same source | yes | yes | no → DuckDB | no → DuckDB |
| join cross source | DuckDB | DuckDB | DuckDB | DuckDB |
| writes | no (MVP) | no | no | no |

## Pushdown vs DuckDB

1. Planner gera plano por entidade/fonte.
2. Se todas as ops do step são `yes` na matrix → pushdown.
3. Se join cross-source ou REST agg → fetch com filter/limit máximos → DuckDB.
4. Se op pedida é impossível sem scan absurdo → `UNSUPPORTED` (não “puxar a tabela inteira”).

Authoring (não é query): `qllm catalog from-openapi` gera entities `rest_resource` e um fragmento `options.resources` a partir de GET listáveis. O connector continua lendo só o YAML já no preset.

## Bindings físicos

| type | `binding.kind` | Campos |
|------|----------------|--------|
| postgres/mysql | `table` | `schema`, `table` |
| mongodb | `collection` | `collection` |
| rest | `rest_resource` | `resource` (chave em `sources[].options.resources`) |

## Auth suportada (connection)

- **SQL:** user/password via env; `sslMode` postgres
- **Mongo:** `uriEnv` (credenciais na URI)
- **REST:** `none` | `bearer` (tokenEnv) | `header` (name + valueEnv) | `basic` (userEnv/passwordEnv)

## Timeouts

Cada connector aplica `min(options.timeoutMs|statementTimeoutMs, limits.maxSourceMs)` e propaga cancel do context Go.
