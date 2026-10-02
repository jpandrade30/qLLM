package cataloggen

import (
	"context"
	"database/sql"
	"fmt"

	"qLLM/internal/connector/sqldb"
	"qLLM/internal/protocol"
)

const pgColumnsSQL = `
SELECT c.table_schema, c.table_name, c.column_name, c.data_type,
       CASE WHEN pk.column_name IS NULL THEN 0 ELSE 1 END AS is_pk
FROM information_schema.columns c
LEFT JOIN (
  SELECT kcu.table_schema, kcu.table_name, kcu.column_name
  FROM information_schema.table_constraints tc
  JOIN information_schema.key_column_usage kcu
    ON tc.constraint_name = kcu.constraint_name
   AND tc.table_schema = kcu.table_schema
   AND tc.table_name = kcu.table_name
  WHERE tc.constraint_type = 'PRIMARY KEY'
) pk
  ON pk.table_schema = c.table_schema
 AND pk.table_name = c.table_name
 AND pk.column_name = c.column_name
WHERE c.table_schema NOT IN ('pg_catalog', 'information_schema')
ORDER BY c.table_schema, c.table_name, c.ordinal_position
`

const mysqlColumnsSQL = `
SELECT c.table_schema, c.table_name, c.column_name, c.data_type,
       CASE WHEN pk.column_name IS NULL THEN 0 ELSE 1 END AS is_pk
FROM information_schema.columns c
LEFT JOIN (
  SELECT kcu.table_schema, kcu.table_name, kcu.column_name
  FROM information_schema.table_constraints tc
  JOIN information_schema.key_column_usage kcu
    ON tc.constraint_name = kcu.constraint_name
   AND tc.table_schema = kcu.table_schema
   AND tc.table_name = kcu.table_name
  WHERE tc.constraint_type = 'PRIMARY KEY'
) pk
  ON pk.table_schema = c.table_schema
 AND pk.table_name = c.table_name
 AND pk.column_name = c.column_name
WHERE c.table_schema = DATABASE()
ORDER BY c.table_schema, c.table_name, c.ordinal_position
`

// IntrospectSQL opens the named postgres/mysql source from the preset and reads information_schema.
func IntrospectSQL(ctx context.Context, preset *protocol.Preset, sourceID string, maxSourceMs int) ([]protocol.Entity, error) {
	var src *protocol.Source
	for i := range preset.Sources {
		if preset.Sources[i].ID == sourceID {
			src = &preset.Sources[i]
			break
		}
	}
	if src == nil {
		return nil, protocol.NewError(protocol.ErrConfigError, "unknown source: "+sourceID, nil)
	}
	var conn *sqldb.SQLConnector
	var err error
	var q string
	switch protocol.WireFamily(src.Type) {
	case protocol.SourcePostgres:
		conn, err = sqldb.OpenPostgres(*src, maxSourceMs)
		q = pgColumnsSQL
	case protocol.SourceMySQL:
		conn, err = sqldb.OpenMySQL(*src, maxSourceMs)
		q = mysqlColumnsSQL
	default:
		return nil, protocol.NewError(protocol.ErrUnsupported,
			"catalog introspect supports postgres/mysql and their wire aliases only", map[string]any{"source": sourceID, "type": string(src.Type)})
	}
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	rows, err := scanColumns(ctx, conn.Stdlib(), q)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError,
			fmt.Sprintf("introspect %s failed: %v", sourceID, err), map[string]any{"source": sourceID})
	}
	return EntitiesFromColumns(sourceID, rows), nil
}

// scanColumns implements runtime behavior for this package.
func scanColumns(ctx context.Context, db *sql.DB, query string) ([]ColumnRow, error) {
	rs, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []ColumnRow
	for rs.Next() {
		var r ColumnRow
		var pk int
		if err := rs.Scan(&r.Schema, &r.Table, &r.Column, &r.DataType, &pk); err != nil {
			return nil, err
		}
		r.PK = pk != 0
		out = append(out, r)
	}
	return out, rs.Err()
}
