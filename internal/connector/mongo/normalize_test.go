package mongo

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestNormalizeBSON(t *testing.T) {
	oid := primitive.NewObjectID()
	got := normalize(bson.M{
		"id":   oid,
		"tags": primitive.A{"a", "b"},
		"addr": primitive.D{{Key: "city", Value: "SP"}},
	})
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("%T", got)
	}
	if m["id"] != oid.Hex() {
		t.Fatalf("id %#v", m["id"])
	}
	tags, ok := m["tags"].([]any)
	if !ok || len(tags) != 2 {
		t.Fatalf("tags %#v", m["tags"])
	}
	addr, ok := m["addr"].(map[string]any)
	if !ok || addr["city"] != "SP" {
		t.Fatalf("addr %#v", m["addr"])
	}
}
