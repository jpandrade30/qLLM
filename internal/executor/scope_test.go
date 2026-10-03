package executor

import (
	"testing"

	"qLLM/internal/access"
	"qLLM/internal/connector/def"
	"qLLM/internal/planner"
	"qLLM/internal/protocol"
)

func scopedOrders() *protocol.Entity {
	return &protocol.Entity{
		Name:  "orders",
		Scope: &protocol.EntityScope{Field: "user_id"},
		Fields: []protocol.Field{
			{Name: "user_id", Type: protocol.TypeString},
			{Name: "id", Type: protocol.TypeString},
		},
	}
}

func scopedApp(mode, value string) *access.App {
	return &access.App{
		Name:        "mobile",
		ScopeField:  "user_id",
		ScopeValues: map[string]string{"user_id": value},
		ScopeMode:   mode,
		Tables:      map[string]struct{}{"orders": {}},
	}
}

func TestApplyScopeRejectSpoof(t *testing.T) {
	ent := scopedOrders()
	q := &protocol.QueryIR{
		From:  "orders",
		Where: map[string]any{"op": "eq", "field": "user_id", "value": "7"},
	}
	plan := &planner.Plan{Steps: []def.PushdownStep{{
		Entity: ent, Binding: "orders",
		Where: map[string]any{"op": "eq", "field": "user_id", "value": "7"},
	}}}
	err := applyScope(plan, q, scopedApp(access.ScopeModeReject, "42"))
	if err == nil || err.Code != protocol.ErrForbiddenScope {
		t.Fatalf("got %#v", err)
	}
}

func TestApplyScopeInjectSpoof(t *testing.T) {
	ent := scopedOrders()
	q := &protocol.QueryIR{
		From:  "orders",
		Where: map[string]any{"op": "eq", "field": "user_id", "value": "7"},
	}
	plan := &planner.Plan{Steps: []def.PushdownStep{{
		Entity: ent, Binding: "orders",
		Where: map[string]any{"op": "eq", "field": "user_id", "value": "7"},
	}}}
	if err := applyScope(plan, q, scopedApp(access.ScopeModeInject, "42")); err != nil {
		t.Fatal(err)
	}
	if op, _ := plan.Steps[0].Where["op"].(string); op != "and" {
		t.Fatalf("want and got %#v", plan.Steps[0].Where)
	}
}

func TestApplyScopeMissingValue(t *testing.T) {
	ent := scopedOrders()
	plan := &planner.Plan{Steps: []def.PushdownStep{{Entity: ent, Binding: "orders"}}}
	app := &access.App{Name: "mobile", ScopeField: "user_id", ScopeMode: access.ScopeModeReject}
	err := applyScope(plan, &protocol.QueryIR{From: "orders"}, app)
	if err == nil || err.Code != protocol.ErrForbiddenScope {
		t.Fatalf("got %#v", err)
	}
}

func TestApplyScopeJoinEachEntity(t *testing.T) {
	orders := scopedOrders()
	payments := &protocol.Entity{
		Name:   "payments",
		Scope:  &protocol.EntityScope{Field: "user_id"},
		Fields: []protocol.Field{{Name: "user_id", Type: protocol.TypeString}},
	}
	q := &protocol.QueryIR{
		From:  "orders",
		Joins: []protocol.Join{{From: "payments", As: "payments"}},
	}
	plan := &planner.Plan{Steps: []def.PushdownStep{
		{Entity: orders, Binding: "orders"},
		{Entity: payments, Binding: "payments"},
	}}
	if err := applyScope(plan, q, scopedApp(access.ScopeModeReject, "42")); err != nil {
		t.Fatal(err)
	}
	for _, st := range plan.Steps {
		if st.Where == nil {
			t.Fatal("missing step where")
		}
		if st.Where["field"] != "user_id" || st.Where["value"] != "42" {
			t.Fatalf("step %#v", st.Where)
		}
	}
}

func TestApplyScopeUnscopedAppUnchanged(t *testing.T) {
	ent := scopedOrders()
	where := map[string]any{"op": "eq", "field": "id", "value": "1"}
	plan := &planner.Plan{Steps: []def.PushdownStep{{
		Entity: ent, Binding: "orders", Where: where,
	}}}
	q := &protocol.QueryIR{From: "orders", Where: where}
	if err := applyScope(plan, q, &access.App{Name: "admin"}); err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Where["field"] != "id" {
		t.Fatalf("admin must not inject %#v", plan.Steps[0].Where)
	}
}

func TestApplyScopeMatchingEq(t *testing.T) {
	ent := scopedOrders()
	where := map[string]any{"op": "eq", "field": "user_id", "value": "42"}
	plan := &planner.Plan{Steps: []def.PushdownStep{{
		Entity: ent, Binding: "orders", Where: where,
	}}}
	q := &protocol.QueryIR{From: "orders", Where: where}
	if err := applyScope(plan, q, scopedApp(access.ScopeModeReject, "42")); err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Where["op"] != "eq" {
		t.Fatalf("should not duplicate %#v", plan.Steps[0].Where)
	}
}
