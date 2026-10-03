package kafka

import (
	"context"
	"testing"
	"time"

	"qLLM/internal/connector/def"
	"qLLM/internal/protocol"
)

type recFetch struct {
	fetches []FetchReq
	recs    []Record
	commits int
	groups  int
}

func (r *recFetch) Fetch(_ context.Context, req FetchReq) ([]Record, error) {
	if r.commits > 0 || r.groups > 0 {
		return nil, protocol.NewError(protocol.ErrForbidden, "commit or group used", nil)
	}
	r.fetches = append(r.fetches, req)
	return r.recs, nil
}

func (r *recFetch) OffsetAt(_ context.Context, _ string, _ int32, _ time.Time) (int64, error) {
	if r.commits > 0 || r.groups > 0 {
		return 0, protocol.NewError(protocol.ErrForbidden, "commit or group used", nil)
	}
	return 5, nil
}

func kafkaEntity() *protocol.Entity {
	return &protocol.Entity{
		Name: "events",
		Binding: protocol.Binding{
			Kind: "topic", Topic: "app.events",
			AccessPath: protocol.AccessPath{Partition: []string{"partition"}, Key: "msg_key"},
		},
		Fields: []protocol.Field{
			{Name: "partition", Type: protocol.TypeNumber, Physical: "partition"},
			{Name: "offset", Type: protocol.TypeNumber, Physical: "offset"},
			{Name: "msg_key", Type: protocol.TypeString, Physical: "key"},
			{Name: "value", Type: protocol.TypeString, Physical: "value"},
		},
	}
}

func TestPartitionOffsetNoCommit(t *testing.T) {
	f := &recFetch{recs: []Record{
		{Partition: 0, Offset: 10, Key: []byte("k1"), Value: []byte(`{"n":1}`), Timestamp: time.Unix(0, 0).UTC()},
	}}
	c := New("bus", f, 50, 50, nil)
	res, err := c.Query(context.Background(), def.PushdownStep{
		Entity: kafkaEntity(),
		Select: []def.SelectItem{{Field: "msg_key"}, {Field: "value"}},
		Where: map[string]any{
			"op": "and",
			"args": []any{
				map[string]any{"op": "eq", "field": "partition", "value": 0},
				map[string]any{"op": "eq", "field": "offset", "value": 10},
			},
		},
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.commits != 0 || f.groups != 0 {
		t.Fatal("must not commit or join a group")
	}
	if len(f.fetches) != 1 || f.fetches[0].Offset != 10 {
		t.Fatalf("fetch %#v", f.fetches)
	}
	if res.RowCount != 1 || res.Rows[0][0] != "k1" {
		t.Fatalf("rows %#v", res.Rows)
	}
}

func TestKeyFilterNoMutation(t *testing.T) {
	f := &recFetch{recs: []Record{
		{Partition: 0, Offset: 1, Key: []byte("keep"), Value: []byte("a")},
		{Partition: 0, Offset: 2, Key: []byte("skip"), Value: []byte("b")},
	}}
	c := New("bus", f, 50, 50, nil)
	res, err := c.Query(context.Background(), def.PushdownStep{
		Entity: kafkaEntity(),
		Select: []def.SelectItem{{Field: "msg_key"}, {Field: "value"}},
		Where:  map[string]any{"op": "eq", "field": "msg_key", "value": "keep"},
		Limit:  10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.RowCount != 1 || res.Rows[0][0] != "keep" {
		t.Fatalf("rows %#v", res.Rows)
	}
	if string(f.recs[0].Key) != "keep" || string(f.recs[0].Value) != "a" {
		t.Fatal("records must be unchanged")
	}
}

func TestUnboundedUnsupported(t *testing.T) {
	c := New("bus", &recFetch{}, 10, 10, nil)
	_, err := c.Query(context.Background(), def.PushdownStep{
		Entity: kafkaEntity(),
		Select: []def.SelectItem{{Field: "value"}},
		Limit:  1,
	})
	if err == nil {
		t.Fatal("expected UNSUPPORTED")
	}
	if err.(*protocol.ProtocolError).Code != protocol.ErrUnsupported {
		t.Fatalf("code %v", err)
	}
}
