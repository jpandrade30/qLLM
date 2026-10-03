package executor

import (
	"context"
	"strings"
	"time"

	"qLLM/internal/access"
	"qLLM/internal/appctx"
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
	Idx        *catalogidx.Index
	Reg        *connector.Registry
	Store      *querystore.Store
	ACL        *access.Registry
	StdioApp   string
	StdioScope string
}

// New constructs a value.
func New(idx *catalogidx.Index, reg *connector.Registry, store *querystore.Store) *Executor {
	return &Executor{Idx: idx, Reg: reg, Store: store}
}

// Execute runs a query.
func (e *Executor) Execute(ctx context.Context, q *protocol.QueryIR) *protocol.QueryResponse {
	start := time.Now()
	queryID := uuid.NewString()
	mode := q.Mode
	if mode == "" {
		mode = "sync"
	}

	app, aerr := e.resolveApp(ctx)
	if aerr != nil {
		return fail(queryID, mode, start, aerr, app)
	}

	if err := validate.Query(e.Idx, q); err != nil {
		return fail(queryID, mode, start, err, app)
	}
	if app != nil {
		if err := validate.EnforceACL(e.Idx, q, app.TableSet()); err != nil {
			return fail(queryID, mode, start, err, app)
		}
	}

	budget := time.Duration(e.Idx.Preset.Limits.MaxSyncMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	plan, err := planner.Build(e.Idx, q)
	if err != nil {
		return fail(queryID, mode, start, err, app)
	}
	if serr := applyScope(plan, q, app); serr != nil {
		return fail(queryID, mode, start, serr, app)
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
			out := e.run(bg, queryID, mode, plan, q, start, app)
			e.Store.Update(out)
		}()
		return resp
	}

	return e.run(ctx, queryID, mode, plan, q, start, app)
}

// run implements runtime behavior for this package.
func (e *Executor) run(ctx context.Context, queryID, mode string, plan *planner.Plan, q *protocol.QueryIR, start time.Time, app *access.App) *protocol.QueryResponse {
	srcBudget := time.Duration(e.Idx.Preset.Limits.MaxSourceMs) * time.Millisecond
	metaSteps := []protocol.PlanStepMeta{}
	partials := map[string]*protocol.TabularResult{}

	for _, step := range plan.Steps {
		c, err := e.Reg.Get(step.SourceID)
		if err != nil {
			return fail(queryID, mode, start, asProto(err), app)
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
			return fail(queryID, mode, start, asProto(qerr), app)
		}
		partials[step.Binding] = tab
	}

	var final *protocol.TabularResult
	usedDuck := plan.UseDuckDB

	if plan.UseDuckDB {
		eng, err := duckdblocal.Open()
		if err != nil {
			return fail(queryID, mode, start, protocol.NewError(protocol.ErrInternal, err.Error(), nil), app)
		}
		defer eng.Close()

		for bind, tab := range partials {
			if err := eng.Materialize(ctx, bind, tab); err != nil {
				return fail(queryID, mode, start, protocol.NewError(protocol.ErrInternal, err.Error(), nil), app)
			}
		}

		spec, perr := buildLocalSpec(plan, q)
		if perr != nil {
			return fail(queryID, mode, start, perr, app)
		}
		tab, err := eng.Execute(ctx, spec)
		if err != nil {
			if ctx.Err() != nil {
				return fail(queryID, mode, start, protocol.NewError(protocol.ErrTimeout, "query budget exceeded", nil), app)
			}
			return fail(queryID, mode, start, asProto(err), app)
		}
		final = tab
	} else {
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
			App:       appName(app),
			Plan: &protocol.PlanMeta{
				UsedDuckDB: usedDuck,
				Steps:      metaSteps,
			},
		},
	}
}

// buildLocalSpec implements runtime behavior for this package.
func buildLocalSpec(plan *planner.Plan, q *protocol.QueryIR) (duckdblocal.QuerySpec, *protocol.ProtocolError) {
	root := plan.Steps[0].Binding
	spec := duckdblocal.QuerySpec{
		RootBind: root,
		Limit:    plan.Limit,
		Offset:   plan.Offset,
	}
	for _, j := range plan.Joins {
		js := duckdblocal.JoinSpec{
			Type:       j.Type,
			RightTable: j.RightBind,
		}
		for _, on := range j.On {
			lb, lc := splitRef(on.Left, root)
			rb, rc := splitRef(on.Right, j.RightBind)
			js.On = append(js.On, duckdblocal.JoinOn{
				LeftBind: lb, LeftCol: lc, RightBind: rb, RightCol: rc,
			})
		}
		spec.Joins = append(spec.Joins, js)
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
	where, perr := buildWhere(q.Where, root)
	if perr != nil {
		return spec, perr
	}
	spec.Where = where
	for _, o := range q.OrderBy {
		dir := o.Dir
		if dir == "" {
			dir = "asc"
		}
		spec.OrderBy = append(spec.OrderBy, duckdblocal.OrderSpec{As: colAlias(o.Field), Dir: dir})
	}
	return spec, nil
}

// buildWhere implements runtime behavior for this package.
func buildWhere(w map[string]any, root string) (*duckdblocal.WhereExpr, *protocol.ProtocolError) {
	if w == nil {
		return nil, nil
	}
	op, _ := w["op"].(string)
	if op == "" {
		return nil, protocol.NewError(protocol.ErrUnsupported, "where missing op", nil)
	}
	switch op {
	case "and", "or":
		args, _ := w["args"].([]any)
		out := &duckdblocal.WhereExpr{Op: op}
		for _, a := range args {
			m, _ := a.(map[string]any)
			child, err := buildWhere(m, root)
			if err != nil {
				return nil, err
			}
			if child != nil {
				out.Args = append(out.Args, *child)
			}
		}
		return out, nil
	case "not":
		args, _ := w["args"].([]any)
		if len(args) != 1 {
			return nil, protocol.NewError(protocol.ErrUnsupported, "not requires one arg", nil)
		}
		m, _ := args[0].(map[string]any)
		child, err := buildWhere(m, root)
		if err != nil {
			return nil, err
		}
		return &duckdblocal.WhereExpr{Op: "not", Args: []duckdblocal.WhereExpr{*child}}, nil
	default:
		field, _ := w["field"].(string)
		if field == "" && op != "is_null" && op != "not_null" {
			// is_null/not_null still need field
		}
		if field == "" {
			return nil, protocol.NewError(protocol.ErrUnsupported, "where compare missing field", map[string]any{"op": op})
		}
		b, c := splitRef(field, root)
		return &duckdblocal.WhereExpr{Op: op, Bind: b, Col: c, Value: w["value"]}, nil
	}
}

// splitRef implements runtime behavior for this package.
func splitRef(ref, defaultBind string) (bind, col string) {
	if i := strings.LastIndex(ref, "."); i >= 0 {
		return ref[:i], ref[i+1:]
	}
	return defaultBind, ref
}

// colAlias implements runtime behavior for this package.
func colAlias(ref string) string {
	if i := strings.LastIndex(ref, "."); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

// fail implements runtime behavior for this package.
func fail(id, mode string, start time.Time, err *protocol.ProtocolError, app *access.App) *protocol.QueryResponse {
	return &protocol.QueryResponse{
		ProtocolVersion: protocol.ProtocolVersion,
		QueryID:         id,
		Status:          protocol.StatusFailed,
		Error:           err,
		Meta: &protocol.QueryMeta{
			ElapsedMs: time.Since(start).Milliseconds(),
			Mode:      mode,
			App:       appName(app),
		},
	}
}

// appName implements runtime behavior for this package.
func appName(app *access.App) string {
	if app == nil {
		return ""
	}
	return app.Name
}

// resolveApp implements runtime behavior for this package.
func (e *Executor) resolveApp(ctx context.Context) (*access.App, *protocol.ProtocolError) {
	if e.ACL == nil {
		return nil, nil
	}
	if app := appctx.App(ctx); app != nil {
		return app, nil
	}
	if e.StdioApp != "" {
		app := e.ACL.LookupName(e.StdioApp)
		if app == nil {
			return nil, protocol.NewError(protocol.ErrForbidden, "unknown app: "+e.StdioApp, map[string]any{"app": e.StdioApp})
		}
		return app.WithStdioScope(e.StdioScope)
	}
	return nil, protocol.NewError(protocol.ErrUnauthorized, "app required: Authorization Bearer key or --app / QLLM_APP", nil)
}

// asProto implements runtime behavior for this package.
func asProto(err error) *protocol.ProtocolError {
	if pe, ok := err.(*protocol.ProtocolError); ok {
		return pe
	}
	return protocol.NewError(protocol.ErrInternal, err.Error(), nil)
}
