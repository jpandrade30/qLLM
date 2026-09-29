package config

import (
	"os"
	"path/filepath"
	"testing"

	"qLLM/internal/protocol"
)

func TestIsLoopbackAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8088", true},
		{"localhost:8088", true},
		{"[::1]:8089", true},
		{":8088", false},
		{"0.0.0.0:8088", false},
		{"192.168.1.1:8088", false},
	}
	for _, tc := range cases {
		if got := IsLoopbackAddr(tc.addr); got != tc.want {
			t.Errorf("%q: got %v want %v", tc.addr, got, tc.want)
		}
	}
}

func TestCheckBindPolicy(t *testing.T) {
	if err := CheckBindPolicy("127.0.0.1:8088", "", false); err != nil {
		t.Fatal(err)
	}
	if err := CheckBindPolicy(":8088", "", false); err == nil {
		t.Fatal("expected refuse")
	}
	if err := CheckBindPolicy(":8088", "tok", false); err != nil {
		t.Fatal(err)
	}
	if err := CheckBindPolicy(":8088", "", true); err != nil {
		t.Fatal(err)
	}
}

func TestConfinePath(t *testing.T) {
	base := t.TempDir()
	ok := filepath.Join(base, "qllm.preset.yaml")
	if _, err := ConfinePath(base, ok); err != nil {
		t.Fatal(err)
	}
	escape := filepath.Join(base, "..", "outside.yaml")
	if _, err := ConfinePath(base, escape); err == nil {
		t.Fatal("expected escape rejection")
	}
}

func TestMergeServeSettingsAuthEnv(t *testing.T) {
	t.Setenv("QLLM_TEST_TOKEN", "abc")
	addr := "0.0.0.0:9"
	env := "QLLM_TEST_TOKEN"
	s, err := MergeServeSettings(nil, ServeFlagOverrides{
		Addr:         &addr,
		AuthTokenEnv: &env,
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.AuthToken != "abc" {
		t.Fatalf("token=%q", s.AuthToken)
	}
}

func TestMergeServeSettingsEmptyAuthEnvFails(t *testing.T) {
	t.Setenv("QLLM_TEST_TOKEN_EMPTY", "")
	env := "QLLM_TEST_TOKEN_EMPTY"
	_, err := MergeServeSettings(nil, ServeFlagOverrides{AuthTokenEnv: &env})
	if err == nil {
		t.Fatal("expected empty auth env to fail")
	}
}

func TestMergeServeSettingsRejectsCORSStar(t *testing.T) {
	_, err := MergeServeSettings(nil, ServeFlagOverrides{
		CORSOrigins:    []string{"*"},
		CORSOriginsSet: true,
	})
	if err == nil {
		t.Fatal("expected reject *")
	}
}

func TestValidateRuntimeRejectsStar(t *testing.T) {
	rc := &protocol.RuntimeConfig{
		Serve: protocol.ServeConfig{
			CORS: &protocol.CORSConfig{Origins: []string{"*"}},
		},
	}
	if err := validateRuntimeConfig(rc); err == nil {
		t.Fatal("expected reject *")
	}
}

func TestLoadAccessMissing(t *testing.T) {
	dir := t.TempDir()
	f, path, err := LoadAccess("", dir)
	if err != nil {
		t.Fatal(err)
	}
	if f != nil || path != "" {
		t.Fatalf("want absent, got %v %s", f, path)
	}
}

func TestLoadAccessFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "qllm.access.yaml")
	if err := os.WriteFile(p, []byte("apps:\n  - name: a\n    key: k\n    tables: [customers]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, path, err := LoadAccess("", dir)
	if err != nil {
		t.Fatal(err)
	}
	if path != p || f == nil || len(f.Apps) != 1 {
		t.Fatalf("path=%s file=%v", path, f)
	}
}

func TestApplyEnvFileFillsEmpty(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "qllm.env.yaml")
	if err := os.WriteFile(p, []byte("env:\n  QLLM_TEST_SEED: from-yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("QLLM_TEST_SEED", "")
	if err := ApplyEnvFile(dir); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("QLLM_TEST_SEED") != "from-yaml" {
		t.Fatalf("got %q", os.Getenv("QLLM_TEST_SEED"))
	}
}

func TestApplyEnvFileDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "qllm.env.yaml")
	if err := os.WriteFile(p, []byte("env:\n  QLLM_TEST_KEEP: yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("QLLM_TEST_KEEP", "secret")
	if err := ApplyEnvFile(dir); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("QLLM_TEST_KEEP") != "secret" {
		t.Fatalf("got %q", os.Getenv("QLLM_TEST_KEEP"))
	}
}
