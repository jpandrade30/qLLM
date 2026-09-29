# 02 — Architecture

## Componentes

```text
┌─────────────────────────────────────────────────────────┐
│  Clients (agent tool / curl / future py/node SDK)       │
│  HTTP JSON  and/or  MCP (stdio | streamable-http | SSE) │
└──────────────────────────┬──────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────┐
│  qLLM Runtime (Go)                                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │ Preset load │→ │ Catalog     │→ │ IR validate     │  │
│  └─────────────┘  └─────────────┘  └────────┬────────┘  │
│                                             │           │
│  ┌───────────── Planner ────────────────────▼─────────┐ │
│  │ pushdown vs local │ budget │ cancel │ explain meta │ │
│  └─────────────┬───────────────────┬──────────────────┘ │
│                │                   │                    │
│       ┌────────▼────────┐  ┌───────▼────────┐           │
│       │ Connectors      │  │ DuckDB local   │           │
│       │ pg/mysql/mongo  │→ │ join / calc    │           │
│       │ rest            │  │ (se precisar)  │           │
│       └─────────────────┘  └────────────────┘           │
│  ┌────────────────────────────────────────────────────┐ │
│  │ Query store (in-memory v1): id → status/result     │ │
│  └────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
         │              │              │            │
    Postgres        MySQL          Mongo         REST
```

## Fluxo de execução

1. Resolver paths de config (ver abaixo) → carregar **preset** + **catalog** do disco
2. Validar IR contra JSON Schema + catalog (entidades/campos/ops/aliases)
3. Planejar: quais steps pushdown, quais locais; estimar se cabe no budget
4. Executar com cancelamento e timeouts
5. Normalizar para resultado tabular
6. Responder sync ou registrar job async (mesmo budget)

## Onde ficam as specs (preset / catalog)

O harness (`fixtures/` compose/seed) **não** descreve entidades. Só `deploy/image/config` (ou o `--config-dir` de outro projeto) entra no grafo do runtime (D18).

```text
my-app/                         # projeto que USA o qLLM
  qllm.project.yaml             # opcional: aponta preset + catalog
  qllm.preset.yaml              # sources, auth env, limits
  qllm.catalog.yaml             # entidades lógicas → bindings físicos
  qllm.config.yaml              # opcional: bind, Bearer auth, CORS, body caps
  qllm.access.yaml              # opcional: apps, keys (${ENV} ou literal), tabelas
```

Resolução (ordem):

1. Flags explícitas: `--preset PATH` + `--catalog PATH`
2. `--project FILE` (`qllm.project.yaml` com paths relativos ao arquivo)
3. `--config-dir DIR` → `DIR/qllm.preset.yaml` + `DIR/qllm.catalog.yaml` (também aceita `.json`)
4. CWD: `./qllm.preset.yaml` + `./qllm.catalog.yaml`

`qllm.config.*` (opcional): `--runtime-config PATH`, ou ao lado do preset em `--config-dir` / CWD. Precedência serve: defaults seguros → arquivo → flags.

```bash
qllm serve --http --config-dir ./config
qllm serve --http --preset ./p.yaml --catalog ./c.yaml
qllm query --config-dir ./config -f ./ir.json
```

Detalhes normativos: [03-protocol-schemas.md](03-protocol-schemas.md) § Project layout (D14).

## Modos de serving

| Modo | Uso |
|------|-----|
| `qllm serve --http --config-dir …` | API local (default `127.0.0.1:8088`) |
| `qllm serve --mcp --config-dir …` | Agente local via stdio (sem ingress) |
| `qllm serve --mcp-http …` | MCP Streamable HTTP `/mcp` + SSE (default `127.0.0.1:8089`) |
| `qllm serve --http --mcp-http …` | REST `/v1` e MCP HTTP em portas distintas |
| `qllm query --config-dir … -f ir.json` | CLI one-shot (dev) |
| `qllm sql --config-dir … -f query.sql` | CLI SQL (dialeto `"2"` default; DuckDB) |

Auth (Bearer via `authTokenEnv`, ou keys em `qllm.access.yaml`) aplica-se a HTTP `/v1` e MCP HTTP, não a MCP stdio. Com access file, stdio exige `--app` / `QLLM_APP`. CORS allowlist aplica-se **só** a MCP HTTP (REST `/v1` não envia headers CORS).

## Layout de repo (alvo)

```text
qLLM/
  planning/           # specs (esta pasta)
  planning/schemas/   # JSON Schema oficiais
  .cursor/rules/
  .cursor/skills/
  cmd/qllm/           # CLI + serve
  internal/           # runtime (não exportar cedo)
  proto/ or api/      # OpenAPI gerada a partir dos schemas
  deploy/image/config/ # preset, catalog, access, serve, env (formato dos dados)
  fixtures/           # seed, test-api, golden IR/SQL — não catalog
  clients/            # (fase 2) python/, node/
```

## Fronteiras claras

| Responsabilidade do qLLM | Não é do qLLM |
|--------------------------|---------------|
| Validar IR, pushdown básico, fail-fast, tabular uniforme | Indexar banco alheio |
| Mapear lógico→físico via catalog | Modelar domínio do cliente |
| Cancelar ao estourar budget | Jobs ETL de horas |
| Expor catalog ao agente | SQL fora do sandbox D15 (qualquer statement, scan de arquivo, schema físico) |
