//go:build duckdb

package duckdblocal_test

import (
	"context"
	"strings"
	"testing"

	"qLLM/internal/duckdblocal"
	"qLLM/internal/protocol"
	"qLLM/internal/result"
)

func seedInvoicesCustomers(t *testing.T, eng duckdblocal.Engine, ctx context.Context) {
	t.Helper()
	_ = eng.Materialize(ctx, "invoices", result.New(
		[]protocol.Column{
			{Name: "id", Type: protocol.TypeString},
			{Name: "status", Type: protocol.TypeString},
			{Name: "total", Type: protocol.TypeNumber},
		},
		[][]any{
			{"i1", "paid", 100},
			{"i2", "open", 50},
			{"i3", "paid", 75},
		},
		false,
	))
	_ = eng.Materialize(ctx, "customers", result.New(
		[]protocol.Column{
			{Name: "id", Type: protocol.TypeString},
			{Name: "name", Type: protocol.TypeString},
		},
		[][]any{
			{"c1", "Ada"},
			{"c2", "Bob"},
		},
		false,
	))
}

func TestSQLDialect1ExecFeatures(t *testing.T) {
	eng, err := duckdblocal.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	ctx := context.Background()
	seedInvoicesCustomers(t, eng, ctx)

	cases := []struct {
		sql      string
		minRows  int
		colNames []string
	}{
		{`SELECT DISTINCT status FROM invoices LIMIT 10`, 1, []string{"status"}},
		{`SELECT CASE WHEN status = 'paid' THEN 1 ELSE 0 END AS flag FROM invoices LIMIT 10`, 1, []string{"flag"}},
		{`SELECT id FROM invoices WHERE status LIKE 'p%' LIMIT 10`, 1, []string{"id"}},
		{`SELECT id FROM invoices WHERE total BETWEEN 40 AND 100 LIMIT 10`, 1, []string{"id"}},
		{`SELECT total + 1 AS bumped FROM invoices LIMIT 10`, 1, []string{"bumped"}},
		{`SELECT status FROM invoices GROUP BY status HAVING COUNT(*) > 1 LIMIT 10`, 1, []string{"status"}},
		{`SELECT COALESCE(NULLIF(status, ''), 'x') AS s FROM invoices LIMIT 10`, 1, []string{"s"}},
		{`SELECT LOWER(status) AS ls FROM invoices LIMIT 10`, 1, []string{"ls"}},
		{`SELECT ROUND(total, 0) AS r FROM invoices LIMIT 10`, 1, []string{"r"}},
		{`SELECT CAST(total AS INTEGER) AS ti FROM invoices LIMIT 10`, 1, []string{"ti"}},
	}
	for _, tc := range cases {
		tab, err := eng.ExecSQL(ctx, tc.sql)
		if err != nil {
			t.Fatalf("sql=%q err=%v", tc.sql, err)
		}
		if tab.RowCount < tc.minRows {
			t.Fatalf("sql=%q rows=%d", tc.sql, tab.RowCount)
		}
		for _, want := range tc.colNames {
			found := false
			for _, c := range tab.Columns {
				if c.Name == want {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("sql=%q cols=%v want %q", tc.sql, tab.Columns, want)
			}
		}
	}
}

func TestSQLDialect2SetOpsAndWindows(t *testing.T) {
	eng, err := duckdblocal.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	ctx := context.Background()
	seedInvoicesCustomers(t, eng, ctx)

	tab, err := eng.ExecSQL(ctx, `SELECT id FROM invoices LIMIT 10 UNION ALL SELECT id FROM customers LIMIT 10`)
	if err != nil {
		t.Fatal(err)
	}
	if tab.RowCount < 3 {
		t.Fatalf("union rows=%d", tab.RowCount)
	}

	tab, err = eng.ExecSQL(ctx, `SELECT COUNT(DISTINCT status) AS n FROM invoices LIMIT 10`)
	if err != nil {
		t.Fatal(err)
	}
	if tab.RowCount != 1 {
		t.Fatalf("distinct count rows=%d", tab.RowCount)
	}

	tab, err = eng.ExecSQL(ctx, `SELECT status, ROW_NUMBER() OVER (PARTITION BY status ORDER BY id) AS rn FROM invoices LIMIT 10`)
	if err != nil {
		t.Fatal(err)
	}
	if tab.RowCount < 1 {
		t.Fatal("window")
	}

	tab, err = eng.ExecSQL(ctx, `SELECT TRUE XOR FALSE AS x FROM invoices LIMIT 1`)
	if err != nil {
		t.Fatal(err)
	}
	if tab.RowCount != 1 {
		t.Fatal("xor")
	}
}

func TestSQLDialect2Qualify(t *testing.T) {
	eng, err := duckdblocal.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	ctx := context.Background()
	seedInvoicesCustomers(t, eng, ctx)

	_, err = eng.ExecSQL(ctx, `SELECT status FROM invoices GROUP BY status QUALIFY COUNT(*) >= 2 LIMIT 10`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSQLFamilyDateTrunc(t *testing.T) {
	eng, err := duckdblocal.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	ctx := context.Background()
	_ = eng.Materialize(ctx, "invoices", result.New(
		[]protocol.Column{{Name: "issued_at", Type: protocol.TypeTimestamp}},
		[][]any{{"2024-06-15T12:00:00Z"}},
		false,
	))
	_, err = eng.ExecSQL(ctx, `SELECT date_trunc('month', issued_at::TIMESTAMP) AS m FROM invoices LIMIT 1`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSQLCTEJoinQualifyLimit(t *testing.T) {
	eng, err := duckdblocal.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	ctx := context.Background()
	_ = eng.Materialize(ctx, "customers", result.New(
		[]protocol.Column{
			{Name: "id", Type: protocol.TypeString},
			{Name: "full_name", Type: protocol.TypeString},
			{Name: "country", Type: protocol.TypeString},
		},
		[][]any{{"c1", "Ada", "BR"}, {"c2", "Bob", "US"}},
		false,
	))
	_ = eng.Materialize(ctx, "invoices", result.New(
		[]protocol.Column{
			{Name: "id", Type: protocol.TypeString},
			{Name: "customer_id", Type: protocol.TypeString},
			{Name: "total", Type: protocol.TypeNumber},
			{Name: "status", Type: protocol.TypeString},
			{Name: "issued_at", Type: protocol.TypeTimestamp},
		},
		[][]any{
			{"i1", "c1", 100, "paid", "2025-03-01T00:00:00Z"},
			{"i2", "c1", 50, "paid", "2025-06-01T00:00:00Z"},
			{"i3", "c2", 20, "open", "2025-01-01T00:00:00Z"},
		},
		false,
	))
	_ = eng.Materialize(ctx, "support_tickets", result.New(
		[]protocol.Column{
			{Name: "id", Type: protocol.TypeString},
			{Name: "customer_id", Type: protocol.TypeString},
			{Name: "status", Type: protocol.TypeString},
		},
		[][]any{{"t1", "c1", "open"}, {"t2", "c1", "closed"}},
		false,
	))

	extractSQL := `SELECT EXTRACT(YEAR FROM issued_at::TIMESTAMP) AS y FROM invoices LIMIT 1`
	if _, err = eng.ExecSQL(ctx, extractSQL); err != nil {
		t.Fatalf("EXTRACT: %v", err)
	}

	qualifyWindow := `
WITH revenue AS (
  SELECT c.id, c.full_name, c.country, SUM(i.total) AS paid_total
  FROM customers c
  JOIN invoices i ON i.customer_id = c.id
  WHERE i.status = 'paid' AND EXTRACT(YEAR FROM i.issued_at) = 2025
  GROUP BY c.id, c.full_name, c.country
)
SELECT r.full_name, r.country, r.paid_total, COUNT(st.id) AS open_tickets,
  ROW_NUMBER() OVER (ORDER BY r.paid_total DESC) AS posicao
FROM revenue r
LEFT JOIN support_tickets st ON st.customer_id = r.id AND st.status = 'open'
GROUP BY r.id, r.full_name, r.country, r.paid_total
QUALIFY ROW_NUMBER() OVER (ORDER BY r.paid_total DESC) <= 10
ORDER BY posicao
LIMIT 10`

	tab, err := eng.ExecSQL(ctx, qualifyWindow)
	if err != nil {
		t.Fatalf("CTE QUALIFY window: %v", err)
	}
	if tab.RowCount < 1 {
		t.Fatal("expected rows")
	}

	qualifyAlias := strings.Replace(qualifyWindow,
		"QUALIFY ROW_NUMBER() OVER (ORDER BY r.paid_total DESC) <= 10",
		"QUALIFY posicao <= 10", 1)
	if _, err = eng.ExecSQL(ctx, qualifyAlias); err != nil {
		t.Logf("QUALIFY select-list alias not supported (use window in QUALIFY): %v", err)
	}
}
