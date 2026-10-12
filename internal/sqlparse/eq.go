package sqlparse

import (
	"strings"

	"qLLM/internal/protocol"
)

// EqFilter is a simple equality found in an outer WHERE AND-chain.
type EqFilter struct {
	Qual  string // optional table/alias qualifier
	Field string
	Value string // normalized scalar text (unquoted)
}

// EqExtract holds equality filters from the outermost WHERE.
type EqExtract struct {
	Filters []EqFilter
	// Ambiguous is true when OR / NOT / IN (or non-eq compare) appears in that WHERE,
	// so callers should treat constrained fields as conflicting rather than trusting Filters alone.
	Ambiguous bool
}

// ExtractEqualityFilters finds field = literal (and literal = field) under the first
// top-level WHERE of the SQL statement. Only AND-chains are considered reliable;
// OR/NOT/IN mark Ambiguous.
func ExtractEqualityFilters(sql string) (*EqExtract, *protocol.ProtocolError) {
	toks, err := lex(strings.TrimSpace(sql))
	if err != nil {
		return nil, err
	}
	out := &EqExtract{}
	start, end := whereSpan(toks)
	if start < 0 {
		return out, nil
	}
	depth := 0
	i := start
	for i < end {
		t := toks[i]
		if t.kind == 'p' && t.val == "(" {
			depth++
			i++
			continue
		}
		if t.kind == 'p' && t.val == ")" {
			if depth > 0 {
				depth--
			}
			i++
			continue
		}
		if depth == 0 && t.kind == 'i' {
			u := strings.ToUpper(t.val)
			if u == "OR" || u == "NOT" || u == "IN" || u == "BETWEEN" || u == "LIKE" || u == "ILIKE" {
				out.Ambiguous = true
			}
		}
		if depth == 0 {
			if eq, next, ok := tryEqAt(toks, i, end); ok {
				out.Filters = append(out.Filters, eq)
				i = next
				continue
			}
		}
		i++
	}
	return out, nil
}

// ValuesForField returns equality values for a logical field name (case-insensitive), ignoring qualifier.
func (e *EqExtract) ValuesForField(field string) []string {
	if e == nil {
		return nil
	}
	want := strings.ToLower(field)
	var out []string
	for _, f := range e.Filters {
		if strings.ToLower(f.Field) == want {
			out = append(out, f.Value)
		}
	}
	return out
}

func whereSpan(toks []tok) (start, end int) {
	depth := 0
	start = -1
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.kind == 'p' && t.val == "(" {
			depth++
			continue
		}
		if t.kind == 'p' && t.val == ")" {
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth != 0 {
			continue
		}
		if t.kind == 'i' && strings.EqualFold(t.val, "WHERE") {
			start = i + 1
			continue
		}
		if start >= 0 && t.kind == 'i' {
			u := strings.ToUpper(t.val)
			switch u {
			case "GROUP", "ORDER", "LIMIT", "HAVING", "QUALIFY", "UNION", "INTERSECT", "EXCEPT", "OFFSET", "FETCH":
				return start, i
			}
		}
	}
	if start < 0 {
		return -1, -1
	}
	return start, len(toks)
}

func tryEqAt(toks []tok, i, end int) (EqFilter, int, bool) {
	// qual.field = lit  |  field = lit
	if i+2 < end {
		if left, next, ok := readFieldRef(toks, i, end); ok {
			if next < end && toks[next].kind == 'p' && toks[next].val == "=" {
				if lit, n2, ok2 := readLiteral(toks, next+1, end); ok2 {
					return EqFilter{Qual: left.qual, Field: left.field, Value: lit}, n2, true
				}
			}
		}
	}
	// lit = qual.field | lit = field
	if i+2 < end {
		if lit, n1, ok := readLiteral(toks, i, end); ok {
			if n1 < end && toks[n1].kind == 'p' && toks[n1].val == "=" {
				if right, n2, ok2 := readFieldRef(toks, n1+1, end); ok2 {
					return EqFilter{Qual: right.qual, Field: right.field, Value: lit}, n2, true
				}
			}
		}
	}
	return EqFilter{}, i, false
}

type fieldRef struct {
	qual  string
	field string
}

func readFieldRef(toks []tok, i, end int) (fieldRef, int, bool) {
	if i >= end || toks[i].kind != 'i' {
		return fieldRef{}, i, false
	}
	if isSQLKeyword(toks[i].val) {
		return fieldRef{}, i, false
	}
	if i+2 < end && toks[i+1].kind == 'p' && toks[i+1].val == "." && toks[i+2].kind == 'i' {
		return fieldRef{qual: toks[i].val, field: toks[i+2].val}, i + 3, true
	}
	return fieldRef{field: toks[i].val}, i + 1, true
}

func readLiteral(toks []tok, i, end int) (string, int, bool) {
	if i >= end {
		return "", i, false
	}
	t := toks[i]
	switch t.kind {
	case 's':
		return unquoteSQLString(t.val), i + 1, true
	case 'n':
		return t.val, i + 1, true
	case 'i':
		u := strings.ToUpper(t.val)
		if u == "TRUE" || u == "FALSE" || u == "NULL" {
			return strings.ToLower(t.val), i + 1, true
		}
	}
	return "", i, false
}

func unquoteSQLString(s string) string {
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		inner := s[1 : len(s)-1]
		return strings.ReplaceAll(inner, "''", "'")
	}
	return s
}
