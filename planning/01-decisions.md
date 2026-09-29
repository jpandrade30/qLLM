# 01 — Decisions (ADR curto)

Formato: **Decisão** → **Por quê** → **Consequência**.

---

### D01 — Core em Go + compute local (DuckDB-ready)

- **Decisão:** Runtime principal em Go; joins/aggs cross-source no motor local (`internal/duckdblocal`). MVP usa implementação pure Go (sem CGO) com a mesma interface; DuckDB embutido pode substituir depois.
- **Por quê:** Binário único multiplataforma; concurrency para N fontes; evita bloqueio de toolchain CGO no Windows.
- **Consequência:** Single-source com pushdown nativo pode pular o motor local. Flag `meta.plan.usedDuckDB` indica compute local.
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

- **Decisão:** Read-only; allowlist de entidades do catalog; `limit` obrigatório; Query IR continua o contrato JSON. SQL do agente só no caminho fechado (D15): uma statement `SELECT`, nomes lógicos do catalog, tabelas da key (D16), sem função que leia arquivo/rede.
- **Por quê:** Tool de agente é superfície de risco.
- **Consequência:** Modo admin/raw (se existir) fica explícito e fora do default. `execute_sql` / `POST /v1/sql` não substituem o IR.

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

### D14 — Runtime serve config separado do preset (data-plane)

- **Decisão:** Bind addresses, Bearer auth (`authTokenEnv`), CORS allowlist, body size caps e `insecureBind` vivem em `qllm.config.yaml` (opcional) ou flags CLI — **não** no preset. Preset continua sources + limits + `*Env` de conexões.
- **Por quê:** Preset é o contrato de dados (Query IR / catalog); serve é superfície de exposição. Misturar CORS/auth no preset acopla deploy a catalogs versionados.
- **Consequência:** Defaults seguros (loopback, CORS off). Precedência: defaults → `qllm.config.*` → flags. Segredo do token só via env. Schema: `schemas/runtime-config.schema.json`.

### D15 — Dialeto SQL versionado ao lado do Query IR

- **Decisão:** `POST /v1/sql` e MCP `execute_sql` aceitam `{ "version"?, "sql" }`. `version` omitido = dialeto mais novo (`"2"`). `"1"` permanece válido e congelado (sem `UNION`/`INTERSECT`/`EXCEPT`/`QUALIFY`). Desconhecida = `UNSUPPORTED_VERSION`. Query IR e `protocolVersion` `0.1.0` não mudam.
- **Por quê:** Parser/joins/transforms no DuckDB após fetch das colunas citadas; IR permanece para clientes existentes. Inventário Databricks-like: [`07-sql-dialect.md`](07-sql-dialect.md).
- **Consequência:** Sem pushdown de `WHERE`/join neste caminho. Build `-tags duckdb` obrigatório para executar SQL. Parser valida tabelas/colunas/ACL; DuckDB executa o `SELECT` (denylist de I/O).

### D16 — Acesso por app (`qllm.access.yaml`)

- **Decisão:** Arquivo opcional lista `apps[]` com `name`, `key` (literal ou `${ENV_NAME}`) e `tables` (nomes de entidade do catalog). Arquivo presente substitui o Bearer único (`authTokenEnv`). A key escolhe o app; a allowlist vale para IR e SQL. MCP stdio usa `--app` / `QLLM_APP`.
- **Por quê:** Kubernetes injeta Secret em env; yaml aponta `${ENV}` sem copiar o valor para o git.
- **Consequência:** Sem o arquivo, comportamento D14 (um token, catalog inteiro). Com o arquivo, catalog/`howtouseme` filtrados às tabelas da key.
