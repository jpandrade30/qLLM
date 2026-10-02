package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"qLLM/internal/connector/def"
	"qLLM/internal/protocol"
)

func testEntity() *protocol.Entity {
	return &protocol.Entity{
		Name:    "users",
		Binding: protocol.Binding{Kind: "rest_resource", Resource: "users"},
		Fields: []protocol.Field{
			{Name: "id", Type: protocol.TypeString, Physical: "id"},
			{Name: "email", Type: protocol.TypeString, Physical: "email"},
		},
	}
}

func openTest(t *testing.T, base string, resources map[string]any) *Connector {
	t.Helper()
	t.Setenv("QLLM_REST_URL", base)
	c, err := Open(protocol.Source{
		ID:         "legacy_api",
		Type:       protocol.SourceREST,
		Connection: map[string]any{"baseUrlEnv": "QLLM_REST_URL"},
		Options:    map[string]any{"resources": resources},
	}, OpenOpts{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestQueryListItemsKeyAndPages(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if r.URL.Path != "/users" {
			t.Fatalf("path %s", r.URL.Path)
		}
		off := r.URL.Query().Get("offset")
		var records []map[string]any
		if off == "" || off == "0" {
			records = []map[string]any{{"id": "1", "email": "a@x"}, {"id": "2", "email": "b@x"}}
		} else {
			records = []map[string]any{{"id": "3", "email": "c@x"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"records": records})
	}))
	defer srv.Close()

	c := openTest(t, srv.URL, map[string]any{
		"users": map[string]any{
			"list": map[string]any{
				"method": "GET", "path": "/users",
				"itemsKey": "records", "maxPages": 2, "pageSize": 2,
			},
		},
	})
	res, err := c.Query(context.Background(), def.PushdownStep{
		Entity: testEntity(),
		Select: []def.SelectItem{{Field: "id"}, {Field: "email"}},
		Limit:  10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("pages %d", n)
	}
	if res.RowCount != 3 {
		t.Fatalf("rows %d", res.RowCount)
	}
}

func TestQueryGetById(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "42", "email": "z@x"})
	}))
	defer srv.Close()

	c := openTest(t, srv.URL, map[string]any{
		"users": map[string]any{
			"list":    map[string]any{"method": "GET", "path": "/users"},
			"getById": map[string]any{"method": "GET", "path": "/users/{id}"},
		},
	})
	res, err := c.Query(context.Background(), def.PushdownStep{
		Entity: testEntity(),
		Select: []def.SelectItem{{Field: "id"}, {Field: "email"}},
		Where:  map[string]any{"op": "eq", "field": "id", "value": "42"},
		Limit:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/users/42" {
		t.Fatalf("path %s", gotPath)
	}
	if res.RowCount != 1 || res.Rows[0][1] != "z@x" {
		t.Fatalf("row %#v", res.Rows)
	}
}

func TestFillPathMissingParamFallsBackToList(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "1", "email": "a@x"}})
	}))
	defer srv.Close()

	c := openTest(t, srv.URL, map[string]any{
		"users": map[string]any{
			"list":    map[string]any{"method": "GET", "path": "/users"},
			"getById": map[string]any{"method": "GET", "path": "/users/{id}"},
		},
	})
	_, err := c.Query(context.Background(), def.PushdownStep{
		Entity: testEntity(),
		Select: []def.SelectItem{{Field: "email"}},
		Where:  map[string]any{"op": "eq", "field": "email", "value": "a@x"},
		Limit:  5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/users" {
		t.Fatalf("path %s", gotPath)
	}
}
