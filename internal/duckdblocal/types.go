package duckdblocal

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"qLLM/internal/protocol"
	"qLLM/internal/result"
)

// QuerySpec is the structured local-compute request (shared by pure Go and DuckDB backends).
type JoinOn struct {
	LeftBind  string
	LeftCol   string
	RightBind string
	RightCol  string
}

type JoinSpec struct {
	Type       string // left|inner
	RightTable string
	On         []JoinOn // AND of equality pairs
}

type SelectSpec struct {
	Bind string // empty for pure agg without field
	Col  string
	Agg  string // count|sum|avg|min|max or empty
	As   string
}

// WhereExpr mirrors Query IR where trees (and/or/not + compare leaves).
type WhereExpr struct {
	Op    string // and|or|not|eq|neq|gt|gte|lt|lte|in|nin|contains|is_null|not_null
	Args  []WhereExpr
	Bind  string
	Col   string
	Value any
}

type OrderSpec struct {
	As  string
	Dir string
}

type QuerySpec struct {
	RootBind string
	Joins    []JoinSpec
	Select   []SelectSpec
	Where    *WhereExpr
	GroupBy  []SelectSpec
	OrderBy  []OrderSpec
	Limit    int
	Offset   int
}

// Engine is the local compute façade (pure Go or DuckDB via build tags).
type Engine interface {
	Close() error
	Materialize(ctx context.Context, table string, tab *protocol.TabularResult) error
	Execute(ctx context.Context, spec QuerySpec) (*protocol.TabularResult, error)
	ExecSQL(ctx context.Context, sql string) (*protocol.TabularResult, error)
}

type nrow struct {
	byBind map[string][]any
}

// copyBind implements runtime behavior for this package.
func copyBind(m map[string][]any) map[string][]any {
	out := make(map[string][]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// cell implements runtime behavior for this package.
func cell(tables map[string]*protocol.TabularResult, byBind map[string][]any, bind, col string) any {
	row := byBind[bind]
	tab := tables[bind]
	if row == nil || tab == nil {
		return nil
	}
	i := colIndex(tab, col)
	if i < 0 || i >= len(row) {
		return nil
	}
	return row[i]
}

// projectStructured implements runtime behavior for this package.
func projectStructured(tables map[string]*protocol.TabularResult, byBind map[string][]any, sels []SelectSpec) []any {
	out := make([]any, len(sels))
	for i, s := range sels {
		out[i] = cell(tables, byBind, s.Bind, s.Col)
	}
	return out
}

// matchWhere implements runtime behavior for this package.
func matchWhere(tables map[string]*protocol.TabularResult, byBind map[string][]any, w *WhereExpr) (bool, error) {
	if w == nil {
		return true, nil
	}
	switch strings.ToLower(w.Op) {
	case "and":
		for i := range w.Args {
			ok, err := matchWhere(tables, byBind, &w.Args[i])
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil
	case "or":
		if len(w.Args) == 0 {
			return true, nil
		}
		for i := range w.Args {
			ok, err := matchWhere(tables, byBind, &w.Args[i])
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	case "not":
		if len(w.Args) != 1 {
			return false, protocol.NewError(protocol.ErrUnsupported, "not requires one arg", nil)
		}
		ok, err := matchWhere(tables, byBind, &w.Args[0])
		if err != nil {
			return false, err
		}
		return !ok, nil
	default:
		v := cell(tables, byBind, w.Bind, w.Col)
		ok, err := compare(v, w.Op, w.Value)
		return ok, err
	}
}

// compare implements runtime behavior for this package.
func compare(left any, op string, right any) (bool, error) {
	switch strings.ToLower(op) {
	case "eq":
		return fmt.Sprint(left) == fmt.Sprint(right), nil
	case "neq":
		return fmt.Sprint(left) != fmt.Sprint(right), nil
	case "gt", "gte", "lt", "lte":
		if left == nil || right == nil {
			return false, nil
		}
		lf, lok := asFloat(left)
		rf, rok := asFloat(right)
		if lok && rok {
			switch op {
			case "gt":
				return lf > rf, nil
			case "gte":
				return lf >= rf, nil
			case "lt":
				return lf < rf, nil
			case "lte":
				return lf <= rf, nil
			}
		}
		ls, rs := fmt.Sprint(left), fmt.Sprint(right)
		switch op {
		case "gt":
			return ls > rs, nil
		case "gte":
			return ls >= rs, nil
		case "lt":
			return ls < rs, nil
		case "lte":
			return ls <= rs, nil
		}
		return false, nil
	case "in":
		list, err := asAnySlice(right)
		if err != nil {
			return false, err
		}
		ls := fmt.Sprint(left)
		for _, item := range list {
			if ls == fmt.Sprint(item) {
				return true, nil
			}
		}
		return false, nil
	case "nin":
		ok, err := compare(left, "in", right)
		if err != nil {
			return false, err
		}
		return !ok, nil
	case "contains":
		return strings.Contains(fmt.Sprint(left), fmt.Sprint(right)), nil
	case "is_null":
		return left == nil, nil
	case "not_null":
		return left != nil, nil
	default:
		return false, protocol.NewError(protocol.ErrUnsupported, "unsupported local where op: "+op, map[string]any{"op": op})
	}
}

// asAnySlice implements runtime behavior for this package.
func asAnySlice(v any) ([]any, error) {
	switch t := v.(type) {
	case []any:
		return t, nil
	case nil:
		return nil, protocol.NewError(protocol.ErrUnsupported, "in/nin value must be an array", nil)
	default:
		return nil, protocol.NewError(protocol.ErrUnsupported, "in/nin value must be an array", nil)
	}
}

// asFloat implements runtime behavior for this package.
func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(t, 64)
		return f, err == nil
	default:
		f, err := strconv.ParseFloat(fmt.Sprint(v), 64)
		return f, err == nil
	}
}

// agg implements runtime behavior for this package.
func agg(tables map[string]*protocol.TabularResult, rows []nrow, s SelectSpec) any {
	switch strings.ToLower(s.Agg) {
	case "count":
		if s.Col == "" {
			return len(rows)
		}
		n := 0
		for _, r := range rows {
			if cell(tables, r.byBind, s.Bind, s.Col) != nil {
				n++
			}
		}
		return n
	case "sum", "avg", "min", "max":
		vals := []float64{}
		for _, r := range rows {
			if f, ok := asFloat(cell(tables, r.byBind, s.Bind, s.Col)); ok {
				vals = append(vals, f)
			}
		}
		if len(vals) == 0 {
			return nil
		}
		switch strings.ToLower(s.Agg) {
		case "sum":
			var s float64
			for _, v := range vals {
				s += v
			}
			return s
		case "avg":
			var s float64
			for _, v := range vals {
				s += v
			}
			return s / float64(len(vals))
		case "min":
			m := vals[0]
			for _, v := range vals[1:] {
				if v < m {
					m = v
				}
			}
			return m
		case "max":
			m := vals[0]
			for _, v := range vals[1:] {
				if v > m {
					m = v
				}
			}
			return m
		}
	}
	return nil
}

// cmpAny implements runtime behavior for this package.
func cmpAny(a, b any) int {
	af, aok := asFloat(a)
	bf, bok := asFloat(b)
	if aok && bok {
		if af < bf {
			return -1
		}
		if af > bf {
			return 1
		}
		return 0
	}
	as, bs := fmt.Sprint(a), fmt.Sprint(b)
	return strings.Compare(as, bs)
}

// colIndex implements runtime behavior for this package.
func colIndex(tab *protocol.TabularResult, name string) int {
	for i, c := range tab.Columns {
		if c.Name == name {
			return i
		}
	}
	return -1
}

// joinMatch implements runtime behavior for this package.
func joinMatch(tables map[string]*protocol.TabularResult, row nrow, j JoinSpec, rr []any, right *protocol.TabularResult) bool {
	for _, on := range j.On {
		lv := row.byBind[on.LeftBind]
		if lv == nil {
			return false
		}
		lt := tables[on.LeftBind]
		if lt == nil {
			return false
		}
		lIdx := colIndex(lt, on.LeftCol)
		rIdx := colIndex(right, on.RightCol)
		if lIdx < 0 || rIdx < 0 || lIdx >= len(lv) || rIdx >= len(rr) {
			return false
		}
		if fmt.Sprint(lv[lIdx]) != fmt.Sprint(rr[rIdx]) {
			return false
		}
	}
	return true
}

// applyOffsetLimit implements runtime behavior for this package.
func applyOffsetLimit(rows [][]any, offset, limit int) [][]any {
	if offset > 0 {
		if offset >= len(rows) {
			return rows[:0]
		}
		rows = rows[offset:]
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

// ExecutePure runs structured multi-join / where / agg over in-memory tables.
func ExecutePure(tables map[string]*protocol.TabularResult, spec QuerySpec) (*protocol.TabularResult, error) {
	root, ok := tables[spec.RootBind]
	if !ok {
		return nil, fmt.Errorf("missing table %s", spec.RootBind)
	}

	rows := make([]nrow, 0, len(root.Rows))
	for _, r := range root.Rows {
		cp := append([]any{}, r...)
		rows = append(rows, nrow{byBind: map[string][]any{spec.RootBind: cp}})
	}

	for _, j := range spec.Joins {
		if len(j.On) == 0 {
			return nil, protocol.NewError(protocol.ErrUnsupported, "join requires at least one on pair", nil)
		}
		right, ok := tables[j.RightTable]
		if !ok {
			return nil, fmt.Errorf("missing table %s", j.RightTable)
		}
		for _, on := range j.On {
			if tables[on.LeftBind] == nil {
				return nil, fmt.Errorf("missing left table %s", on.LeftBind)
			}
			if colIndex(tables[on.LeftBind], on.LeftCol) < 0 || colIndex(right, on.RightCol) < 0 {
				return nil, fmt.Errorf("join columns not found %s.%s = %s.%s", on.LeftBind, on.LeftCol, on.RightBind, on.RightCol)
			}
		}

		next := make([]nrow, 0, len(rows))
		for _, row := range rows {
			matched := false
			for _, rr := range right.Rows {
				if joinMatch(tables, row, j, rr, right) {
					matched = true
					nb := copyBind(row.byBind)
					nb[j.RightTable] = append([]any{}, rr...)
					next = append(next, nrow{byBind: nb})
				}
			}
			if !matched && strings.EqualFold(j.Type, "left") {
				nb := copyBind(row.byBind)
				nulls := make([]any, len(right.Columns))
				nb[j.RightTable] = nulls
				next = append(next, nrow{byBind: nb})
			}
		}
		rows = next
	}

	if spec.Where != nil {
		filtered := rows[:0]
		for _, row := range rows {
			ok, err := matchWhere(tables, row.byBind, spec.Where)
			if err != nil {
				return nil, err
			}
			if ok {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}

	hasAgg := false
	for _, s := range spec.Select {
		if s.Agg != "" {
			hasAgg = true
			break
		}
	}

	var outCols []protocol.Column
	var outRows [][]any

	if !hasAgg {
		for _, s := range spec.Select {
			as := s.As
			if as == "" {
				as = s.Col
			}
			typ := protocol.TypeString
			if tab := tables[s.Bind]; tab != nil {
				if i := colIndex(tab, s.Col); i >= 0 {
					typ = tab.Columns[i].Type
				}
			}
			outCols = append(outCols, protocol.Column{Name: as, Type: typ})
		}
		for _, row := range rows {
			outRows = append(outRows, projectStructured(tables, row.byBind, spec.Select))
		}
	} else {
		type gbKey string
		groups := map[gbKey][]nrow{}
		orderKeys := []gbKey{}
		for _, row := range rows {
			parts := make([]string, 0, len(spec.GroupBy))
			for _, g := range spec.GroupBy {
				parts = append(parts, fmt.Sprint(cell(tables, row.byBind, g.Bind, g.Col)))
			}
			k := gbKey(strings.Join(parts, "\x1e"))
			if _, ok := groups[k]; !ok {
				orderKeys = append(orderKeys, k)
			}
			groups[k] = append(groups[k], row)
		}
		for _, s := range spec.Select {
			as := s.As
			if as == "" {
				as = s.Col
			}
			typ := protocol.TypeString
			if s.Agg != "" {
				typ = protocol.TypeNumber
			} else if tab := tables[s.Bind]; tab != nil {
				if i := colIndex(tab, s.Col); i >= 0 {
					typ = tab.Columns[i].Type
				}
			}
			outCols = append(outCols, protocol.Column{Name: as, Type: typ})
		}
		for _, k := range orderKeys {
			grows := groups[k]
			line := make([]any, len(spec.Select))
			for i, s := range spec.Select {
				if s.Agg == "" {
					line[i] = cell(tables, grows[0].byBind, s.Bind, s.Col)
					continue
				}
				line[i] = agg(tables, grows, s)
			}
			outRows = append(outRows, line)
		}
	}

	if len(spec.OrderBy) > 0 {
		idx := map[string]int{}
		for i, c := range outCols {
			idx[c.Name] = i
		}
		sort.SliceStable(outRows, func(i, j int) bool {
			for _, o := range spec.OrderBy {
				ci, ok := idx[o.As]
				if !ok {
					continue
				}
				cmp := cmpAny(outRows[i][ci], outRows[j][ci])
				if cmp == 0 {
					continue
				}
				if strings.EqualFold(o.Dir, "desc") {
					return cmp > 0
				}
				return cmp < 0
			}
			return false
		})
	}

	outRows = applyOffsetLimit(outRows, spec.Offset, spec.Limit)
	return result.New(outCols, outRows, false), nil
}
