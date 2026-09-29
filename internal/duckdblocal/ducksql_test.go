package duckdblocal_test

import (
	"strings"
	"testing"

	"qLLM/internal/duckdblocal"
)

func TestBuildDuckSQLJoinWhereOffset(t *testing.T) {
	sql, args, err := duckdblocal.BuildDuckSQL(duckdblocal.QuerySpec{
		RootBind: "c",
		Joins: []duckdblocal.JoinSpec{{
			Type: "left", RightTable: "i",
			On: []duckdblocal.JoinOn{
				{LeftBind: "c", LeftCol: "id", RightBind: "i", RightCol: "customer_id"},
			},
		}},
		Select: []duckdblocal.SelectSpec{
			{Bind: "c", Col: "id", As: "id"},
			{Bind: "i", Col: "total", Agg: "sum", As: "total"},
		},
		Where: &duckdblocal.WhereExpr{Op: "and", Args: []duckdblocal.WhereExpr{
			{Op: "eq", Bind: "c", Col: "id", Value: "c1"},
			{Op: "in", Bind: "i", Col: "id", Value: []any{"i1", "i2"}},
		}},
		GroupBy: []duckdblocal.SelectSpec{{Bind: "c", Col: "id"}},
		OrderBy: []duckdblocal.OrderSpec{{As: "total", Dir: "desc"}},
		Limit:   10,
		Offset:  2,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`LEFT JOIN "i"`,
		`WHERE`,
		`GROUP BY`,
		`ORDER BY "total" DESC`,
		`LIMIT 10`,
		`OFFSET 2`,
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("missing %q in %s", want, sql)
		}
	}
	if len(args) != 3 {
		t.Fatalf("args=%d %#v", len(args), args)
	}
}
