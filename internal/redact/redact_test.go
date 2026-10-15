package redact

import (
	"testing"

	"qLLM/internal/protocol"
	"qLLM/internal/sqlparse"
)

func TestFieldNamesScoped(t *testing.T) {
	cat := &protocol.Catalog{
		Entities: []protocol.Entity{
			{Name: "customers", Fields: []protocol.Field{
				{Name: "id", Redact: true},
				{Name: "name"},
			}},
			{Name: "orders", Fields: []protocol.Field{
				{Name: "id"},
				{Name: "customer_id", Redact: true},
			}},
		},
	}
	got := FieldNames(cat, []string{"customers"})
	if _, ok := got["id"]; !ok {
		t.Fatal("want customers.id")
	}
	if _, ok := got["customer_id"]; ok {
		t.Fatal("orders out of scope")
	}
}

func TestApplyByNameAndType(t *testing.T) {
	tab := &protocol.TabularResult{
		Columns: []protocol.Column{
			{Name: "id", Type: protocol.TypeNumber},
			{Name: "name", Type: protocol.TypeString},
		},
		Rows: [][]any{{42, "Ana"}, {7, "Bo"}},
	}
	Apply(tab, map[string]struct{}{"id": {}}, nil)
	if tab.Columns[0].Type != protocol.TypeString {
		t.Fatalf("type=%v", tab.Columns[0].Type)
	}
	if tab.Rows[0][0] != Placeholder || tab.Rows[1][0] != Placeholder {
		t.Fatalf("rows=%v", tab.Rows)
	}
	if tab.Rows[0][1] != "Ana" {
		t.Fatalf("name touched: %v", tab.Rows[0][1])
	}
}

func TestApplyExtraOutAlias(t *testing.T) {
	tab := &protocol.TabularResult{
		Columns: []protocol.Column{{Name: "x", Type: protocol.TypeString}, {Name: "name", Type: protocol.TypeString}},
		Rows:    [][]any{{"secret", "Ana"}},
	}
	Apply(tab, map[string]struct{}{"id": {}}, map[string]struct{}{"x": {}})
	if tab.Rows[0][0] != Placeholder {
		t.Fatalf("got %#v", tab.Rows[0][0])
	}
}

func TestExtraOutNamesFromIR(t *testing.T) {
	q := &protocol.QueryIR{
		Select: []any{
			"id",
			map[string]any{"agg": "count", "field": "id", "as": "n"},
			"name",
		},
	}
	extra := ExtraOutNamesFromIR(q, map[string]struct{}{"id": {}})
	if _, ok := extra["id"]; !ok {
		t.Fatal("want id")
	}
	if _, ok := extra["n"]; !ok {
		t.Fatal("want agg as n")
	}
	if _, ok := extra["name"]; ok {
		t.Fatal("name not redacted")
	}
}

func TestExtraOutNamesFromSQL(t *testing.T) {
	parsed := &sqlparse.Result{
		SelectOuts: []sqlparse.SelectOut{
			{OutName: "x", Fields: []string{"id"}},
			{OutName: "name", Fields: []string{"name"}},
		},
	}
	extra := ExtraOutNamesFromSQL(parsed, map[string]struct{}{"id": {}})
	if _, ok := extra["x"]; !ok {
		t.Fatal("want alias x")
	}
}

func TestApplySelectOutsPositional(t *testing.T) {
	tab := &protocol.TabularResult{
		Columns: []protocol.Column{
			{Name: "concat", Type: protocol.TypeString},
			{Name: "name", Type: protocol.TypeString},
		},
		Rows: [][]any{{"42x", "Ana"}},
	}
	outs := []sqlparse.SelectOut{
		{OutName: "concat", Fields: []string{"id"}},
		{OutName: "name", Fields: []string{"name"}},
	}
	ApplySelectOuts(tab, map[string]struct{}{"id": {}}, outs)
	if tab.Rows[0][0] != Placeholder {
		t.Fatalf("expr col %#v", tab.Rows[0][0])
	}
	if tab.Rows[0][1] != "Ana" {
		t.Fatalf("name %#v", tab.Rows[0][1])
	}
}
