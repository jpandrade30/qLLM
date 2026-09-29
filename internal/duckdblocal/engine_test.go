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
			{Type: "left", RightTable: "a", On: []duckdblocal.JoinOn{
				{LeftBind: "c", LeftCol: "id", RightBind: "a", RightCol: "customer_id"},
			}},
			{Type: "inner", RightTable: "i", On: []duckdblocal.JoinOn{
				{LeftBind: "c", LeftCol: "id", RightBind: "i", RightCol: "customer_id"},
			}},
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
		OrderBy: []duckdblocal.OrderSpec{{As: "total_billed", Dir: "desc"}},
		Limit:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tab.RowCount != 2 {
		t.Fatalf("rows=%d %#v", tab.RowCount, tab.Rows)
	}
}

func TestWhereOpsAndOrNot(t *testing.T) {
	eng := mustEng(t)
	ctx := context.Background()
	_ = eng.Materialize(ctx, "t", result.New(
		[]protocol.Column{{Name: "id", Type: protocol.TypeString}, {Name: "n", Type: protocol.TypeNumber}, {Name: "name", Type: protocol.TypeString}},
		[][]any{{"a", 1, "alice"}, {"b", 2, "bob"}, {"c", nil, "carol"}},
		false,
	))

	tab, err := eng.Execute(ctx, duckdblocal.QuerySpec{
		RootBind: "t",
		Select:   []duckdblocal.SelectSpec{{Bind: "t", Col: "id", As: "id"}},
		Where: &duckdblocal.WhereExpr{Op: "or", Args: []duckdblocal.WhereExpr{
			{Op: "eq", Bind: "t", Col: "id", Value: "a"},
			{Op: "gte", Bind: "t", Col: "n", Value: 2},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tab.RowCount != 2 {
		t.Fatalf("or rows=%d", tab.RowCount)
	}

	tab, err = eng.Execute(ctx, duckdblocal.QuerySpec{
		RootBind: "t",
		Select:   []duckdblocal.SelectSpec{{Bind: "t", Col: "id", As: "id"}},
		Where: &duckdblocal.WhereExpr{Op: "not", Args: []duckdblocal.WhereExpr{
			{Op: "eq", Bind: "t", Col: "id", Value: "a"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tab.RowCount != 2 {
		t.Fatalf("not rows=%d", tab.RowCount)
	}

	tab, err = eng.Execute(ctx, duckdblocal.QuerySpec{
		RootBind: "t",
		Select:   []duckdblocal.SelectSpec{{Bind: "t", Col: "id", As: "id"}},
		Where:    &duckdblocal.WhereExpr{Op: "in", Bind: "t", Col: "id", Value: []any{"a", "c"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tab.RowCount != 2 {
		t.Fatalf("in rows=%d", tab.RowCount)
	}

	tab, err = eng.Execute(ctx, duckdblocal.QuerySpec{
		RootBind: "t",
		Select:   []duckdblocal.SelectSpec{{Bind: "t", Col: "id", As: "id"}},
		Where:    &duckdblocal.WhereExpr{Op: "contains", Bind: "t", Col: "name", Value: "li"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tab.RowCount != 1 || tab.Rows[0][0] != "a" {
		t.Fatalf("contains %#v", tab.Rows)
	}

	tab, err = eng.Execute(ctx, duckdblocal.QuerySpec{
		RootBind: "t",
		Select:   []duckdblocal.SelectSpec{{Bind: "t", Col: "id", As: "id"}},
		Where:    &duckdblocal.WhereExpr{Op: "is_null", Bind: "t", Col: "n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tab.RowCount != 1 || tab.Rows[0][0] != "c" {
		t.Fatalf("is_null %#v", tab.Rows)
	}
}

func TestOffsetLimit(t *testing.T) {
	eng := mustEng(t)
	ctx := context.Background()
	_ = eng.Materialize(ctx, "t", result.New(
		[]protocol.Column{{Name: "id", Type: protocol.TypeString}},
		[][]any{{"1"}, {"2"}, {"3"}, {"4"}},
		false,
	))
	tab, err := eng.Execute(ctx, duckdblocal.QuerySpec{
		RootBind: "t",
		Select:   []duckdblocal.SelectSpec{{Bind: "t", Col: "id", As: "id"}},
		OrderBy:  []duckdblocal.OrderSpec{{As: "id", Dir: "asc"}},
		Offset:   1,
		Limit:    2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tab.RowCount != 2 || tab.Rows[0][0] != "2" || tab.Rows[1][0] != "3" {
		t.Fatalf("%#v", tab.Rows)
	}
}

func TestCompositeJoinOn(t *testing.T) {
	eng := mustEng(t)
	ctx := context.Background()
	_ = eng.Materialize(ctx, "l", result.New(
		[]protocol.Column{{Name: "a", Type: protocol.TypeString}, {Name: "b", Type: protocol.TypeString}},
		[][]any{{"1", "x"}, {"1", "y"}, {"2", "x"}},
		false,
	))
	_ = eng.Materialize(ctx, "r", result.New(
		[]protocol.Column{{Name: "a", Type: protocol.TypeString}, {Name: "b", Type: protocol.TypeString}, {Name: "v", Type: protocol.TypeString}},
		[][]any{{"1", "x", "ok"}},
		false,
	))
	tab, err := eng.Execute(ctx, duckdblocal.QuerySpec{
		RootBind: "l",
		Joins: []duckdblocal.JoinSpec{{
			Type: "inner", RightTable: "r",
			On: []duckdblocal.JoinOn{
				{LeftBind: "l", LeftCol: "a", RightBind: "r", RightCol: "a"},
				{LeftBind: "l", LeftCol: "b", RightBind: "r", RightCol: "b"},
			},
		}},
		Select: []duckdblocal.SelectSpec{
			{Bind: "l", Col: "a", As: "a"},
			{Bind: "r", Col: "v", As: "v"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tab.RowCount != 1 || tab.Rows[0][1] != "ok" {
		t.Fatalf("%#v", tab.Rows)
	}
}

func TestUnknownWhereOpFails(t *testing.T) {
	eng := mustEng(t)
	ctx := context.Background()
	_ = eng.Materialize(ctx, "t", result.New(
		[]protocol.Column{{Name: "id", Type: protocol.TypeString}},
		[][]any{{"a"}},
		false,
	))
	_, err := eng.Execute(ctx, duckdblocal.QuerySpec{
		RootBind: "t",
		Select:   []duckdblocal.SelectSpec{{Bind: "t", Col: "id", As: "id"}},
		Where:    &duckdblocal.WhereExpr{Op: "bogus", Bind: "t", Col: "id", Value: "a"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if pe, ok := err.(*protocol.ProtocolError); !ok || pe.Code != protocol.ErrUnsupported {
		t.Fatalf("got %v", err)
	}
}

func mustEng(t *testing.T) duckdblocal.Engine {
	t.Helper()
	eng, err := duckdblocal.Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	return eng
}
