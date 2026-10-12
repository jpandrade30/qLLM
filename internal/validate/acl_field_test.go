package validate

import (
	"testing"

	"qLLM/internal/catalogidx"
	"qLLM/internal/protocol"
)

type stubACL struct {
	tables map[string]struct{}
	fields map[string]map[string]struct{}
}

func (s stubACL) Allows(entity string) bool {
	_, ok := s.tables[entity]
	return ok
}

func (s stubACL) AllowsField(entity, field string) bool {
	if !s.Allows(entity) {
		return false
	}
	set, ok := s.fields[entity]
	if !ok {
		return true
	}
	_, allowed := set[field]
	return allowed
}

func TestEnforceACLFieldForbidden(t *testing.T) {
	preset := &protocol.Preset{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Limits:          protocol.Limits{DefaultLimit: 10, MaxLimit: 100, ReadOnly: true},
		Sources:         []protocol.Source{{ID: "s", Type: protocol.SourcePostgres}},
	}
	catalog := &protocol.Catalog{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Entities: []protocol.Entity{{
			Name: "customers", Source: "s", PrimaryKey: []string{"id"},
			Fields: []protocol.Field{
				{Name: "id", Type: protocol.TypeString},
				{Name: "email", Type: protocol.TypeString},
				{Name: "phone", Type: protocol.TypeString},
			},
		}},
	}
	idx, err := catalogidx.New(preset, catalog)
	if err != nil {
		t.Fatal(err)
	}
	app := stubACL{
		tables: map[string]struct{}{"customers": {}},
		fields: map[string]map[string]struct{}{"customers": {"id": {}, "email": {}}},
	}
	lim := 10
	q := &protocol.QueryIR{
		ProtocolVersion: protocol.ProtocolVersion,
		From:            "customers",
		Select:          []any{"phone"},
		Limit:           &lim,
	}
	got := EnforceACL(idx, q, app)
	if got == nil || got.Code != protocol.ErrForbidden {
		t.Fatalf("want FORBIDDEN phone, got %v", got)
	}
	q.Select = []any{"email"}
	if err := EnforceACL(idx, q, app); err != nil {
		t.Fatalf("email should pass: %v", err)
	}
}
