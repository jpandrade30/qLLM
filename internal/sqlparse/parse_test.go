package sqlparse

import (
	"testing"

	"qLLM/internal/protocol"
)

func TestParseJoin(t *testing.T) {
	r, err := Parse(`SELECT c.id, i.total FROM customers c INNER JOIN invoices i ON i.customer_id = c.id LIMIT 100`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Tables) != 2 {
		t.Fatalf("tables=%v", r.Tables)
	}
	if r.Limit == nil || *r.Limit != 100 {
		t.Fatalf("limit=%v", r.Limit)
	}
	found := map[string]bool{}
	for _, c := range r.Columns {
		found[c.Qual+"."+c.Name] = true
	}
	if !found["c.id"] || !found["i.total"] {
		t.Fatalf("cols=%v", r.Columns)
	}
}

func TestRejectInsert(t *testing.T) {
	_, err := Parse(`INSERT INTO customers VALUES (1)`)
	if err == nil || err.Code != protocol.ErrInvalidSQL {
		t.Fatalf("got %v", err)
	}
}

func TestRejectMulti(t *testing.T) {
	_, err := Parse(`SELECT 1; SELECT 2`)
	if err == nil {
		t.Fatal("expected multi")
	}
}

func TestRejectReadCSV(t *testing.T) {
	_, err := Parse(`SELECT * FROM read_csv('x.csv')`)
	if err == nil {
		t.Fatal("expected ban")
	}
}

func TestRejectSchema(t *testing.T) {
	_, err := Parse(`SELECT * FROM public.customers`)
	if err == nil {
		t.Fatal("expected schema ban")
	}
}

func TestStar(t *testing.T) {
	r, err := Parse(`SELECT * FROM customers`)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Star || len(r.Tables) != 1 || r.Tables[0].Name != "customers" {
		t.Fatalf("%+v", r)
	}
}
