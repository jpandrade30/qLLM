package sqlparse

import "testing"

func TestExtractEqualityFiltersSimple(t *testing.T) {
	ex, err := ExtractEqualityFilters(`SELECT id FROM orders WHERE user_id = '42' AND status = 'open' LIMIT 10`)
	if err != nil {
		t.Fatal(err)
	}
	if ex.Ambiguous {
		t.Fatal("expected unambiguous")
	}
	vals := ex.ValuesForField("user_id")
	if len(vals) != 1 || vals[0] != "42" {
		t.Fatalf("user_id %#v", vals)
	}
	if got := ex.ValuesForField("status"); len(got) != 1 || got[0] != "open" {
		t.Fatalf("status %#v", got)
	}
}

func TestExtractEqualityFiltersQualifiedAndReversed(t *testing.T) {
	ex, err := ExtractEqualityFilters(`SELECT * FROM orders o WHERE '42' = o.user_id`)
	if err != nil {
		t.Fatal(err)
	}
	vals := ex.ValuesForField("user_id")
	if len(vals) != 1 || vals[0] != "42" {
		t.Fatalf("%#v filters=%#v", vals, ex.Filters)
	}
	if ex.Filters[0].Qual != "o" {
		t.Fatalf("qual %#v", ex.Filters[0])
	}
}

func TestExtractEqualityFiltersORAmbiguous(t *testing.T) {
	ex, err := ExtractEqualityFilters(`SELECT id FROM orders WHERE user_id = '42' OR user_id = '7'`)
	if err != nil {
		t.Fatal(err)
	}
	if !ex.Ambiguous {
		t.Fatal("expected ambiguous")
	}
}

func TestExtractEqualityFiltersNoWhere(t *testing.T) {
	ex, err := ExtractEqualityFilters(`SELECT id FROM orders LIMIT 5`)
	if err != nil {
		t.Fatal(err)
	}
	if len(ex.Filters) != 0 || ex.Ambiguous {
		t.Fatalf("%#v", ex)
	}
}
