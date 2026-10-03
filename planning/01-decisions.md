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

- **Decisão:** Agente fala `orders`; preset mapeia para `postgres.sales.orders` / collection Mongo / resource REST. O arquivo de catalog no disco é obrigatório no serve. Pode ser escrito à mão **ou** gerado por `qllm catalog introspect` / `from-openapi` e depois revisado (relations, aliases).
- **Por quê:** Domínio estável entre projetos; físicos mudam. Geração não substitui review nem vira schema em runtime.
- **Consequência:** Sem introspect em cada query. Output é YAML versionado. Mongo sample-collection fica fase posterior (1b).

### D04 — Fail-fast ~15s

- **Decisão:** Budget total de execução sync padrão 15s; timeout por fonte ≤ budget; estouro = erro tipado + cancel.
- **Por quê:** >15s em banco/API é problema de design/ops alheio; não é escopo do qLLM.
- **Consequência:** Sem modo “espera longa”. Async existe para desacoplar HTTP, não para workloads de minutos.

### D05 — Async HTTP como mecanismo de protocolo, não feature de longa duração

- **Decisão:** `POST` aceita query → `202` + `queryId` quando necessário; `GET status/result` com TTL curto.
- **Por quê:** Ingress costuma matar HTTP ~60s; não depender de aumentar timeout alheio.
- **Consequência:** Jobs async também respeitam o mesmo budget ~15s (fail-fast). Async ≠ query longa.

### D06 — Surface mínima de tools

- **Decisão:** MCP: `how_to_use_me`, `describe_catalog`, `execute_sql`. Sem N tools por tabela. Query IR permanece em HTTP (`POST /v1/queries`) e CLI `qllm query`, não como tool MCP.
- **Por quê:** Agente usa um caminho de query (SQL no catalog). IR continua para clients HTTP.
- **Consequência:** Descriptions interpolam o catalog carregado. Sem `execute_query` / `get_query` no MCP.

### D07 — Clients Python/Node = fase posterior

- **Decisão:** MVP só binário + HTTP/MCP. SDKs thin depois.
- **Por quê:** Protocolo estável primeiro reduz churn triplo.
- **Consequência:** Qualquer linguagem usa HTTP/JSON no início.

### D08 — Harness de teste: Rancher Desktop (containerd + compose)

- **Decisão:** Subir Postgres, MySQL, Mongo, API REST e qLLM via `nerdctl compose` (Rancher containerd). Sem path Kubernetes default.
- **Por quê:** Um comando; DNS de serviço igual ao bake `qllm.env.yaml`; APIs/bancos oficiais.
- **Consequência:** `docker-compose.yml` + `Dockerfile`; `fixtures/` só sobe satélites (seed, test-api, golden IR/SQL). Formato/acesso/conexões da instância: **`deploy/image/config`**. Harness de processo ≠ YAML de produto (D18).

### D09 — Fontes v1 + experimentais 0.2.0

- **Decisão:** v1 no harness: Postgres, MySQL, MongoDB, REST. **0.2.0 experimental (sem CI/compose):** `mssql`, `sqlite`, `clickhouse`, `dynamodb`, `cassandra`, `ksql` (só pull). Dynamo/Cassandra/ksql exigem `binding.accessPath`; query sem eq na chave → `UNSUPPORTED`.
- **Por quê:** SQL tabular extra vs KV/stream com access path. Comunidade valida drivers.
- **Consequência:** Capability flags por connector; REST/KV sem pretender ser SQL completo. Harness D08 **não** sobe these engines.

### D10 — Segurança default

- **Decisão:** Read-only; allowlist de entidades do catalog; `limit` obrigatório; Query IR continua o contrato JSON. SQL do agente só no caminho fechado (D15): uma statement `SELECT`, nomes lógicos do catalog, tabelas da key (D16), sem função que leia arquivo/rede.
- **Por quê:** Tool de agente é superfície de risco.
- **Consequência:** Modo admin/raw (se existir) fica explícito e fora do default. `execute_sql` / `POST /v1/sql` não substituem o IR.

### D11 — Versionamento do protocolo

- **Decisão:** Campo `protocolVersion` (semver string, começar em `0.1.0`).
- **Por quê:** Clients e presets precisam detectar incompatibilidade.
- **Consequência:** Bump minor = additive; major = breaking em IR/API. **0.2.0** = novos `sources[].type` + `accessPath`. Arquivos **0.1.0** continuam válidos. Runtime responde `protocolVersion: 0.2.0`.

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
- **Consequência:** Defaults seguros (loopback, CORS off). Precedência: defaults → `qllm.config.*` → flags. Segredo do token só via env. Comparação Bearer: HMAC-SHA256 com pepper de processo (`qllm-bearer-compare-v1`) + `hmac.Equal` (digest de 32 bytes; não ramifica em `len(token)`). Schema: `schemas/runtime-config.schema.json`. `qllm.env.yaml` pode usar `${VAR}` (um token, igual access `key`); processo/Secret ganha; placeholder vazio não é gravado.

### D15 — Dialeto SQL versionado ao lado do Query IR

- **Decisão:** `POST /v1/sql` e MCP `execute_sql` aceitam `{ "version"?, "sql" }`. `version` omitido = dialeto mais novo (`"2"`). `"1"` permanece válido e congelado (sem `UNION`/`INTERSECT`/`EXCEPT`/`QUALIFY`). Desconhecida = `UNSUPPORTED_VERSION`. Query IR **shape** 0.1.0 não muda; o documento de protocolo passa a **0.2.0** só por tipos de source.
- **Por quê:** Parser/joins/transforms no DuckDB após fetch das colunas citadas; IR permanece para clientes existentes. Inventário Databricks-like: [`07-sql-dialect.md`](07-sql-dialect.md).
- **Consequência:** Sem pushdown de `WHERE`/join neste caminho. Build `-tags duckdb` obrigatório para executar SQL. Parser valida tabelas/colunas/ACL; DuckDB executa o `SELECT` (denylist de I/O).

### D16 — Acesso por app (`qllm.access.yaml`)

- **Decisão:** Arquivo opcional lista `apps[]` com `name`, `key` (literal ou `${ENV_NAME}`) e `tables` (nomes de entidade do catalog). Arquivo presente substitui o Bearer único (`authTokenEnv`). A key escolhe o app; a allowlist vale para IR e SQL. MCP stdio usa `--app` / `QLLM_APP`.
- **Por quê:** Kubernetes injeta Secret em env; yaml aponta `${ENV}` sem copiar o valor para o git.
- **Consequência:** Sem o arquivo, comportamento D14 (um token, catalog inteiro). Com o arquivo, catalog/`howtouseme` filtrados às tabelas da key. Escopo por linha (D21) é opcional no mesmo arquivo.

### D17 — Superfície de agente = Query IR + catalog SQL; GraphQL fora

- **Decisão:** MCP: catalog SQL (`execute_sql`) só. HTTP ainda tem Query IR (`POST /v1/queries`) e SQL (`POST /v1/sql`). GraphQL **nunca** é API qLLM.
- **Por quê:** SQL no DuckDB após fetch já cobre expressões ricas; um segundo dialeto de documento (GraphQL) duplica contrato e confunde o agente.
- **Consequência:** HTTP/MCP (`how_to_use_me`, tools) **não** citam GraphQL. Superfície do agente: Query IR + catalog SQL. Spec interna (D17) registra a exclusão; o modelo não recebe o vocabulário.

### D18 — Harness de teste é um mundo separado

- **Decisão:** Compose, seeds e `fixtures/` existem para **provar** o runtime. O processo qLLM **só vê** preset/catalog/config/access/env do `--config-dir` (ou `--preset`+`--catalog` / `--project` / CWD com esses arquivos). Sem YAML descrevendo fonte/entidade/host, isso **não existe** para `describe_catalog`, IR ou SQL.
- **Por quê:** Se o binário ou defaults de produção incorporarem DNS `postgres`, entidades `invoices` do demo, token `change-me`, o próximo ambiente real quebra ou finge que o demo é o produto.
- **Consequência:** Zero lista de hosts/entidades de demo em `internal/`. Sem YAML válido → `CONFIG_ERROR`, nunca fallback para `fixtures/`. A imagem **deste repo** bakeia `deploy/image/config` (o “projeto” desta instância de harness). `fixtures/` não descreve schema. Produção: outro diretório/ConfigMap, não reutilizar seed/compose. Introspect/from-openapi escrevem no config-dir alvo.

### D19 — Fontes chave/stream são só leitura e não destrutivas

- **Decisão:** `redis` e `kafka` (experimentais, sem harness) leem sem alterar dado, chave, offset, grupo ou TTL. Redis: allowlist `GET`/`HGET*`/`HGETALL`/`LRANGE`/`SSCAN`/`ZRANGE`/`XRANGE`/`TYPE`/`EXISTS`; nunca `DEL`/`SET`/`POP*`/`XACK`/`KEYS`. Kafka: fetch direto na partição **sem** consumer group, sem `CommitOffsets`, sem produce. Query sem igualdade no `accessPath` (Redis key; Kafka partition+offset, key, ou timestamp) → `UNSUPPORTED`.
- **Por quê:** Consumer group commita offset no broker; `XACK`/`DEL`/`SET` mudam o store. O agente não pode “marcar como lido” nem apagar.
- **Consequência:** ACL recomendada no Redis/Kafka (só Read/Describe) é a barreira real; o runtime é a segunda. Sem Avro/Protobuf no Kafka (fase 1: JSON/raw). Sem `SCAN`/`KEYS` no Redis. Sem compose.

### D20 — Campo REST omitido pela API (`fromFilter`)

- **Decisão:** `fields[].fromFilter: true` (só entidades cuja fonte é `rest`) diz que a API **não devolve** aquele campo; o runtime preenche a coluna com o valor de um `eq` no topo do WHERE (`eq` sozinho ou `and` de `eq`). Sem esse `eq` → `INVALID_IR`. Se o corpo **trouxer** o campo com valor diferente do filtro → `SOURCE_ERROR`. Valores sob `or` / `not` não alimentam o preenchimento. Não é coluna calculada: só ecoa um filtro que o caller já enviou.
- **Por quê:** APIs de saldo/perfil recebem `user_id` na query/path e devolvem `{"saldo":5300}`. Sem o eco, `GROUP BY` / join na chave viram `null`.
- **Consequência:** schema de catalog ganha `fromFilter` opcional (aditivo; `protocolVersion` 0.2.0). A API continua responsável por filtrar; o qLLM só reconstrói a chave para agregação/join.

### D21 — Escopo na credencial, não na query

- **Decisão:** `qllm.access.yaml` tem **uma entrada por tipo de app** (`mobile`, `admin`), nunca por usuário. O código do usuário vive na chave derivada `app.scopeValue.expiryUnix.hmac` (HMAC-SHA256 do `keySecret`, Base64URL). O catálogo marca entidades com `scope.field` (coluna opcional `scope.column`). O runtime força `eq` nessa coluna. `scopeMode` default `reject` (filtro conflitante → `FORBIDDEN_SCOPE`); `inject` faz AND. Tools de query **sem** campo novo. Variante estática `scope: { user_id: "acme" }` só para poucos principals fixos. MCP stdio: `QLLM_SCOPE` / `--scope`.
- **Por quê:** Se o modelo pudesse passar o `user_id` na tool, trocaria 42 por 7.
- **Consequência:** `key` e `keySecret` são mutuamente exclusivos. App com `scope` precisa que cada `tables[]` tenha `scope` no catalog ou esteja em `unscopedTables`. App sem `scope` (admin) não injeta. Sem denylist; revogação = expiração curta ou rotacionar o segredo. RLS/view no banco continua recomendado.
