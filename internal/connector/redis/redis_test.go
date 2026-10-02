package redis

import (
	"context"
	"strings"
	"testing"

	"qLLM/internal/connector/def"
	"qLLM/internal/protocol"
)

type recKV struct {
	cmds []string
	typ  string
	get  string
	hash map[string]string
}

func (r *recKV) Do(_ context.Context, cmd string, args ...any) (any, error) {
	u := strings.ToUpper(cmd)
	if _, ok := allowedRedisCmds[u]; !ok {
		return nil, protocol.NewError(protocol.ErrForbidden, "blocked "+u, nil)
	}
	r.cmds = append(r.cmds, u)
	switch u {
	case "TYPE":
		return r.typ, nil
	case "GET":
		return r.get, nil
	case "HGETALL":
		return r.hash, nil
	case "LRANGE", "ZRANGE":
		return []string{"a", "b"}, nil
	case "SSCAN":
		return []any{"0", []string{"a"}}, nil
	case "XRANGE":
		return []any{[]any{"1-0", []any{"name", "hello"}}}, nil
	default:
		return nil, nil
	}
}

func entity() *protocol.Entity {
	return &protocol.Entity{
		Name: "users",
		Binding: protocol.Binding{
			Kind: "key", KeyPattern: "user:{id}",
			AccessPath: protocol.AccessPath{Partition: []string{"id"}},
		},
		Fields: []protocol.Field{
			{Name: "id", Type: protocol.TypeString, Physical: "id"},
			{Name: "email", Type: protocol.TypeString, Physical: "email"},
			{Name: "value", Type: protocol.TypeString, Physical: "value"},
		},
	}
}

func TestGetJSONNoWrite(t *testing.T) {
	kv := &recKV{typ: "string", get: `{"id":"42","email":"a@x"}`}
	c := New("cache", kv, nil)
	res, err := c.Query(context.Background(), def.PushdownStep{
		Entity: entity(),
		Select: []def.SelectItem{{Field: "id"}, {Field: "email"}},
		Where:  map[string]any{"op": "eq", "field": "id", "value": "42"},
		Limit:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.RowCount != 1 || res.Rows[0][0] != "42" {
		t.Fatalf("rows %#v", res.Rows)
	}
	for _, cmd := range kv.cmds {
		if _, ok := allowedRedisCmds[cmd]; !ok {
			t.Fatalf("disallowed command %s", cmd)
		}
		switch cmd {
		case "DEL", "SET", "EXPIRE", "XACK", "LPOP", "KEYS", "SCAN":
			t.Fatalf("destructive %s", cmd)
		}
	}
}

func TestMissingKeyUnsupported(t *testing.T) {
	c := New("cache", &recKV{}, nil)
	_, err := c.Query(context.Background(), def.PushdownStep{
		Entity: entity(),
		Select: []def.SelectItem{{Field: "id"}},
		Limit:  1,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	pe := err.(*protocol.ProtocolError)
	if pe.Code != protocol.ErrUnsupported {
		t.Fatalf("code %s", pe.Code)
	}
}

func TestForbiddenCommand(t *testing.T) {
	kv := &recKV{}
	_, err := kv.Do(context.Background(), "DEL", "user:1")
	if err == nil {
		t.Fatal("expected forbidden")
	}
}
