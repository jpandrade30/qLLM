package duckdblocal_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"qLLM/internal/duckdblocal"
	"qLLM/internal/protocol"
	"qLLM/internal/result"
)

// Golden-style local join: mimics fixtures/queries/invoices_customers_join.json shape
// without live connectors (cross-source join → local engine).
func TestGoldenInvoicesCustomersJoinShape(t *testing.T) {
	eng, err := duckdblocal.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	ctx := context.Background()

	_ = eng.Materialize(ctx, "invoices", result.New(
		[]protocol.Column{
			{Name: "id", Type: protocol.TypeString},
			{Name: "customer_id", Type: protocol.TypeString},
			{Name: "total", Type: protocol.TypeNumber},
			{Name: "status", Type: protocol.TypeString},
		},
		[][]any{
			{"i1", "c1", 100.0, "paid"},
			{"i2", "c2", 50.0, "open"},
		},
		false,
	))
	_ = eng.Materialize(ctx, "customers", result.New(
		[]protocol.Column{
			{Name: "id", Type: protocol.TypeString},
			{Name: "email", Type: protocol.TypeString},
		},
		[][]any{
			{"c1", "a@x.com"},
			{"c2", "b@x.com"},
		},
		false,
	))

	tab, err := eng.Execute(ctx, duckdblocal.QuerySpec{
		RootBind: "invoices",
		Joins: []duckdblocal.JoinSpec{{
			Type: "inner", RightTable: "customers",
			On: []duckdblocal.JoinOn{
				{LeftBind: "invoices", LeftCol: "customer_id", RightBind: "customers", RightCol: "id"},
			},
		}},
		Select: []duckdblocal.SelectSpec{
			{Bind: "invoices", Col: "id", As: "id"},
			{Bind: "customers", Col: "email", As: "email"},
			{Bind: "invoices", Col: "total", As: "total"},
		},
		Where: &duckdblocal.WhereExpr{Op: "eq", Bind: "invoices", Col: "status", Value: "paid"},
		Limit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tab.RowCount != 1 {
		t.Fatalf("rows=%d %#v", tab.RowCount, tab.Rows)
	}
	if tab.Rows[0][1] != "a@x.com" {
		t.Fatalf("%#v", tab.Rows[0])
	}
}

func TestFixtureIRFilesExist(t *testing.T) {
	root := filepath.Join("..", "..", "fixtures", "queries")
	for _, name := range []string{"invoices_customers_join.json", "events_customers_join.json"} {
		p := filepath.Join(root, name)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		var ir protocol.QueryIR
		if err := json.Unmarshal(b, &ir); err != nil {
			t.Fatalf("%s parse: %v", name, err)
		}
		if len(ir.Joins) == 0 {
			t.Fatalf("%s expected joins", name)
		}
	}
}
