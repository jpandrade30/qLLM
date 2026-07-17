package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"qLLM/internal/config"
	"qLLM/internal/connector/def"
	"qLLM/internal/connector/sqlbuild"
	"qLLM/internal/protocol"
	"qLLM/internal/result"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type SQLConnector struct {
	id      string
	srcType protocol.SourceType
	db      *sql.DB
	dialect sqlbuild.Dialect
	caps    def.Caps
}

func OpenPostgres(src protocol.Source) (*SQLConnector, error) {
	host, err := config.EnvString(src.Connection, "hostEnv")
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	user, err := config.EnvString(src.Connection, "userEnv")
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	pass, err := config.EnvString(src.Connection, "passwordEnv")
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	dbName := config.ConnString(src.Connection, "database", "")
	port := config.ConnInt(src.Connection, "port", 5432)
	ssl := config.ConnString(src.Connection, "sslMode", "disable")
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, pass, dbName, ssl)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), nil)
	}
	db.SetMaxOpenConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	return &SQLConnector{
		id: src.ID, srcType: protocol.SourcePostgres, db: db,
		dialect: sqlbuild.Postgres,
		caps: def.Caps{
			Filter: true, Project: true, Agg: true, GroupBy: true,
			JoinSameSource: true, OrderBy: true, Limit: true,
		},
	}, nil
}

func OpenMySQL(src protocol.Source) (*SQLConnector, error) {
	host, err := config.EnvString(src.Connection, "hostEnv")
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	user, err := config.EnvString(src.Connection, "userEnv")
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	pass, err := config.EnvString(src.Connection, "passwordEnv")
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	dbName := config.ConnString(src.Connection, "database", "")
	port := config.ConnInt(src.Connection, "port", 3306)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=false",
		user, pass, host, port, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), nil)
	}
	db.SetMaxOpenConns(5)
	return &SQLConnector{
		id: src.ID, srcType: protocol.SourceMySQL, db: db,
		dialect: sqlbuild.MySQL,
		caps: def.Caps{
			Filter: true, Project: true, Agg: true, GroupBy: true,
			JoinSameSource: true, OrderBy: true, Limit: true,
		},
	}, nil
}

func (c *SQLConnector) ID() string                       { return c.id }
func (c *SQLConnector) Type() protocol.SourceType        { return c.srcType }
func (c *SQLConnector) Capabilities() def.Caps     { return c.caps }
func (c *SQLConnector) Close() error                     { return c.db.Close() }

func (c *SQLConnector) Query(ctx context.Context, step def.PushdownStep) (*protocol.TabularResult, error) {
	built, err := sqlbuild.Build(c.dialect, step)
	if err != nil {
		return nil, err
	}
	rows, err := c.db.QueryContext(ctx, built.SQL, built.Args...)
	if err != nil {
		if ctx.Err() != nil {
			return nil, protocol.NewError(protocol.ErrTimeout,
				fmt.Sprintf("source %s exceeded timeout", c.id),
				map[string]any{"source": c.id})
		}
		return nil, protocol.NewError(protocol.ErrSourceError, err.Error(),
			map[string]any{"source": c.id})
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), nil)
	}
	columns := make([]protocol.Column, len(colNames))
	for i, n := range colNames {
		columns[i] = protocol.Column{Name: n, Type: inferOutType(step, n)}
	}

	out := [][]any{}
	for rows.Next() {
		raw := make([]any, len(colNames))
		ptrs := make([]any, len(colNames))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), nil)
		}
		row := make([]any, len(raw))
		for i, v := range raw {
			row[i] = normalizeSQLValue(v)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), nil)
	}
	truncated := step.Limit > 0 && len(out) >= step.Limit
	return result.New(columns, out, truncated), nil
}

func inferOutType(step def.PushdownStep, name string) protocol.LogicalType {
	for _, s := range step.Select {
		as := s.As
		if as == "" {
			as = s.Field
		}
		if as == name {
			if s.Agg != "" {
				return protocol.TypeNumber
			}
			return def.FieldType(step.Entity, s.Field)
		}
	}
	return protocol.TypeString
}

func normalizeSQLValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(t)
	case time.Time:
		return t.UTC().Format(time.RFC3339)
	default:
		return t
	}
}
