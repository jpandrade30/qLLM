package mcpserver_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"qLLM/internal/catalogidx"
	"qLLM/internal/executor"
	"qLLM/internal/mcpserver"
	"qLLM/internal/protocol"
	"qLLM/internal/querystore"
)

func TestDescriptionsIncludeLoadedCatalogNames(t *testing.T) {
	preset := &protocol.Preset{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Limits:          protocol.Limits{DefaultLimit: 10, MaxLimit: 100, ReadOnly: true},
		Sources:         []protocol.Source{{ID: "crm_pg", Type: protocol.SourcePostgres}},
	}
	catalog := &protocol.Catalog{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Entities: []protocol.Entity{
			{
				Name: "customers", Source: "crm_pg", Description: "CRM customers",
				Fields: []protocol.Field{
					{Name: "id", Type: protocol.TypeString, Description: "Customer primary key"},
					{Name: "email", Type: protocol.TypeString, Description: "Unique email"},
				},
			},
			{
				Name: "invoices", Source: "crm_pg",
				Fields: []protocol.Field{
					{Name: "id", Type: protocol.TypeString},
					{Name: "customer_id", Type: protocol.TypeString, Description: "FK to customers.id"},
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
	d := mcpserver.DescriptionsFor(idx)
	if !strings.Contains(d.ExecuteSQL, "customers") {
		t.Fatalf("sql: %s", d.ExecuteSQL)
	}
	if !strings.Contains(d.ExecuteSQL, "COUNT") {
		t.Fatalf("sql functions missing: %s", d.ExecuteSQL)
	}
	if !strings.Contains(d.ExecuteSQL, "email") {
		t.Fatalf("fields missing: %s", d.ExecuteSQL)
	}
	if !strings.Contains(d.ExecuteSQL, "Customer primary key") {
		t.Fatalf("field description missing: %s", d.ExecuteSQL)
	}
	if !strings.Contains(d.ExecuteSQL, "invoices.customer_id=customers.id") {
		t.Fatalf("relation missing: %s", d.ExecuteSQL)
	}
	emptyIdx, err := catalogidx.New(preset, &protocol.Catalog{ProtocolVersion: protocol.ProtocolVersion, Project: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	empty := mcpserver.DescriptionsFor(emptyIdx)
	if strings.Contains(empty.DescribeCatalog, "invoices") {
		t.Fatal("empty catalog leaked demo entity")
	}
	if !strings.Contains(empty.DescribeCatalog, "none") {
		t.Fatalf("want none marker: %s", empty.DescribeCatalog)
	}
}

func testMCP(t *testing.T, opts mcpserver.HTTPOptions) http.Handler {
	t.Helper()
	preset := &protocol.Preset{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Limits:          protocol.Limits{DefaultLimit: 10, MaxLimit: 100, MaxSyncMs: 1000, ReadOnly: true},
		Sources:         []protocol.Source{{ID: "crm_pg", Type: protocol.SourcePostgres}},
	}
	catalog := &protocol.Catalog{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Entities: []protocol.Entity{{
			Name: "customers", Source: "crm_pg",
			Fields: []protocol.Field{{Name: "id", Type: protocol.TypeString}},
		}},
	}
	idx, err := catalogidx.New(preset, catalog)
	if err != nil {
		t.Fatal(err)
	}
	store := querystore.New(time.Minute)
	exec := executor.New(idx, nil, store)
	return mcpserver.Handler(mcpserver.New(idx, exec, store), opts)
}

func TestMCPHTTPOptionsNoCORSByDefault(t *testing.T) {
	h := testMCP(t, mcpserver.HTTPOptions{})
	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("CORS should be disabled by default")
	}
}

func TestMCPHTTPOptionsCORSAllowlist(t *testing.T) {
	h := testMCP(t, mcpserver.HTTPOptions{
		CORS: protocol.CORSConfig{Origins: []string{"http://ok.example"}},
	})
	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	req.Header.Set("Origin", "http://ok.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://ok.example" {
		t.Fatalf("got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestMCPHTTPUnknownPath(t *testing.T) {
	h := testMCP(t, mcpserver.HTTPOptions{})
	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}
