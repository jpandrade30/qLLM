package cataloggen

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"qLLM/internal/protocol"
)

func TestMapSQLType(t *testing.T) {
	cases := map[string]protocol.LogicalType{
		"integer":                  protocol.TypeNumber,
		"bigint":                   protocol.TypeNumber,
		"numeric":                  protocol.TypeNumber,
		"boolean":                  protocol.TypeBoolean,
		"timestamp with time zone": protocol.TypeTimestamp,
		"date":                     protocol.TypeTimestamp,
		"jsonb":                    protocol.TypeJSON,
		"character varying":        protocol.TypeString,
		"text":                     protocol.TypeString,
	}
	for in, want := range cases {
		if got := MapSQLType(in); got != want {
			t.Errorf("%s: got %s want %s", in, got, want)
		}
	}
}

func TestEntitiesFromColumnsDisambiguatesSchema(t *testing.T) {
	rows := []ColumnRow{
		{Schema: "public", Table: "users", Column: "id", DataType: "uuid", PK: true},
		{Schema: "public", Table: "users", Column: "email", DataType: "text"},
		{Schema: "other", Table: "users", Column: "id", DataType: "int", PK: true},
	}
	ents := EntitiesFromColumns("crm_pg", rows)
	if len(ents) != 2 {
		t.Fatalf("len=%d", len(ents))
	}
	names := map[string]bool{}
	for _, e := range ents {
		names[e.Name] = true
		if e.Source != "crm_pg" || e.Binding.Kind != "table" {
			t.Fatalf("bad entity %+v", e)
		}
	}
	if !names["public_users"] || !names["other_users"] {
		t.Fatalf("names=%v", names)
	}
}

func TestMergeSourceEntities(t *testing.T) {
	existing := &protocol.Catalog{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "p",
		Entities: []protocol.Entity{
			{Name: "keep", Source: "other"},
			{Name: "old", Source: "crm_pg"},
		},
	}
	got := MergeSourceEntities(existing, "crm_pg", []protocol.Entity{{Name: "new", Source: "crm_pg"}})
	if len(got.Entities) != 2 {
		t.Fatalf("len=%d", len(got.Entities))
	}
	if got.Entities[0].Name != "keep" || got.Entities[1].Name != "new" {
		t.Fatalf("%+v", got.Entities)
	}
}

func TestFromOpenAPIListAndGetById(t *testing.T) {
	raw := []byte(`
openapi: "3.0.0"
paths:
  /users:
    get:
      parameters:
        - in: query
          name: email
      responses:
        "200":
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/User"
  /users/{id}:
    get:
      responses:
        "200":
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
  /items/{id}:
    get: {}
components:
  schemas:
    User:
      type: object
      properties:
        id: { type: string }
        email: { type: string }
`)
	out, err := FromOpenAPI(raw, "legacy_api")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Entities) != 1 || out.Entities[0].Name != "users" {
		t.Fatalf("entities=%+v", out.Entities)
	}
	if out.Entities[0].Binding.Kind != "rest_resource" || out.Entities[0].Binding.Resource != "users" {
		t.Fatalf("binding=%+v", out.Entities[0].Binding)
	}
	res, _ := out.Resources["users"].(map[string]any)
	if res == nil {
		t.Fatal("missing users resource")
	}
	list, _ := res["list"].(map[string]any)
	if list["path"] != "/users" {
		t.Fatalf("list=%v", list)
	}
	if _, ok := res["getById"]; !ok {
		t.Fatal("expected getById")
	}
	foundEmail := false
	for _, f := range out.Entities[0].Fields {
		if f.Name == "email" {
			foundEmail = true
		}
	}
	if !foundEmail {
		t.Fatalf("fields=%v", out.Entities[0].Fields)
	}
}

func TestFromOpenAPIFixtureFile(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "openapi", "minimal.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := FromOpenAPI(raw, "legacy_api")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Entities) != 1 || out.Entities[0].Name != "users" {
		t.Fatalf("%+v", out.Entities)
	}
}

func TestIntrospectSQLRejectsREST(t *testing.T) {
	p := &protocol.Preset{
		Sources: []protocol.Source{{ID: "legacy_api", Type: protocol.SourceREST}},
	}
	_, err := IntrospectSQL(context.Background(), p, "legacy_api", 1000)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEncodeCatalogQuotesProtocolVersion(t *testing.T) {
	raw, err := EncodeCatalog(&protocol.Catalog{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "p",
		Entities:        []protocol.Entity{{Name: "t", Source: "s", Binding: protocol.Binding{Kind: "table", Table: "t"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `protocolVersion: "0.1.0"`) {
		t.Fatalf("want quoted semver:\n%s", raw)
	}
}
