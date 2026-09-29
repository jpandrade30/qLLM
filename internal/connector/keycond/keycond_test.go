package keycond

import (
	"testing"

	"qLLM/internal/protocol"
)

func TestRequireEqMissing(t *testing.T) {
	_, err := RequireEq(map[string]any{"field": "other", "op": "eq", "value": "x"}, []string{"pk"})
	if err == nil || err.Code != protocol.ErrUnsupported {
		t.Fatalf("got %v", err)
	}
}

func TestRequireEqOK(t *testing.T) {
	eqs, err := RequireEq(map[string]any{"field": "pk", "op": "eq", "value": "a"}, []string{"pk"})
	if err != nil {
		t.Fatal(err)
	}
	if eqs["pk"] != "a" {
		t.Fatalf("%v", eqs)
	}
}
