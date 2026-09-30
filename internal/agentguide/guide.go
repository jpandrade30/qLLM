package agentguide

import (
	"strings"

	"qLLM/internal/protocol"
)

// Build returns the LLM guide for MCP/HTTP agents: catalog SQL only.
func Build(preset *protocol.Preset, catalog *protocol.Catalog) protocol.HowToUseMeResponse {
	entities := make([]string, 0, len(catalog.Entities))
	for _, e := range catalog.Entities {
		entities = append(entities, e.Name)
	}
	limits := preset.Limits
	tableHint := "catalog entity names"
	if len(entities) > 0 {
		shown := entities
		if len(shown) > 8 {
			shown = shown[:8]
		}
		tableHint = "catalog entity names (" + strings.Join(shown, ", ") + ")"
	}

	return protocol.HowToUseMeResponse{
		ProtocolVersion: protocol.ProtocolVersion,
		Purpose: "qLLM is a read-only multi-source query runtime. Tools: how_to_use_me, describe_catalog, execute_sql. " +
			"Query with catalog SQL (DuckDB after fetch; FROM/JOIN tables = catalog entity names). " +
			"Call how_to_use_me, then describe_catalog, then execute_sql.",
		Workflow: []string{
			"1. Call how_to_use_me — SQL dialect guide.",
			"2. Call describe_catalog — copy entity/field/relation names exactly.",
			"3. execute_sql (version omitted = latest \"" + protocol.SQLDialectLatest + "\").",
			"4. On error, read error.code + message; fix SQL or catalog names.",
		},
		Never: []string{
			"Never generate MongoDB queries or raw REST URLs as the agent API.",
			"Never send SQL except execute_sql / POST /v1/sql using catalog entity names as tables.",
			"Never invent entity names, field names, or join keys — use the catalog only.",
			"Never invent camelCase fields (customerId) when the catalog has snake_case (customer_id).",
			"Never use schema.table (public.customers) or physical DB names — catalog entity names only.",
		},
		NotSupported: []string{
			"SQL dialect \"1\": UNION / INTERSECT / EXCEPT / QUALIFY (omit version or set \"2\")",
			"Mutations (INSERT/UPDATE/DELETE/MERGE) and file/network table functions (read_csv, httpfs, …)",
			"Schema-qualified tables (public.orders) — catalog entity names only",
			"OFFSET without LIMIT: runtime injects defaultLimit; unbounded scan is not allowed",
		},
		Endpoints: []protocol.HowToEndpoint{
			{Method: "GET", Path: "/v1/howtouseme", Use: "This SQL guide. Call first."},
			{Method: "GET", Path: "/v1/health", Use: "Liveness + protocolVersion."},
			{Method: "GET", Path: "/v1/catalog", Use: "Authoritative entities, fields, relations, sources."},
			{Method: "POST", Path: "/v1/sql", Use: "Execute catalog SQL. Body {sql, version?}. version omitted = latest \"" + protocol.SQLDialectLatest + "\"."},
		},
		Grammar: "SELECT … FROM <catalog_entity> [JOIN <catalog_entity> …] WHERE … GROUP BY … HAVING … ORDER BY … LIMIT n  (dialect " + protocol.SQLDialectLatest + "; see sql)",
		SQL: protocol.HowToSQLGuide{
			LatestVersion:     protocol.SQLDialectLatest,
			SupportedVersions: []string{protocol.SQLDialect1, protocol.SQLDialect2},
			Tool:              "execute_sql",
			HTTP:              "POST /v1/sql  body: {\"sql\":\"…\", \"version\"?:\"" + protocol.SQLDialectLatest + "\"}",
			Rules: []string{
				"FROM/JOIN tables must be " + tableHint + ", never schema.table.",
				"SELECT output aliases (AS rnk) are fine — they are not catalog fields.",
				"version omitted = latest (" + protocol.SQLDialectLatest + "). \"1\" is frozen (no set ops / QUALIFY).",
				"LIMIT required or injected (defaultLimit). Prefer explicit LIMIT.",
				"Read-only: no INSERT/UPDATE/DELETE/MERGE/DDL; no read_csv/httpfs/glob.",
				"Cross-source joins OK: fetch cited columns, then DuckDB runs the SELECT.",
				"Use DuckDB function names (date_trunc, json_extract, list_contains) — not Spark/Databricks-only aliases.",
				"Join keys: copy relations[].on from describe_catalog.",
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
			Examples: sqlExamplesFromCatalog(catalog),
		},
		Rules: []string{
			"Call how_to_use_me then describe_catalog before inventing any name.",
			"Query with execute_sql (catalog entity names as tables).",
			"Prefer small limits; filter early.",
			"Use catalog relations for join keys.",
			"On INVALID_SQL / SOURCE_ERROR, fix SQL or catalog names — see error.message.",
		},
		Errors: []protocol.HowToError{
			{Code: "INVALID_SQL", When: "SQL outside dialect / security denylist"},
			{Code: "UNSUPPORTED_VERSION", When: "sql.version unknown (supported: 1, 2)"},
			{Code: "UNKNOWN_ENTITY", When: "FROM/JOIN table is not a catalog entity name"},
			{Code: "UNKNOWN_FIELD", When: "column not on that catalog entity"},
			{Code: "LIMIT_EXCEEDED", When: "limit > maxLimit"},
			{Code: "FORBIDDEN", When: "entity not in app ACL tables"},
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

// fieldName implements runtime behavior for this package.
func fieldName(e *protocol.Entity, prefer ...string) string {
	if e != nil {
		for _, p := range prefer {
			for _, f := range e.Fields {
				if f.Name == p {
					return p
				}
			}
		}
		if len(e.Fields) > 0 {
			return e.Fields[0].Name
		}
	}
	return "id"
}

// joinOn implements runtime behavior for this package.
func joinOn(a, b *protocol.Entity) (left, right string) {
	for _, r := range a.Relations {
		if r.To == b.Name && len(r.On) > 0 && len(r.On[0]) == 2 {
			return r.On[0][0], r.On[0][1]
		}
	}
	for _, r := range b.Relations {
		if r.To == a.Name && len(r.On) > 0 && len(r.On[0]) == 2 {
			return r.On[0][1], r.On[0][0]
		}
	}
	return fieldName(a, "id"), fieldName(b, "id")
}

// sqlExamplesFromCatalog implements runtime behavior for this package.
func sqlExamplesFromCatalog(catalog *protocol.Catalog) []protocol.HowToSQLExample {
	if catalog == nil || len(catalog.Entities) == 0 {
		return nil
	}
	a := &catalog.Entities[0]
	fa := fieldName(a, "id", "status", "email")
	fb := fieldName(a, "status", "email", "name", fa)
	ex := []protocol.HowToSQLExample{
		{Title: "Filter + limit", SQL: "SELECT " + fa + " FROM " + a.Name + " LIMIT 20"},
		{Title: "Aggregate + HAVING", SQL: "SELECT " + fb + ", COUNT(*) AS n FROM " + a.Name + " GROUP BY " + fb + " HAVING COUNT(*) > 1 LIMIT 20"},
		{Title: "Window RANK", SQL: "SELECT " + fa + ", RANK() OVER (ORDER BY " + fa + ") AS rnk FROM " + a.Name + " LIMIT 50"},
		{Title: "QUALIFY (dialect 2)", SQL: "SELECT " + fa + " FROM " + a.Name + " QUALIFY ROW_NUMBER() OVER (ORDER BY " + fa + ") = 1 LIMIT 20"},
	}
	if len(catalog.Entities) > 1 {
		b := &catalog.Entities[1]
		left, right := joinOn(a, b)
		gb := fieldName(b, "id", "email")
		ex = append(ex,
			protocol.HowToSQLExample{Title: "Join catalog entities", SQL: "SELECT a." + fa + ", b." + gb + " FROM " + a.Name + " a INNER JOIN " + b.Name + " b ON a." + left + " = b." + right + " LIMIT 50"},
			protocol.HowToSQLExample{Title: "UNION ALL (dialect 2)", SQL: "SELECT " + fa + " FROM " + a.Name + " LIMIT 20 UNION ALL SELECT " + gb + " FROM " + b.Name + " LIMIT 20"},
		)
	}
	return ex
}
