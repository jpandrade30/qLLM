# Segurança multi-usuário (chaves com escopo)

O acesso por linha vem da **credencial**, não de um campo em `execute_sql` ou no Query IR. O modelo não escreve o código do usuário.

O `qllm.access.yaml` tem **uma entrada por tipo de app** (`mobile`, `support`, `admin`). Incluir o usuário 43 não exige mudança no YAML.

## Template (padrão)

```yaml
apps:
  - name: mobile
    keySecret: ${QLLM_MOBILE_SECRET}
    tables: [orders, profile, products]
    unscopedTables: [products]
    scope:
      field: user_id
```

O backend gera `mobile.42.<expiryUnix>.<hmac>` e envia em `Authorization: Bearer`. O qLLM valida HMAC e expiração e força `user_id = '42'` nas entidades com `scope`.

MCP stdio: `--app mobile` e `--scope 42` (ou `QLLM_SCOPE`).

## Exceção estática

Poucos principals fixos podem usar `key` + `scope: { user_id: "acme" }`.

Filtro de outro usuário no Query IR: `FORBIDDEN_SCOPE` (`scopeMode: reject`). No SQL de catálogo o escopo é **injetado** no fetch (resultado vazio, não erro). App sem `scope` (admin) não injeta. Entidade sem `scope` no catálogo não é protegida; use `unscopedTables` de forma explícita. RLS no banco continua recomendado.

Grafo LangGraph: compile uma vez; o Bearer vai em `config["configurable"]["qllm_token"]`; cada `execute_sql` abre uma sessão MCP curta. Exemplo: [`deploy/prd/enforced/`](../../deploy/prd/enforced/README.md).

## Constraints no request (D23)

Opcional em `execute_sql` / `POST /v1/sql`: `constraints` + `constraintMode` (`validate` | `inject`). Preferir amarrar o mapa no host. Não substitui o Bearer derivado (D21). Detalhe em [en/multi-user-safety.md](../en/multi-user-safety.md).
