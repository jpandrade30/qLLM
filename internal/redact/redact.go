// Package redact masks catalog fields marked fields[].redact (D25) in query results.
package redact

import (
	"strings"

	"qLLM/internal/protocol"
	"qLLM/internal/sqlparse"
)

// Placeholder is the cell value written for redacted columns.
const Placeholder = "[REDACTED]"

// FieldNames returns logical field names with redact:true on the given entities.
// If entityNames is empty, all catalog entities are scanned. When the same logical
// name is redacted on any in-scope entity, it is included (conservative).
func FieldNames(cat *protocol.Catalog, entityNames []string) map[string]struct{} {
	out := map[string]struct{}{}
	if cat == nil {
		return out
	}
	want := map[string]struct{}{}
	for _, n := range entityNames {
		if n != "" {
			want[n] = struct{}{}
		}
	}
	for i := range cat.Entities {
		e := &cat.Entities[i]
		if len(want) > 0 {
			if _, ok := want[e.Name]; !ok {
				continue
			}
		}
		for _, f := range e.Fields {
			if f.Redact {
				out[f.Name] = struct{}{}
			}
		}
	}
	return out
}

// BaseName strips an optional table/alias qualifier (a.b → b).
func BaseName(col string) string {
	if i := strings.LastIndex(col, "."); i >= 0 && i+1 < len(col) {
		return col[i+1:]
	}
	return col
}

// Apply masks result columns whose name (or base name) is in fieldNames or extraOutNames.
// Masked cells become Placeholder; column type becomes string.
func Apply(tab *protocol.TabularResult, fieldNames, extraOutNames map[string]struct{}) {
	if tab == nil || (len(fieldNames) == 0 && len(extraOutNames) == 0) {
		return
	}
	for i := range tab.Columns {
		name := tab.Columns[i].Name
		base := BaseName(name)
		if hit(fieldNames, name) || hit(fieldNames, base) ||
			hit(extraOutNames, name) || hit(extraOutNames, base) {
			maskCol(tab, i)
		}
	}
}

// ApplySelectOuts additionally masks by SELECT-item position when outs align 1:1 with columns
// (not SELECT *). An item is masked if any referenced field is redacted.
func ApplySelectOuts(tab *protocol.TabularResult, fieldNames map[string]struct{}, outs []sqlparse.SelectOut) {
	if tab == nil || len(fieldNames) == 0 || len(outs) == 0 {
		return
	}
	if len(outs) != len(tab.Columns) {
		return
	}
	for i, so := range outs {
		if so.Star {
			continue
		}
		for _, f := range so.Fields {
			if hit(fieldNames, f) || hit(fieldNames, BaseName(f)) {
				maskCol(tab, i)
				break
			}
		}
	}
}

// ExtraOutNamesFromIR adds output column names that originate from redacted fields
// (plain select refs and agg "as" aliases).
func ExtraOutNamesFromIR(q *protocol.QueryIR, fieldNames map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	if q == nil || len(fieldNames) == 0 {
		return out
	}
	for _, item := range q.Select {
		switch v := item.(type) {
		case string:
			base := BaseName(v)
			if hit(fieldNames, base) {
				out[base] = struct{}{}
				out[v] = struct{}{}
			}
		case map[string]any:
			field, _ := v["field"].(string)
			as, _ := v["as"].(string)
			base := BaseName(field)
			if field != "" && hit(fieldNames, base) {
				if as != "" {
					out[as] = struct{}{}
				} else if base != "" {
					out[base] = struct{}{}
				}
			}
		}
	}
	return out
}

// ExtraOutNamesFromSQL adds aliases (and simple out names) for SELECT items that
// reference redacted fields.
func ExtraOutNamesFromSQL(parsed *sqlparse.Result, fieldNames map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	if parsed == nil || len(fieldNames) == 0 {
		return out
	}
	for _, so := range parsed.SelectOuts {
		if so.Star || so.OutName == "" {
			continue
		}
		for _, f := range so.Fields {
			if hit(fieldNames, f) || hit(fieldNames, BaseName(f)) {
				out[so.OutName] = struct{}{}
				break
			}
		}
	}
	return out
}

func hit(set map[string]struct{}, k string) bool {
	if k == "" || set == nil {
		return false
	}
	_, ok := set[k]
	return ok
}

func maskCol(tab *protocol.TabularResult, i int) {
	if i < 0 || i >= len(tab.Columns) {
		return
	}
	tab.Columns[i].Type = protocol.TypeString
	for r := range tab.Rows {
		if i < len(tab.Rows[r]) {
			tab.Rows[r][i] = Placeholder
		}
	}
}
