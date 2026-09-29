//go:build !duckdb

package duckdblocal

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"qLLM/internal/protocol"
	"qLLM/internal/result"
)

type pureEngine struct {
	tables map[string]*protocol.TabularResult
}

// Open returns the pure-Go local engine (default build).
func Open() (Engine, error) {
	return &pureEngine{tables: map[string]*protocol.TabularResult{}}, nil
}

func (e *pureEngine) Close() error { return nil }

func (e *pureEngine) Materialize(ctx context.Context, table string, tab *protocol.TabularResult) error {
	_ = ctx
	cp := *tab
	cp.Rows = append([][]any{}, tab.Rows...)
	e.tables[table] = &cp
	return nil
}

func (e *pureEngine) Execute(ctx context.Context, spec QuerySpec) (*protocol.TabularResult, error) {
	_ = ctx
	return ExecutePure(e.tables, spec)
}

func (e *pureEngine) ExecSQL(ctx context.Context, sqlStr string) (*protocol.TabularResult, error) {
	_ = ctx
	_ = sqlStr
	return nil, protocol.NewError(protocol.ErrUnsupported,
		"SQL dialect requires a DuckDB build (go build -tags duckdb)", nil)
}

// QuerySQL kept for simple tests / single-table SELECT *.
func (e *pureEngine) QuerySQL(ctx context.Context, sqlStr string) (*protocol.TabularResult, error) {
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
