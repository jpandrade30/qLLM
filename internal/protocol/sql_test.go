package protocol

import "testing"

func TestResolveSQLVersionDefault(t *testing.T) {
	v, err := ResolveSQLVersion("")
	if err != nil || v != SQLDialectLatest {
		t.Fatalf("v=%s err=%v", v, err)
	}
	if SQLDialectLatest != SQLDialect2 {
		t.Fatalf("latest=%s", SQLDialectLatest)
	}
}

func TestResolveSQLVersionOneAndTwo(t *testing.T) {
	for _, ver := range []string{SQLDialect1, SQLDialect2} {
		v, err := ResolveSQLVersion(ver)
		if err != nil || v != ver {
			t.Fatalf("ver=%s v=%s err=%v", ver, v, err)
		}
	}
}

func TestResolveSQLVersionUnknown(t *testing.T) {
	_, err := ResolveSQLVersion("99")
	if err == nil || err.Code != ErrUnsupportedVersion {
		t.Fatalf("got %v", err)
	}
}
