package graphql

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"qLLM/internal/connector/def"
	"qLLM/internal/protocol"
)

func TestOpenRejectsMutationDocument(t *testing.T) {
	t.Setenv("QLLM_GQL_URL_TEST", "http://127.0.0.1:9")
	_, err := Open(protocol.Source{
		ID:   "g",
		Type: protocol.SourceGraphQL,
		Connection: map[string]any{
			"baseUrlEnv": "QLLM_GQL_URL_TEST",
		},
		Options: map[string]any{
			"operations": map[string]any{
				"users": map[string]any{
					"document":  `mutation { x }`,
					"itemsPath": "data.x",
				},
			},
		},
	}, OpenOpts{})
	if err == nil {
		t.Fatal("expected config error")
	}
}

func TestQueryHappyPath(t *testing.T) {
	t.Setenv("QLLM_GQL_URL_TEST", "http://example.invalid") // overwritten by server URL below
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		q, _ := body["query"].(string)
		if ValidateDocument(q) != nil {
			t.Errorf("server got forbidden document")
		}
		vars, _ := body["variables"].(map[string]any)
		if vars["id"] != "u1" {
			t.Errorf("vars=%v", vars)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"users": []any{
					map[string]any{"id": "u1", "name": "Ada"},
				},
			},
		})
	}))
	defer srv.Close()
	t.Setenv("QLLM_GQL_URL_TEST", srv.URL)

	c, err := Open(protocol.Source{
		ID:   "g",
		Type: protocol.SourceGraphQL,
		Connection: map[string]any{
			"baseUrlEnv": "QLLM_GQL_URL_TEST",
			"auth":       map[string]any{"type": "none"},
		},
		Options: map[string]any{
			"operations": map[string]any{
				"users": map[string]any{
					"document":  `query Users($id: ID) { users(id: $id) { id name } }`,
					"variables": []any{"id"},
					"itemsPath": "data.users",
				},
			},
		},
	}, OpenOpts{MaxResponseBodyBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	ent := &protocol.Entity{
		Name:    "users",
		Source:  "g",
		Binding: protocol.Binding{Kind: "graphql_operation", Resource: "users"},
		Fields: []protocol.Field{
			{Name: "id", Type: protocol.TypeString, Physical: "id"},
			{Name: "name", Type: protocol.TypeString, Physical: "name"},
		},
	}
	tab, err := c.Query(context.Background(), def.PushdownStep{
		Entity: ent,
		Select: []def.SelectItem{{Field: "id"}, {Field: "name"}},
		Where:  map[string]any{"field": "id", "op": "eq", "value": "u1"},
		Limit:  10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tab.RowCount != 1 || tab.Rows[0][0] != "u1" {
		t.Fatalf("got %+v", tab)
	}
}

func TestQueryMissingVariable(t *testing.T) {
	t.Setenv("QLLM_GQL_URL_TEST2", "http://127.0.0.1:1")
	c, err := Open(protocol.Source{
		ID:   "g",
		Type: protocol.SourceGraphQL,
		Connection: map[string]any{"baseUrlEnv": "QLLM_GQL_URL_TEST2"},
		Options: map[string]any{
			"operations": map[string]any{
				"users": map[string]any{
					"document":  `query { users { id } }`,
					"variables": []any{"id"},
					"itemsPath": "data.users",
				},
			},
		},
	}, OpenOpts{})
	if err != nil {
		t.Fatal(err)
	}
	ent := &protocol.Entity{
		Name:    "users",
		Binding: protocol.Binding{Kind: "graphql_operation", Resource: "users"},
		Fields:  []protocol.Field{{Name: "id", Type: protocol.TypeString, Physical: "id"}},
	}
	_, err = c.Query(context.Background(), def.PushdownStep{
		Entity: ent,
		Select: []def.SelectItem{{Field: "id"}},
		Limit:  5,
	})
	if err == nil {
		t.Fatal("expected UNSUPPORTED for missing variable")
	}
}
