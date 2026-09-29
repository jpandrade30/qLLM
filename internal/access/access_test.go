package access

import (
	"testing"

	"qLLM/internal/catalogidx"
	"qLLM/internal/protocol"
)

func testIdx(t *testing.T) *catalogidx.Index {
	t.Helper()
	preset := &protocol.Preset{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Limits:          protocol.Limits{DefaultLimit: 10, MaxLimit: 100, ReadOnly: true},
		Sources:         []protocol.Source{{ID: "s", Type: protocol.SourcePostgres}},
	}
	catalog := &protocol.Catalog{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Entities: []protocol.Entity{
			{Name: "customers", Source: "s", Fields: []protocol.Field{{Name: "id", Type: protocol.TypeString}}},
			{Name: "invoices", Source: "s", Fields: []protocol.Field{{Name: "id", Type: protocol.TypeString}}},
		},
	}
	idx, err := catalogidx.New(preset, catalog)
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestExpandKeyEnv(t *testing.T) {
	t.Setenv("QLLM_CRM_AGENT_KEY", "secret-from-env")
	idx := testIdx(t)
	reg, err := Resolve(&protocol.AccessFile{Apps: []protocol.AccessApp{
		{Name: "crm", Key: "${QLLM_CRM_AGENT_KEY}", Tables: []string{"customers"}},
	}}, idx)
	if err != nil {
		t.Fatal(err)
	}
	if reg.LookupBearer("secret-from-env") == nil || reg.LookupBearer("secret-from-env").Name != "crm" {
		t.Fatal("expected env key match")
	}
}

func TestExpandKeyLiteral(t *testing.T) {
	idx := testIdx(t)
	reg, err := Resolve(&protocol.AccessFile{Apps: []protocol.AccessApp{
		{Name: "dev", Key: "dev-only-literal", Tables: []string{"invoices"}},
	}}, idx)
	if err != nil {
		t.Fatal(err)
	}
	if reg.LookupBearer("dev-only-literal") == nil {
		t.Fatal("literal key")
	}
}

func TestExpandKeyBadPlaceholder(t *testing.T) {
	idx := testIdx(t)
	_, err := Resolve(&protocol.AccessFile{Apps: []protocol.AccessApp{
		{Name: "x", Key: "prefix-${FOO}", Tables: []string{"customers"}},
	}}, idx)
	if err == nil || err.Code != protocol.ErrConfigError {
		t.Fatalf("got %v", err)
	}
}

func TestExpandKeyEmptyEnv(t *testing.T) {
	t.Setenv("QLLM_EMPTY_KEY", "")
	idx := testIdx(t)
	_, err := Resolve(&protocol.AccessFile{Apps: []protocol.AccessApp{
		{Name: "x", Key: "${QLLM_EMPTY_KEY}", Tables: []string{"customers"}},
	}}, idx)
	if err == nil {
		t.Fatal("expected empty env error")
	}
}

func TestLookupBearerDifferentLengths(t *testing.T) {
	idx := testIdx(t)
	reg, err := Resolve(&protocol.AccessFile{Apps: []protocol.AccessApp{
		{Name: "crm", Key: "short", Tables: []string{"customers"}},
		{Name: "bill", Key: "much-longer-key", Tables: []string{"invoices"}},
	}}, idx)
	if err != nil {
		t.Fatal(err)
	}
	if a := reg.LookupBearer("short"); a == nil || a.Name != "crm" {
		t.Fatalf("short: %+v", a)
	}
	if a := reg.LookupBearer("much-longer-key"); a == nil || a.Name != "bill" {
		t.Fatalf("long: %+v", a)
	}
	if a := reg.LookupBearer("nope"); a != nil {
		t.Fatalf("miss leaked %s", a.Name)
	}
	if a := reg.LookupBearer(""); a != nil {
		t.Fatalf("empty leaked %s", a.Name)
	}
}

func TestUnknownTable(t *testing.T) {
	idx := testIdx(t)
	_, err := Resolve(&protocol.AccessFile{Apps: []protocol.AccessApp{
		{Name: "x", Key: "k", Tables: []string{"nope"}},
	}}, idx)
	if err == nil {
		t.Fatal("expected unknown table")
	}
}
