# Multi-user safety (scoped keys)

Row access is bound to the **credential**, not to a field on `execute_sql` or the Query IR. The model never writes the user code.

`qllm.access.yaml` holds **one entry per app type** (`mobile`, `support`, `admin`). Adding user 43 does not require a YAML change.

## Template (default)

```yaml
apps:
  - name: mobile
    keySecret: ${QLLM_MOBILE_SECRET}
    tables: [orders, profile, products]
    unscopedTables: [products]
    scope:
      field: user_id
```

The customer's backend mints `mobile.42.<expiryUnix>.<hmac>` (HMAC-SHA256 of `mobile.42.<expiry>` with `keySecret`, Base64URL) and puts it in `Authorization: Bearer`. qLLM verifies HMAC and expiry, then forces `user_id = '42'` on every entity that declares `scope`.

```yaml
entities:
  - name: orders
    scope: { field: user_id }
```

MCP stdio has no header: `--app mobile` plus `--scope 42` (or `QLLM_SCOPE`).

## Static exception

A few long-lived principals (one partner) may use a fixed key and `scope: { user_id: "acme" }`.

## Behavior

| Query | Result (`scopeMode: reject`, default) |
|-------|----------------------------------------|
| no filter | runs as `user_id = '42'` |
| `user_id = '42'` | same, no extra predicate |
| `user_id = '7'` | `FORBIDDEN_SCOPE` |
| `scopeMode: inject` | forced `AND` wins (often empty) |

An app **without** `scope` (admin) is unchanged. A scoped entity with no value on the key is refused.

`qllm validate` fails if a scoped app lists a table that has no entity `scope` and is not in `unscopedTables`.

## Limits

The catalog is the first layer: an entity without `scope` is unprotected. Columns in allowed rows stay visible. The source account still sees everything — keep RLS or a view in the database.

Catalog SQL (`execute_sql`) always **injects** the scope on the source fetch. A spoof `WHERE user_id = '7'` does not return `FORBIDDEN_SCOPE` on that path; other users' rows are never loaded, so the result is empty. Query IR still uses `scopeMode` (`reject` by default).

## LangGraph / compiled graphs

Compile the graph once. Do not put the user code on the tool schema. Pass the minted Bearer in `config["configurable"]["qllm_token"]` and open a short MCP session per `execute_sql` call (`/mcp` is stateless). Example: [`deploy/prd/enforced/`](../../deploy/prd/enforced/README.md).
