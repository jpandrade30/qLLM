package sqldb

import (
	"testing"

	"qLLM/internal/protocol"
)

func TestResolveMaxOpenConnsDefault(t *testing.T) {
	n, err := resolveMaxOpenConns(nil)
	if err != nil || n != 5 {
		t.Fatalf("got %d %v want 5 nil", n, err)
	}
	n, err = resolveMaxOpenConns(map[string]any{})
	if err != nil || n != 5 {
		t.Fatalf("empty map: got %d %v", n, err)
	}
}

func TestResolveMaxOpenConnsLiteral(t *testing.T) {
	n, err := resolveMaxOpenConns(map[string]any{"maxOpenConns": 12})
	if err != nil || n != 12 {
		t.Fatalf("got %d %v", n, err)
	}
	n, err = resolveMaxOpenConns(map[string]any{"maxOpenConns": float64(8)})
	if err != nil || n != 8 {
		t.Fatalf("float64: got %d %v", n, err)
	}
}

func TestResolveMaxOpenConnsEnv(t *testing.T) {
	t.Setenv("QLLM_TEST_MAX_OPEN", "7")
	n, err := resolveMaxOpenConns(map[string]any{"maxOpenConnsEnv": "QLLM_TEST_MAX_OPEN", "maxOpenConns": 3})
	if err != nil || n != 7 {
		t.Fatalf("env should win: got %d %v", n, err)
	}
}

func TestResolveMaxOpenConnsInvalid(t *testing.T) {
	cases := []struct {
		name string
		conn map[string]any
		env  map[string]string
	}{
		{"bad type", map[string]any{"maxOpenConns": "x"}, nil},
		{"zero", map[string]any{"maxOpenConns": 0}, nil},
		{"over cap", map[string]any{"maxOpenConns": 101}, nil},
		{"empty env name", map[string]any{"maxOpenConnsEnv": ""}, nil},
		{"empty env value", map[string]any{"maxOpenConnsEnv": "QLLM_TEST_MAX_OPEN_EMPTY"}, map[string]string{"QLLM_TEST_MAX_OPEN_EMPTY": ""}},
		{"non int env", map[string]any{"maxOpenConnsEnv": "QLLM_TEST_MAX_OPEN_BAD"}, map[string]string{"QLLM_TEST_MAX_OPEN_BAD": "abc"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, err := resolveMaxOpenConns(tc.conn)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestPoolMaxOpenSQLiteClamp(t *testing.T) {
	src := protocol.Source{
		ID:   "local",
		Type: protocol.SourceSQLite,
		Connection: map[string]any{
			"maxOpenConns": 50,
		},
	}
	n, perr := poolMaxOpen(src)
	if perr != nil || n != 1 {
		t.Fatalf("sqlite clamp: got %d %v want 1", n, perr)
	}
}

func TestPoolMaxOpenPostgresLiteral(t *testing.T) {
	src := protocol.Source{
		ID:   "pg",
		Type: protocol.SourcePostgres,
		Connection: map[string]any{
			"maxOpenConns": 9,
		},
	}
	n, perr := poolMaxOpen(src)
	if perr != nil || n != 9 {
		t.Fatalf("got %d %v", n, perr)
	}
}
