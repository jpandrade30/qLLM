package executor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"qLLM/internal/connector"
	"qLLM/internal/protocol"
	"qLLM/internal/validate"
)

func TestFromFilterGroupByAndJoin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/balance":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"saldo": 5300}})
		case "/users":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "42", "name": "Ana"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("QLLM_REST_URL", srv.URL)
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
				"balance": map[string]any{"list": map[string]any{"method": "GET", "path": "/balance"}},
				"users":   map[string]any{"list": map[string]any{"method": "GET", "path": "/users"}},
			}},
		}},
	}
	c := &protocol.Catalog{
		ProtocolVersion: "0.2.0",
		Project:         "test",
		Entities: []protocol.Entity{
			{
				Name: "balance", Source: "bank_api",
				Binding: protocol.Binding{Kind: "rest_resource", Resource: "balance"},
				Fields: []protocol.Field{
					{Name: "user_id", Type: protocol.TypeString, Physical: "user_id", FromFilter: true},
					{Name: "saldo", Type: protocol.TypeNumber, Physical: "saldo"},
				},
			},
			{
				Name: "users", Source: "bank_api",
				Binding: protocol.Binding{Kind: "rest_resource", Resource: "users"},
				Fields: []protocol.Field{
					{Name: "id", Type: protocol.TypeString, Physical: "id"},
					{Name: "name", Type: protocol.TypeString, Physical: "name"},
				},
			},
		},
	}
	idx, err := validate.Bundle(p, c)
	if err != nil {
		t.Fatal(err)
	}
	reg, oerr := connector.OpenAll(p, connector.OpenOpts{})
	if oerr != nil {
		t.Fatal(oerr)
	}
	defer reg.Close()
	ex := New(idx, reg, nil)
	lim := 10

	agg := ex.Execute(context.Background(), &protocol.QueryIR{
		From:    "balance",
		Select:  []any{"user_id", map[string]any{"agg": "sum", "field": "saldo", "as": "total"}},
		Where:   map[string]any{"op": "eq", "field": "user_id", "value": "42"},
		GroupBy: []string{"user_id"},
		Limit:   &lim,
	})
	if agg.Status != protocol.StatusSucceeded {
		t.Fatalf("agg %#v", agg.Error)
	}
	if agg.Result == nil || agg.Result.RowCount != 1 {
		t.Fatalf("agg rows %#v", agg.Result)
	}
	if agg.Result.Rows[0][0] != "42" {
		t.Fatalf("group key %#v", agg.Result.Rows[0])
	}

	join := ex.Execute(context.Background(), &protocol.QueryIR{
		From: "balance",
		As:   "balance",
		Joins: []protocol.Join{{
			Type: "inner", From: "users", As: "users",
			On: []protocol.JoinOn{{Left: "balance.user_id", Right: "users.id"}},
		}},
		Select: []any{"users.name", "balance.saldo", "balance.user_id"},
		Where:  map[string]any{"op": "eq", "field": "balance.user_id", "value": "42"},
		Limit:  &lim,
	})
	if join.Status != protocol.StatusSucceeded {
		t.Fatalf("join %#v", join.Error)
	}
	if join.Result == nil || join.Result.RowCount != 1 {
		t.Fatalf("join rows %#v", join.Result)
	}
	if join.Result.Rows[0][0] != "Ana" || join.Result.Rows[0][2] != "42" {
		t.Fatalf("join row %#v", join.Result.Rows[0])
	}
}
