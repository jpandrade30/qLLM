package mcpserver

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"qLLM/internal/catalogidx"
	"qLLM/internal/protocol"
)

const descCap = 24000

const sqlCapabilities = `SQL dialect latest "` + protocol.SQLDialectLatest + `" (omit version). SELECT only; FROM/JOIN = catalog entity names, never schema.table. LIMIT required or injected.
Supported: WHERE AND/OR/NOT IN IS NULL; HAVING DISTINCT CASE LIKE/ILIKE BETWEEN; WITH; INNER/LEFT JOIN; COUNT SUM AVG MIN MAX FILTER stddev array_agg COUNT(DISTINCT); CONCAT LOWER UPPER TRIM SUBSTRING REPLACE LENGTH; ABS ROUND CEIL FLOOR POWER SQRT CAST TRY_CAST; date_trunc EXTRACT CURRENT_DATE; COALESCE NULLIF IFF.
Dialect 2: UNION INTERSECT EXCEPT QUALIFY; windows ROW_NUMBER RANK DENSE_RANK LAG LEAD NTILE OVER; XOR.
Reject: INSERT UPDATE DELETE MERGE DDL; read_csv httpfs glob; public.x; multiple statements.`

// ToolDescriptions interpolates loaded catalog names into MCP tool descriptions.
type ToolDescriptions struct {
	HowToUseMe      string
	DescribeCatalog string
	ExecuteSQL      string
}

// entityNames implements runtime behavior for this package.
func entityNames(idx *catalogidx.Index) []string {
	if idx == nil || idx.Catalog == nil {
		return nil
	}
	names := make([]string, 0, len(idx.Catalog.Entities))
	for _, e := range idx.Catalog.Entities {
		names = append(names, e.Name)
	}
	return names
}

// formatNames implements runtime behavior for this package.
func formatNames(names []string) string {
	if len(names) == 0 {
		return "(none — catalog YAML has no entities)"
	}
	const max = 40
	if len(names) <= max {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:max], ", ") + ", …"
}

// capRunes implements runtime behavior for this package.
func capRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	if n < 2 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

// fieldLine implements runtime behavior for this package.
func fieldLine(f protocol.Field) string {
	var b strings.Builder
	b.WriteString(f.Name)
	b.WriteByte(' ')
	b.WriteString(string(f.Type))
	if d := strings.TrimSpace(f.Description); d != "" {
		b.WriteString(" — ")
		b.WriteString(d)
	}
	return b.String()
}

// relationLine implements runtime behavior for this package.
func relationLine(e protocol.Entity, r protocol.Relation) string {
	on := "id=id"
	if len(r.On) > 0 && len(r.On[0]) == 2 {
		on = e.Name + "." + r.On[0][0] + "=" + r.To + "." + r.On[0][1]
	}
	kind := r.Type
	if kind == "" {
		kind = "many_to_one"
	}
	name := r.Name
	if name == "" {
		name = r.To
	}
	return name + " " + kind + " " + on
}

// catalogBody implements runtime behavior for this package.
func catalogBody(idx *catalogidx.Index) string {
	if idx == nil || idx.Catalog == nil || len(idx.Catalog.Entities) == 0 {
		return "Tables: (none — catalog YAML has no entities)."
	}
	var b strings.Builder
	b.WriteString("Tables (use these names in FROM/JOIN):\n")
	for _, e := range idx.Catalog.Entities {
		b.WriteString("- ")
		b.WriteString(e.Name)
		if d := strings.TrimSpace(e.Description); d != "" {
			b.WriteString(" — ")
			b.WriteString(d)
		}
		if len(e.PrimaryKey) > 0 {
			b.WriteString(". pk=")
			b.WriteString(strings.Join(e.PrimaryKey, ","))
		}
		if len(e.Aliases) > 0 {
			b.WriteString(". aliases=")
			b.WriteString(strings.Join(e.Aliases, ","))
		}
		b.WriteByte('\n')
		for _, f := range e.Fields {
			b.WriteString("    ")
			b.WriteString(fieldLine(f))
			b.WriteByte('\n')
		}
		if len(e.Relations) == 0 {
			continue
		}
		b.WriteString("    joins: ")
		parts := make([]string, 0, len(e.Relations))
		for _, r := range e.Relations {
			parts = append(parts, relationLine(e, r))
		}
		b.WriteString(strings.Join(parts, "; "))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// executeSQLDescription implements runtime behavior for this package.
func executeSQLDescription(idx *catalogidx.Index) string {
	var b strings.Builder
	b.WriteString("Run a catalog SQL SELECT (DuckDB after fetch). Args: sql (required), version (optional 1|2, omit=latest ")
	b.WriteString(protocol.SQLDialectLatest)
	b.WriteString("), optional constraints (field→scalar; host-bound preferred) and constraintMode validate|inject (default validate).\n")
	b.WriteString(sqlCapabilities)
	b.WriteByte('\n')
	b.WriteString(catalogBody(idx))
	return b.String()
}

// DescriptionsFor builds capped MCP descriptions from the loaded catalog only.
func DescriptionsFor(idx *catalogidx.Index) ToolDescriptions {
	names := entityNames(idx)
	list := formatNames(names)
	n := len(names)
	d := ToolDescriptions{
		HowToUseMe:      capRunes("Return the SQL dialect guide. Catalog tables: "+list+". Prefer execute_sql whose description already lists tables, fields, joins, and SQL functions.", descCap),
		DescribeCatalog: capRunes(fmt.Sprintf("Return the logical catalog JSON (%d entities: %s) if you need the full dump. execute_sql description already includes fields and relations.", n, list), descCap),
		ExecuteSQL:      capRunes(executeSQLDescription(idx), descCap),
	}
	return d
}
