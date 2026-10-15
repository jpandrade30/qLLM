package sqlparse

import "testing"

func TestSelectOutAliasAndFields(t *testing.T) {
	r, err := Parse(`SELECT id AS x, name FROM customers LIMIT 5`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.SelectOuts) != 2 {
		t.Fatalf("SelectOuts=%+v", r.SelectOuts)
	}
	if r.SelectOuts[0].OutName != "x" || len(r.SelectOuts[0].Fields) != 1 || r.SelectOuts[0].Fields[0] != "id" {
		t.Fatalf("first=%+v", r.SelectOuts[0])
	}
	if r.SelectOuts[1].OutName != "name" {
		t.Fatalf("second=%+v", r.SelectOuts[1])
	}
}

func TestSelectOutStar(t *testing.T) {
	r, err := Parse(`SELECT * FROM customers LIMIT 5`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.SelectOuts) != 1 || !r.SelectOuts[0].Star {
		t.Fatalf("SelectOuts=%+v", r.SelectOuts)
	}
}
