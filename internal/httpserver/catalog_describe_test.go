package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"qLLM/internal/catalogidx"
	"qLLM/internal/protocol"
)

func TestCatalogDescribe(t *testing.T) {
	preset := &protocol.Preset{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Limits:          protocol.Limits{DefaultLimit: 100, MaxLimit: 1000, MaxSyncMs: 15000, ReadOnly: true},
		Sources:         []protocol.Source{{ID: "crm_pg", Type: protocol.SourcePostgres}},
	}
	catalog := &protocol.Catalog{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Entities: []protocol.Entity{
			{
				Name: "customers", Source: "crm_pg",
				Fields: []protocol.Field{{Name: "id", Type: protocol.TypeString}},
			},
			{
				Name: "addresses", Source: "crm_pg",
				Fields: []protocol.Field{
					{Name: "id", Type: protocol.TypeString},
					{Name: "customer_id", Type: protocol.TypeString},
				},
				Relations: []protocol.Relation{{
					Name: "customer", To: "customers", Type: "many_to_one",
					On: [][]string{{"customer_id", "id"}},
				}},
			},
		},
	}
	idx, err := catalogidx.New(preset, catalog)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Idx: idx}

	body, _ := json.Marshal(protocol.CatalogDescribeRequest{Tables: []string{"customers", "addresses"}})
	req := httptest.NewRequest(http.MethodPost, "/v1/catalog/describe", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var out protocol.CatalogDescribeResponse
	if e := json.Unmarshal(rec.Body.Bytes(), &out); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out.Description, "customers") || !strings.Contains(out.Description, "addresses") {
		t.Fatalf("description=%s", out.Description)
	}
	if len(out.Entities) != 2 {
		t.Fatalf("entities=%v", out.Entities)
	}

	bad, _ := json.Marshal(protocol.CatalogDescribeRequest{Tables: []string{"nope"}})
	req2 := httptest.NewRequest(http.MethodPost, "/v1/catalog/describe", bytes.NewReader(bad))
	rec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec2, req2)
	if rec2.Code == http.StatusOK {
		t.Fatalf("want error for unknown table: %s", rec2.Body.String())
	}
}
