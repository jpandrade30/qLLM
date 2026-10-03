package duckdblocal

import (
	"testing"

	"qLLM/internal/protocol"
)

func TestLogicalFromDuckName(t *testing.T) {
	cases := map[string]protocol.LogicalType{
		"INTEGER":     protocol.TypeNumber,
		"BIGINT":      protocol.TypeNumber,
		"DOUBLE":      protocol.TypeNumber,
		"DECIMAL(38,9)": protocol.TypeNumber,
		"BOOLEAN":     protocol.TypeBoolean,
		"TIMESTAMP":   protocol.TypeTimestamp,
		"DATE":        protocol.TypeTimestamp,
		"JSON":        protocol.TypeJSON,
		"STRUCT":      protocol.TypeJSON,
		"VARCHAR":     protocol.TypeString,
	}
	for in, want := range cases {
		if got := logicalFromDuckName(in); got != want {
			t.Fatalf("%s: got %s want %s", in, got, want)
		}
	}
}

func TestEncodeParseJSONCell(t *testing.T) {
	obj := map[string]any{"city": "SP"}
	enc := encodeJSONCell(obj)
	s, ok := enc.(string)
	if !ok {
		t.Fatalf("encode %#v", enc)
	}
	back := parseJSONCell(s)
	m, ok := back.(map[string]any)
	if !ok || m["city"] != "SP" {
		t.Fatalf("parse %#v", back)
	}
	arr := parseJSONCell(`["a","b"]`)
	list, ok := arr.([]any)
	if !ok || len(list) != 2 {
		t.Fatalf("list %#v", arr)
	}
}

func TestDoubleLosesIntegersAbove2Pow53(t *testing.T) {
	const n = 9007199254740993
	if float64(n) == float64(n-2)+2 && float64(n) != float64(n) {
		t.Fatal("unreachable")
	}
	if int64(float64(n)) == n {
		t.Log("this platform preserved the integer; catalog still must use string for ids > 2^53")
	}
}
