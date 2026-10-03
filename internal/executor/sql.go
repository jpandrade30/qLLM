package executor

import (
	"context"
	"fmt"
	"os"
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

const sqlLogMaxRunes = 4096

// logExecuteSQL implements runtime behavior for this package.
func logExecuteSQL(req *protocol.SQLRequest, resp *protocol.QueryResponse) {
	if resp == nil {
		return
	}
	_, _ = os.Stderr.WriteString(formatExecuteSQLLog(req, resp))
}

// formatExecuteSQLLog implements runtime behavior for this package.
func formatExecuteSQLLog(req *protocol.SQLRequest, resp *protocol.QueryResponse) string {
	sql := ""
	version := ""
	if req != nil {
		sql = req.SQL
		version = req.Version
	}
	if r := []rune(sql); len(r) > sqlLogMaxRunes {
		sql = string(r[:sqlLogMaxRunes]) + "…"
	}
	sql = strings.ReplaceAll(sql, "\r\n", "\n")
	sql = strings.TrimSpace(sql)

	var b strings.Builder
	b.WriteString("---- execute_sql ----\n")
	fmt.Fprintf(&b, "  status    %s\n", resp.Status)
	fmt.Fprintf(&b, "  queryId   %s\n", resp.QueryID)
	if resp.Meta != nil {
		fmt.Fprintf(&b, "  elapsed   %dms\n", resp.Meta.ElapsedMs)
		if resp.Meta.App != "" {
			fmt.Fprintf(&b, "  app       %s\n", resp.Meta.App)
		}
	}
	if version != "" {
		fmt.Fprintf(&b, "  dialect   %s\n", version)
	}
	if resp.Result != nil {
		fmt.Fprintf(&b, "  rows      %d\n", len(resp.Result.Rows))
	}
	if resp.Error != nil {
		fmt.Fprintf(&b, "  error     %s  %s\n", resp.Error.Code, resp.Error.Message)
	}
	b.WriteString("\n")
	if sql == "" {
		b.WriteString("  (empty sql)\n")
	} else {
		for _, line := range strings.Split(sql, "\n") {
			b.WriteString("  ")
			b.WriteString(strings.TrimRight(line, " \t"))
			b.WriteByte('\n')
		}
	}
	b.WriteString("--------------------\n")
	return b.String()
}

// ExecuteSQL runs a query.
func (e *Executor) ExecuteSQL(ctx context.Context, req *protocol.SQLRequest) (resp *protocol.QueryResponse) {
	defer func() { logExecuteSQL(req, resp) }()
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

	for _, sc := range scans {
		if _, serr := sqlScopeWhere(sc.entity, app); serr != nil {
			return fail(queryID, mode, start, serr, app)
		}
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
		where, serr := sqlScopeWhere(sc.entity, app)
		if serr != nil {
			return fail(queryID, mode, start, serr, app)
		}
		step := def.PushdownStep{
			SourceID: sc.entity.Source,
			Entity:   sc.entity,
			Binding:  sc.entity.Name,
			Select:   sc.selects,
			Where:    where,
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

func sqlScopeWhere(ent *protocol.Entity, app *access.App) (map[string]any, *protocol.ProtocolError) {
	if app == nil || !app.HasScope() || ent == nil || ent.Scope == nil || ent.Scope.Field == "" {
		return nil, nil
	}
	col := ent.Scope.FilterField()
	val, ok := app.ScopeValue(ent.Scope.Field)
	if !ok {
		return nil, protocol.NewError(protocol.ErrForbiddenScope,
			"this key has no scope value for "+ent.Scope.Field,
			map[string]any{"field": ent.Scope.Field, "entity": ent.Name})
	}
	return map[string]any{"op": "eq", "field": col, "value": val}, nil
}

type sqlScan struct {
	entity  *protocol.Entity
	selects []def.SelectItem
}

// planSQLScans implements runtime behavior for this package.
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

	needCol := func(ent *protocol.Entity, field string) {
		m := need[ent.Name]
		if m == nil {
			m = map[string]struct{}{}
			need[ent.Name] = m
		}
		m[field] = struct{}{}
	}
	cteEntities := func(qual string) []*protocol.Entity {
		var ents []*protocol.Entity
		for _, n := range parsed.CTEBind[strings.ToLower(qual)] {
			if ent := alias[strings.ToLower(n)]; ent != nil {
				ents = append(ents, ent)
			}
		}
		return ents
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
				ents := cteEntities(c.Qual)
				if len(ents) == 0 {
					return nil, protocol.NewError(protocol.ErrUnknownEntity, "unknown table alias: "+c.Qual, map[string]any{"alias": c.Qual})
				}
				for _, src := range ents {
					allFields(src)
				}
				continue
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
				ents := cteEntities(c.Qual)
				if len(ents) == 0 {
					return nil, protocol.NewError(protocol.ErrUnknownEntity, "unknown table alias: "+c.Qual, map[string]any{"alias": c.Qual})
				}
				matched := false
				for _, src := range ents {
					if _, ok := e.Idx.Field(src, c.Name); ok {
						needCol(src, c.Name)
						matched = true
					}
				}
				if !matched {
					// Computed CTE column (paid_total) or SELECT alias (posicao).
					continue
				}
				continue
			}
			if _, ok := e.Idx.Field(ent, c.Name); !ok {
				return nil, protocol.NewError(protocol.ErrUnknownField, "unknown field: "+c.Name, map[string]any{"field": c.Name, "entity": ent.Name})
			}
			needCol(ent, c.Name)
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
		// UNION (and similar) cites the same logical name on more than one entity.
		// Fetch the column from each; DuckDB still executes the original SQL.
		for _, ent := range hits {
			m := need[ent.Name]
			if m == nil {
				m = map[string]struct{}{}
				need[ent.Name] = m
			}
			m[c.Name] = struct{}{}
		}
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
