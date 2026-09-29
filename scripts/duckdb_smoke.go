//go:build ignore

// DuckDB local engine smoke (requires CGO + gcc + -tags duckdb when using DuckDB Open).
//
//	go run -tags duckdb ./scripts/duckdb_smoke.go
//
// After scripts/dev-shell.ps1 on Windows.

package main

import (
	"context"
	"fmt"
	"os"

	"qLLM/internal/duckdblocal"
	"qLLM/internal/protocol"
	"qLLM/internal/result"
)

func main() {
	eng, err := duckdblocal.Open()
	if err != nil {
		fail(err)
	}
	defer eng.Close()
	ctx := context.Background()
	if err := eng.Materialize(ctx, "a", result.New(
		[]protocol.Column{{Name: "id", Type: protocol.TypeString}},
		[][]any{{"1"}, {"2"}},
		false,
	)); err != nil {
		fail(err)
	}
	tab, err := eng.Execute(ctx, duckdblocal.QuerySpec{
		RootBind: "a",
		Select:   []duckdblocal.SelectSpec{{Bind: "a", Col: "id", As: "id"}},
		Limit:    10,
	})
	if err != nil {
		fail(err)
	}
	fmt.Printf("local engine smoke ok rows=%d\n", tab.RowCount)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "smoke failed: %v\n", err)
	os.Exit(1)
}
