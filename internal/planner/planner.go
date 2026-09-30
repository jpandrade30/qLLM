package planner

import (
	"strings"

	"qLLM/internal/catalogidx"
	"qLLM/internal/connector/def"
	"qLLM/internal/protocol"
)

type PlannedJoin struct {
	Type      string
	LeftBind  string
	RightBind string
	On        []protocol.JoinOn
}

type Plan struct {
	Steps     []def.PushdownStep
	Joins     []PlannedJoin
	UseDuckDB bool
	Limit     int
	Offset    int
	SelectOut []def.SelectItem // final projection names when duckdb
	Bindings  map[string]*protocol.Entity
}

// Build builds a value.
func Build(idx *catalogidx.Index, q *protocol.QueryIR) (*Plan, *protocol.ProtocolError) {
	limit := idx.Preset.Limits.DefaultLimit
	if q.Limit != nil {
		limit = *q.Limit
	}
	offset := 0
	if q.Offset != nil {
		offset = *q.Offset
	}

	bindings := map[string]*protocol.Entity{}
	bindOrder := []string{}

	add := func(ref, as string) (*protocol.Entity, string, *protocol.ProtocolError) {
		ent, err := idx.ResolveEntity(ref)
		if err != nil {
			return nil, "", err
		}
		name := as
		if name == "" {
			name = ent.Name
		}
		bindings[name] = ent
		bindOrder = append(bindOrder, name)
		return ent, name, nil
	}

	rootEnt, rootBind, err := add(q.From, q.As)
	if err != nil {
		return nil, err
	}
	_ = rootEnt

	plan := &Plan{
		Limit:    limit,
		Offset:   offset,
		Bindings: bindings,
	}

	// Determine if cross-source
	sources := map[string]struct{}{}
	sources[bindings[rootBind].Source] = struct{}{}
	for _, j := range q.Joins {
		ent, bname, err := add(j.From, j.As)
		if err != nil {
			return nil, err
		}
		sources[ent.Source] = struct{}{}
		plan.Joins = append(plan.Joins, PlannedJoin{
			Type: j.Type, LeftBind: rootBind, RightBind: bname, On: j.On,
		})
		// For multi-join, left is previous — simplify: chain from root for MVP pairwise later
	}
	cross := len(sources) > 1
	needLocalAgg := false

	// REST cannot agg
	for _, b := range bindings {
		src, _ := idx.Source(b.Source)
		if src != nil && src.Type == protocol.SourceREST {
			for _, item := range q.Select {
				if m, ok := item.(map[string]any); ok {
					if _, has := m["agg"]; has {
						needLocalAgg = true
					}
				}
			}
		}
	}

	plan.UseDuckDB = cross || needLocalAgg || len(q.Joins) > 0 && cross

	// Same-source joins on SQL can stay pushdown later; MVP: any join => fetch sides + duckdb if cross or always duckdb for joins
	if len(q.Joins) > 0 {
		plan.UseDuckDB = true
	}

	for _, bname := range bindOrder {
		ent := bindings[bname]
		step := def.PushdownStep{
			SourceID: ent.Source,
			Entity:   ent,
			Binding:  bname,
			Limit:    limit,
			Offset:   0,
		}
		if plan.UseDuckDB {
			// fetch projected fields needed
			step.Select = fieldsForBinding(q, bname, ent)
			step.Where = filterWhereForBinding(q.Where, bname, len(bindings) == 1)
			if step.Limit < idx.Preset.Limits.MaxLimit {
				// leave limit
			}
		} else {
			step.Select = parseSelect(q.Select, bname, len(bindings) == 1)
			step.Where = q.Where
			step.GroupBy = q.GroupBy
			step.OrderBy = q.OrderBy
			step.Limit = limit
			step.Offset = offset
		}
		plan.Steps = append(plan.Steps, step)
	}

	return plan, nil
}

// parseSelect implements runtime behavior for this package.
func parseSelect(items []any, bind string, single bool) []def.SelectItem {
	out := []def.SelectItem{}
	for _, item := range items {
		switch v := item.(type) {
		case string:
			field := v
			if i := strings.LastIndex(v, "."); i >= 0 {
				if !single && v[:i] != bind {
					continue
				}
				field = v[i+1:]
			}
			out = append(out, def.SelectItem{Field: field, As: field})
		case map[string]any:
			agg, _ := v["agg"].(string)
			field, _ := v["field"].(string)
			as, _ := v["as"].(string)
			if field != "" {
				if i := strings.LastIndex(field, "."); i >= 0 {
					if !single && field[:i] != bind {
						continue
					}
					field = field[i+1:]
				}
			}
			out = append(out, def.SelectItem{Agg: agg, Field: field, As: as})
		}
	}
	return out
}

// fieldsForBinding implements runtime behavior for this package.
func fieldsForBinding(q *protocol.QueryIR, bind string, ent *protocol.Entity) []def.SelectItem {
	needed := map[string]struct{}{}
	addRef := func(ref string) {
		if ref == "" {
			return
		}
		if i := strings.LastIndex(ref, "."); i >= 0 {
			if ref[:i] == bind || ref[:i] == ent.Name {
				needed[ref[i+1:]] = struct{}{}
			}
			return
		}
		// unqualified — include if single handled elsewhere; for duckdb fetch all mentioned
		needed[ref] = struct{}{}
	}
	for _, item := range q.Select {
		switch v := item.(type) {
		case string:
			addRef(v)
		case map[string]any:
			if f, ok := v["field"].(string); ok {
				addRef(f)
			}
		}
	}
	for _, g := range q.GroupBy {
		addRef(g)
	}
	for _, j := range q.Joins {
		for _, on := range j.On {
			addRef(on.Left)
			addRef(on.Right)
		}
	}
	walkWhereFields(q.Where, addRef)

	if len(needed) == 0 {
		for _, f := range ent.Fields {
			needed[f.Name] = struct{}{}
		}
	}
	out := make([]def.SelectItem, 0, len(needed))
	for f := range needed {
		if _, ok := fieldExists(ent, f); ok {
			out = append(out, def.SelectItem{Field: f, As: f})
		}
	}
	return out
}

// fieldExists implements runtime behavior for this package.
func fieldExists(e *protocol.Entity, name string) (*protocol.Field, bool) {
	for i := range e.Fields {
		if e.Fields[i].Name == name {
			return &e.Fields[i], true
		}
	}
	return nil, false
}

// walkWhereFields implements runtime behavior for this package.
func walkWhereFields(w map[string]any, add func(string)) {
	if w == nil {
		return
	}
	if op, ok := w["op"].(string); ok && (op == "and" || op == "or" || op == "not") {
		args, _ := w["args"].([]any)
		for _, a := range args {
			m, _ := a.(map[string]any)
			walkWhereFields(m, add)
		}
		return
	}
	if f, ok := w["field"].(string); ok {
		add(f)
	}
}

// filterWhereForBinding implements runtime behavior for this package.
func filterWhereForBinding(w map[string]any, bind string, single bool) map[string]any {
	if w == nil || single {
		return w
	}
	// MVP: only push predicates clearly bound to this alias
	return filterWhere(w, bind)
}

// filterWhere implements runtime behavior for this package.
func filterWhere(w map[string]any, bind string) map[string]any {
	if w == nil {
		return nil
	}
	if op, ok := w["op"].(string); ok && (op == "and" || op == "or") {
		args, _ := w["args"].([]any)
		kept := []any{}
		for _, a := range args {
			m, _ := a.(map[string]any)
			if fw := filterWhere(m, bind); fw != nil {
				kept = append(kept, fw)
			}
		}
		if len(kept) == 0 {
			return nil
		}
		if len(kept) == 1 {
			return kept[0].(map[string]any)
		}
		return map[string]any{"op": op, "args": kept}
	}
	if f, ok := w["field"].(string); ok {
		if i := strings.LastIndex(f, "."); i >= 0 {
			if f[:i] != bind {
				return nil
			}
		}
		return w
	}
	return nil
}
