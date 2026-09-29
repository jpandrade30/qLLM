package agentguide_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"qLLM/internal/agentguide"
	"qLLM/internal/protocol"
)

func TestBuildSQLGuideOnly(t *testing.T) {
	preset := &protocol.Preset{
		Limits: protocol.Limits{DefaultLimit: 100, MaxLimit: 1000, MaxSyncMs: 15000, ReadOnly: true},
	}
	catalog := &protocol.Catalog{
		Project: "demo",
		Entities: []protocol.Entity{
			{Name: "customers", Fields: []protocol.Field{{Name: "id"}, {Name: "email"}}},
			{Name: "addresses", Fields: []protocol.Field{{Name: "id"}, {Name: "customer_id"}},
				Relations: []protocol.Relation{{To: "customers", On: [][]string{{"customer_id", "id"}}}},
			},
		},
	}
	out := agentguide.Build(preset, catalog)
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ToLower(string(raw))
	if strings.Contains(s, "graphql") {
		t.Fatal("must not mention graphql")
	}
	if strings.Contains(s, "execute_query") {
		t.Fatal("must not mention execute_query")
	}
	if strings.Contains(s, "/v1/queries") {
		t.Fatal("must not advertise /v1/queries to the agent")
	}
	if strings.Contains(s, `"queryir"`) || strings.Contains(s, `"ir":`) {
		t.Fatalf("must not ship Query IR docs: %s", raw)
	}
	if out.SQL.LatestVersion != protocol.SQLDialectLatest || len(out.SQL.Examples) == 0 {
		t.Fatalf("sql guide incomplete: %+v", out.SQL)
	}
	if !strings.Contains(out.Purpose, "execute_sql") {
		t.Fatal("purpose should mention execute_sql")
	}
	join := ""
	for _, ex := range out.SQL.Examples {
		if strings.Contains(ex.SQL, "JOIN") {
			join = ex.SQL
		}
	}
	if join == "" || !strings.Contains(join, "customer_id") {
		t.Fatalf("join example should use catalog relation, got %q", join)
	}
}

func TestBuildExamplesUseLoadedEntitiesOnly(t *testing.T) {
	preset := &protocol.Preset{Limits: protocol.Limits{DefaultLimit: 10, MaxLimit: 100}}
	catalog := &protocol.Catalog{Project: "p", Entities: []protocol.Entity{{Name: "widgets"}}}
	out := agentguide.Build(preset, catalog)
	for _, ex := range out.SQL.Examples {
		if strings.Contains(ex.SQL, "invoices") {
			t.Fatalf("demo table leaked: %s", ex.SQL)
		}
		if !strings.Contains(ex.SQL, "widgets") {
			t.Fatalf("expected widgets in %s", ex.SQL)
		}
	}
	empty := agentguide.Build(preset, &protocol.Catalog{Project: "p"})
	if len(empty.SQL.Examples) != 0 {
		t.Fatal("empty catalog must not invent demo SQL examples")
	}
}

func TestBuildHowToOmitsGraphQL(t *testing.T) {
	preset := &protocol.Preset{Limits: protocol.Limits{DefaultLimit: 10, MaxLimit: 100}}
	catalog := &protocol.Catalog{Project: "p", Entities: []protocol.Entity{{Name: "customers"}}}
	out := agentguide.Build(preset, catalog)
	blob := strings.ToLower(fmt.Sprintf("%v%v%v%v", out.Never, out.NotSupported, out.Purpose, out.Workflow))
	if strings.Contains(blob, "graphql") {
		t.Fatal("how_to_use_me must not mention GraphQL")
	}
}
