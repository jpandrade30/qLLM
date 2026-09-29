package duckdblocal

import (
	"fmt"
	"strings"

	"qLLM/internal/protocol"
)

func duckType(t protocol.LogicalType) string {
	switch t {
	case protocol.TypeNumber:
		return "DOUBLE"
	case protocol.TypeBoolean:
		return "BOOLEAN"
	case protocol.TypeTimestamp:
		return "TIMESTAMP"
	case protocol.TypeJSON:
		return "JSON"
	default:
		return "VARCHAR"
	}
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func qual(bind, col string) string {
	return quoteIdent(bind) + "." + quoteIdent(col)
}

// BuildDuckSQL renders a parameterized DuckDB SQL statement from QuerySpec.
func BuildDuckSQL(spec QuerySpec) (string, []any, error) {
	var args []any
	var b strings.Builder
	b.WriteString("SELECT ")
	sels := make([]string, 0, len(spec.Select))
	for _, s := range spec.Select {
		as := s.As
		if as == "" {
			as = s.Col
		}
		asQ := quoteIdent(as)
		if s.Agg != "" {
			agg := strings.ToUpper(s.Agg)
			if s.Col == "" && strings.EqualFold(s.Agg, "count") {
				sels = append(sels, "COUNT(*) AS "+asQ)
			} else {
				sels = append(sels, agg+"("+qual(s.Bind, s.Col)+") AS "+asQ)
			}
		} else {
			sels = append(sels, qual(s.Bind, s.Col)+" AS "+asQ)
		}
	}
	if len(sels) == 0 {
		return "", nil, protocol.NewError(protocol.ErrUnsupported, "empty select", nil)
	}
	b.WriteString(strings.Join(sels, ", "))
	b.WriteString(" FROM ")
	b.WriteString(quoteIdent(spec.RootBind))
	b.WriteString(" AS ")
	b.WriteString(quoteIdent(spec.RootBind))

	for _, j := range spec.Joins {
		jt := "INNER JOIN"
		if strings.EqualFold(j.Type, "left") {
			jt = "LEFT JOIN"
		}
		b.WriteString(" ")
		b.WriteString(jt)
		b.WriteString(" ")
		b.WriteString(quoteIdent(j.RightTable))
		b.WriteString(" AS ")
		b.WriteString(quoteIdent(j.RightTable))
		b.WriteString(" ON ")
		ons := make([]string, 0, len(j.On))
		for _, on := range j.On {
			ons = append(ons, qual(on.LeftBind, on.LeftCol)+" = "+qual(on.RightBind, on.RightCol))
		}
		b.WriteString(strings.Join(ons, " AND "))
	}

	if spec.Where != nil {
		clause, err := whereSQL(spec.Where, &args)
		if err != nil {
			return "", nil, err
		}
		if clause != "" {
			b.WriteString(" WHERE ")
			b.WriteString(clause)
		}
	}

	if len(spec.GroupBy) > 0 {
		parts := make([]string, 0, len(spec.GroupBy))
		for _, g := range spec.GroupBy {
			parts = append(parts, qual(g.Bind, g.Col))
		}
		b.WriteString(" GROUP BY ")
		b.WriteString(strings.Join(parts, ", "))
	}

	if len(spec.OrderBy) > 0 {
		parts := make([]string, 0, len(spec.OrderBy))
		for _, o := range spec.OrderBy {
			dir := "ASC"
			if strings.EqualFold(o.Dir, "desc") {
				dir = "DESC"
			}
			parts = append(parts, quoteIdent(o.As)+" "+dir)
		}
		b.WriteString(" ORDER BY ")
		b.WriteString(strings.Join(parts, ", "))
	}

	if spec.Limit > 0 {
		b.WriteString(fmt.Sprintf(" LIMIT %d", spec.Limit))
	}
	if spec.Offset > 0 {
		b.WriteString(fmt.Sprintf(" OFFSET %d", spec.Offset))
	}
	return b.String(), args, nil
}

func whereSQL(w *WhereExpr, args *[]any) (string, error) {
	if w == nil {
		return "", nil
	}
	switch strings.ToLower(w.Op) {
	case "and", "or":
		parts := make([]string, 0, len(w.Args))
		for i := range w.Args {
			p, err := whereSQL(&w.Args[i], args)
			if err != nil {
				return "", err
			}
			if p != "" {
				parts = append(parts, "("+p+")")
			}
		}
		if len(parts) == 0 {
			return "", nil
		}
		sep := " AND "
		if strings.EqualFold(w.Op, "or") {
			sep = " OR "
		}
		return strings.Join(parts, sep), nil
	case "not":
		if len(w.Args) != 1 {
			return "", protocol.NewError(protocol.ErrUnsupported, "not requires one arg", nil)
		}
		p, err := whereSQL(&w.Args[0], args)
		if err != nil {
			return "", err
		}
		return "NOT (" + p + ")", nil
	case "eq", "neq", "gt", "gte", "lt", "lte":
		op := map[string]string{
			"eq": "=", "neq": "<>", "gt": ">", "gte": ">=", "lt": "<", "lte": "<=",
		}[strings.ToLower(w.Op)]
		*args = append(*args, w.Value)
		return qual(w.Bind, w.Col) + " " + op + " ?", nil
	case "in", "nin":
		list, ok := w.Value.([]any)
		if !ok {
			return "", protocol.NewError(protocol.ErrUnsupported, "in/nin value must be an array", nil)
		}
		if len(list) == 0 {
			if strings.EqualFold(w.Op, "in") {
				return "1=0", nil
			}
			return "1=1", nil
		}
		ph := make([]string, len(list))
		for i, v := range list {
			*args = append(*args, v)
			ph[i] = "?"
		}
		clause := qual(w.Bind, w.Col) + " IN (" + strings.Join(ph, ",") + ")"
		if strings.EqualFold(w.Op, "nin") {
			clause = qual(w.Bind, w.Col) + " NOT IN (" + strings.Join(ph, ",") + ")"
		}
		return clause, nil
	case "contains":
		*args = append(*args, fmt.Sprint(w.Value))
		return "CAST(" + qual(w.Bind, w.Col) + " AS VARCHAR) LIKE '%' || ? || '%'", nil
	case "is_null":
		return qual(w.Bind, w.Col) + " IS NULL", nil
	case "not_null":
		return qual(w.Bind, w.Col) + " IS NOT NULL", nil
	default:
		return "", protocol.NewError(protocol.ErrUnsupported, "unsupported local where op: "+w.Op, map[string]any{"op": w.Op})
	}
}
