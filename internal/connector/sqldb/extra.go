package sqldb

import (
	"crypto/tls"
	"database/sql"
	"net"
	"net/url"
	"strconv"

	"qLLM/internal/config"
	"qLLM/internal/connector/def"
	"qLLM/internal/connector/sqlbuild"
	"qLLM/internal/protocol"

	"github.com/ClickHouse/clickhouse-go/v2"
	_ "github.com/microsoft/go-mssqldb"
	_ "modernc.org/sqlite"
)

// OpenMSSQL opens a source or engine.
func OpenMSSQL(src protocol.Source, maxSourceMs int) (*SQLConnector, error) {
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
	port := config.ConnInt(src.Connection, "port", 1433)
	encrypt := config.ConnString(src.Connection, "encrypt", "true")

	q := url.Values{}
	q.Set("database", dbName)
	q.Set("encrypt", encrypt)
	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(user, pass),
		Host:     net.JoinHostPort(host, strconv.Itoa(port)),
		RawQuery: q.Encode(),
	}
	db, err := sql.Open("sqlserver", u.String())
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, "mssql open failed", map[string]any{"source": src.ID})
	}
	db.SetMaxOpenConns(5)
	_ = maxSourceMs
	return &SQLConnector{
		id: src.ID, srcType: protocol.SourceMSSQL, db: db,
		dialect: sqlbuild.MSSQL,
		caps:    sqlCaps(),
	}, nil
}

// OpenSQLite opens a source or engine.
func OpenSQLite(src protocol.Source, maxSourceMs int) (*SQLConnector, error) {
	path, err := config.EnvString(src.Connection, "pathEnv")
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, "sqlite open failed", map[string]any{"source": src.ID})
	}
	db.SetMaxOpenConns(1)
	_ = maxSourceMs
	return &SQLConnector{
		id: src.ID, srcType: protocol.SourceSQLite, db: db,
		dialect: sqlbuild.SQLite,
		caps:    sqlCaps(),
	}, nil
}

// OpenClickHouse opens a source or engine.
func OpenClickHouse(src protocol.Source, maxSourceMs int) (*SQLConnector, error) {
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
	port := config.ConnInt(src.Connection, "port", 9000)
	opts := &clickhouse.Options{
		Addr: []string{net.JoinHostPort(host, strconv.Itoa(port))},
		Auth: clickhouse.Auth{Database: dbName, Username: user, Password: pass},
	}
	if config.ConnBool(src.Connection, "secure", false) {
		opts.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	_ = maxSourceMs
	db := clickhouse.OpenDB(opts)
	db.SetMaxOpenConns(5)
	return &SQLConnector{
		id: src.ID, srcType: protocol.SourceClickHouse, db: db,
		dialect: sqlbuild.ClickHouse,
		caps:    sqlCaps(),
	}, nil
}

// sqlCaps implements runtime behavior for this package.
func sqlCaps() def.Caps {
	return def.Caps{
		Filter: true, Project: true, Agg: true, GroupBy: true,
		JoinSameSource: true, OrderBy: true, Limit: true,
	}
}
