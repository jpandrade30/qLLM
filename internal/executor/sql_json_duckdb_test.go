//go:build duckdb

package executor

import (
	"context"
	"testing"

	"qLLM/internal/catalogidx"
	"qLLM/internal/connector"
	"qLLM/internal/connector/def"
	"qLLM/internal/protocol"
	"qLLM/internal/validate"
)

type jsonRecConn struct{ id string }

func (c *jsonRecConn) ID() string                { return c.id }
func (c *jsonRecConn) Type() protocol.SourceType { return protocol.SourcePostgres }
func (c *jsonRecConn) Capabilities() def.Caps {
	return def.Caps{Filter: true, Project: true, Limit: true}
}
func (c *jsonRecConn) Close() error { return nil }
func (c *jsonRecConn) Query(_ context.Context, _ def.PushdownStep) (*protocol.TabularResult, error) {
	return &protocol.TabularResult{
		Columns: []protocol.Column{
			{Name: "id", Type: protocol.TypeString},
			{Name: "price", Type: protocol.TypeNumber},
			{Name: "ts", Type: protocol.TypeTimestamp},
			{Name: "addr", Type: protocol.TypeJSON},
			{Name: "tags", Type: protocol.TypeJSON},
		},
		Rows: [][]any{
			{"1", 12.5, "2026-01-02T03:04:05Z", map[string]any{"city": "SP"}, []any{"a", "b"}},
		},
		RowCount: 1,
	}, nil
}

func TestExecuteSQLJSONRoundTripAndTypes(t *testing.T) {
	preset := &protocol.Preset{
		ProtocolVersion: "0.2.0",
		Project:         "t",
		Limits: protocol.Limits{
			MaxSyncMs: 15000, MaxSourceMs: 12000, DefaultLimit: 100, MaxLimit: 1000, ReadOnly: true,
		},
		Sources: []protocol.Source{{ID: "shop", Type: protocol.SourcePostgres}},
	}
	catalog := &protocol.Catalog{
		ProtocolVersion: "0.2.0",
		Project:         "t",
		Entities: []protocol.Entity{{
			Name:   "items",
			Source: "shop",
			Fields: []protocol.Field{
				{Name: "id", Type: protocol.TypeString, Physical: "id"},
				{Name: "price", Type: protocol.TypeNumber, Physical: "price"},
				{Name: "ts", Type: protocol.TypeTimestamp, Physical: "ts"},
				{Name: "addr", Type: protocol.TypeJSON, Physical: "addr", Shape: "{city}"},
				{Name: "tags", Type: protocol.TypeJSON, Physical: "tags", Shape: "string[]"},
			},
		}},
	}
	idx, err := catalogidx.New(preset, catalog)
	if err != nil {
		t.Fatal(err)
	}
	ex := New(idx, connector.NewRegistry(map[string]def.Connector{"shop": &jsonRecConn{id: "shop"}}), nil)
	resp := ex.ExecuteSQL(context.Background(), &protocol.SQLRequest{
		SQL: "SELECT id, price, ts, addr, tags FROM items LIMIT 10",
	})
	if resp.Error != nil {
		t.Fatalf("%#v", resp.Error)
	}
	if err := validate.QueryResponse(resp); err != nil {
		t.Fatal(err)
	}
	if resp.Result == nil || resp.Result.RowCount != 1 {
		t.Fatalf("result %#v", resp.Result)
	}
	types := map[string]protocol.LogicalType{}
	for _, c := range resp.Result.Columns {
		types[c.Name] = c.Type
	}
	if types["price"] != protocol.TypeNumber {
		t.Fatalf("price type %s", types["price"])
	}
	if types["id"] != protocol.TypeString {
		t.Fatalf("id type %s", types["id"])
	}
	if types["addr"] != protocol.TypeJSON || types["tags"] != protocol.TypeJSON {
		t.Fatalf("json types %#v", types)
	}
	row := resp.Result.Rows[0]
	// column order follows SELECT
	addr, ok := row[3].(map[string]any)
	if !ok || addr["city"] != "SP" {
		t.Fatalf("addr %#v", row[3])
	}
	tags, ok := row[4].([]any)
	if !ok || len(tags) != 2 {
		t.Fatalf("tags %#v", row[4])
	}
}
