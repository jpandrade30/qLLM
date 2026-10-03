//go:build duckdb

package executor

import (
	"context"
	"testing"

	"qLLM/internal/access"
	"qLLM/internal/connector"
	"qLLM/internal/connector/def"
	"qLLM/internal/protocol"
)

func TestExecuteSQLAppliesScopeWhere(t *testing.T) {
	idx := sqlScopeBundle(t)
	rec := &recConn{id: "shop"}
	ex := New(idx, connector.NewRegistry(map[string]def.Connector{"shop": rec}), nil)
	ex.ACL = &access.Registry{Apps: []*access.App{{
		Name: "mobile", ScopeField: "user_id",
		ScopeValues: map[string]string{"user_id": "42"},
		ScopeMode:   access.ScopeModeReject,
		Tables:      map[string]struct{}{"orders": {}},
	}}}
	ex.StdioApp = "mobile"
	resp := ex.ExecuteSQL(context.Background(), &protocol.SQLRequest{
		SQL: "SELECT id FROM orders WHERE user_id = '7' LIMIT 10",
	})
	if resp.Status != protocol.StatusSucceeded {
		t.Fatalf("status %#v", resp.Error)
	}
	if rec.last.Where == nil || rec.last.Where["field"] != "user_id" || rec.last.Where["value"] != "42" {
		t.Fatalf("where %#v", rec.last.Where)
	}
	if resp.Result == nil || resp.Result.RowCount != 0 {
		t.Fatalf("spoof sql must see only scoped fetch, got %#v", resp.Result)
	}
}

func TestExecuteSQLAdminNoScopeWhere(t *testing.T) {
	idx := sqlScopeBundle(t)
	rec := &recConn{id: "shop"}
	ex := New(idx, connector.NewRegistry(map[string]def.Connector{"shop": rec}), nil)
	ex.ACL = &access.Registry{Apps: []*access.App{{
		Name: "admin", Tables: map[string]struct{}{"orders": {}},
	}}}
	ex.StdioApp = "admin"
	resp := ex.ExecuteSQL(context.Background(), &protocol.SQLRequest{
		SQL: "SELECT id FROM orders LIMIT 10",
	})
	if resp.Status != protocol.StatusSucceeded {
		t.Fatalf("status %#v", resp.Error)
	}
	if rec.last.Where != nil {
		t.Fatalf("admin must not inject %#v", rec.last.Where)
	}
}
