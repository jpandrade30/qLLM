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

O executável **não inventa** conexões em código no caminho feliz. Cada projeto consumidor versiona YAML/JSON:

```text
my-app/                         # projeto que USA o qLLM
  qllm.project.yaml             # opcional: aponta preset + catalog
  qllm.preset.yaml              # sources, auth env, limits
  qllm.catalog.yaml             # entidades lógicas → bindings físicos
```

Resolução (ordem):

1. Flags explícitas: `--preset PATH` + `--catalog PATH`
2. `--project FILE` (`qllm.project.yaml` com paths relativos ao arquivo)
3. `--config-dir DIR` → `DIR/qllm.preset.yaml` + `DIR/qllm.catalog.yaml` (também aceita `.json`)
4. CWD: `./qllm.preset.yaml` + `./qllm.catalog.yaml`

```bash
qllm serve --http --config-dir ./config
qllm serve --http --preset ./p.yaml --catalog ./c.yaml
qllm query --config-dir ./config -f ./ir.json
```

Detalhes normativos: [03-protocol-schemas.md](03-protocol-schemas.md) § Project layout.

## Modos de serving

| Modo | Uso |
|------|-----|
| `qllm serve --http --config-dir …` | API atrás de ingress / local |
| `qllm serve --mcp --config-dir …` | Agente local via stdio (sem ingress) |
| `qllm serve --mcp-http --mcp-addr :8089 …` | MCP Streamable HTTP `/mcp` + SSE `/sse` (Jupyter/LangChain) |
| `qllm serve --http --mcp-http …` | REST `/v1` e MCP HTTP em portas distintas |
| `qllm query --config-dir … -f ir.json` | CLI one-shot (dev) |

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
  deploy/dev/         # K8s/Rancher harness
  fixtures/           # seeds, OpenAPI da API de teste
  clients/            # (fase 2) python/, node/
```

## Fronteiras claras

| Responsabilidade do qLLM | Não é do qLLM |
|--------------------------|---------------|
| Validar IR, pushdown básico, fail-fast, tabular uniforme | Indexar banco alheio |
| Mapear lógico→físico via catalog | Modelar domínio do cliente |
| Cancelar ao estourar budget | Jobs ETL de horas |
| Expor catalog ao agente | Gerar SQL livre sem IR |
