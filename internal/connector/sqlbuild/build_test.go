package sqlbuild_test

import (
	"strings"
	"testing"
	"time"

	"qLLM/internal/connector/def"
	"qLLM/internal/connector/sqlbuild"
	"qLLM/internal/protocol"
)

func TestPostgresTimestampParamTyped(t *testing.T) {
	e := &protocol.Entity{
		Name: "customers",
		Fields: []protocol.Field{
			{Name: "created_at", Type: protocol.TypeTimestamp, Physical: "created_at"},
			{Name: "country", Type: protocol.TypeString, Physical: "country"},
		},
		Binding: protocol.Binding{Kind: "table", Schema: "public", Table: "customers"},
	}
	step := def.PushdownStep{
		Entity: e,
		Select: []def.SelectItem{{Field: "country", As: "country"}},
		Where: map[string]any{
			"op": "and",
			"args": []any{
				map[string]any{"field": "country", "op": "eq", "value": "BR"},
				map[string]any{"field": "created_at", "op": "gte", "value": "2024-01-01T00:00:00Z"},
			},
		},
		Limit: 10,
	}
	built, err := sqlbuild.Build(sqlbuild.Postgres, step)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(built.SQL, "$1::text") {
		t.Fatalf("expected $1::text in SQL: %s", built.SQL)
	}
	if !strings.Contains(built.SQL, "$2::timestamptz") {
		t.Fatalf("expected $2::timestamptz in SQL: %s", built.SQL)
	}
	if strings.Contains(built.SQL, "$3") {
		t.Fatalf("unexpected $3 with only 2 args: %s", built.SQL)
	}
	if len(built.Args) != 2 {
		t.Fatalf("args=%d", len(built.Args))
	}
	if _, ok := built.Args[1].(time.Time); !ok {
		t.Fatalf("expected time.Time arg, got %T (%v)", built.Args[1], built.Args[1])
	}
}

func TestContainsEscapesLikeWildcards(t *testing.T) {
	e := &protocol.Entity{
		Name: "customers",
		Fields: []protocol.Field{
			{Name: "email", Type: protocol.TypeString, Physical: "email"},
		},
		Binding: protocol.Binding{Kind: "table", Schema: "public", Table: "customers"},
	}
	step := def.PushdownStep{
		Entity: e,
		Select: []def.SelectItem{{Field: "email", As: "email"}},
		Where:  map[string]any{"field": "email", "op": "contains", "value": "a%b_c"},
		Limit:  10,
	}
	built, err := sqlbuild.Build(sqlbuild.Postgres, step)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(built.SQL, "ESCAPE") {
		t.Fatalf("expected ESCAPE in SQL: %s", built.SQL)
	}
	got, _ := built.Args[0].(string)
	if got != `a\%b\_c` {
		t.Fatalf("escaped arg=%q", got)
	}
}

func TestMSSQLTop(t *testing.T) {
	e := &protocol.Entity{
		Name:    "t",
		Fields:  []protocol.Field{{Name: "id", Type: protocol.TypeString, Physical: "id"}},
		Binding: protocol.Binding{Kind: "table", Schema: "dbo", Table: "t"},
	}
	built, err := sqlbuild.Build(sqlbuild.MSSQL, def.PushdownStep{
		Entity: e, Select: []def.SelectItem{{Field: "id", As: "id"}}, Limit: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(built.SQL, "TOP (5)") {
		t.Fatalf("%s", built.SQL)
	}
}
