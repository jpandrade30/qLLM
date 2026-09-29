package cryptox

import "testing"

func TestHMACEqualMatch(t *testing.T) {
	if !HMACEqual("secret", "secret") {
		t.Fatal("expected match")
	}
}

func TestHMACEqualReject(t *testing.T) {
	if HMACEqual("secret", "Secret") {
		t.Fatal("case")
	}
	if HMACEqual("short", "much-longer-token") {
		t.Fatal("length")
	}
	if HMACEqual("", "x") {
		t.Fatal("empty got")
	}
	if HMACEqual("x", "") {
		t.Fatal("empty want")
	}
}

func TestHMACEqualEmptyEmpty(t *testing.T) {
	if !HMACEqual("", "") {
		t.Fatal("two empties")
	}
}
