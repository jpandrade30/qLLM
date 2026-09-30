package executor

import (
	"strings"
	"testing"

	"qLLM/internal/protocol"
)

func TestFormatExecuteSQLLogMultiline(t *testing.T) {
	got := formatExecuteSQLLog(
		&protocol.SQLRequest{SQL: "SELECT 1\nFROM vehicles\nLIMIT 5"},
		&protocol.QueryResponse{
			QueryID: "qid-1",
			Status:  protocol.StatusSucceeded,
			Result:  &protocol.TabularResult{Rows: [][]any{{1}, {2}}},
			Meta:    &protocol.QueryMeta{ElapsedMs: 50, App: "fleet-dispatcher"},
		},
	)
	if strings.Contains(got, `\n`) {
		t.Fatal("sql must not be slog-escaped")
	}
	if !strings.Contains(got, "FROM vehicles") {
		t.Fatalf("missing sql body:\n%s", got)
	}
	if !strings.Contains(got, "status    succeeded") || !strings.Contains(got, "rows      2") {
		t.Fatalf("missing header fields:\n%s", got)
	}
}
