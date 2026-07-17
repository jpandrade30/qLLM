package duckdblocal_test

import (
	"context"
	"testing"

	"qLLM/internal/duckdblocal"
	"qLLM/internal/protocol"
	"qLLM/internal/result"
)

func TestMultiJoinAgg(t *testing.T) {
	eng, err := duckdblocal.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	ctx := context.Background()
	_ = eng.Materialize(ctx, "c", result.New(
		[]protocol.Column{{Name: "id", Type: protocol.TypeString}, {Name: "email", Type: protocol.TypeString}},
		[][]any{{"c1", "a@x.com"}, {"c2", "b@x.com"}},
		false,
	))
	_ = eng.Materialize(ctx, "a", result.New(
		[]protocol.Column{{Name: "id", Type: protocol.TypeString}, {Name: "customer_id", Type: protocol.TypeString}, {Name: "city", Type: protocol.TypeString}},
		[][]any{{"a1", "c1", "SP"}, {"a2", "c2", "RJ"}},
		false,
	))
	_ = eng.Materialize(ctx, "i", result.New(
		[]protocol.Column{{Name: "id", Type: protocol.TypeString}, {Name: "customer_id", Type: protocol.TypeString}, {Name: "total", Type: protocol.TypeNumber}},
		[][]any{{"i1", "c1", 100}, {"i2", "c1", 50}, {"i3", "c2", 20}},
		false,
	))

	tab, err := eng.Execute(ctx, duckdblocal.QuerySpec{
		RootBind: "c",
		Joins: []duckdblocal.JoinSpec{
			{Type: "left", RightTable: "a", LeftBind: "c", LeftCol: "id", RightBind: "a", RightCol: "customer_id"},
			{Type: "inner", RightTable: "i", LeftBind: "c", LeftCol: "id", RightBind: "i", RightCol: "customer_id"},
		},
		Select: []duckdblocal.SelectSpec{
			{Bind: "c", Col: "id", As: "id"},
			{Bind: "a", Col: "city", As: "city"},
			{Bind: "i", Col: "total", Agg: "sum", As: "total_billed"},
			{Bind: "i", Col: "id", Agg: "count", As: "n"},
		},
		GroupBy: []duckdblocal.SelectSpec{
			{Bind: "c", Col: "id"},
			{Bind: "a", Col: "city"},
		},
		OrderBy: []struct {
			As  string
			Dir string
		}{{As: "total_billed", Dir: "desc"}},
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tab.RowCount != 2 {
		t.Fatalf("rows=%d %#v", tab.RowCount, tab.Rows)
	}
}
