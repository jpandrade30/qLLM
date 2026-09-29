package cassandra

import (
	"context"
	"fmt"
	"strings"

	"qLLM/internal/config"
	"qLLM/internal/connector/def"
	"qLLM/internal/connector/keycond"
	"qLLM/internal/protocol"
	"qLLM/internal/result"

	"github.com/gocql/gocql"
)

type Connector struct {
	id      string
	session *gocql.Session
	ks      string
	caps    def.Caps
}

func Open(src protocol.Source) (*Connector, error) {
	ks := config.ConnString(src.Connection, "keyspace", "")
	if ks == "" {
		return nil, protocol.NewError(protocol.ErrConfigError, "cassandra connection.keyspace is required", map[string]any{"source": src.ID})
	}
	var hosts []string
	if _, ok := src.Connection["hostsEnv"]; ok {
		h, err := config.EnvString(src.Connection, "hostsEnv")
		if err != nil {
			return nil, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
		}
		for _, p := range strings.Split(h, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				hosts = append(hosts, p)
			}
		}
	} else {
		h, err := config.EnvString(src.Connection, "hostEnv")
		if err != nil {
			return nil, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
		}
		hosts = []string{h}
	}
	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = ks
	cluster.Port = config.ConnInt(src.Connection, "port", 9042)
	user := config.OptionalEnvString(src.Connection, "userEnv")
	pass := config.OptionalEnvString(src.Connection, "passwordEnv")
	if user != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{Username: user, Password: pass}
	}
	sess, err := cluster.CreateSession()
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError, "cassandra connect failed: "+err.Error(), map[string]any{"source": src.ID})
	}
	return &Connector{
		id: src.ID, session: sess, ks: ks,
		caps: def.Caps{Filter: true, Project: true, Limit: true},
	}, nil
}

func (c *Connector) ID() string                { return c.id }
func (c *Connector) Type() protocol.SourceType { return protocol.SourceCassandra }
func (c *Connector) Capabilities() def.Caps    { return c.caps }
func (c *Connector) Close() error {
	c.session.Close()
	return nil
}

func (c *Connector) Query(ctx context.Context, step def.PushdownStep) (*protocol.TabularResult, error) {
	for _, s := range step.Select {
		if s.Agg != "" {
			return nil, protocol.NewError(protocol.ErrUnsupported, "cassandra does not push down aggregations", map[string]any{"source": c.id})
		}
	}
	table := step.Entity.Binding.Table
	if table == "" {
		return nil, protocol.NewError(protocol.ErrConfigError, "cassandra entity needs binding.table", nil)
	}
	pks := step.Entity.Binding.AccessPath.PartitionKeys()
	eqs, perr := keycond.RequireEq(step.Where, pks)
	if perr != nil {
		return nil, perr
	}
	sel := step.Select
	if len(sel) == 0 {
		for _, f := range step.Entity.Fields {
			sel = append(sel, def.SelectItem{Field: f.Name, As: f.Name})
		}
	}
	cols := make([]string, 0, len(sel))
	outCols := make([]protocol.Column, 0, len(sel))
	for _, s := range sel {
		as := s.As
		if as == "" {
			as = s.Field
		}
		cols = append(cols, qident(def.PhysicalName(step.Entity, s.Field)))
		outCols = append(outCols, protocol.Column{Name: as, Type: def.FieldType(step.Entity, s.Field)})
	}
	var where []string
	var args []any
	for _, k := range pks {
		where = append(where, qident(def.PhysicalName(step.Entity, k))+" = ?")
		args = append(args, eqs[k])
	}
	cql := fmt.Sprintf("SELECT %s FROM %s.%s WHERE %s",
		strings.Join(cols, ", "), qident(c.ks), qident(table), strings.Join(where, " AND "))
	if step.Limit > 0 {
		cql += fmt.Sprintf(" LIMIT %d", step.Limit)
	}
	iter := c.session.Query(cql, args...).WithContext(ctx).Iter()
	var rows [][]any
	for {
		raw := make(map[string]any)
		if !iter.MapScan(raw) {
			break
		}
		row := make([]any, len(sel))
		for i, s := range sel {
			row[i] = raw[def.PhysicalName(step.Entity, s.Field)]
		}
		rows = append(rows, row)
	}
	if err := iter.Close(); err != nil {
		if ctx.Err() != nil {
			return nil, protocol.NewError(protocol.ErrTimeout, fmt.Sprintf("source %s exceeded timeout", c.id), map[string]any{"source": c.id})
		}
		return nil, protocol.NewError(protocol.ErrSourceError, fmt.Sprintf("source %s query failed", c.id), map[string]any{"source": c.id, "cause": err.Error()})
	}
	truncated := step.Limit > 0 && len(rows) >= step.Limit
	return result.New(outCols, rows, truncated), nil
}

func qident(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
