package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"qLLM/internal/protocol"
)

func TestResolveMissingConfigDirIsConfigError(t *testing.T) {
	dir := t.TempDir()
	_, err := Resolve(Options{ConfigDir: dir})
	if err == nil {
		t.Fatal("expected CONFIG_ERROR")
	}
	pe, ok := err.(*protocol.ProtocolError)
	if !ok || pe.Code != protocol.ErrConfigError {
		t.Fatalf("got %v", err)
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "invoices") {
		t.Fatalf("must not mention demo entities: %v", err)
	}
}

func TestLoadBundleEmptyDirNoDemoCatalog(t *testing.T) {
	dir := t.TempDir()
	_, _, _, err := LoadBundle(Options{ConfigDir: dir})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "invoices") {
		t.Fatal(err)
	}
}

func TestResolveUsesOnlyGivenDir(t *testing.T) {
	dir := t.TempDir()
	preset := filepath.Join(dir, "qllm.preset.yaml")
	catalog := filepath.Join(dir, "qllm.catalog.yaml")
	if err := os.WriteFile(preset, []byte("protocolVersion: \"0.1.0\"\nproject: x\nlimits: {defaultLimit: 1, maxLimit: 10, readOnly: true}\nsources: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalog, []byte("protocolVersion: \"0.1.0\"\nproject: x\nentities: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Resolve(Options{ConfigDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if p.Preset != preset || p.Catalog != catalog {
		t.Fatalf("%+v", p)
	}
}
