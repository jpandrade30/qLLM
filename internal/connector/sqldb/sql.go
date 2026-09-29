package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"qLLM/internal/config"
	"qLLM/internal/connector/def"
	"qLLM/internal/connector/sqlbuild"
	"qLLM/internal/protocol"
	"qLLM/internal/result"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

type SQLConnector struct {
	id      string
	srcType protocol.SourceType
	db      *sql.DB
	dialect sqlbuild.Dialect
	caps    def.Caps
}

func statementTimeoutMs(src protocol.Source, maxSourceMs int) int {
	ms := maxSourceMs
	if src.Options != nil {
		if v, ok := asInt(src.Options["statementTimeoutMs"]); ok && v > 0 {
			if ms <= 0 || v < ms {
				ms = v
			}
		}
		if v, ok := asInt(src.Options["timeoutMs"]); ok && v > 0 {
			if ms <= 0 || v < ms {
				ms = v
			}
		}
	}
	return ms
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func OpenPostgres(src protocol.Source, maxSourceMs int) (*SQLConnector, error) {
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
	ssl := config.ConnString(src.Connection, "sslMode", "require")

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, pass),
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		Path:   "/" + dbName,
	}
	q := u.Query()
	q.Set("sslmode", ssl)
	if to := statementTimeoutMs(src, maxSourceMs); to > 0 {
		q.Set("options", fmt.Sprintf("-c statement_timeout=%d", to))
	}
	u.RawQuery = q.Encode()

	cfg, err := pgx.ParseConfig(u.String())
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, "postgres config invalid", map[string]any{"source": src.ID})
	}

	db := stdlib.OpenDB(*cfg)
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

func OpenMySQL(src protocol.Source, maxSourceMs int) (*SQLConnector, error) {
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
	tlsName := config.ConnString(src.Connection, "tls", "")

	mc := mysql.NewConfig()
	mc.User = user
	mc.Passwd = pass
	mc.Net = "tcp"
	mc.Addr = net.JoinHostPort(host, strconv.Itoa(port))
	mc.DBName = dbName
	mc.ParseTime = true
	mc.AllowNativePasswords = true
	// MultiStatements stays false (driver default). Do not put it in Params:
	// those keys are SET as session variables and MySQL errors 1193.
	// Local/dev MySQL images have no TLS. Driver v1.10 defaults to TLS and fails the query.
	if tlsName != "" {
		mc.TLSConfig = tlsName
	} else {
		mc.TLSConfig = "false"
	}

	connector, err := mysql.NewConnector(mc)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, "mysql connector failed", map[string]any{"source": src.ID})
	}
	db := sql.OpenDB(connector)
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

func (c *SQLConnector) ID() string                { return c.id }
func (c *SQLConnector) Type() protocol.SourceType { return c.srcType }
func (c *SQLConnector) Capabilities() def.Caps    { return c.caps }
func (c *SQLConnector) Close() error              { return c.db.Close() }

// Stdlib exposes the pool for catalog introspect (information_schema). Not used on the query path.
func (c *SQLConnector) Stdlib() *sql.DB { return c.db }

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
		return nil, protocol.NewError(protocol.ErrSourceError,
			fmt.Sprintf("source %s query failed", c.id),
			map[string]any{"source": c.id, "cause": err.Error()})
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError,
			fmt.Sprintf("source %s columns failed", c.id), map[string]any{"source": c.id})
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
			return nil, protocol.NewError(protocol.ErrSourceError,
				fmt.Sprintf("source %s scan failed", c.id), map[string]any{"source": c.id})
		}
		row := make([]any, len(raw))
		for i, v := range raw {
			row[i] = normalizeSQLValue(v)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError,
			fmt.Sprintf("source %s rows failed", c.id), map[string]any{"source": c.id})
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
