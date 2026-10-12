package executor

import (
	"context"
	"testing"

	"qLLM/internal/access"
	"qLLM/internal/connector"
	"qLLM/internal/connector/def"
	"qLLM/internal/protocol"
)

func TestValidateSQLConstraintsMismatch(t *testing.T) {
	err := validateSQLConstraints(
		`SELECT id FROM orders WHERE user_id = '7' LIMIT 10`,
		map[string]string{"user_id": "42"},
	)
	if err == nil || err.Code != protocol.ErrForbiddenScope {
		t.Fatalf("got %#v", err)
	}
}

func TestValidateSQLConstraintsMatch(t *testing.T) {
	if err := validateSQLConstraints(
		`SELECT id FROM orders WHERE user_id = '42' LIMIT 10`,
		map[string]string{"user_id": "42"},
	); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSQLConstraintsAbsentOK(t *testing.T) {
	if err := validateSQLConstraints(
		`SELECT id FROM orders LIMIT 10`,
		map[string]string{"user_id": "42"},
	); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSQLConstraintsORAmbiguous(t *testing.T) {
	err := validateSQLConstraints(
		`SELECT id FROM orders WHERE user_id = '42' OR user_id = '7'`,
		map[string]string{"user_id": "42"},
	)
	if err == nil || err.Code != protocol.ErrForbiddenScope {
		t.Fatalf("got %#v", err)
	}
}

func TestCheckConstraintsVsD21(t *testing.T) {
	app := &access.App{ScopeField: "user_id", ScopeValues: map[string]string{"user_id": "42"}}
	if err := checkConstraintsVsD21(app, map[string]string{"user_id": "42"}); err != nil {
		t.Fatal(err)
	}
	err := checkConstraintsVsD21(app, map[string]string{"user_id": "7"})
	if err == nil || err.Code != protocol.ErrForbiddenScope {
		t.Fatalf("got %#v", err)
	}
}

func TestExecuteSQLConstraintValidateMismatch(t *testing.T) {
	idx := sqlScopeBundle(t)
	rec := &recConn{id: "shop"}
	ex := New(idx, connector.NewRegistry(map[string]def.Connector{"shop": rec}), nil)
	ex.ACL = &access.Registry{Apps: []*access.App{{
		Name: "admin", Tables: map[string]struct{}{"orders": {}},
	}}}
	ex.StdioApp = "admin"
	resp := ex.ExecuteSQL(context.Background(), &protocol.SQLRequest{
		SQL:            "SELECT id FROM orders WHERE user_id = '7' LIMIT 10",
		Constraints:    map[string]any{"user_id": "42"},
		ConstraintMode: protocol.ConstraintModeValidate,
	})
	if resp.Error == nil || resp.Error.Code != protocol.ErrForbiddenScope {
		t.Fatalf("got %#v", resp.Error)
	}
}

func TestExecuteSQLConstraintUnknownField(t *testing.T) {
	idx := sqlScopeBundle(t)
	rec := &recConn{id: "shop"}
	ex := New(idx, connector.NewRegistry(map[string]def.Connector{"shop": rec}), nil)
	ex.ACL = &access.Registry{Apps: []*access.App{{
		Name: "admin", Tables: map[string]struct{}{"orders": {}},
	}}}
	ex.StdioApp = "admin"
	resp := ex.ExecuteSQL(context.Background(), &protocol.SQLRequest{
		SQL:         "SELECT id FROM orders LIMIT 10",
		Constraints: map[string]any{"nope": "x"},
	})
	if resp.Error == nil || resp.Error.Code != protocol.ErrInvalidSQL {
		t.Fatalf("got %#v", resp.Error)
	}
}

func TestExecuteSQLConstraintD21Conflict(t *testing.T) {
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
		SQL:         "SELECT id FROM orders LIMIT 10",
		Constraints: map[string]any{"user_id": "7"},
	})
	if resp.Error == nil || resp.Error.Code != protocol.ErrForbiddenScope {
		t.Fatalf("got %#v", resp.Error)
	}
}

func TestMergeConstraintWhereInject(t *testing.T) {
	ent := &protocol.Entity{
		Name:   "orders",
		Fields: []protocol.Field{{Name: "user_id", Type: protocol.TypeString}},
	}
	w := mergeConstraintWhere(nil, ent, map[string]string{"user_id": "42"}, protocol.ConstraintModeInject)
	if w["op"] != "eq" || w["field"] != "user_id" || w["value"] != "42" {
		t.Fatalf("%#v", w)
	}
	w2 := mergeConstraintWhere(nil, ent, map[string]string{"user_id": "42"}, protocol.ConstraintModeValidate)
	if w2 != nil {
		t.Fatalf("validate must not inject %#v", w2)
	}
}
