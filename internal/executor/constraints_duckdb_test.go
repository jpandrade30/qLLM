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

func TestExecuteSQLConstraintInject(t *testing.T) {
	idx := sqlScopeBundle(t)
	rec := &recConn{id: "shop"}
	ex := New(idx, connector.NewRegistry(map[string]def.Connector{"shop": rec}), nil)
	ex.ACL = &access.Registry{Apps: []*access.App{{
		Name: "admin", Tables: map[string]struct{}{"orders": {}},
	}}}
	ex.StdioApp = "admin"
	resp := ex.ExecuteSQL(context.Background(), &protocol.SQLRequest{
		SQL:            "SELECT id FROM orders LIMIT 10",
		Constraints:    map[string]any{"user_id": "42"},
		ConstraintMode: protocol.ConstraintModeInject,
	})
	if resp.Status != protocol.StatusSucceeded {
		t.Fatalf("status %#v", resp.Error)
	}
	if rec.last.Where == nil || rec.last.Where["field"] != "user_id" || rec.last.Where["value"] != "42" {
		t.Fatalf("where %#v", rec.last.Where)
	}
}
