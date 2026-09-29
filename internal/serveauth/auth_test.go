package serveauth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"qLLM/internal/access"
	"qLLM/internal/appctx"
	"qLLM/internal/catalogidx"
	"qLLM/internal/protocol"
)

func TestMiddlewarePass(t *testing.T) {
	h := Middleware("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/catalog", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestMiddlewareReject(t *testing.T) {
	h := Middleware("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/catalog", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestMiddlewareHealthExempt(t *testing.T) {
	h := Middleware("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestCORSDisabled(t *testing.T) {
	h := CORS(protocol.CORSConfig{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("expected no CORS when origins empty")
	}
}

func TestCORSAllowlist(t *testing.T) {
	h := CORS(protocol.CORSConfig{Origins: []string{"http://ok.example"}},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Origin", "http://ok.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://ok.example" {
		t.Fatalf("got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestAppsMiddleware(t *testing.T) {
	preset := &protocol.Preset{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Limits:          protocol.Limits{DefaultLimit: 10, MaxLimit: 100, ReadOnly: true},
		Sources:         []protocol.Source{{ID: "s", Type: protocol.SourcePostgres}},
	}
	catalog := &protocol.Catalog{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Entities:        []protocol.Entity{{Name: "customers", Source: "s", Fields: []protocol.Field{{Name: "id", Type: protocol.TypeString}}}},
	}
	idx, err := catalogidx.New(preset, catalog)
	if err != nil {
		t.Fatal(err)
	}
	reg, aerr := access.Resolve(&protocol.AccessFile{Apps: []protocol.AccessApp{
		{Name: "crm", Key: "secret-app", Tables: []string{"customers"}},
	}}, idx)
	if aerr != nil {
		t.Fatal(aerr)
	}
	h := AppsMiddleware(reg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app := appctx.App(r.Context())
		if app == nil || app.Name != "crm" {
			t.Errorf("app=%v", app)
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/catalog", nil)
	req.Header.Set("Authorization", "Bearer secret-app")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
