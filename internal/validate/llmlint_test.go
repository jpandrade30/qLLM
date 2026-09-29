package validate_test

import (
	"testing"

	"qLLM/internal/protocol"
	"qLLM/internal/validate"
)

func intPtr(n int) *int { return &n }

func TestLLMLintRejectsSQLOpsInIR(t *testing.T) {
	for _, w := range []map[string]any{
		{"field": "status", "op": "=", "value": "paid"},
		{"field": "email", "op": "like", "value": "%x%"},
	} {
		q := &protocol.QueryIR{
			From:   "invoices",
			Select: []any{"id"},
			Where:  w,
			Limit:  intPtr(10),
		}
		if err := validate.Query(nil, q); err == nil || err.Code != protocol.ErrInvalidIR {
			t.Fatalf("where=%v got %v", w, err)
		}
	}
}

func TestLLMLintRejectsMongoWhereShape(t *testing.T) {
	q := &protocol.QueryIR{
		From:   "invoices",
		Select: []any{"id"},
		Where: map[string]any{
			"and": []any{map[string]any{"field": "status", "op": "eq", "value": "paid"}},
		},
		Limit: intPtr(10),
	}
	if err := validate.Query(nil, q); err == nil || err.Code != protocol.ErrInvalidIR {
		t.Fatalf("got %v", err)
	}
}
