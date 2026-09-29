package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"qLLM/internal/catalogidx"
	"qLLM/internal/protocol"
)

func TestHowToUseMe(t *testing.T) {
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
				Name:   "customers",
				Source: "crm_pg",
				Fields: []protocol.Field{{Name: "id", Type: protocol.TypeString}},
			},
		},
	}
	idx, err := catalogidx.New(preset, catalog)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Idx: idx}
	req := httptest.NewRequest(http.MethodGet, "/v1/howtouseme", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(strings.ToLower(rec.Body.String()), "graphql") {
		t.Fatal("howtouseme must not mention GraphQL")
	}
	var out protocol.HowToUseMeResponse
	if e := json.Unmarshal(rec.Body.Bytes(), &out); e != nil {
		t.Fatal(e)
	}
	if len(out.Never) == 0 || out.SQL.LatestVersion == "" {
		t.Fatalf("incomplete LLM guide: never=%d sql=%+v", len(out.Never), out.SQL)
	}
	if out.Project.Name != "demo" || len(out.Project.EntityNames) != 1 {
		t.Fatalf("project meta: %+v", out.Project)
	}
	if strings.Contains(strings.ToLower(rec.Body.String()), "execute_query") {
		t.Fatal("howtouseme must not mention execute_query")
	}
	if strings.Contains(rec.Body.String(), "/v1/queries") {
		t.Fatal("howtouseme must not advertise /v1/queries")
	}
}
