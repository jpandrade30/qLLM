package executor

import (
	"context"
	"testing"

	"qLLM/internal/access"
	"qLLM/internal/catalogidx"
	"qLLM/internal/connector"
	"qLLM/internal/connector/def"
	"qLLM/internal/protocol"
)

func TestSQLScopeWhere(t *testing.T) {
	ent := &protocol.Entity{
		Name:  "orders",
		Scope: &protocol.EntityScope{Field: "user_id"},
	}
	app := &access.App{
		ScopeField:  "user_id",
		ScopeValues: map[string]string{"user_id": "42"},
	}
	w, err := sqlScopeWhere(ent, app)
	if err != nil {
		t.Fatal(err)
	}
	if w["op"] != "eq" || w["field"] != "user_id" || w["value"] != "42" {
		t.Fatalf("%#v", w)
	}
	if w, err = sqlScopeWhere(ent, &access.App{Name: "admin"}); err != nil || w != nil {
		t.Fatalf("admin %#v %v", w, err)
	}
	if _, err = sqlScopeWhere(ent, &access.App{ScopeField: "user_id"}); err == nil || err.Code != protocol.ErrForbiddenScope {
		t.Fatalf("missing %#v", err)
	}
}

type recConn struct {
	id   string
	last def.PushdownStep
}

func (c *recConn) ID() string                { return c.id }
func (c *recConn) Type() protocol.SourceType { return protocol.SourcePostgres }
func (c *recConn) Capabilities() def.Caps {
	return def.Caps{Filter: true, Project: true, Limit: true}
}
func (c *recConn) Close() error { return nil }
func (c *recConn) Query(_ context.Context, step def.PushdownStep) (*protocol.TabularResult, error) {
	c.last = step
	return &protocol.TabularResult{
		Columns:  []protocol.Column{{Name: "user_id", Type: protocol.TypeString}, {Name: "id", Type: protocol.TypeString}},
		Rows:     [][]any{{"42", "o1"}},
		RowCount: 1,
	}, nil
}

func sqlScopeBundle(t *testing.T) *catalogidx.Index {
	t.Helper()
	preset := &protocol.Preset{
		ProtocolVersion: "0.2.0",
		Project:         "t",
		Limits: protocol.Limits{
			MaxSyncMs: 15000, MaxSourceMs: 12000, DefaultLimit: 100, MaxLimit: 1000, ReadOnly: true,
		},
		Sources: []protocol.Source{{ID: "shop", Type: protocol.SourcePostgres}},
	}
	catalog := &protocol.Catalog{
		ProtocolVersion: "0.2.0",
		Project:         "t",
		Entities: []protocol.Entity{{
			Name: "orders", Source: "shop",
			Scope: &protocol.EntityScope{Field: "user_id"},
			Fields: []protocol.Field{
				{Name: "user_id", Type: protocol.TypeString, Physical: "user_id"},
				{Name: "id", Type: protocol.TypeString, Physical: "id"},
			},
		}},
	}
	idx, err := catalogidx.New(preset, catalog)
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestExecuteSQLMissingScope(t *testing.T) {
	idx := sqlScopeBundle(t)
	rec := &recConn{id: "shop"}
	ex := New(idx, connector.NewRegistry(map[string]def.Connector{"shop": rec}), nil)
	ex.ACL = &access.Registry{Apps: []*access.App{{
		Name: "mobile", ScopeField: "user_id",
		ScopeMode: access.ScopeModeReject,
		Tables:    map[string]struct{}{"orders": {}},
	}}}
	ex.StdioApp = "mobile"
	resp := ex.ExecuteSQL(context.Background(), &protocol.SQLRequest{
		SQL: "SELECT id FROM orders LIMIT 10",
	})
	if resp.Error == nil || resp.Error.Code != protocol.ErrForbiddenScope {
		t.Fatalf("got %#v", resp.Error)
	}
}
