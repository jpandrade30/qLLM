package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	var out protocol.HowToUseMeResponse
	if e := json.Unmarshal(rec.Body.Bytes(), &out); e != nil {
		t.Fatal(e)
	}
	if len(out.Never) == 0 || out.Where.InvalidExample == nil {
		t.Fatalf("incomplete LLM guide: never=%d where=%+v", len(out.Never), out.Where)
	}
	if out.Project.Name != "demo" || len(out.Project.EntityNames) != 1 {
		t.Fatalf("project meta: %+v", out.Project)
	}
	foundAnd := false
	for _, ex := range out.Examples {
		if w, ok := ex.IR["where"].(map[string]any); ok && w["op"] == "and" {
			foundAnd = true
		}
	}
	if !foundAnd {
		t.Fatal("expected and example in howtouseme")
	}
}
