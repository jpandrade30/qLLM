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

func balanceEntity() *protocol.Entity {
	return &protocol.Entity{
		Name:    "balance",
		Binding: protocol.Binding{Kind: "rest_resource", Resource: "balance"},
		Fields: []protocol.Field{
			{Name: "user_id", Type: protocol.TypeString, Physical: "user_id", FromFilter: true},
			{Name: "saldo", Type: protocol.TypeNumber, Physical: "saldo"},
		},
	}
}

func TestQueryFromFilterFillsMissingKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("user_id") != "42" {
			t.Fatalf("query %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{"saldo": 5300}})
	}))
	defer srv.Close()

	c := openTest(t, srv.URL, map[string]any{
		"balance": map[string]any{"list": map[string]any{"method": "GET", "path": "/balance"}},
	})
	res, err := c.Query(context.Background(), def.PushdownStep{
		Entity: balanceEntity(),
		Select: []def.SelectItem{{Field: "user_id"}, {Field: "saldo"}},
		Where:  map[string]any{"op": "eq", "field": "user_id", "value": "42"},
		Limit:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.RowCount != 1 {
		t.Fatalf("rows %d", res.RowCount)
	}
	if res.Rows[0][0] != "42" {
		t.Fatalf("user_id %#v", res.Rows[0][0])
	}
	if res.Rows[0][1] != float64(5300) {
		t.Fatalf("saldo %#v", res.Rows[0][1])
	}
}

func TestQueryFromFilterTwoUsersDistinct(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("user_id")
		saldo := 100
		if id == "7" {
			saldo = 200
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{"saldo": saldo}})
	}))
	defer srv.Close()

	c := openTest(t, srv.URL, map[string]any{
		"balance": map[string]any{"list": map[string]any{"method": "GET", "path": "/balance"}},
	})
	seen := map[string]any{}
	for _, uid := range []string{"42", "7"} {
		res, err := c.Query(context.Background(), def.PushdownStep{
			Entity: balanceEntity(),
			Select: []def.SelectItem{{Field: "user_id"}, {Field: "saldo"}},
			Where:  map[string]any{"op": "eq", "field": "user_id", "value": uid},
			Limit:  1,
		})
		if err != nil {
			t.Fatal(err)
		}
		seen[res.Rows[0][0].(string)] = res.Rows[0][1]
	}
	if len(seen) != 2 || seen["42"] != float64(100) || seen["7"] != float64(200) {
		t.Fatalf("groups %#v", seen)
	}
}

func TestQueryFromFilterMissingEq(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{"saldo": 1}})
	}))
	defer srv.Close()

	c := openTest(t, srv.URL, map[string]any{
		"balance": map[string]any{"list": map[string]any{"method": "GET", "path": "/balance"}},
	})
	_, err := c.Query(context.Background(), def.PushdownStep{
		Entity: balanceEntity(),
		Select: []def.SelectItem{{Field: "user_id"}, {Field: "saldo"}},
		Limit:  1,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	pe, ok := err.(*protocol.ProtocolError)
	if !ok || pe.Code != protocol.ErrInvalidIR {
		t.Fatalf("err %#v", err)
	}
}

func TestQueryFromFilterOrDoesNotFill(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{"saldo": 1}})
	}))
	defer srv.Close()

	c := openTest(t, srv.URL, map[string]any{
		"balance": map[string]any{"list": map[string]any{"method": "GET", "path": "/balance"}},
	})
	_, err := c.Query(context.Background(), def.PushdownStep{
		Entity: balanceEntity(),
		Select: []def.SelectItem{{Field: "user_id"}, {Field: "saldo"}},
		Where: map[string]any{
			"op": "or",
			"args": []any{
				map[string]any{"op": "eq", "field": "user_id", "value": "42"},
				map[string]any{"op": "eq", "field": "user_id", "value": "7"},
			},
		},
		Limit: 1,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	pe, ok := err.(*protocol.ProtocolError)
	if !ok || pe.Code != protocol.ErrInvalidIR {
		t.Fatalf("err %#v", err)
	}
}

func TestQueryFromFilterGetByIdBareObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/balance/42" {
			t.Fatalf("path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"saldo": 5300})
	}))
	defer srv.Close()

	c := openTest(t, srv.URL, map[string]any{
		"balance": map[string]any{
			"list":    map[string]any{"method": "GET", "path": "/balance"},
			"getById": map[string]any{"method": "GET", "path": "/balance/{user_id}"},
		},
	})
	res, err := c.Query(context.Background(), def.PushdownStep{
		Entity: balanceEntity(),
		Select: []def.SelectItem{{Field: "user_id"}, {Field: "saldo"}},
		Where:  map[string]any{"op": "eq", "field": "user_id", "value": "42"},
		Limit:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.RowCount != 1 || res.Rows[0][0] != "42" || res.Rows[0][1] != float64(5300) {
		t.Fatalf("row %#v", res.Rows)
	}
}

func TestQueryFromFilterMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{"user_id": "7", "saldo": 5300}})
	}))
	defer srv.Close()

	c := openTest(t, srv.URL, map[string]any{
		"balance": map[string]any{"list": map[string]any{"method": "GET", "path": "/balance"}},
	})
	_, err := c.Query(context.Background(), def.PushdownStep{
		Entity: balanceEntity(),
		Select: []def.SelectItem{{Field: "user_id"}, {Field: "saldo"}},
		Where:  map[string]any{"op": "eq", "field": "user_id", "value": "42"},
		Limit:  1,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	pe, ok := err.(*protocol.ProtocolError)
	if !ok || pe.Code != protocol.ErrSourceError {
		t.Fatalf("err %#v", err)
	}
}

func TestQueryJSONFieldUntouched(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"id":   "1",
			"addr": map[string]any{"city": "SP", "zip": "01000"},
			"tags": []any{"a", "b"},
		}})
	}))
	defer srv.Close()

	ent := &protocol.Entity{
		Name:    "users",
		Binding: protocol.Binding{Kind: "rest_resource", Resource: "users"},
		Fields: []protocol.Field{
			{Name: "id", Type: protocol.TypeString, Physical: "id"},
			{Name: "addr", Type: protocol.TypeJSON, Physical: "addr", Shape: "{city, zip}"},
			{Name: "tags", Type: protocol.TypeJSON, Physical: "tags", Shape: "string[]"},
		},
	}
	c := openTest(t, srv.URL, map[string]any{
		"users": map[string]any{"list": map[string]any{"method": "GET", "path": "/users"}},
	})
	res, err := c.Query(context.Background(), def.PushdownStep{
		Entity: ent,
		Select: []def.SelectItem{{Field: "id"}, {Field: "addr"}, {Field: "tags"}},
		Limit:  10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Columns[1].Type != protocol.TypeJSON || res.Columns[2].Type != protocol.TypeJSON {
		t.Fatalf("cols %#v", res.Columns)
	}
	addr, ok := res.Rows[0][1].(map[string]any)
	if !ok || addr["city"] != "SP" {
		t.Fatalf("addr %#v", res.Rows[0][1])
	}
	tags, ok := res.Rows[0][2].([]any)
	if !ok || len(tags) != 2 {
		t.Fatalf("tags %#v", res.Rows[0][2])
	}
}
