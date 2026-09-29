package sqlbuild

import (
	"fmt"
	"strings"
	"time"

	"qLLM/internal/connector/def"
	"qLLM/internal/protocol"
)

type Dialect int

const (
	Postgres Dialect = iota
	MySQL
	MSSQL
	SQLite
	ClickHouse
)

type Built struct {
	SQL  string
	Args []any
}

func Build(d Dialect, step def.PushdownStep) (Built, error) {
	e := step.Entity
	table := qualifyTable(d, e.Binding)
	var b strings.Builder
	args := []any{}
	ph := func() string {
		if d == Postgres {
			return fmt.Sprintf("$%d", len(args)+1)
		}
		return "?"
	}

	b.WriteString("SELECT ")
	if d == MSSQL && step.Limit > 0 && step.Offset <= 0 {
		b.WriteString(fmt.Sprintf("TOP (%d) ", step.Limit))
	}
	if len(step.Select) == 0 {
		b.WriteString("*")
	} else {
		parts := make([]string, 0, len(step.Select))
		for _, s := range step.Select {
			if s.Agg != "" {
				expr := ""
				switch strings.ToLower(s.Agg) {
				case "count":
					if s.Field == "" {
						expr = "COUNT(*)"
					} else {
						expr = fmt.Sprintf("COUNT(%s)", quoteIdent(d, def.PhysicalName(e, s.Field)))
					}
				case "sum", "avg", "min", "max":
					expr = fmt.Sprintf("%s(%s)", strings.ToUpper(s.Agg), quoteIdent(d, def.PhysicalName(e, s.Field)))
				default:
					return Built{}, protocol.NewError(protocol.ErrUnsupported, "unsupported agg: "+s.Agg, nil)
				}
				parts = append(parts, fmt.Sprintf("%s AS %s", expr, quoteIdent(d, s.As)))
			} else {
				phys := quoteIdent(d, def.PhysicalName(e, s.Field))
				as := s.As
				if as == "" {
					as = s.Field
				}
				parts = append(parts, fmt.Sprintf("%s AS %s", phys, quoteIdent(d, as)))
			}
		}
		b.WriteString(strings.Join(parts, ", "))
	}
	b.WriteString(" FROM ")
	b.WriteString(table)

	if step.Where != nil {
		clause, wargs, err := whereSQL(d, e, step.Where, &args, ph)
		if err != nil {
			return Built{}, err
		}
		if clause != "" {
			b.WriteString(" WHERE ")
			b.WriteString(clause)
			_ = wargs
		}
	}

	if len(step.GroupBy) > 0 {
		cols := make([]string, 0, len(step.GroupBy))
		for _, g := range step.GroupBy {
			field := g
			if i := strings.LastIndex(g, "."); i >= 0 {
				field = g[i+1:]
			}
			cols = append(cols, quoteIdent(d, def.PhysicalName(e, field)))
		}
		b.WriteString(" GROUP BY ")
		b.WriteString(strings.Join(cols, ", "))
	}

	if len(step.OrderBy) > 0 {
		parts := make([]string, 0, len(step.OrderBy))
		for _, o := range step.OrderBy {
			field := o.Field
			if i := strings.LastIndex(field, "."); i >= 0 {
				field = field[i+1:]
			}
			dir := "ASC"
			if strings.EqualFold(o.Dir, "desc") {
				dir = "DESC"
			}
			// may be output alias
			parts = append(parts, quoteIdent(d, field)+" "+dir)
		}
		b.WriteString(" ORDER BY ")
		b.WriteString(strings.Join(parts, ", "))
	}

	if d == MSSQL {
		if len(step.OrderBy) == 0 && (step.Offset > 0 || (step.Limit > 0 && step.Offset > 0)) {
			b.WriteString(" ORDER BY (SELECT NULL)")
		}
		if step.Offset > 0 {
			fetch := step.Limit
			if fetch <= 0 {
				fetch = 1
			}
			b.WriteString(fmt.Sprintf(" OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", step.Offset, fetch))
		}
	} else {
		if step.Limit > 0 {
			b.WriteString(fmt.Sprintf(" LIMIT %d", step.Limit))
		}
		if step.Offset > 0 {
			b.WriteString(fmt.Sprintf(" OFFSET %d", step.Offset))
		}
	}

	return Built{SQL: b.String(), Args: args}, nil
}

func qualifyTable(d Dialect, bind protocol.Binding) string {
	if bind.Schema != "" {
		return quoteIdent(d, bind.Schema) + "." + quoteIdent(d, bind.Table)
	}
	return quoteIdent(d, bind.Table)
}

func quoteIdent(d Dialect, name string) string {
	switch d {
	case MySQL, SQLite, ClickHouse:
		return "`" + strings.ReplaceAll(name, "`", "``") + "`"
	case MSSQL:
		return "[" + strings.ReplaceAll(name, "]", "]]") + "]"
	default:
		return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
	}
}

func whereSQL(d Dialect, e *protocol.Entity, w map[string]any, args *[]any, ph func() string) (string, []any, error) {
	if op, ok := w["op"].(string); ok {
		switch op {
		case "and", "or":
			argsList, _ := w["args"].([]any)
			parts := make([]string, 0, len(argsList))
			for _, a := range argsList {
				m, ok := a.(map[string]any)
				if !ok {
					b, _ := jsonMarshal(a)
					_ = jsonUnmarshal(b, &m)
				}
				c, _, err := whereSQL(d, e, m, args, ph)
				if err != nil {
					return "", nil, err
				}
				if c != "" {
					parts = append(parts, "("+c+")")
				}
			}
			sep := " AND "
			if op == "or" {
				sep = " OR "
			}
			return strings.Join(parts, sep), *args, nil
		case "not":
			argsList, _ := w["args"].([]any)
			if len(argsList) != 1 {
				return "", nil, protocol.NewError(protocol.ErrInvalidIR, "not requires one arg", nil)
			}
			m, _ := argsList[0].(map[string]any)
			c, _, err := whereSQL(d, e, m, args, ph)
			if err != nil {
				return "", nil, err
			}
			return "NOT (" + c + ")", *args, nil
		}
	}

	field, _ := w["field"].(string)
	op, _ := w["op"].(string)
	if field == "" || op == "" {
		return "", nil, protocol.NewError(protocol.ErrInvalidIR, "invalid compare expr", nil)
	}
	if i := strings.LastIndex(field, "."); i >= 0 {
		field = field[i+1:]
	}
	col := quoteIdent(d, def.PhysicalName(e, field))

	switch op {
	case "is_null":
		return col + " IS NULL", *args, nil
	case "not_null":
		return col + " IS NOT NULL", *args, nil
	case "in", "nin":
		vals, ok := w["value"].([]any)
		if !ok {
			return "", nil, protocol.NewError(protocol.ErrInvalidIR, "in/nin requires array value", nil)
		}
		placeholders := make([]string, 0, len(vals))
		for _, v := range vals {
			p := typedPlaceholder(d, e, field, ph)
			*args = append(*args, coerceValue(e, field, v))
			placeholders = append(placeholders, p)
		}
		clause := col + " IN (" + strings.Join(placeholders, ",") + ")"
		if op == "nin" {
			clause = col + " NOT IN (" + strings.Join(placeholders, ",") + ")"
		}
		return clause, *args, nil
	case "contains":
		p := ph()
		if d == Postgres {
			p = p + "::text"
		}
		*args = append(*args, escapeLike(fmt.Sprintf("%v", w["value"])))
		if d == Postgres {
			return col + " LIKE '%' || " + p + " || '%' ESCAPE '\\'", *args, nil
		}
		if d == MSSQL {
			return col + " LIKE '%' + " + p + " + '%' ESCAPE '\\'", *args, nil
		}
		return col + " LIKE CONCAT('%', " + p + ", '%') ESCAPE '\\\\'", *args, nil
	default:
		sqlOp := map[string]string{
			"eq": "=", "neq": "<>", "gt": ">", "gte": ">=", "lt": "<", "lte": "<=",
		}[op]
		if sqlOp == "" {
			return "", nil, protocol.NewError(protocol.ErrUnsupported, "unsupported op: "+op, nil)
		}
		p := typedPlaceholder(d, e, field, ph)
		*args = append(*args, coerceValue(e, field, w["value"]))
		return col + " " + sqlOp + " " + p, *args, nil
	}
}

func coerceValue(e *protocol.Entity, logicalField string, v any) any {
	switch def.FieldType(e, logicalField) {
	case protocol.TypeTimestamp:
		switch t := v.(type) {
		case time.Time:
			return t.UTC()
		case string:
			if parsed, err := time.Parse(time.RFC3339, t); err == nil {
				return parsed.UTC()
			}
			if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
				return parsed.UTC()
			}
			if parsed, err := time.Parse("2006-01-02", t); err == nil {
				return parsed.UTC()
			}
		}
	case protocol.TypeNumber:
		switch t := v.(type) {
		case float64, float32, int, int64, int32:
			return t
		case string:
			return t
		}
	}
	return v
}

func typedPlaceholder(d Dialect, e *protocol.Entity, logicalField string, ph func() string) string {
	p := ph()
	if d != Postgres {
		return p
	}
	// Always cast: bare $n often yields SQLSTATE 42P18 with pgx.
	switch def.FieldType(e, logicalField) {
	case protocol.TypeTimestamp:
		return p + "::timestamptz"
	case protocol.TypeNumber:
		return p + "::float8"
	case protocol.TypeBoolean:
		return p + "::boolean"
	case protocol.TypeJSON:
		return p + "::jsonb"
	default:
		return p + "::text"
	}
}

func jsonMarshal(v any) ([]byte, error) {
	return jsonMarshalImpl(v)
}

func jsonUnmarshal(b []byte, v any) error {
	return jsonUnmarshalImpl(b, v)
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
