# 多用户安全（作用域密钥）

行级访问绑定在**凭据**上，而不是 `execute_sql` 或 Query IR 的新字段。模型不会写入用户代码。

`qllm.access.yaml` **按应用类型各一条**（`mobile`、`support`、`admin`）。新增用户 43 不必改 YAML。

## 模板（默认）

```yaml
apps:
  - name: mobile
    keySecret: ${QLLM_MOBILE_SECRET}
    tables: [orders, profile, products]
    unscopedTables: [products]
    scope:
      field: user_id
```

后端签发 `mobile.42.<expiryUnix>.<hmac>`，放入 `Authorization: Bearer`。qLLM 校验 HMAC 与过期时间，并在声明了 `scope` 的实体上强制 `user_id = '42'`。

stdio MCP：`--app mobile` 加上 `--scope 42`（或 `QLLM_SCOPE`）。

## 静态例外

少数长期主体可用固定 `key` 和 `scope: { user_id: "acme" }`。

Query IR 过滤其他用户时返回 `FORBIDDEN_SCOPE`（默认 `scopeMode: reject`）。目录 SQL 在拉取时**注入**范围（结果为空）。没有 `scope` 的应用（管理员）不注入。目录中未声明 `scope` 的实体不受保护，需写明 `unscopedTables`。数据库 RLS/视图仍建议作为第二层。

LangGraph：图只编译一次；Bearer 放在 `config["configurable"]["qllm_token"]`；每次 `execute_sql` 打开短 MCP 会话。示例：[`deploy/prd/enforced/`](../../deploy/prd/enforced/README.md)。
