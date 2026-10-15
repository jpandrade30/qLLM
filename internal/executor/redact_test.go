package executor

import (
	"testing"

	"qLLM/internal/catalogidx"
	"qLLM/internal/protocol"
	"qLLM/internal/redact"
	"qLLM/internal/sqlparse"
)

func TestApplyRedactIRMasksAndKeepsOther(t *testing.T) {
	idx, err := catalogidx.New(
		&protocol.Preset{
			ProtocolVersion: "0.2.0", Project: "t",
			Limits:  protocol.Limits{MaxSyncMs: 5_000, MaxSourceMs: 5_000, DefaultLimit: 10, MaxLimit: 100, ReadOnly: true},
			Sources: []protocol.Source{{ID: "s", Type: protocol.SourceREST, Connection: map[string]any{"baseUrlEnv": "X"}}},
		},
		&protocol.Catalog{
			ProtocolVersion: "0.2.0", Project: "t",
			Entities: []protocol.Entity{{
				Name: "customers", Source: "s",
				Binding: protocol.Binding{Kind: "rest_resource", Resource: "customers"},
				Fields: []protocol.Field{
					{Name: "id", Type: protocol.TypeString, Physical: "id", Redact: true},
					{Name: "name", Type: protocol.TypeString, Physical: "name"},
				},
			}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	e := &Executor{Idx: idx}
	tab := &protocol.TabularResult{
		Columns: []protocol.Column{
			{Name: "id", Type: protocol.TypeString},
			{Name: "name", Type: protocol.TypeString},
		},
		Rows: [][]any{{"c1", "Ana"}, {"c2", "Bo"}},
	}
	q := &protocol.QueryIR{
		From:   "customers",
		Select: []any{"id", "name"},
	}
	e.applyRedact(tab, []string{"customers"}, q, nil)
	if tab.Rows[0][0] != redact.Placeholder || tab.Rows[1][0] != redact.Placeholder {
		t.Fatalf("id cells %#v", tab.Rows)
	}
	if tab.Rows[0][1] != "Ana" {
		t.Fatalf("name %#v", tab.Rows[0][1])
	}
}

func TestApplyRedactSQLAliasProvenance(t *testing.T) {
	idx, err := catalogidx.New(
		&protocol.Preset{
			ProtocolVersion: "0.2.0", Project: "t",
			Limits:  protocol.Limits{MaxSyncMs: 5_000, MaxSourceMs: 5_000, DefaultLimit: 10, MaxLimit: 100, ReadOnly: true},
			Sources: []protocol.Source{{ID: "s", Type: protocol.SourceSQLite, Connection: map[string]any{"path": ":memory:"}}},
		},
		&protocol.Catalog{
			ProtocolVersion: "0.2.0", Project: "t",
			Entities: []protocol.Entity{{
				Name: "customers", Source: "s",
				Binding: protocol.Binding{Kind: "table", Table: "customers"},
				Fields: []protocol.Field{
					{Name: "id", Type: protocol.TypeString, Physical: "id", Redact: true},
					{Name: "name", Type: protocol.TypeString, Physical: "name"},
				},
			}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	parsed, perr := sqlparse.Parse(`SELECT id AS x, name FROM customers LIMIT 10`)
	if perr != nil {
		t.Fatal(perr)
	}
	if len(parsed.SelectOuts) < 2 || parsed.SelectOuts[0].OutName != "x" {
		t.Fatalf("SelectOuts=%+v", parsed.SelectOuts)
	}
	e := &Executor{Idx: idx}
	tab := &protocol.TabularResult{
		Columns: []protocol.Column{{Name: "x", Type: protocol.TypeString}, {Name: "name", Type: protocol.TypeString}},
		Rows:    [][]any{{"secret-id", "Ana"}},
	}
	e.applyRedact(tab, []string{"customers"}, nil, parsed)
	if tab.Rows[0][0] != redact.Placeholder {
		t.Fatalf("alias x %#v", tab.Rows[0][0])
	}
	if tab.Rows[0][1] != "Ana" {
		t.Fatalf("name %#v", tab.Rows[0][1])
	}
}
