# 01 — Decisions (ADR curto)

Formato: **Decisão** → **Por quê** → **Consequência**.

---

### D01 — Core em Go + DuckDB local

- **Decisão:** Runtime principal em Go; DuckDB só para join/agg/cálculo local pós-fetch.
- **Por quê:** Binário único multiplataforma; concurrency para N fontes; DuckDB é cola analítica sem virar warehouse.
- **Consequência:** Single-source com pushdown nativo pode pular DuckDB. Embed via go-duckdb (ou subprocess se necessário — preferir embed).

### D02 — Contrato público = JSON (preset, catalog, IR, API)

- **Decisão:** Tudo que cruza fronteira é JSON validável (JSON Schema).
- **Por quê:** Agentes e clients tipam/validam; evita dialeto por fonte.
- **Consequência:** Mudança de schema é breaking change versionada (`protocolVersion`).

### D03 — Catálogo lógico ≠ schema físico

- **Decisão:** Agente fala `orders`; preset mapeia para `postgres.sales.orders` / collection Mongo / resource REST.
- **Por quê:** Domínio estável entre projetos; físicos mudam.
- **Consequência:** Catalog é obrigatório no MVP (pode ser manual; introspecção depois).

### D04 — Fail-fast ~15s

- **Decisão:** Budget total de execução sync padrão 15s; timeout por fonte ≤ budget; estouro = erro tipado + cancel.
- **Por quê:** >15s em banco/API é problema de design/ops alheio; não é escopo do qLLM.
- **Consequência:** Sem modo “espera longa”. Async existe para desacoplar HTTP, não para workloads de minutos.

### D05 — Async HTTP como mecanismo de protocolo, não feature de longa duração

- **Decisão:** `POST` aceita query → `202` + `queryId` quando necessário; `GET status/result` com TTL curto.
- **Por quê:** Ingress costuma matar HTTP ~60s; não depender de aumentar timeout alheio.
- **Consequência:** Jobs async também respeitam o mesmo budget ~15s (fail-fast). Async ≠ query longa.

### D06 — Surface mínima de tools

- **Decisão:** `describe_catalog` + `execute_query` (+ `get_query` se async).
- **Por quê:** Elimina N tools por fonte.
- **Consequência:** Descrições ricas no catalog; IR expressivo o suficiente para filtros/proj/agg/join básico.

### D07 — Clients Python/Node = fase posterior

- **Decisão:** MVP só binário + HTTP/MCP. SDKs thin depois.
- **Por quê:** Protocolo estável primeiro reduz churn triplo.
- **Consequência:** Qualquer linguagem usa HTTP/JSON no início.

### D08 — Harness de teste: Rancher Desktop (containerd + K8s)

- **Decisão:** Subir Postgres, MySQL, Mongo e API REST de teste via manifests/compose compatível com Rancher.
- **Por quê:** Ambiente local próximo de produção K8s; APIs/bancos de teste podem ser Python/imagens oficiais.
- **Consequência:** Docs e scripts em `deploy/dev` (a criar); não exigir Go nas fixtures.

### D09 — Fontes v1

- **Decisão:** Postgres, MySQL, MongoDB, REST (OpenAPI/recursos declarados).
- **Por quê:** Cobrem o dia a dia citado.
- **Consequência:** Capability flags por connector; REST sem pretender ser SQL completo.

### D10 — Segurança default

- **Decisão:** Read-only; allowlist de entidades do catalog; `limit` obrigatório; sem raw query livre no caminho do agente.
- **Por quê:** Tool de agente é superfície de risco.
- **Consequência:** Modo admin/raw (se existir) fica explícito e fora do default.

### D11 — Versionamento do protocolo

- **Decisão:** Campo `protocolVersion` (semver string, começar em `0.1.0`).
- **Por quê:** Clients e presets precisam detectar incompatibilidade.
- **Consequência:** Bump minor = additive; major = breaking em IR/API.

### D12 — Nomes lógicos + alias (nunca citar só o campo físico)

- **Decisão:** Com N fontes REST (ou qualquer fonte) compartilhando `id`/`email`, a citação correta é pela **entidade lógica única** (`crm_users.email` vs `erp_users.email`), opcionalmente com **alias de query** (`as: cu` → `cu.email`). `source.id` distingue conexões; `entity.name` distingue o que se consulta.
- **Por quê:** Campo físico igual entre APIs é normal; ambiguidade se resolve no catalog/IR, não no fio HTTP cru.
- **Consequência:** Proibido depender de “o campo email” sem binding; IR com >1 entidade exige FieldRef qualificado (entity ou alias). Ver `03-protocol-schemas.md` § Naming.

### D13 — Spec do projeto = arquivos YAML/JSON lidos pelo executável

- **Decisão:** Preset + catalog vivem em arquivos no disco (YAML ou JSON). O binário recebe `--config-dir` ou paths explícitos / `qllm.project.yaml` que aponta para eles. Em `serve`, carrega uma vez no startup (reload explícito depois, se houver).
- **Por quê:** Maleável por projeto sem recompilar; fácil versionar no git do projeto consumidor.
- **Consequência:** Não há “spec embutida só em código” no caminho feliz. Layout e flags: `03` § Project layout + `02-architecture`.
