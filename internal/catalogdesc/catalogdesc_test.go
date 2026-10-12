package catalogdesc

import (
	"strings"
	"testing"

	"qLLM/internal/catalogidx"
	"qLLM/internal/protocol"
)

func sampleCatalog() *protocol.Catalog {
	return &protocol.Catalog{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Entities: []protocol.Entity{
			{
				Name: "customers", Source: "crm_pg", PrimaryKey: []string{"id"},
				Fields: []protocol.Field{
					{Name: "id", Type: protocol.TypeString},
					{Name: "email", Type: protocol.TypeString},
				},
			},
			{
				Name: "invoices", Source: "billing",
				Fields: []protocol.Field{
					{Name: "id", Type: protocol.TypeString},
					{Name: "customer_id", Type: protocol.TypeString},
					{Name: "total", Type: protocol.TypeNumber},
				},
				Relations: []protocol.Relation{{
					Name: "customer", To: "customers", Type: "many_to_one",
					On: [][]string{{"customer_id", "id"}},
				}},
			},
			{
				Name: "orphans", Source: "crm_pg",
				Fields: []protocol.Field{{Name: "id", Type: protocol.TypeString}},
				Relations: []protocol.Relation{{
					Name: "ghost", To: "missing_table", Type: "many_to_one",
					On: [][]string{{"id", "id"}},
				}},
			},
			{
				Name: "bad_join", Source: "crm_pg",
				Fields: []protocol.Field{{Name: "id", Type: protocol.TypeString}},
				Relations: []protocol.Relation{{
					Name: "cust", To: "customers", Type: "many_to_one",
					On: [][]string{{"nope", "id"}},
				}},
			},
		},
	}
}

func TestFilterCatalogHappy(t *testing.T) {
	got, err := FilterCatalog(sampleCatalog(), []string{"invoices", "customers"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entities) != 2 || got.Entities[0].Name != "invoices" || got.Entities[1].Name != "customers" {
		t.Fatalf("order/names: %+v", got.Entities)
	}
}

func TestFilterCatalogUnknown(t *testing.T) {
	_, err := FilterCatalog(sampleCatalog(), []string{"customers", "nope"})
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("got %v", err)
	}
}

func TestValidateSliceOK(t *testing.T) {
	sliced, err := FilterCatalog(sampleCatalog(), []string{"customers", "invoices"})
	if err != nil {
		t.Fatal(err)
	}
	issues := ValidateSlice(sliced)
	if len(issues) != 0 {
		t.Fatalf("issues=%+v", issues)
	}
}

func TestValidateSliceOutsideWarning(t *testing.T) {
	sliced, err := FilterCatalog(sampleCatalog(), []string{"orphans"})
	if err != nil {
		t.Fatal(err)
	}
	issues := ValidateSlice(sliced)
	if !HasWarnings(issues) || HasErrors(issues) {
		t.Fatalf("want warning only: %+v", issues)
	}
	if issues[0].Code != "relation_outside_slice" {
		t.Fatalf("code=%s", issues[0].Code)
	}
}

func TestValidateSliceMissingLocalField(t *testing.T) {
	sliced, err := FilterCatalog(sampleCatalog(), []string{"bad_join", "customers"})
	if err != nil {
		t.Fatal(err)
	}
	issues := ValidateSlice(sliced)
	if !HasErrors(issues) {
		t.Fatalf("want error: %+v", issues)
	}
	found := false
	for _, i := range issues {
		if i.Code == "relation_missing_local_field" {
			found = true
		}
	}
	if !found {
		t.Fatalf("issues=%+v", issues)
	}
}

func TestDescriptionsForFullCatalog(t *testing.T) {
	preset := &protocol.Preset{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Limits:          protocol.Limits{DefaultLimit: 10, MaxLimit: 100, MaxSyncMs: 1000, ReadOnly: true},
		Sources: []protocol.Source{
			{ID: "crm_pg", Type: protocol.SourcePostgres},
			{ID: "billing", Type: protocol.SourceMySQL},
		},
	}
	idx, err := catalogidx.New(preset, sampleCatalog())
	if err != nil {
		t.Fatal(err)
	}
	d := DescriptionsFor(idx)
	if !strings.Contains(d.ExecuteSQL, "customers") || !strings.Contains(d.ExecuteSQL, "invoices") {
		t.Fatalf("full catalog missing entities: %s", d.ExecuteSQL)
	}
	if !strings.Contains(d.ExecuteSQL, "customer_id") {
		t.Fatalf("field missing: %s", d.ExecuteSQL)
	}
	limited := DescriptionsForAllow(idx, &catalogidx.ACL{Tables: map[string]struct{}{"customers": {}}})
	if !strings.Contains(limited.ExecuteSQL, "customers") {
		t.Fatalf("limited missing customers: %s", limited.ExecuteSQL)
	}
	if strings.Contains(limited.ExecuteSQL, "invoices") {
		t.Fatalf("limited leaked invoices: %s", limited.ExecuteSQL)
	}
	if strings.Contains(limited.HowToUseMe, "invoices") || strings.Contains(limited.DescribeCatalog, "invoices") {
		t.Fatal("limited how/describe leaked invoices")
	}
	fieldLim := DescriptionsForAllow(idx, &catalogidx.ACL{
		Tables: map[string]struct{}{"customers": {}},
		Fields: map[string]map[string]struct{}{"customers": {"id": {}}},
	})
	if !strings.Contains(fieldLim.ExecuteSQL, "id") {
		t.Fatalf("field allow missing id: %s", fieldLim.ExecuteSQL)
	}
	if strings.Contains(fieldLim.ExecuteSQL, "email") {
		t.Fatalf("field allow leaked email: %s", fieldLim.ExecuteSQL)
	}
}

func TestExecuteSQLDescriptionSlice(t *testing.T) {
	sliced, err := FilterCatalog(sampleCatalog(), []string{"customers"})
	if err != nil {
		t.Fatal(err)
	}
	preset := &protocol.Preset{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Limits:          protocol.Limits{DefaultLimit: 10, MaxLimit: 100, MaxSyncMs: 1000, ReadOnly: true},
		Sources:         []protocol.Source{{ID: "crm_pg", Type: protocol.SourcePostgres}},
	}
	idx, perr := catalogidx.New(preset, sliced)
	if perr != nil {
		t.Fatal(perr)
	}
	desc := ExecuteSQLDescription(idx)
	if !strings.Contains(desc, "customers") {
		t.Fatal(desc)
	}
	if strings.Contains(desc, "invoices") {
		t.Fatalf("slice leaked invoices: %s", desc)
	}
}
