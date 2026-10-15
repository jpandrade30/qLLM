package catalogdesc

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"qLLM/internal/catalogidx"
	"qLLM/internal/protocol"
)

const DescCap = 24000

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

// Issue is a relation/join validation finding for a catalog slice.
type Issue struct {
	Severity string `json:"severity"` // "error" | "warning"
	Code     string `json:"code"`
	Message  string `json:"message"`
	Entity   string `json:"entity,omitempty"`
	Relation string `json:"relation,omitempty"`
}

// FilterCatalog returns a catalog with only the named entities, in names order.
func FilterCatalog(cat *protocol.Catalog, names []string) (*protocol.Catalog, error) {
	if cat == nil {
		return nil, fmt.Errorf("catalog is nil")
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("at least one table name is required")
	}
	byName := make(map[string]protocol.Entity, len(cat.Entities))
	for _, e := range cat.Entities {
		byName[e.Name] = e
	}
	seen := make(map[string]struct{}, len(names))
	out := make([]protocol.Entity, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("empty table name")
		}
		if _, dup := seen[name]; dup {
			return nil, fmt.Errorf("duplicate table name: %s", name)
		}
		seen[name] = struct{}{}
		e, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("unknown catalog entity: %s", name)
		}
		out = append(out, e)
	}
	return &protocol.Catalog{
		ProtocolVersion: cat.ProtocolVersion,
		Project:         cat.Project,
		Entities:        out,
	}, nil
}

// ValidateSlice checks relations and join fields within a (usually filtered) catalog.
// Relation targets outside the catalog entity set are warnings (code relation_outside_slice).
// Missing join fields or empty relation targets are errors.
func ValidateSlice(cat *protocol.Catalog) []Issue {
	if cat == nil {
		return nil
	}
	ents := make(map[string]*protocol.Entity, len(cat.Entities))
	for i := range cat.Entities {
		e := &cat.Entities[i]
		ents[e.Name] = e
	}
	var issues []Issue
	for i := range cat.Entities {
		e := &cat.Entities[i]
		for _, r := range e.Relations {
			relName := r.Name
			if relName == "" {
				relName = r.To
			}
			if strings.TrimSpace(r.To) == "" {
				issues = append(issues, Issue{
					Severity: "error", Code: "relation_missing_target",
					Message: "relation has empty to", Entity: e.Name, Relation: relName,
				})
				continue
			}
			target, ok := ents[r.To]
			if !ok {
				issues = append(issues, Issue{
					Severity: "warning", Code: "relation_outside_slice",
					Message: fmt.Sprintf("relation to %q is outside the selected tables", r.To),
					Entity:  e.Name, Relation: relName,
				})
				continue
			}
			if len(r.On) == 0 {
				issues = append(issues, Issue{
					Severity: "error", Code: "relation_missing_on",
					Message: "relation has no on pairs", Entity: e.Name, Relation: relName,
				})
				continue
			}
			for _, pair := range r.On {
				if len(pair) != 2 {
					issues = append(issues, Issue{
						Severity: "error", Code: "relation_bad_on",
						Message: "on pair must be [local, remote]", Entity: e.Name, Relation: relName,
					})
					continue
				}
				local, remote := pair[0], pair[1]
				if !entityHasField(e, local) {
					issues = append(issues, Issue{
						Severity: "error", Code: "relation_missing_local_field",
						Message: fmt.Sprintf("local field %q not on entity %s", local, e.Name),
						Entity:  e.Name, Relation: relName,
					})
				}
				if !entityHasField(target, remote) {
					issues = append(issues, Issue{
						Severity: "error", Code: "relation_missing_remote_field",
						Message: fmt.Sprintf("remote field %q not on entity %s", remote, r.To),
						Entity:  e.Name, Relation: relName,
					})
				}
			}
		}
	}
	return issues
}

func entityHasField(e *protocol.Entity, name string) bool {
	for _, f := range e.Fields {
		if f.Name == name {
			return true
		}
	}
	return false
}

// HasErrors reports whether any issue has severity error.
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == "error" {
			return true
		}
	}
	return false
}

// HasWarnings reports whether any issue has severity warning.
func HasWarnings(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == "warning" {
			return true
		}
	}
	return false
}

// DescribeResult is the filtered execute_sql-style description for a table subset.
type DescribeResult struct {
	Description string            `json:"description"`
	Entities    []string          `json:"entities"`
	Issues      []Issue           `json:"issues"`
	Catalog     *protocol.Catalog `json:"catalog"`
}

// Describe filters the catalog to names, validates relations, and builds the description.
// strict promotes relation_outside_slice warnings to a failure (returned error).
func Describe(preset *protocol.Preset, cat *protocol.Catalog, names []string, strict bool) (*DescribeResult, *protocol.ProtocolError) {
	sliced, err := FilterCatalog(cat, names)
	if err != nil {
		msg := err.Error()
		code := protocol.ErrInvalidIR
		if strings.Contains(msg, "unknown catalog entity") {
			code = protocol.ErrUnknownEntity
		}
		return nil, protocol.NewError(code, msg, nil)
	}
	issues := ValidateSlice(sliced)
	if issues == nil {
		issues = []Issue{}
	}
	if HasErrors(issues) || (strict && HasWarnings(issues)) {
		details := make([]map[string]any, 0, len(issues))
		for _, iss := range issues {
			details = append(details, map[string]any{
				"severity": iss.Severity, "code": iss.Code, "message": iss.Message,
				"entity": iss.Entity, "relation": iss.Relation,
			})
		}
		return &DescribeResult{
				Description: "",
				Entities:    names,
				Issues:      issues,
				Catalog:     sliced,
			}, protocol.NewError(protocol.ErrInvalidIR, "catalog slice validation failed",
			map[string]any{"issues": details})
	}
	idx, perr := catalogidx.New(preset, sliced)
	if perr != nil {
		return nil, perr
	}
	return &DescribeResult{
		Description: ExecuteSQLDescription(idx),
		Entities:    EntityNames(idx),
		Issues:      issues,
		Catalog:     sliced,
	}, nil
}

// EntityNames lists entity names from an index.
func EntityNames(idx *catalogidx.Index) []string {
	if idx == nil || idx.Catalog == nil {
		return nil
	}
	names := make([]string, 0, len(idx.Catalog.Entities))
	for _, e := range idx.Catalog.Entities {
		names = append(names, e.Name)
	}
	return names
}

// ExecuteSQLDescription builds the MCP execute_sql tool description body.
func ExecuteSQLDescription(idx *catalogidx.Index) string {
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
	return DescriptionsForAllow(idx, nil)
}

// DescriptionsForAllow builds capped MCP descriptions for the ACL catalog projection (D16/D24).
// nil acl = full catalog.
func DescriptionsForAllow(idx *catalogidx.Index, acl *catalogidx.ACL) ToolDescriptions {
	view := idx
	if idx != nil {
		cp := *idx
		cp.Catalog = idx.CatalogFor(acl)
		view = &cp
	}
	names := EntityNames(view)
	list := formatNames(names)
	n := len(names)
	return ToolDescriptions{
		HowToUseMe:      CapRunes("Return the SQL dialect guide. Catalog tables: "+list+". Prefer execute_sql whose description already lists tables, fields, joins, and SQL functions.", DescCap),
		DescribeCatalog: CapRunes(fmt.Sprintf("Return the logical catalog JSON (%d entities: %s) if you need the full dump. execute_sql description already includes fields and relations.", n, list), DescCap),
		ExecuteSQL:      CapRunes(ExecuteSQLDescription(view), DescCap),
	}
}

// CapRunes truncates s to at most n runes, appending … when truncated.
func CapRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	if n < 2 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

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

func fieldLine(f protocol.Field) string {
	var b strings.Builder
	b.WriteString(f.Name)
	b.WriteByte(' ')
	b.WriteString(string(f.Type))
	if f.Redact {
		b.WriteString(" [REDACTED in results]")
	}
	if d := strings.TrimSpace(f.Description); d != "" {
		b.WriteString(" — ")
		b.WriteString(d)
	}
	return b.String()
}

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
