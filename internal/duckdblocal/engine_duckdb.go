//go:build duckdb

package duckdblocal

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"qLLM/internal/protocol"
	"qLLM/internal/result"

	_ "github.com/duckdb/duckdb-go/v2"
)

type duckEngine struct {
	db *sql.DB
}

// Open returns the embedded DuckDB local engine (-tags duckdb).
func Open() (Engine, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("SET enable_external_access=false"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("duckdb set enable_external_access: %w", err)
	}
	return &duckEngine{db: db}, nil
}

func (e *duckEngine) Close() error {
	if e.db == nil {
		return nil
	}
	return e.db.Close()
}

func (e *duckEngine) Materialize(ctx context.Context, table string, tab *protocol.TabularResult) error {
	name := quoteIdent(table)
	_, _ = e.db.ExecContext(ctx, "DROP TABLE IF EXISTS "+name)
	cols := make([]string, len(tab.Columns))
	for i, c := range tab.Columns {
		cols[i] = quoteIdent(c.Name) + " " + duckType(c.Type)
	}
	create := "CREATE TEMP TABLE " + name + " (" + strings.Join(cols, ", ") + ")"
	if _, err := e.db.ExecContext(ctx, create); err != nil {
		return fmt.Errorf("duckdb create %s: %w", table, err)
	}
	if len(tab.Columns) == 0 {
		return nil
	}
	placeholders := make([]string, len(tab.Columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	insert := "INSERT INTO " + name + " VALUES (" + strings.Join(placeholders, ", ") + ")"
		for _, row := range tab.Rows {
		args := make([]any, len(tab.Columns))
		for i, col := range tab.Columns {
			if i < len(row) {
				args[i] = coerceDuckCell(col.Type, row[i])
			}
		}
		if _, err := e.db.ExecContext(ctx, insert, args...); err != nil {
			return fmt.Errorf("duckdb insert %s: %w", table, err)
		}
	}
	return nil
}

func (e *duckEngine) Execute(ctx context.Context, spec QuerySpec) (*protocol.TabularResult, error) {
	sqlStr, args, err := BuildDuckSQL(spec)
	if err != nil {
		return nil, err
	}
	rows, err := e.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("duckdb query: %w", err)
	}
	defer rows.Close()
	return scanDuckRows(rows, spec)
}

func (e *duckEngine) ExecSQL(ctx context.Context, sqlStr string) (*protocol.TabularResult, error) {
	rows, err := e.db.QueryContext(ctx, sqlStr)
	if err != nil {
		return nil, fmt.Errorf("duckdb sql: %w", err)
	}
	defer rows.Close()
	return scanDuckRows(rows, QuerySpec{})
}

func scanDuckRows(rows *sql.Rows, spec QuerySpec) (*protocol.TabularResult, error) {
	colNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	columns := make([]protocol.Column, len(colNames))
	for i, n := range colNames {
		typ := protocol.TypeString
		for _, s := range spec.Select {
			as := s.As
			if as == "" {
				as = s.Col
			}
			if as == n {
				if s.Agg != "" {
					typ = protocol.TypeNumber
				}
				break
			}
		}
		columns[i] = protocol.Column{Name: n, Type: typ}
	}

	out := [][]any{}
	for rows.Next() {
		raw := make([]any, len(colNames))
		ptrs := make([]any, len(colNames))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make([]any, len(raw))
		for i, v := range raw {
			switch t := v.(type) {
			case []byte:
				row[i] = string(t)
			default:
				row[i] = t
			}
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result.New(columns, out, false), nil
}

func coerceDuckCell(t protocol.LogicalType, v any) any {
	if v == nil || t != protocol.TypeTimestamp {
		return v
	}
	switch x := v.(type) {
	case time.Time:
		return x.UTC().Format(time.RFC3339)
	case string:
		return x
	case int64:
		if x > 1_000_000_000_000 {
			return time.UnixMilli(x).UTC().Format(time.RFC3339)
		}
		return time.Unix(x, 0).UTC().Format(time.RFC3339)
	case int:
		return coerceDuckCell(t, int64(x))
	case float64:
		return coerceDuckCell(t, int64(x))
	default:
		return v
	}
}
