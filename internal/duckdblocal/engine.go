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

// Engine is an in-process local compute layer (pure Go).
type Engine struct {
	tables map[string]*protocol.TabularResult
}

func Open() (*Engine, error) {
	return &Engine{tables: map[string]*protocol.TabularResult{}}, nil
}

func (e *Engine) Close() error { return nil }

type nrow struct {
	byBind map[string][]any
}

func (e *Engine) Materialize(ctx context.Context, table string, tab *protocol.TabularResult) error {
	_ = ctx
	cp := *tab
	cp.Rows = append([][]any{}, tab.Rows...)
	e.tables[table] = &cp
	return nil
}

type JoinSpec struct {
	Type       string // left|inner
	RightTable string
	LeftBind   string
	LeftCol    string
	RightBind  string
	RightCol   string
}

type SelectSpec struct {
	Bind  string // empty for pure agg without field
	Col   string
	Agg   string // count|sum|avg|min|max or empty
	As    string
}

type PredSpec struct {
	Bind string
	Col  string
	Op   string // eq,neq,gt,gte,lt,lte
	Value any
}

type QuerySpec struct {
	RootBind string
	Joins    []JoinSpec
	Select   []SelectSpec
	Where    []PredSpec // AND only for MVP structured path
	GroupBy  []SelectSpec // Bind+Col
	OrderBy  []struct {
		As  string
		Dir string
	}
	Limit int
}

// Execute runs a structured multi-join / agg query over materialized tables.
func (e *Engine) Execute(ctx context.Context, spec QuerySpec) (*protocol.TabularResult, error) {
	_ = ctx
	root, ok := e.tables[spec.RootBind]
	if !ok {
		return nil, fmt.Errorf("missing table %s", spec.RootBind)
	}

	// rows as map[bind][]any aligned with each table's columns
	rows := make([]nrow, 0, len(root.Rows))
	for _, r := range root.Rows {
		cp := append([]any{}, r...)
		rows = append(rows, nrow{byBind: map[string][]any{spec.RootBind: cp}})
	}

	for _, j := range spec.Joins {
		right, ok := e.tables[j.RightTable]
		if !ok {
			return nil, fmt.Errorf("missing table %s", j.RightTable)
		}
		leftTab := e.tables[j.LeftBind]
		if leftTab == nil && j.LeftBind == spec.RootBind {
			leftTab = root
		}
		// left col index from original left table schema
		lt := e.tables[j.LeftBind]
		if lt == nil {
			return nil, fmt.Errorf("missing left table %s", j.LeftBind)
		}
		lIdx := colIndex(lt, j.LeftCol)
		rIdx := colIndex(right, j.RightCol)
		if lIdx < 0 || rIdx < 0 {
			return nil, fmt.Errorf("join columns not found %s.%s = %s.%s", j.LeftBind, j.LeftCol, j.RightBind, j.RightCol)
		}

		next := make([]nrow, 0, len(rows))
		for _, row := range rows {
			lv := row.byBind[j.LeftBind]
			if lv == nil {
				continue
			}
			matched := false
			for _, rr := range right.Rows {
				if fmt.Sprint(lv[lIdx]) == fmt.Sprint(rr[rIdx]) {
					matched = true
					nb := copyBind(row.byBind)
					nb[j.RightBind] = append([]any{}, rr...)
					next = append(next, nrow{byBind: nb})
				}
			}
			if !matched && strings.EqualFold(j.Type, "left") {
				nb := copyBind(row.byBind)
				nulls := make([]any, len(right.Columns))
				nb[j.RightBind] = nulls
				next = append(next, nrow{byBind: nb})
			}
		}
		rows = next
	}

	// WHERE
	if len(spec.Where) > 0 {
		filtered := rows[:0]
		for _, row := range rows {
			if matchPreds(e, row.byBind, spec.Where) {
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
			if tab := e.tables[s.Bind]; tab != nil {
				if i := colIndex(tab, s.Col); i >= 0 {
					typ = tab.Columns[i].Type
				}
			}
			outCols = append(outCols, protocol.Column{Name: as, Type: typ})
		}
		for _, row := range rows {
			outRows = append(outRows, projectStructured(e, row.byBind, spec.Select))
		}
	} else {
		// group
		type gbKey string
		groups := map[gbKey][]nrow{}
		orderKeys := []gbKey{}
		for _, row := range rows {
			parts := make([]string, 0, len(spec.GroupBy))
			for _, g := range spec.GroupBy {
				parts = append(parts, fmt.Sprint(cell(e, row.byBind, g.Bind, g.Col)))
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
			} else if tab := e.tables[s.Bind]; tab != nil {
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
					line[i] = cell(e, grows[0].byBind, s.Bind, s.Col)
					continue
				}
				line[i] = agg(e, grows, s)
			}
			outRows = append(outRows, line)
		}
	}

	// order by output alias
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

	if spec.Limit > 0 && len(outRows) > spec.Limit {
		outRows = outRows[:spec.Limit]
	}
	return result.New(outCols, outRows, false), nil
}

func copyBind(m map[string][]any) map[string][]any {
	out := make(map[string][]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cell(e *Engine, byBind map[string][]any, bind, col string) any {
	row := byBind[bind]
	tab := e.tables[bind]
	if row == nil || tab == nil {
		return nil
	}
	i := colIndex(tab, col)
	if i < 0 || i >= len(row) {
		return nil
	}
	return row[i]
}

func projectStructured(e *Engine, byBind map[string][]any, sels []SelectSpec) []any {
	out := make([]any, len(sels))
	for i, s := range sels {
		out[i] = cell(e, byBind, s.Bind, s.Col)
	}
	return out
}

func matchPreds(e *Engine, byBind map[string][]any, preds []PredSpec) bool {
	for _, p := range preds {
		v := cell(e, byBind, p.Bind, p.Col)
		if !compare(v, p.Op, p.Value) {
			return false
		}
	}
	return true
}

func compare(left any, op string, right any) bool {
	switch op {
	case "eq":
		return fmt.Sprint(left) == fmt.Sprint(right)
	case "neq":
		return fmt.Sprint(left) != fmt.Sprint(right)
	case "gt", "gte", "lt", "lte":
		lf, lok := asFloat(left)
		rf, rok := asFloat(right)
		if lok && rok {
			switch op {
			case "gt":
				return lf > rf
			case "gte":
				return lf >= rf
			case "lt":
				return lf < rf
			case "lte":
				return lf <= rf
			}
		}
		ls, rs := fmt.Sprint(left), fmt.Sprint(right)
		switch op {
		case "gt":
			return ls > rs
		case "gte":
			return ls >= rs
		case "lt":
			return ls < rs
		case "lte":
			return ls <= rs
		}
	}
	return true
}

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

func agg(e *Engine, rows []nrow, s SelectSpec) any {
	switch strings.ToLower(s.Agg) {
	case "count":
		if s.Col == "" {
			return len(rows)
		}
		n := 0
		for _, r := range rows {
			if cell(e, r.byBind, s.Bind, s.Col) != nil {
				n++
			}
		}
		return n
	case "sum", "avg", "min", "max":
		vals := []float64{}
		for _, r := range rows {
			if f, ok := asFloat(cell(e, r.byBind, s.Bind, s.Col)); ok {
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

func colIndex(tab *protocol.TabularResult, name string) int {
	for i, c := range tab.Columns {
		if c.Name == name {
			return i
		}
	}
	return -1
}

// QuerySQL kept for simple tests / single-table SELECT *.
func (e *Engine) QuerySQL(ctx context.Context, sqlStr string) (*protocol.TabularResult, error) {
	_ = ctx
	sqlStr = strings.TrimSpace(sqlStr)
	upper := strings.ToUpper(sqlStr)
	if strings.HasPrefix(upper, "SELECT * FROM") && !strings.Contains(upper, " JOIN ") {
		parts := strings.Fields(sqlStr)
		var table string
		limit := -1
		for i, p := range parts {
			if strings.EqualFold(p, "FROM") && i+1 < len(parts) {
				table = strings.Trim(parts[i+1], `"`)
			}
			if strings.EqualFold(p, "LIMIT") && i+1 < len(parts) {
				limit, _ = strconv.Atoi(parts[i+1])
			}
		}
		tab, ok := e.tables[table]
		if !ok {
			return nil, fmt.Errorf("unknown table %s", table)
		}
		rows := tab.Rows
		if limit >= 0 && len(rows) > limit {
			rows = rows[:limit]
		}
		return result.New(tab.Columns, rows, false), nil
	}
	return nil, fmt.Errorf("QuerySQL only supports SELECT * FROM \"t\"; use Execute for joins")
}
