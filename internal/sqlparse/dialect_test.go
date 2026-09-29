package sqlparse_test

import (
	"testing"

	"qLLM/internal/protocol"
	"qLLM/internal/sqlparse"
)

func TestDialect1FeaturesParse(t *testing.T) {
	cases := []struct {
		sql    string
		tables []string
	}{
		{`SELECT status FROM invoices GROUP BY status HAVING COUNT(*) > 1 LIMIT 10`, []string{"invoices"}},
		{`SELECT DISTINCT status FROM invoices LIMIT 10`, []string{"invoices"}},
		{`SELECT CASE WHEN status = 'paid' THEN 1 ELSE 0 END AS x FROM invoices LIMIT 10`, []string{"invoices"}},
		{`SELECT id FROM invoices WHERE status LIKE 'p%' LIMIT 10`, []string{"invoices"}},
		{`SELECT id FROM invoices WHERE total BETWEEN 1 AND 100 LIMIT 10`, []string{"invoices"}},
		{`WITH t AS (SELECT id, status FROM invoices LIMIT 100) SELECT status FROM t LIMIT 10`, []string{"invoices"}},
		{`SELECT id FROM (SELECT id FROM invoices LIMIT 100) s LIMIT 10`, []string{"invoices"}},
		{`SELECT total + 1 AS bumped FROM invoices LIMIT 10`, []string{"invoices"}},
		{`SELECT id FROM invoices ORDER BY id LIMIT 5 OFFSET 2`, []string{"invoices"}},
	}
	for _, tc := range cases {
		r, err := sqlparse.ParseWithVersion(tc.sql, protocol.SQLDialect1)
		if err != nil {
			t.Fatalf("sql=%q err=%v", tc.sql, err)
		}
		names := map[string]bool{}
		for _, tb := range r.Tables {
			names[tb.Name] = true
		}
		for _, want := range tc.tables {
			if !names[want] {
				t.Fatalf("sql=%q tables=%v want %q", tc.sql, r.Tables, want)
			}
		}
	}
}

func TestDialect1RejectsSetOps(t *testing.T) {
	_, err := sqlparse.ParseWithVersion(
		`SELECT id FROM invoices LIMIT 10 UNION SELECT id FROM customers LIMIT 10`,
		protocol.SQLDialect1,
	)
	if err == nil || err.Code != protocol.ErrInvalidSQL {
		t.Fatalf("got %v", err)
	}
}

func TestDialect1RejectsQualify(t *testing.T) {
	_, err := sqlparse.ParseWithVersion(
		`SELECT status FROM invoices GROUP BY status QUALIFY COUNT(*) > 1 LIMIT 10`,
		protocol.SQLDialect1,
	)
	if err == nil || err.Code != protocol.ErrInvalidSQL {
		t.Fatalf("got %v", err)
	}
}

func TestDialect2SetOpsParse(t *testing.T) {
	sql := `SELECT id FROM invoices LIMIT 10 UNION ALL SELECT id FROM customers LIMIT 10`
	r, err := sqlparse.ParseWithVersion(sql, protocol.SQLDialect2)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, tb := range r.Tables {
		got[tb.Name] = true
	}
	if !got["invoices"] || !got["customers"] {
		t.Fatalf("tables=%v", r.Tables)
	}
}

func TestDialect2WindowsAndDistinctCount(t *testing.T) {
	cases := []string{
		`SELECT ROW_NUMBER() OVER (PARTITION BY status ORDER BY id) AS rn FROM invoices LIMIT 10`,
		`SELECT COUNT(DISTINCT status) AS n FROM invoices LIMIT 10`,
		`SELECT status FROM invoices GROUP BY status QUALIFY COUNT(*) > 1 LIMIT 10`,
		`SELECT TRUE XOR FALSE AS x FROM invoices LIMIT 1`,
	}
	for _, sql := range cases {
		if _, err := sqlparse.ParseWithVersion(sql, protocol.SQLDialect2); err != nil {
			t.Fatalf("sql=%q err=%v", sql, err)
		}
	}
}

func TestWindowAliasesNotCollectedAsColumns(t *testing.T) {
	sql := `SELECT id, status, total, RANK() OVER (ORDER BY total DESC) AS rnk, LAG(total) OVER (ORDER BY total) AS prev_total FROM invoices LIMIT 50`
	r, err := sqlparse.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range r.Columns {
		if c.Name == "rnk" || c.Name == "prev_total" {
			t.Fatalf("alias collected as column: %+v", r.Columns)
		}
	}
	found := map[string]bool{}
	for _, c := range r.Columns {
		found[c.Name] = true
	}
	if !found["id"] || !found["status"] || !found["total"] {
		t.Fatalf("want real fields, got %v", r.Columns)
	}
}

func TestInjectLimitAppends(t *testing.T) {
	got := sqlparse.InjectLimit(`SELECT id FROM invoices OFFSET 5`, 100)
	if got != `SELECT id FROM invoices OFFSET 5 LIMIT 100` {
		t.Fatalf("got %q", got)
	}
}
