# Qué hace qLLM y qué no hace

Un **runtime de consulta de solo lectura** (Go). Lee un **preset** (fuentes, límites, `*Env`) y un **catálogo lógico** (las entidades y campos que un agente puede citar). Ejecuta Query IR (JSON) o SQL de catálogo; los joins y agregaciones que no admiten pushdown pasan al cómputo local (DuckDB en la compilación de la imagen).

Protocolo anunciado en las respuestas: **0.2.0**. Los archivos de preset, catálogo e IR de la versión **0.1.0** siguen siendo válidos.

## Qué puede hacer

- Exponer **HTTP** `/v1` y/o **MCP** (stdio o Streamable HTTP en `/mcp`, más SSE en `/sse`).
- Validar la configuración (`qllm validate`) sin hacer E/S en las fuentes.
- Ejecutar un IR (`qllm query`) o un archivo SQL (`qllm sql`) contra las fuentes del preset.
- Generar un **borrador** de catálogo: `introspect` (postgres/mysql) y `from-openapi` (REST). Debes revisar relations y alias antes de servir.
- Aislar apps con `qllm.access.yaml` (una clave Bearer por app más una lista de entidades permitidas).
- Fallar rápido: presupuesto síncrono habitual de **~15 s** (`limits.maxSyncMs`), timeout por fuente, y después `TIMEOUT` y cancelación.

## Qué no puede hacer (y esto no es un backlog disfrazado)

| Fuera del alcance | Motivo |
|-------------------|--------|
| GraphQL | Nunca será una API de qLLM (D17). |
| SQL crudo del agente contra `public.tabla` o el schema físico | Las tablas son **nombres de entidades** del catálogo. |
| Una tool MCP por tabla, o `execute_query` en MCP | Solo `how_to_use_me`, `describe_catalog` y `execute_sql`. El IR queda en HTTP y en la CLI. |
| Escrituras (`INSERT`/`UPDATE`/…), DDL, `PRAGMA`, `read_csv`, varias sentencias | Solo lectura, más una denylist. |
| Warehouse / Spark / Databricks (`PIVOT`, Unity, `ai_*`, historial Delta) | Un inventario de nombres, no un clon. |
| Scan en Dynamo, `ALLOW FILTERING` en Cassandra, `EMIT CHANGES` en ksql | Sin igualdad en el `accessPath`, el resultado es `UNSUPPORTED`. |
| Esperar minutos por una fuente lenta | Falla rápido; el HTTP asíncrono no es un job de larga duración. |
| SDK de Python/Node en el MVP | HTTP y MCP son la API (fase 2). |
| Introspección de Mongo o de fuentes experimentales | `introspect` solo admite postgres y mysql. |
| Recargar el YAML sin reiniciar | `serve` lo carga todo al arrancar. |
| CORS en el listener HTTP `/v1` | El CORS está en **MCP HTTP**. REST `/v1` no envía cabeceras de CORS. |
| CORS con comodín `*` | Rechazado. |
| Token Bearer en el YAML | Solo por variable de entorno (`authTokenEnv`, `key: ${VAR}`, `qllm.env.yaml`). |

## Superficie del agente

1. `how_to_use_me` / `GET /v1/howtouseme`: la guía más lo que nunca debe inventarse.
2. `describe_catalog` / `GET /v1/catalog`: las entidades (filtradas cuando hay ACL).
3. Consulta con **`execute_sql` / `POST /v1/sql`** (MCP y HTTP), **o** con Query IR en **`POST /v1/queries`** / `qllm query` (no es una tool MCP).

## Dos mundos de datos

| Mundo | Dónde | Entidades típicas |
|-------|-------|-------------------|
| Demo / goldens | `nerdctl compose` más `deploy/image/config` | `customers`, `invoices`, … |
| Simulación fleet-ops | `scripts/prd-tst/prd-tst-up` (`deploy/prd-tst`) | `vehicles`, `depots`, `gps_samples`, … |

No ejecutes ambos a la vez. El proceso **solo ve** el `--config-dir` (o el directorio actual / `--project`). Compose no "inyecta" un catálogo en el binario.
