package agentguide_test

import (
	"strings"
	"testing"

	"qLLM/internal/agentguide"
	"qLLM/internal/protocol"
)

func TestBuildHasLLMContract(t *testing.T) {
	preset := &protocol.Preset{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Limits:          protocol.Limits{DefaultLimit: 100, MaxLimit: 1000, MaxSyncMs: 15000, ReadOnly: true},
	}
	catalog := &protocol.Catalog{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Entities:        []protocol.Entity{{Name: "customers"}},
	}
	out := agentguide.Build(preset, catalog)
	if len(out.Never) == 0 || len(out.NotSupported) == 0 {
		t.Fatal("missing never/notSupported")
	}
	joined := strings.Join(out.NotSupported, " ")
	if !strings.Contains(joined, "Query IR") || !strings.Contains(joined, "Mutations") {
		t.Fatalf("notSupported should document IR vs SQL: %v", out.NotSupported)
	}
	if out.SQL.LatestVersion != protocol.SQLDialectLatest || len(out.SQL.Examples) == 0 {
		t.Fatalf("sql guide incomplete: %+v", out.SQL)
	}
	if !strings.Contains(out.Purpose, "execute_sql") {
		t.Fatal("purpose should mention execute_sql")
	}
	if len(out.Where.Shapes) == 0 || out.Where.InvalidExample == nil || out.Where.Fix == nil {
		t.Fatalf("where guide incomplete: %+v", out.Where)
	}
	if _, ok := out.Where.InvalidExample["and"]; !ok {
		t.Fatal("invalidExample should show Mongo-style and key")
	}
	if len(out.InvalidExamples) < 4 {
		t.Fatalf("want invalidExamples, got %d", len(out.InvalidExamples))
	}
	foundAnd := false
	for _, ex := range out.Examples {
		if w, ok := ex.IR["where"].(map[string]any); ok {
			if w["op"] == "and" {
				foundAnd = true
			}
		}
	}
	if !foundAnd {
		t.Fatal("expected an example with where.op=and")
	}
	if out.Grammar == "" || len(out.FieldRefRules) == 0 {
		t.Fatal("missing grammar or fieldRefRules")
	}
}
