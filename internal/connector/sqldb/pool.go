package sqldb

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"

	"qLLM/internal/protocol"
)

const (
	defaultMaxOpenConns = 5
	sqliteMaxOpenConns  = 1
	maxOpenConnsCap     = 100
)

// poolMaxOpen resolves database/sql MaxOpenConns for a source.
// Precedence: maxOpenConnsEnv (non-empty int) → maxOpenConns literal → default 5
// (sqlite effective max is always 1).
func poolMaxOpen(src protocol.Source) (int, *protocol.ProtocolError) {
	n, err := resolveMaxOpenConns(src.Connection)
	if err != nil {
		return 0, protocol.NewError(protocol.ErrConfigError, err.Error(), map[string]any{"source": src.ID})
	}
	if protocol.WireFamily(src.Type) == protocol.SourceSQLite {
		return sqliteMaxOpenConns, nil
	}
	return n, nil
}

func resolveMaxOpenConns(conn map[string]any) (int, error) {
	if conn == nil {
		return defaultMaxOpenConns, nil
	}
	if _, ok := conn["maxOpenConnsEnv"]; ok {
		name, ok := conn["maxOpenConnsEnv"].(string)
		if !ok || name == "" {
			return 0, fmt.Errorf("connection.maxOpenConnsEnv must be a non-empty string")
		}
		raw := os.Getenv(name)
		if raw == "" {
			return 0, fmt.Errorf("environment variable %s is empty", name)
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			return 0, fmt.Errorf("environment variable %s must be an integer", name)
		}
		return validateMaxOpenConns(n)
	}
	if v, ok := conn["maxOpenConns"]; ok {
		n, err := asPositiveInt(v)
		if err != nil {
			return 0, fmt.Errorf("connection.maxOpenConns: %w", err)
		}
		return validateMaxOpenConns(n)
	}
	return defaultMaxOpenConns, nil
}

func validateMaxOpenConns(n int) (int, error) {
	if n < 1 || n > maxOpenConnsCap {
		return 0, fmt.Errorf("maxOpenConns must be between 1 and %d (got %d)", maxOpenConnsCap, n)
	}
	return n, nil
}

func asPositiveInt(v any) (int, error) {
	switch n := v.(type) {
	case int:
		return n, nil
	case int64:
		return int(n), nil
	case float64:
		if n != float64(int(n)) {
			return 0, fmt.Errorf("must be an integer")
		}
		return int(n), nil
	default:
		return 0, fmt.Errorf("must be an integer")
	}
}

// applyMaxOpenConns sets the pool cap on db from the source connection config.
func applyMaxOpenConns(db *sql.DB, src protocol.Source) *protocol.ProtocolError {
	n, err := poolMaxOpen(src)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(n)
	return nil
}
