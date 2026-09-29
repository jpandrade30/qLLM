//go:build duckdb

package duckdblocal_test

import (
	"context"
	"testing"

	"qLLM/internal/duckdblocal"
	"qLLM/internal/protocol"
	"qLLM/internal/result"
)

// Runs only with: CGO_ENABLED=1 go test -tags duckdb ./internal/duckdblocal/
func TestDuckDBEngineSmoke(t *testing.T) {
	eng, err := duckdblocal.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	ctx := context.Background()
	if err := eng.Materialize(ctx, "t", result.New(
		[]protocol.Column{{Name: "id", Type: protocol.TypeString}, {Name: "n", Type: protocol.TypeNumber}},
		[][]any{{"a", 1.0}, {"b", 2.0}},
		false,
	)); err != nil {
		t.Fatal(err)
	}
	tab, err := eng.Execute(ctx, duckdblocal.QuerySpec{
		RootBind: "t",
		Select:   []duckdblocal.SelectSpec{{Bind: "t", Col: "id", As: "id"}},
		Where:    &duckdblocal.WhereExpr{Op: "gte", Bind: "t", Col: "n", Value: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tab.RowCount != 1 || tab.Rows[0][0] != "b" {
		t.Fatalf("%#v", tab.Rows)
	}
}
