package planner

import (
	"testing"

	"qLLM/internal/protocol"
	"qLLM/internal/validate"
)

func TestBuildKeepsFromFilterInDuckDBStep(t *testing.T) {
	p := &protocol.Preset{
		ProtocolVersion: "0.2.0",
		Project:         "test",
		Limits: protocol.Limits{
			MaxSyncMs: 15000, MaxSourceMs: 12000, DefaultLimit: 100, MaxLimit: 1000, ReadOnly: true,
		},
		Sources: []protocol.Source{{
			ID: "bank_api", Type: protocol.SourceREST,
			Connection: map[string]any{"baseUrlEnv": "QLLM_REST_URL"},
			Options: map[string]any{"resources": map[string]any{
				"balance": map[string]any{"list": map[string]any{"path": "/balance"}},
			}},
		}},
	}
	c := &protocol.Catalog{
		ProtocolVersion: "0.2.0",
		Project:         "test",
		Entities: []protocol.Entity{{
			Name: "balance", Source: "bank_api",
			Binding: protocol.Binding{Kind: "rest_resource", Resource: "balance"},
			Fields: []protocol.Field{
				{Name: "user_id", Type: protocol.TypeString, Physical: "user_id", FromFilter: true},
				{Name: "saldo", Type: protocol.TypeNumber, Physical: "saldo"},
			},
		}},
	}
	idx, err := validate.Bundle(p, c)
	if err != nil {
		t.Fatal(err)
	}
	lim := 10
	plan, perr := Build(idx, &protocol.QueryIR{
		From:    "balance",
		Select:  []any{map[string]any{"agg": "sum", "field": "saldo", "as": "total"}},
		Where:   map[string]any{"op": "eq", "field": "user_id", "value": "42"},
		GroupBy: []string{"user_id"},
		Limit:   &lim,
	})
	if perr != nil {
		t.Fatal(perr)
	}
	if !plan.UseDuckDB {
		t.Fatal("REST agg must use DuckDB")
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("steps %d", len(plan.Steps))
	}
	got := map[string]bool{}
	for _, s := range plan.Steps[0].Select {
		got[s.Field] = true
	}
	if !got["user_id"] || !got["saldo"] {
		t.Fatalf("select %#v", plan.Steps[0].Select)
	}
	if plan.Steps[0].Where == nil {
		t.Fatal("where dropped")
	}
}
