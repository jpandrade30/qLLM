package mcpserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"qLLM/internal/catalogidx"
	"qLLM/internal/executor"
	"qLLM/internal/mcpserver"
	"qLLM/internal/protocol"
	"qLLM/internal/querystore"
)

func testMCP(t *testing.T) http.Handler {
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
	return mcpserver.Handler(mcpserver.New(idx, exec, store))
}

func TestMCPHTTPOptionsCORS(t *testing.T) {
	h := testMCP(t)
	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("missing CORS origin")
	}
}

func TestMCPHTTPUnknownPath(t *testing.T) {
	h := testMCP(t)
	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}
