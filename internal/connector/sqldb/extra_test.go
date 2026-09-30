package sqldb

import (
	"testing"

	"qLLM/internal/protocol"
)

func TestOpenSQLiteMissingPathEnv(t *testing.T) {
	_, err := OpenSQLite(protocol.Source{ID: "s", Type: protocol.SourceSQLite, Connection: map[string]any{}}, 1000)
	if err == nil {
		t.Fatal("expected CONFIG_ERROR")
	}
	pe, ok := err.(*protocol.ProtocolError)
	if !ok || pe.Code != protocol.ErrConfigError {
		t.Fatalf("got %v", err)
	}
}

func TestOpenMSSQLMissingHost(t *testing.T) {
	_, err := OpenMSSQL(protocol.Source{ID: "s", Connection: map[string]any{
		"userEnv": "U", "passwordEnv": "P", "database": "d",
	}}, 1000)
	if err == nil {
		t.Fatal("expected error")
	}
}
