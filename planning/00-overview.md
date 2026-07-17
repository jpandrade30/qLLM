# 00 — Overview

## Problema

Dados interligados em múltiplas fontes (REST, MongoDB, MySQL, Postgres, …). Hoje o custo diário é:

1. N tools por fonte (às vezes várias por fonte só para ler)
2. Transformação/cruzamento/cálculo fora das tools
3. Reconfigurar conexões/auth/instâncias em cada projeto

## Solução

**qLLM**: runtime de query portátil (binário Go) com:

- **Project preset** — fontes, auth, limites em YAML/JSON no disco (`--config-dir` / `--project`)
- **Catalog** — entidades lógicas únicas (desambiguam campos iguais entre APIs); aliases de catalog + `as` no IR
- **Query IR (JSON)** — contrato único de consulta/agregação
- **Adapters** — traduzem IR → Postgres / MySQL / Mongo / REST
- **DuckDB** — join/cálculo local só quando necessário
- **Tool/API surface mínima** — `describe_catalog` + `execute_query` (+ status/result se async)

Clients Python/Node ficam para depois; o protocolo (HTTP e/ou MCP) é a API.

## Objetivos

- Reduzir complexidade do dia: poucas tools, preset reutilizável, IR validável
- Fail-fast: budget curto (~15s); query lenta é problema da fonte/ops, não do qLLM
- Maleável: N instâncias do mesmo tipo (`crm_mongo`, `logs_mongo`, …)
- Testável: harness em Rancher Desktop (containerd + Kubernetes)

## Não-objetivos (v1)

- Federated warehouse / optimizer estilo Trino
- Corrigir performance de bancos de terceiros
- Jobs de minutos / contornar timeout de ingress com conexão eterna
- SDKs Python/Node no MVP
- DSL textual humana (açúcar sobre o IR pode vir depois)
- Raw SQL/Mongo livre como interface principal do agente

## Princípio de produto

> O qLLM protege o usuário e o agente de ficar refém de fonte lenta ou schema confuso. Não é DBA remoto nem motor analítico de longa duração.
