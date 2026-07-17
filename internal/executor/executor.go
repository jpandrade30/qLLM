package executor

import (
	"context"
	"strings"
	"time"

	"qLLM/internal/catalogidx"
	"qLLM/internal/connector"
	"qLLM/internal/duckdblocal"
	"qLLM/internal/planner"
	"qLLM/internal/protocol"
	"qLLM/internal/querystore"
	"qLLM/internal/validate"

	"github.com/google/uuid"
)

type Executor struct {
	Idx     *catalogidx.Index
	Reg     *connector.Registry
	Store   *querystore.Store
}

func New(idx *catalogidx.Index, reg *connector.Registry, store *querystore.Store) *Executor {
	return &Executor{Idx: idx, Reg: reg, Store: store}
}

func (e *Executor) Execute(ctx context.Context, q *protocol.QueryIR) *protocol.QueryResponse {
	start := time.Now()
	queryID := uuid.NewString()
	mode := q.Mode
	if mode == "" {
		mode = "sync"
	}

	if err := validate.Query(e.Idx, q); err != nil {
		return fail(queryID, mode, start, err)
	}

	budget := time.Duration(e.Idx.Preset.Limits.MaxSyncMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	plan, err := planner.Build(e.Idx, q)
	if err != nil {
		return fail(queryID, mode, start, err)
	}

	if mode == "async" && e.Store != nil {
		resp := &protocol.QueryResponse{
			ProtocolVersion: protocol.ProtocolVersion,
			QueryID:         queryID,
			Status:          protocol.StatusAccepted,
		}
		e.Store.Put(resp)
		go func() {
			bg := context.Background()
			bg, cancel := context.WithTimeout(bg, budget)
			defer cancel()
			running := &protocol.QueryResponse{
				ProtocolVersion: protocol.ProtocolVersion,
				QueryID:         queryID,
				Status:          protocol.StatusRunning,
			}
			e.Store.Update(running)
			out := e.run(bg, queryID, mode, plan, q, start)
			e.Store.Update(out)
		}()
		return resp
	}

	return e.run(ctx, queryID, mode, plan, q, start)
}

func (e *Executor) run(ctx context.Context, queryID, mode string, plan *planner.Plan, q *protocol.QueryIR, start time.Time) *protocol.QueryResponse {
	srcBudget := time.Duration(e.Idx.Preset.Limits.MaxSourceMs) * time.Millisecond
	metaSteps := []protocol.PlanStepMeta{}
	partials := map[string]*protocol.TabularResult{}

	for _, step := range plan.Steps {
		c, err := e.Reg.Get(step.SourceID)
		if err != nil {
			return fail(queryID, mode, start, asProto(err))
		}
		stepCtx, cancel := context.WithTimeout(ctx, srcBudget)
		t0 := time.Now()
		tab, qerr := c.Query(stepCtx, step)
		elapsed := time.Since(t0).Milliseconds()
		cancel()
		metaSteps = append(metaSteps, protocol.PlanStepMeta{
			Source: step.SourceID, Pushdown: !plan.UseDuckDB, ElapsedMs: elapsed,
		})
		if qerr != nil {
			return fail(queryID, mode, start, asProto(qerr))
		}
		partials[step.Binding] = tab
	}

	var final *protocol.TabularResult
	usedDuck := plan.UseDuckDB

	if plan.UseDuckDB {
		eng, err := duckdblocal.Open()
		if err != nil {
			return fail(queryID, mode, start, protocol.NewError(protocol.ErrInternal, err.Error(), nil))
		}
		defer eng.Close()

		for bind, tab := range partials {
			if err := eng.Materialize(ctx, bind, tab); err != nil {
				return fail(queryID, mode, start, protocol.NewError(protocol.ErrInternal, err.Error(), nil))
			}
		}

		spec := buildLocalSpec(plan, q)
		tab, err := eng.Execute(ctx, spec)
		if err != nil {
			if ctx.Err() != nil {
				return fail(queryID, mode, start, protocol.NewError(protocol.ErrTimeout, "query budget exceeded", nil))
			}
			return fail(queryID, mode, start, protocol.NewError(protocol.ErrInternal, err.Error(), nil))
		}
		final = tab
	} else {
		// single step
		for _, tab := range partials {
			final = tab
			break
		}
	}

	return &protocol.QueryResponse{
		ProtocolVersion: protocol.ProtocolVersion,
		QueryID:         queryID,
		Status:          protocol.StatusSucceeded,
		Result:          final,
		Meta: &protocol.QueryMeta{
			ElapsedMs: time.Since(start).Milliseconds(),
			Mode:      mode,
			Plan: &protocol.PlanMeta{
				UsedDuckDB: usedDuck,
				Steps:      metaSteps,
			},
		},
	}
}

func buildLocalSpec(plan *planner.Plan, q *protocol.QueryIR) duckdblocal.QuerySpec {
	root := plan.Steps[0].Binding
	spec := duckdblocal.QuerySpec{
		RootBind: root,
		Limit:    plan.Limit,
	}
	for _, j := range plan.Joins {
		for _, on := range j.On {
			lb, lc := splitRef(on.Left, root)
			rb, rc := splitRef(on.Right, j.RightBind)
			spec.Joins = append(spec.Joins, duckdblocal.JoinSpec{
				Type:       j.Type,
				RightTable: j.RightBind,
				LeftBind:   lb,
				LeftCol:    lc,
				RightBind:  rb,
				RightCol:   rc,
			})
		}
	}
	for _, item := range q.Select {
		switch v := item.(type) {
		case string:
			b, c := splitRef(v, root)
			spec.Select = append(spec.Select, duckdblocal.SelectSpec{Bind: b, Col: c, As: colAlias(v)})
		case map[string]any:
			agg, _ := v["agg"].(string)
			field, _ := v["field"].(string)
			as, _ := v["as"].(string)
			b, c := "", ""
			if field != "" {
				b, c = splitRef(field, root)
			}
			if as == "" {
				as = c
			}
			spec.Select = append(spec.Select, duckdblocal.SelectSpec{Bind: b, Col: c, Agg: agg, As: as})
		}
	}
	for _, g := range q.GroupBy {
		b, c := splitRef(g, root)
		spec.GroupBy = append(spec.GroupBy, duckdblocal.SelectSpec{Bind: b, Col: c})
	}
	spec.Where = flattenPreds(q.Where, root)
	for _, o := range q.OrderBy {
		dir := o.Dir
		if dir == "" {
			dir = "asc"
		}
		// order by field may be output alias or qualified field
		as := colAlias(o.Field)
		spec.OrderBy = append(spec.OrderBy, struct {
			As  string
			Dir string
		}{As: as, Dir: dir})
	}
	return spec
}

func flattenPreds(w map[string]any, root string) []duckdblocal.PredSpec {
	if w == nil {
		return nil
	}
	if op, ok := w["op"].(string); ok {
		switch op {
		case "and":
			args, _ := w["args"].([]any)
			out := []duckdblocal.PredSpec{}
			for _, a := range args {
				m, _ := a.(map[string]any)
				out = append(out, flattenPreds(m, root)...)
			}
			return out
		case "or", "not":
			// structured local engine: only AND pushdown; skip complex trees
			return nil
		}
	}
	field, _ := w["field"].(string)
	op, _ := w["op"].(string)
	if field == "" || op == "" {
		return nil
	}
	b, c := splitRef(field, root)
	return []duckdblocal.PredSpec{{Bind: b, Col: c, Op: op, Value: w["value"]}}
}

func splitRef(ref, defaultBind string) (bind, col string) {
	if i := strings.LastIndex(ref, "."); i >= 0 {
		return ref[:i], ref[i+1:]
	}
	return defaultBind, ref
}

func colAlias(ref string) string {
	if i := strings.LastIndex(ref, "."); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

func fail(id, mode string, start time.Time, err *protocol.ProtocolError) *protocol.QueryResponse {
	return &protocol.QueryResponse{
		ProtocolVersion: protocol.ProtocolVersion,
		QueryID:         id,
		Status:          protocol.StatusFailed,
		Error:           err,
		Meta: &protocol.QueryMeta{
			ElapsedMs: time.Since(start).Milliseconds(),
			Mode:      mode,
		},
	}
}

func asProto(err error) *protocol.ProtocolError {
	if pe, ok := err.(*protocol.ProtocolError); ok {
		return pe
	}
	return protocol.NewError(protocol.ErrInternal, err.Error(), nil)
}
