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
