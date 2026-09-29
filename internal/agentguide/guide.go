package agentguide

import "qLLM/internal/protocol"

// Build returns the LLM-oriented how-to-use-me guide for the loaded preset/catalog.
func Build(preset *protocol.Preset, catalog *protocol.Catalog) protocol.HowToUseMeResponse {
	entities := make([]string, 0, len(catalog.Entities))
	for _, e := range catalog.Entities {
		entities = append(entities, e.Name)
	}
	limits := preset.Limits

	return protocol.HowToUseMeResponse{
		ProtocolVersion: protocol.ProtocolVersion,
		Purpose: "qLLM is a read-only multi-source query runtime. Two agent surfaces: (1) JSON Query IR via execute_query / POST /v1/queries; " +
			"(2) catalog SQL via execute_sql / POST /v1/sql (DuckDB after fetch; tables = catalog entity names). " +
			"Call how_to_use_me, then describe_catalog, then choose IR or SQL.",
		Workflow: []string{
			"1. Call how_to_use_me (HTTP GET /v1/howtouseme) — IR contract + SQL dialect guide.",
			"2. Call describe_catalog (GET /v1/catalog) — copy entity/field/relation names exactly.",
			"3a. Simple filters/joins/aggs → execute_query with Query IR ({op,args}, CompareOp eq/gte/…).",
			"3b. Windows, HAVING, CASE, UNION, LIKE, arithmetic → execute_sql (version omitted = latest \"" + protocol.SQLDialectLatest + "\").",
			"4. On error, read error.code + message; fix IR shape or SQL — never paste SQL into execute_query.",
		},
		Never: []string{
			"Never generate MongoDB queries, GraphQL, or raw REST URLs as the agent API.",
			"Never send SQL except POST /v1/sql / execute_sql using catalog entity names as tables.",
			"Never invent entity names, field names, or join keys — use the catalog only.",
			"Never write where as {\"and\":[...]} or {\"or\":[...]} — use {\"op\":\"and\",\"args\":[...]}.",
			"Never use SQL operators (=, !=, >=, LIKE, BETWEEN) in Query IR — use eq, neq, gte, contains, in, …",
			"Never invent camelCase fields (customerId) when the catalog has snake_case (customer_id).",
			"Never put SQL-only features (HAVING, UNION, CASE, subqueries, windows) in execute_query — use execute_sql.",
			"Never invent aliases that are not introduced by as or entity name in this query (Query IR).",
			"Never mix bare fields + aggregates in IR select without putting every bare field in groupBy.",
			"Never use schema.table (public.customers) or physical DB names — catalog entity names only.",
		},
		NotSupported: []string{
			"Query IR: HAVING, DISTINCT, UNION, CASE, subqueries, LIKE/BETWEEN, XOR, computed select expressions",
			"SQL dialect \"1\": UNION / INTERSECT / EXCEPT / QUALIFY (omit version or set \"2\")",
			"Mutations (INSERT/UPDATE/DELETE/MERGE) and file/network table functions (read_csv, httpfs, …)",
			"Schema-qualified tables (public.orders) — catalog entity names only",
			"OFFSET without LIMIT on SQL path: runtime injects defaultLimit; unbounded scan is not allowed",
		},
		Endpoints: []protocol.HowToEndpoint{
			{Method: "GET", Path: "/v1/howtouseme", Use: "This guide (IR + SQL). Call first."},
			{Method: "GET", Path: "/v1/health", Use: "Liveness + protocolVersion."},
			{Method: "GET", Path: "/v1/catalog", Use: "Authoritative entities, fields, relations, sources."},
			{Method: "POST", Path: "/v1/queries", Use: "Execute Query IR (default mode=sync)."},
			{Method: "POST", Path: "/v1/sql", Use: "Execute catalog SQL. Body {sql, version?}. version omitted = latest \"" + protocol.SQLDialectLatest + "\"."},
			{Method: "GET", Path: "/v1/queries/{queryId}", Use: "Poll async query status."},
			{Method: "GET", Path: "/v1/queries/{queryId}/result", Use: "Fetch async result when ready."},
		},
		Grammar: "" +
			"Query = protocolVersion? from as? joins? select where? groupBy? orderBy? limit? offset? mode?\n" +
			"Join = { type: inner|left, from, as?, on: [{left,right}] }\n" +
			"BoolExpr = {field,op,value?} | {op:and|or, args:[BoolExpr+]} | {op:not, args:[BoolExpr]}\n" +
			"AggExpr = {agg:count|sum|avg|min|max, field?, as}\n" +
			"CompareOp = eq|neq|gt|gte|lt|lte|in|nin|contains|is_null|not_null\n" +
			"FieldRef = field | binding.field\n" +
			"SQL = SELECT … FROM <catalog_entity> … (dialect " + protocol.SQLDialectLatest + "; see sql section)",
		Where: protocol.HowToWhereGuide{
			Shapes: []string{
				`{"field":"status","op":"eq","value":"paid"}`,
				`{"op":"and","args":[BoolExpr, BoolExpr, ...]}`,
				`{"op":"or","args":[BoolExpr, BoolExpr, ...]}`,
				`{"op":"not","args":[BoolExpr]}`,
			},
			ValidExample: map[string]any{
				"op": "and",
				"args": []any{
					map[string]any{"field": "status", "op": "eq", "value": "paid"},
					map[string]any{"field": "total", "op": "gte", "value": 10},
				},
			},
			InvalidExample: map[string]any{
				"and": []any{
					map[string]any{"field": "status", "op": "eq", "value": "paid"},
				},
			},
			Fix: map[string]any{
				"op": "and",
				"args": []any{
					map[string]any{"field": "status", "op": "eq", "value": "paid"},
				},
			},
		},
		FieldRefRules: []string{
			"Single-entity query: bare field names are OK (e.g. email).",
			"With joins or multiple bindings: ALWAYS qualify as binding.field (e.g. inv.total).",
			"If you set as: c, prefer c.id in FieldRefs — do not mix customers.id and c.id in the same query.",
			"Field names must match catalog fields[].name exactly (usually snake_case).",
		},
		JoinRules: []string{
			"joins is an ordered flat list after from — not a nested tree object.",
			"Each join introduces a binding (as if set, else entity name).",
			"on.left / on.right may reference any binding already introduced (root or earlier joins).",
			"Only type inner or left.",
			"Prefer relations[].on from the catalog for join keys — never invent foreign keys.",
			"Use as aliases when joining; then qualify all FieldRefs with those aliases.",
		},
		AggregateRules: []string{
			`count may omit field: {"agg":"count","as":"n"}.`,
			"sum/avg/min/max require field.",
			"No count(distinct …), no * token — omit field for row count.",
			"Every selected bare field that is not an aggregate MUST appear in groupBy.",
			"Agg-only select (no bare fields) may omit groupBy (one result row).",
		},
		OrderByRules: []string{
			`orderBy items: {"field":"...","dir":"asc"|"desc"}.`,
			"field may be a FieldRef OR an output alias from select (e.g. revenue from agg as).",
			"No expressions in orderBy.",
		},
		QueryIR: protocol.HowToQueryIR{
			Shape:      "JSON object — never SQL strings, never Mongo pipelines as the agent API.",
			Required:   []string{"from", "select"},
			Optional:   []string{"protocolVersion", "as", "joins", "where", "groupBy", "orderBy", "limit", "offset", "mode"},
			CompareOps: []string{"eq", "neq", "gt", "gte", "lt", "lte", "in", "nin", "contains", "is_null", "not_null"},
			AggOps:     []string{"count", "sum", "avg", "min", "max"},
			JoinTypes:  []string{"inner", "left"},
			FieldRef:   "field | binding.field — with >1 entity, always binding.field.",
			Notes: []string{
				"from / joins[].from must be entities[].name or entities[].aliases[] from the catalog.",
				"as introduces a query-local binding name; use it in FieldRefs when set.",
				"where boolean ops use {op,args} — not {and:[…]}.",
				"limit omitted → preset defaultLimit; must be <= maxLimit.",
				"Cross-source joins are supported (local join after fetch).",
				"mode async still uses the same fail-fast budget (~maxSyncMs).",
				"For HAVING / windows / UNION / CASE / LIKE use execute_sql instead of stretching IR.",
			},
		},
		SQL: protocol.HowToSQLGuide{
			LatestVersion:     protocol.SQLDialectLatest,
			SupportedVersions: []string{protocol.SQLDialect1, protocol.SQLDialect2},
			Tool:              "execute_sql",
			HTTP:              "POST /v1/sql  body: {\"sql\":\"…\", \"version\"?:\"" + protocol.SQLDialectLatest + "\"}",
			Rules: []string{
				"FROM/JOIN tables must be catalog entity names (e.g. invoices), never schema.table.",
				"SELECT output aliases (AS rnk) are fine — they are not catalog fields.",
				"version omitted = latest (" + protocol.SQLDialectLatest + "). \"1\" is frozen (no set ops / QUALIFY).",
				"LIMIT required or injected (defaultLimit). Prefer explicit LIMIT.",
				"Read-only: no INSERT/UPDATE/DELETE/MERGE/DDL; no read_csv/httpfs/glob.",
				"Cross-source joins OK: fetch cited columns, then DuckDB runs the SELECT.",
				"Use DuckDB function names (date_trunc, json_extract, list_contains) — not Spark/Databricks-only aliases.",
			},
			Supported: []string{
				"WHERE / AND / OR / NOT / IN / IS NULL",
				"HAVING, DISTINCT, CASE, LIKE/ILIKE, BETWEEN",
				"CTE (WITH), subquery in FROM",
				"INNER/LEFT joins on catalog entities",
				"Aggregates: COUNT, SUM, AVG, MIN, MAX, COUNT(*) FILTER, stddev, array_agg",
				"COUNT(DISTINCT field)",
				"String: CONCAT, LOWER/UPPER, TRIM, SUBSTRING, REPLACE, LENGTH",
				"Numeric: + - * /, ABS, ROUND, CEIL, FLOOR, POWER, SQRT, CAST/TRY_CAST",
				"Datetime: date_trunc, EXTRACT, CURRENT_DATE (DuckDB names)",
				"COALESCE, NULLIF, IFF",
			},
			Dialect2Only: []string{
				"UNION / UNION ALL / INTERSECT / EXCEPT",
				"QUALIFY",
				"Windows: ROW_NUMBER, RANK, DENSE_RANK, LAG, LEAD, NTILE, SUM() OVER",
				"Boolean XOR",
			},
			Reject: []string{
				"INSERT/UPDATE/DELETE/MERGE/CREATE/DROP/COPY/ATTACH",
				"read_csv, read_parquet, httpfs, glob, postgres_scan",
				"public.customers (schema-qualified)",
				"Multiple statements separated by ;",
			},
			Examples: []protocol.HowToSQLExample{
				{Title: "Filter + limit", SQL: "SELECT id, total, status FROM invoices WHERE status = 'paid' LIMIT 20"},
				{Title: "Aggregate + HAVING", SQL: "SELECT status, COUNT(*) AS n, SUM(total) AS sum_total FROM invoices GROUP BY status HAVING COUNT(*) > 5 LIMIT 20"},
				{Title: "Cross-source join", SQL: "SELECT c.id, c.email, i.total FROM customers c INNER JOIN invoices i ON i.customer_id = c.id LIMIT 50"},
				{Title: "Window RANK/LAG", SQL: "SELECT id, status, total, RANK() OVER (ORDER BY total DESC) AS rnk, LAG(total) OVER (ORDER BY total) AS prev_total FROM invoices LIMIT 50"},
				{Title: "UNION ALL (dialect 2)", SQL: "SELECT id FROM invoices LIMIT 20 UNION ALL SELECT id FROM customers LIMIT 20"},
				{Title: "QUALIFY (dialect 2)", SQL: "SELECT status, total FROM invoices QUALIFY ROW_NUMBER() OVER (PARTITION BY status ORDER BY total DESC) = 1 LIMIT 20"},
			},
		},
		Rules: []string{
			"Call how_to_use_me then describe_catalog before inventing any name.",
			"Prefer Query IR for simple list/filter/join/agg; use execute_sql for SQL-only features.",
			"Prefer small limits; filter early.",
			"Qualify every FieldRef when the Query IR has joins.",
			"Use catalog relations for join keys.",
			"On INVALID_IR, fix the IR — do not put SELECT into execute_query.",
			"On INVALID_SQL / SOURCE_ERROR, fix SQL or catalog names — see error.message.",
		},
		Examples: []protocol.HowToExample{
			{
				Title: "Simple list",
				IR: map[string]any{
					"protocolVersion": protocol.ProtocolVersion,
					"from":            "customers",
					"select":          []any{"id", "email"},
					"orderBy":         []any{map[string]any{"field": "email", "dir": "asc"}},
					"limit":           10,
				},
			},
			{
				Title: "AND filter + aggregate",
				IR: map[string]any{
					"protocolVersion": protocol.ProtocolVersion,
					"from":            "invoices",
					"select": []any{
						"customer_id",
						map[string]any{"agg": "sum", "field": "total", "as": "revenue"},
						map[string]any{"agg": "count", "as": "n"},
					},
					"where": map[string]any{
						"op": "and",
						"args": []any{
							map[string]any{"field": "status", "op": "eq", "value": "paid"},
							map[string]any{"field": "total", "op": "gte", "value": 10},
						},
					},
					"groupBy": []any{"customer_id"},
					"orderBy": []any{map[string]any{"field": "revenue", "dir": "desc"}},
					"limit":   50,
				},
			},
			{
				Title: "Cross-entity join with aliases",
				IR: map[string]any{
					"protocolVersion": protocol.ProtocolVersion,
					"from":            "invoices",
					"as":              "inv",
					"joins": []any{
						map[string]any{
							"type": "left",
							"from": "customers",
							"as":   "c",
							"on":   []any{map[string]any{"left": "inv.customer_id", "right": "c.id"}},
						},
					},
					"select": []any{"inv.id", "c.email", "inv.total"},
					"where":  map[string]any{"field": "inv.status", "op": "eq", "value": "paid"},
					"limit":  100,
				},
			},
		},
		InvalidExamples: []protocol.HowToInvalidExample{
			{
				Wrong: map[string]any{"where": map[string]any{"and": []any{map[string]any{"field": "status", "op": "eq", "value": "paid"}}}},
				Why:   "Mongo-style boolean keys are invalid. Use op+args.",
				Fix: map[string]any{"where": map[string]any{
					"op": "and", "args": []any{map[string]any{"field": "status", "op": "eq", "value": "paid"}},
				}},
			},
			{
				Wrong: map[string]any{"where": map[string]any{"field": "status", "op": "=", "value": "paid"}},
				Why:   "SQL operators are not CompareOp tokens in Query IR.",
				Fix:   map[string]any{"where": map[string]any{"field": "status", "op": "eq", "value": "paid"}},
			},
			{
				Wrong: map[string]any{"select": []any{"customerId"}},
				Why:   "Field must match catalog exactly (usually customer_id).",
				Fix:   map[string]any{"select": []any{"customer_id"}},
			},
			{
				Wrong: map[string]any{
					"select": []any{"status", map[string]any{"agg": "count", "as": "n"}},
				},
				Why: "Bare field + agg requires groupBy containing every bare field.",
				Fix: map[string]any{
					"select":  []any{"status", map[string]any{"agg": "count", "as": "n"}},
					"groupBy": []any{"status"},
				},
			},
			{
				Wrong: map[string]any{
					"joins": []any{map[string]any{
						"type": "left", "from": "customers",
						"on": []any{map[string]any{"left": "customerId", "right": "id"}},
					}},
				},
				Why: "Invented/camelCase join keys. Use catalog relations and exact field names.",
				Fix: map[string]any{
					"as": "inv",
					"joins": []any{map[string]any{
						"type": "left", "from": "customers", "as": "c",
						"on": []any{map[string]any{"left": "inv.customer_id", "right": "c.id"}},
					}},
				},
			},
		},
		Errors: []protocol.HowToError{
			{Code: "INVALID_IR", When: "JSON/schema/grammar invalid — read message for expected shape"},
			{Code: "INVALID_SQL", When: "SQL outside dialect / security denylist"},
			{Code: "UNSUPPORTED_VERSION", When: "sql.version unknown (supported: 1, 2)"},
			{Code: "UNKNOWN_ENTITY", When: "from/join entity not in catalog"},
			{Code: "UNKNOWN_FIELD", When: "field not on that entity (IR or qualified SQL ref)"},
			{Code: "AMBIGUOUS_FIELD", When: "unqualified field with >1 binding"},
			{Code: "AMBIGUOUS_ALIAS", When: "duplicate as/binding in one query"},
			{Code: "LIMIT_EXCEEDED", When: "limit > maxLimit"},
			{Code: "FORBIDDEN", When: "entity not in app ACL tables"},
			{Code: "UNSUPPORTED", When: "op cannot run on source and cannot degrade"},
			{Code: "TIMEOUT", When: "budget/source exceeded"},
			{Code: "SOURCE_ERROR", When: "backend error (message may include cause)"},
		},
		Project: protocol.HowToProject{
			Name:         catalog.Project,
			EntityNames:  entities,
			DefaultLimit: limits.DefaultLimit,
			MaxLimit:     limits.MaxLimit,
			MaxSyncMs:    limits.MaxSyncMs,
			ReadOnly:     limits.ReadOnly,
		},
	}
}
