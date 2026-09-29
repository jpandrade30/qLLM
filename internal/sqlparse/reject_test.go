package sqlparse_test

import (
	"testing"

	"qLLM/internal/protocol"
	"qLLM/internal/sqlparse"
)

func TestRejectMutations(t *testing.T) {
	for _, sql := range []string{
		`INSERT INTO invoices VALUES (1)`,
		`UPDATE invoices SET status = 'x'`,
		`DELETE FROM invoices`,
		`MERGE INTO invoices USING customers ON TRUE`,
	} {
		_, err := sqlparse.Parse(sql)
		if err == nil || err.Code != protocol.ErrInvalidSQL {
			t.Fatalf("sql=%q got %v", sql, err)
		}
	}
}

func TestRejectIOFunctions(t *testing.T) {
	for _, sql := range []string{
		`SELECT * FROM read_csv('x.csv')`,
		`SELECT * FROM httpfs('http://x')`,
	} {
		_, err := sqlparse.Parse(sql)
		if err == nil || err.Code != protocol.ErrInvalidSQL {
			t.Fatalf("sql=%q got %v", sql, err)
		}
	}
}

func TestRejectSchemaQualified(t *testing.T) {
	_, err := sqlparse.Parse(`SELECT * FROM public.invoices`)
	if err == nil || err.Code != protocol.ErrInvalidSQL {
		t.Fatalf("got %v", err)
	}
}

func TestRejectMultiStatement(t *testing.T) {
	_, err := sqlparse.Parse(`SELECT 1; SELECT 2`)
	if err == nil || err.Code != protocol.ErrInvalidSQL {
		t.Fatalf("got %v", err)
	}
}

func TestTrailingSemicolonIsOneStatement(t *testing.T) {
	sql := `
SELECT
  c.id,
  c.full_name,
  c.email,
  COUNT(st.id) as tickets_abertos,
  s.plan
FROM customers c
LEFT JOIN subscriptions s ON c.id = s.customer_id AND s.status = 'active'
LEFT JOIN support_tickets st ON c.id = st.customer_id AND st.status = 'open'
GROUP BY c.id, c.full_name, c.email, s.plan
ORDER BY tickets_abertos DESC
LIMIT 50;
`
	r, err := sqlparse.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	if r.Limit == nil || *r.Limit != 50 {
		t.Fatalf("limit=%v", r.Limit)
	}
	found := map[string]bool{}
	for _, t := range r.Tables {
		found[t.Name] = true
	}
	for _, n := range []string{"customers", "subscriptions", "support_tickets"} {
		if !found[n] {
			t.Fatalf("missing table %s in %+v", n, r.Tables)
		}
	}
}
