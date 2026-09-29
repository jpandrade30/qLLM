package cataloggen

import (
	"strings"

	"qLLM/internal/protocol"
)

// ColumnRow is one information_schema.columns row (or equivalent).
type ColumnRow struct {
	Schema   string
	Table    string
	Column   string
	DataType string
	PK       bool
}

// MapSQLType maps vendor type names to catalog logical types.
func MapSQLType(dt string) protocol.LogicalType {
	s := strings.ToLower(strings.TrimSpace(dt))
	switch {
	case strings.Contains(s, "bool"):
		return protocol.TypeBoolean
	case strings.Contains(s, "timestamp"), strings.Contains(s, "date"), strings.Contains(s, "time"):
		return protocol.TypeTimestamp
	case strings.Contains(s, "json"):
		return protocol.TypeJSON
	case strings.Contains(s, "int"), strings.Contains(s, "numeric"), strings.Contains(s, "decimal"),
		strings.Contains(s, "float"), strings.Contains(s, "double"), strings.Contains(s, "real"),
		strings.Contains(s, "money"), strings.Contains(s, "serial"):
		return protocol.TypeNumber
	default:
		return protocol.TypeString
	}
}

// EntitiesFromColumns groups rows into catalog entities. Logical name = table name
// unless the same table name appears in multiple schemas (then schema_table).
func EntitiesFromColumns(sourceID string, rows []ColumnRow) []protocol.Entity {
	type key struct{ schema, table string }
	order := make([]key, 0)
	seen := map[key]bool{}
	tableCount := map[string]int{}
	for _, r := range rows {
		k := key{r.Schema, r.Table}
		if !seen[k] {
			seen[k] = true
			order = append(order, k)
			tableCount[r.Table]++
		}
	}
	byKey := map[key][]ColumnRow{}
	for _, r := range rows {
		byKey[key{r.Schema, r.Table}] = append(byKey[key{r.Schema, r.Table}], r)
	}
	ents := make([]protocol.Entity, 0, len(order))
	for _, k := range order {
		cols := byKey[k]
		name := k.table
		if tableCount[k.table] > 1 {
			name = k.schema + "_" + k.table
		}
		var pk []string
		fields := make([]protocol.Field, 0, len(cols))
		for _, c := range cols {
			fields = append(fields, protocol.Field{
				Name:     c.Column,
				Type:     MapSQLType(c.DataType),
				Physical: c.Column,
			})
			if c.PK {
				pk = append(pk, c.Column)
			}
		}
		ents = append(ents, protocol.Entity{
			Name:        name,
			Description: "Draft from information_schema; review relations/aliases before serve.",
			Source:      sourceID,
			Binding: protocol.Binding{
				Kind:   "table",
				Schema: k.schema,
				Table:  k.table,
			},
			PrimaryKey: pk,
			Fields:     fields,
		})
	}
	return ents
}

// MergeSourceEntities replaces entities whose Source matches sourceID.
func MergeSourceEntities(existing *protocol.Catalog, sourceID string, incoming []protocol.Entity) *protocol.Catalog {
	out := &protocol.Catalog{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "",
	}
	if existing != nil {
		out.ProtocolVersion = existing.ProtocolVersion
		if out.ProtocolVersion == "" {
			out.ProtocolVersion = protocol.ProtocolVersion
		}
		out.Project = existing.Project
		for _, e := range existing.Entities {
			if e.Source != sourceID {
				out.Entities = append(out.Entities, e)
			}
		}
	} else {
		out.ProtocolVersion = protocol.ProtocolVersion
	}
	out.Entities = append(out.Entities, incoming...)
	return out
}
