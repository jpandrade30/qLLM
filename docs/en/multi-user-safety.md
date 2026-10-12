# Multi-user safety (scoped keys)

Row access is bound to the **credential**, not to a required field on `execute_sql` or the Query IR. The strong path is a derived Bearer so the model never writes the user code.

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

## Behavior (credential scope)

| Query | Result (`scopeMode: reject`, default) |
|-------|----------------------------------------|
| no filter | runs as `user_id = '42'` |
| `user_id = '42'` | same, no extra predicate |
| `user_id = '7'` | `FORBIDDEN_SCOPE` on Query IR; catalog SQL injects on fetch (spoof rows never load) |
| `scopeMode: inject` | forced `AND` wins (often empty) |

An app **without** `scope` (admin) is unchanged. A scoped entity with no value on the key is refused.

`qllm validate` fails if a scoped app lists a table that has no entity `scope` and is not in `unscopedTables`.

## Request constraints (lighter assist, D23)

Optional on `POST /v1/sql` / MCP `execute_sql`:

```json
{
  "sql": "SELECT id FROM orders WHERE user_id = '42' LIMIT 20",
  "constraints": { "user_id": "42" },
  "constraintMode": "validate"
}
```

| Mode | Behavior |
|------|----------|
| `validate` (default when `constraints` is set) | If SQL has an equality on that field with another value → `FORBIDDEN_SCOPE`. Field absent from WHERE → **allowed** (does not stop `SELECT *`). |
| `inject` | Also forces `eq` on the source fetch for entities that own the field; conflicting SQL equality still aborts. |

Prefer binding `constraints` in the **host** (BFF / LangGraph) after auth — do not ask the model to invent the subject. If credential scope already sets field `F`, a constraint for `F` must match or the request fails with `FORBIDDEN_SCOPE`.

| Layer | Use when |
|-------|----------|
| Derived Bearer (D21) | Real multi-tenant; LLM never sees the subject |
| `constraintMode: inject` | Trusted host knows the subject; simpler than HMAC keys |
| `constraintMode: validate` | Extra check that a filter the model wrote matches host intent — **not enough alone** |

## Limits

The catalog is the first layer: an entity without `scope` is unprotected. Columns in allowed rows stay visible. The source account still sees everything — keep RLS or a view in the database.

Catalog SQL (`execute_sql`) always **injects** D21 scope on the source fetch when present. Query IR still uses `scopeMode` (`reject` by default).

## LangGraph / compiled graphs

Compile the graph once. Prefer minted Bearer in `config["configurable"]["qllm_token"]` and a short MCP session per `execute_sql` call (`/mcp` is stateless). If you use `constraints` instead, set the map server-side. Example: [`deploy/prd/enforced/`](../../deploy/prd/enforced/README.md).
