package executor

import (
	"context"
	"strings"
	"time"

	"qLLM/internal/access"
	"qLLM/internal/connector/def"
	"qLLM/internal/duckdblocal"
	"qLLM/internal/protocol"
	"qLLM/internal/sqlparse"
	"qLLM/internal/validate"

	"github.com/google/uuid"
)

func (e *Executor) ExecuteSQL(ctx context.Context, req *protocol.SQLRequest) *protocol.QueryResponse {
	start := time.Now()
	queryID := uuid.NewString()
	mode := "sync"
	app, aerr := e.resolveApp(ctx)
	if aerr != nil {
		return fail(queryID, mode, start, aerr, app)
	}
	if err := validate.SQLRequest(req); err != nil {
		return fail(queryID, mode, start, err, app)
	}
	dialect, verr := protocol.ResolveSQLVersion(req.Version)
	if verr != nil {
		return fail(queryID, mode, start, verr, app)
	}
	parsed, perr := sqlparse.ParseWithVersion(req.SQL, dialect)
	if perr != nil {
		return fail(queryID, mode, start, perr, app)
	}
	scans, perr := e.planSQLScans(parsed, app)
	if perr != nil {
		return fail(queryID, mode, start, perr, app)
	}
	limits := e.Idx.Preset.Limits
	sqlText := strings.TrimSpace(req.SQL)
	if parsed.Limit == nil {
		sqlText = sqlparse.InjectLimit(sqlText, limits.DefaultLimit)
	} else if *parsed.Limit > limits.MaxLimit {
		return fail(queryID, mode, start, protocol.NewError(protocol.ErrLimitExceeded,
			"LIMIT exceeds maxLimit", map[string]any{"limit": *parsed.Limit, "maxLimit": limits.MaxLimit}), app)
	} else if *parsed.Limit < 1 {
		return fail(queryID, mode, start, protocol.NewError(protocol.ErrInvalidSQL, "LIMIT must be >= 1", nil), app)
	}

	budget := time.Duration(limits.MaxSyncMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	srcBudget := time.Duration(limits.MaxSourceMs) * time.Millisecond

	eng, err := duckdblocal.Open()
	if err != nil {
		return fail(queryID, mode, start, protocol.NewError(protocol.ErrInternal, err.Error(), nil), app)
	}
	defer eng.Close()

	metaSteps := []protocol.PlanStepMeta{}
	truncated := false
	for _, sc := range scans {
		c, err := e.Reg.Get(sc.entity.Source)
		if err != nil {
			return fail(queryID, mode, start, asProto(err), app)
		}
		step := def.PushdownStep{
			SourceID: sc.entity.Source,
			Entity:   sc.entity,
			Binding:  sc.entity.Name,
			Select:   sc.selects,
			Limit:    limits.MaxLimit,
		}
		stepCtx, cancel := context.WithTimeout(ctx, srcBudget)
		t0 := time.Now()
		tab, qerr := c.Query(stepCtx, step)
		elapsed := time.Since(t0).Milliseconds()
		cancel()
		metaSteps = append(metaSteps, protocol.PlanStepMeta{
			Source: sc.entity.Source, Pushdown: false, ElapsedMs: elapsed,
		})
		if qerr != nil {
			return fail(queryID, mode, start, asProto(qerr), app)
		}
		if tab != nil && tab.Truncated {
			truncated = true
		}
		if err := eng.Materialize(ctx, sc.entity.Name, tab); err != nil {
			return fail(queryID, mode, start, protocol.NewError(protocol.ErrInternal, err.Error(), nil), app)
		}
	}

	tab, err := eng.ExecSQL(ctx, sqlText)
	if err != nil {
		if ctx.Err() != nil {
			return fail(queryID, mode, start, protocol.NewError(protocol.ErrTimeout, "query budget exceeded", nil), app)
		}
		if pe, ok := err.(*protocol.ProtocolError); ok {
			return fail(queryID, mode, start, pe, app)
		}
		return fail(queryID, mode, start, protocol.NewError(protocol.ErrInvalidSQL, err.Error(), nil), app)
	}
	if tab != nil {
		tab.Truncated = tab.Truncated || truncated
	}
	return &protocol.QueryResponse{
		ProtocolVersion: protocol.ProtocolVersion,
		QueryID:         queryID,
		Status:          protocol.StatusSucceeded,
		Result:          tab,
		Meta: &protocol.QueryMeta{
			ElapsedMs: time.Since(start).Milliseconds(),
			Mode:      mode,
			App:       appName(app),
			Plan: &protocol.PlanMeta{
				UsedDuckDB: true,
				Steps:      metaSteps,
			},
		},
	}
}

type sqlScan struct {
	entity  *protocol.Entity
	selects []def.SelectItem
}

func (e *Executor) planSQLScans(parsed *sqlparse.Result, app *access.App) ([]sqlScan, *protocol.ProtocolError) {
	if len(parsed.Tables) == 0 {
		return nil, protocol.NewError(protocol.ErrInvalidSQL, "SELECT requires FROM with catalog tables", nil)
	}
	alias := map[string]*protocol.Entity{}
	order := []*protocol.Entity{}
	seen := map[string]struct{}{}
	for _, t := range parsed.Tables {
		ent, err := e.Idx.ResolveEntity(t.Name)
		if err != nil {
			return nil, err
		}
		if app != nil && !app.Allows(ent.Name) {
			return nil, protocol.NewError(protocol.ErrForbidden, "entity not allowed for this app: "+ent.Name, map[string]any{"entity": ent.Name})
		}
		alias[strings.ToLower(t.Name)] = ent
		alias[strings.ToLower(ent.Name)] = ent
		if t.Alias != "" {
			alias[strings.ToLower(t.Alias)] = ent
		}
		if _, ok := seen[ent.Name]; !ok {
			seen[ent.Name] = struct{}{}
			order = append(order, ent)
		}
	}

	need := map[string]map[string]struct{}{}
	allFields := func(ent *protocol.Entity) {
		m := need[ent.Name]
		if m == nil {
			m = map[string]struct{}{}
			need[ent.Name] = m
		}
		for _, f := range ent.Fields {
			m[f.Name] = struct{}{}
		}
	}

	if parsed.Star {
		for _, ent := range order {
			allFields(ent)
		}
	}
	for _, c := range parsed.Columns {
		if c.Star {
			if c.Qual == "" {
				for _, ent := range order {
					allFields(ent)
				}
				continue
			}
			ent := alias[strings.ToLower(c.Qual)]
			if ent == nil {
				return nil, protocol.NewError(protocol.ErrUnknownEntity, "unknown table alias: "+c.Qual, map[string]any{"alias": c.Qual})
			}
			allFields(ent)
			continue
		}
		if c.Name == "" {
			continue
		}
		if c.Qual != "" {
			ent := alias[strings.ToLower(c.Qual)]
			if ent == nil {
				return nil, protocol.NewError(protocol.ErrUnknownEntity, "unknown table alias: "+c.Qual, map[string]any{"alias": c.Qual})
			}
			if _, ok := e.Idx.Field(ent, c.Name); !ok {
				return nil, protocol.NewError(protocol.ErrUnknownField, "unknown field: "+c.Name, map[string]any{"field": c.Name, "entity": ent.Name})
			}
			m := need[ent.Name]
			if m == nil {
				m = map[string]struct{}{}
				need[ent.Name] = m
			}
			m[c.Name] = struct{}{}
			continue
		}
		var hits []*protocol.Entity
		for _, ent := range order {
			if _, ok := e.Idx.Field(ent, c.Name); ok {
				hits = append(hits, ent)
			}
		}
		if len(hits) == 0 {
			// Bare name not on any cited entity: output alias / expression label (e.g. AS rnk).
			continue
		}
		if len(hits) > 1 {
			return nil, protocol.NewError(protocol.ErrAmbiguousField, "ambiguous field: "+c.Name, map[string]any{"field": c.Name})
		}
		ent := hits[0]
		m := need[ent.Name]
		if m == nil {
			m = map[string]struct{}{}
			need[ent.Name] = m
		}
		m[c.Name] = struct{}{}
	}

	out := make([]sqlScan, 0, len(order))
	for _, ent := range order {
		cols := need[ent.Name]
		if len(cols) == 0 {
			allFields(ent)
			cols = need[ent.Name]
		}
		sc := sqlScan{entity: ent}
		for _, f := range ent.Fields {
			if _, ok := cols[f.Name]; ok {
				sc.selects = append(sc.selects, def.SelectItem{Field: f.Name, As: f.Name})
			}
		}
		out = append(out, sc)
	}
	return out, nil
}
