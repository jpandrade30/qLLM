package validate_test

import (
	"strings"
	"testing"

	"qLLM/internal/protocol"
	"qLLM/internal/validate"
)

func sampleBundle(t *testing.T) (*protocol.Preset, *protocol.Catalog) {
	t.Helper()
	preset := &protocol.Preset{
		ProtocolVersion: "0.1.0",
		Project:         "test",
		Limits: protocol.Limits{
			MaxSyncMs: 15000, MaxSourceMs: 12000, DefaultLimit: 100, MaxLimit: 1000, ReadOnly: true,
		},
		Sources: []protocol.Source{
			{ID: "crm_pg", Type: protocol.SourcePostgres, Connection: map[string]any{
				"hostEnv": "H", "port": 5432, "database": "crm", "userEnv": "U", "passwordEnv": "P",
			}},
			{ID: "billing_mysql", Type: protocol.SourceMySQL, Connection: map[string]any{
				"hostEnv": "H", "port": 3306, "database": "billing", "userEnv": "U", "passwordEnv": "P",
			}},
		},
	}
	catalog := &protocol.Catalog{
		ProtocolVersion: "0.1.0",
		Project:         "test",
		Entities: []protocol.Entity{
			{
				Name: "customers", Source: "crm_pg",
				Binding: protocol.Binding{Kind: "table", Schema: "public", Table: "customers"},
				Fields: []protocol.Field{
					{Name: "id", Type: protocol.TypeString, Physical: "id"},
					{Name: "email", Type: protocol.TypeString, Physical: "email"},
				},
			},
			{
				Name: "invoices", Source: "billing_mysql",
				Binding: protocol.Binding{Kind: "table", Schema: "billing", Table: "invoices"},
				Fields: []protocol.Field{
					{Name: "id", Type: protocol.TypeString, Physical: "id"},
					{Name: "customer_id", Type: protocol.TypeString, Physical: "customer_id"},
					{Name: "total", Type: protocol.TypeNumber, Physical: "total_cents"},
					{Name: "status", Type: protocol.TypeString, Physical: "status"},
				},
			},
		},
	}
	return preset, catalog
}

func TestValidateQueryOK(t *testing.T) {
	p, c := sampleBundle(t)
	idx, err := validate.Bundle(p, c)
	if err != nil {
		t.Fatal(err)
	}
	lim := 50
	q := &protocol.QueryIR{
		From:    "invoices",
		Select:  []any{"customer_id", map[string]any{"agg": "sum", "field": "total", "as": "revenue"}},
		GroupBy: []string{"customer_id"},
		Limit:   &lim,
	}
	if err := validate.Query(idx, q); err != nil {
		t.Fatal(err)
	}
}

func TestAmbiguousField(t *testing.T) {
	p, c := sampleBundle(t)
	idx, err := validate.Bundle(p, c)
	if err != nil {
		t.Fatal(err)
	}
	lim := 10
	q := &protocol.QueryIR{
		From: "invoices", As: "inv",
		Joins: []protocol.Join{{
			Type: "left", From: "customers", As: "c",
			On: []protocol.JoinOn{{Left: "inv.customer_id", Right: "c.id"}},
		}},
		Select: []any{"email"},
		Limit:  &lim,
	}
	err2 := validate.Query(idx, q)
	if err2 == nil || err2.Code != protocol.ErrAmbiguousField {
		t.Fatalf("expected AMBIGUOUS_FIELD, got %v", err2)
	}
}

func TestQualifiedJoinOK(t *testing.T) {
	p, c := sampleBundle(t)
	idx, err := validate.Bundle(p, c)
	if err != nil {
		t.Fatal(err)
	}
	lim := 10
	q := &protocol.QueryIR{
		From: "invoices", As: "inv",
		Joins: []protocol.Join{{
			Type: "left", From: "customers", As: "c",
			On: []protocol.JoinOn{{Left: "inv.customer_id", Right: "c.id"}},
		}},
		Select: []any{"inv.id", "c.email", "inv.total"},
		Limit:  &lim,
	}
	if err := validate.Query(idx, q); err != nil {
		t.Fatal(err)
	}
}

func TestLLMWhereMongoShapeRejected(t *testing.T) {
	p, c := sampleBundle(t)
	idx, err := validate.Bundle(p, c)
	if err != nil {
		t.Fatal(err)
	}
	lim := 10
	q := &protocol.QueryIR{
		From:   "invoices",
		Select: []any{"id"},
		Where: map[string]any{
			"and": []any{
				map[string]any{"field": "status", "op": "eq", "value": "paid"},
			},
		},
		Limit: &lim,
	}
	err2 := validate.Query(idx, q)
	if err2 == nil || err2.Code != protocol.ErrInvalidIR {
		t.Fatalf("expected INVALID_IR, got %v", err2)
	}
	if !strings.Contains(err2.Message, `"op":"and"`) {
		t.Fatalf("expected prescriptive message, got %q", err2.Message)
	}
}

func TestLLMWhereAndArgsOK(t *testing.T) {
	p, c := sampleBundle(t)
	idx, err := validate.Bundle(p, c)
	if err != nil {
		t.Fatal(err)
	}
	lim := 10
	q := &protocol.QueryIR{
		From:   "invoices",
		Select: []any{"id"},
		Where: map[string]any{
			"op": "and",
			"args": []any{
				map[string]any{"field": "status", "op": "eq", "value": "paid"},
				map[string]any{"field": "total", "op": "gte", "value": 10},
			},
		},
		Limit: &lim,
	}
	if err := validate.Query(idx, q); err != nil {
		t.Fatal(err)
	}
}

func TestLLMWhereSQLOpRejected(t *testing.T) {
	p, c := sampleBundle(t)
	idx, err := validate.Bundle(p, c)
	if err != nil {
		t.Fatal(err)
	}
	lim := 10
	q := &protocol.QueryIR{
		From:   "invoices",
		Select: []any{"id"},
		Where:  map[string]any{"field": "status", "op": "=", "value": "paid"},
		Limit:  &lim,
	}
	err2 := validate.Query(idx, q)
	if err2 == nil || err2.Code != protocol.ErrInvalidIR {
		t.Fatalf("expected INVALID_IR, got %v", err2)
	}
	if !strings.Contains(err2.Message, "eq") {
		t.Fatalf("expected eq hint, got %q", err2.Message)
	}
}

func TestLLMBarePlusAggNeedsGroupBy(t *testing.T) {
	p, c := sampleBundle(t)
	idx, err := validate.Bundle(p, c)
	if err != nil {
		t.Fatal(err)
	}
	lim := 10
	q := &protocol.QueryIR{
		From: "invoices",
		Select: []any{
			"status",
			map[string]any{"agg": "count", "as": "n"},
		},
		Limit: &lim,
	}
	err2 := validate.Query(idx, q)
	if err2 == nil || err2.Code != protocol.ErrInvalidIR {
		t.Fatalf("expected INVALID_IR, got %v", err2)
	}
	if !strings.Contains(err2.Message, "groupBy") {
		t.Fatalf("expected groupBy message, got %q", err2.Message)
	}
}

func TestPresetExperimentalSourceTypes(t *testing.T) {
	p := &protocol.Preset{
		ProtocolVersion: "0.2.0",
		Project:         "exp",
		Limits: protocol.Limits{
			MaxSyncMs: 15000, MaxSourceMs: 12000, DefaultLimit: 10, MaxLimit: 100, ReadOnly: true,
		},
		Sources: []protocol.Source{
			{ID: "local_sqlite", Type: protocol.SourceSQLite, Connection: map[string]any{"pathEnv": "QLLM_SQLITE_PATH"}},
			{ID: "items_ddb", Type: protocol.SourceDynamoDB, Connection: map[string]any{"region": "us-east-1"}},
			{ID: "ks", Type: protocol.SourceKSQL, Connection: map[string]any{"baseUrlEnv": "QLLM_KSQL_URL"}},
		},
	}
	if err := validate.Preset(p); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogAccessPath(t *testing.T) {
	c := &protocol.Catalog{
		ProtocolVersion: "0.2.0",
		Project:         "exp",
		Entities: []protocol.Entity{{
			Name: "items", Source: "items_ddb",
			Binding: protocol.Binding{
				Kind: "table", Schema: "main", Table: "items",
				AccessPath: protocol.AccessPath{PK: []string{"pk"}, SK: "sk"},
			},
			Fields: []protocol.Field{{Name: "pk", Type: protocol.TypeString, Physical: "pk"}},
		}},
	}
	if err := validate.Catalog(c); err != nil {
		t.Fatal(err)
	}
}
