package validate

import (
	"testing"

	"qLLM/internal/protocol"
)

func TestQueryResponseSchemaSuccessAndError(t *testing.T) {
	ok := &protocol.QueryResponse{
		ProtocolVersion: protocol.ProtocolVersion,
		QueryID:         "q1",
		Status:          protocol.StatusSucceeded,
		Result: &protocol.TabularResult{
			Columns: []protocol.Column{
				{Name: "id", Type: protocol.TypeString},
				{Name: "price", Type: protocol.TypeNumber},
				{Name: "ok", Type: protocol.TypeBoolean},
				{Name: "ts", Type: protocol.TypeTimestamp},
				{Name: "addr", Type: protocol.TypeJSON},
				{Name: "tags", Type: protocol.TypeJSON},
			},
			Rows: [][]any{
				{"1", 12.5, true, "2026-01-02T03:04:05Z", map[string]any{"city": "SP"}, []any{"a", "b"}},
			},
			RowCount:  1,
			Truncated: false,
		},
		Meta: &protocol.QueryMeta{ElapsedMs: 3, Mode: "sync"},
	}
	if err := QueryResponse(ok); err != nil {
		t.Fatalf("success: %v", err)
	}

	trunc := *ok
	trunc.Result = &protocol.TabularResult{
		Columns:   []protocol.Column{{Name: "id", Type: protocol.TypeString}},
		Rows:      [][]any{{"1"}},
		RowCount:  1,
		Truncated: true,
	}
	if err := QueryResponse(&trunc); err != nil {
		t.Fatalf("truncated: %v", err)
	}

	fail := &protocol.QueryResponse{
		ProtocolVersion: protocol.ProtocolVersion,
		QueryID:         "q2",
		Status:          protocol.StatusFailed,
		Error:           protocol.NewError(protocol.ErrInvalidIR, "bad", nil),
	}
	if err := QueryResponse(fail); err != nil {
		t.Fatalf("error: %v", err)
	}
}

func TestCatalogShapeField(t *testing.T) {
	c := &protocol.Catalog{
		ProtocolVersion: "0.2.0",
		Project:         "demo",
		Entities: []protocol.Entity{{
			Name:   "users",
			Source: "api",
			Binding: protocol.Binding{Kind: "rest_resource", Resource: "users"},
			Fields: []protocol.Field{
				{Name: "id", Type: protocol.TypeString, Physical: "id"},
				{Name: "addr", Type: protocol.TypeJSON, Physical: "addr", Shape: "{street, city}"},
			},
		}},
	}
	// Catalog() only checks JSON schema, not source existence.
	if err := Catalog(c); err != nil {
		t.Fatal(err)
	}
}
