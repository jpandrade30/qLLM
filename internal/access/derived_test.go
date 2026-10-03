package access

import (
	"testing"
	"time"

	"qLLM/internal/catalogidx"
	"qLLM/internal/protocol"
)

func scopeIdx(t *testing.T) *catalogidx.Index {
	t.Helper()
	preset := &protocol.Preset{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Limits:          protocol.Limits{DefaultLimit: 10, MaxLimit: 100, ReadOnly: true},
		Sources:         []protocol.Source{{ID: "s", Type: protocol.SourcePostgres}},
	}
	catalog := &protocol.Catalog{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         "demo",
		Entities: []protocol.Entity{
			{
				Name: "orders", Source: "s",
				Scope:  &protocol.EntityScope{Field: "user_id"},
				Fields: []protocol.Field{{Name: "user_id", Type: protocol.TypeString}, {Name: "id", Type: protocol.TypeString}},
			},
			{
				Name: "profile", Source: "s",
				Scope:  &protocol.EntityScope{Field: "user_id"},
				Fields: []protocol.Field{{Name: "user_id", Type: protocol.TypeString}},
			},
			{
				Name: "products", Source: "s",
				Fields: []protocol.Field{{Name: "id", Type: protocol.TypeString}},
			},
		},
	}
	idx, err := catalogidx.New(preset, catalog)
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestDerivedKeyLookupAndNewUserNoConfig(t *testing.T) {
	idx := scopeIdx(t)
	reg, err := Resolve(&protocol.AccessFile{Apps: []protocol.AccessApp{{
		Name: "mobile", KeySecret: "s3cret", Tables: []string{"orders", "profile", "products"},
		UnscopedTables: []string{"products"},
		Scope:          map[string]string{"field": "user_id"},
	}}}, idx)
	if err != nil {
		t.Fatal(err)
	}
	k42, perr := MintDerivedKey("mobile", "42", time.Now().Add(time.Hour), "s3cret")
	if perr != nil {
		t.Fatal(perr)
	}
	k43, perr := MintDerivedKey("mobile", "43", time.Now().Add(time.Hour), "s3cret")
	if perr != nil {
		t.Fatal(perr)
	}
	a42 := reg.LookupBearer(k42)
	a43 := reg.LookupBearer(k43)
	if a42 == nil || a43 == nil {
		t.Fatal("both users must resolve without YAML change")
	}
	if a42.Name != "mobile" || a43.Name != "mobile" {
		t.Fatalf("app %s %s", a42.Name, a43.Name)
	}
	v42, _ := a42.ScopeValue("user_id")
	v43, _ := a43.ScopeValue("user_id")
	if v42 != "42" || v43 != "43" {
		t.Fatalf("scope %q %q", v42, v43)
	}
}

func TestDerivedKeyExpiredAndTampered(t *testing.T) {
	idx := scopeIdx(t)
	reg, err := Resolve(&protocol.AccessFile{Apps: []protocol.AccessApp{{
		Name: "mobile", KeySecret: "s3cret", Tables: []string{"orders"},
		Scope: map[string]string{"field": "user_id"},
	}}}, idx)
	if err != nil {
		t.Fatal(err)
	}
	expired, perr := MintDerivedKey("mobile", "42", time.Now().Add(-time.Hour), "s3cret")
	if perr != nil {
		t.Fatal(perr)
	}
	if reg.LookupBearer(expired) != nil {
		t.Fatal("expired")
	}
	ok, perr := MintDerivedKey("mobile", "42", time.Now().Add(time.Hour), "s3cret")
	if perr != nil {
		t.Fatal(perr)
	}
	tampered := ok[:len(ok)-1] + "x"
	if reg.LookupBearer(tampered) != nil {
		t.Fatal("tampered")
	}
}

func TestMintBadCharset(t *testing.T) {
	_, err := MintDerivedKey("mobile", "42;drop", time.Now().Add(time.Hour), "s")
	if err == nil || err.Code != protocol.ErrForbiddenScope {
		t.Fatalf("got %#v", err)
	}
}

func TestStaticPartnerAndUnscopedRule(t *testing.T) {
	idx := scopeIdx(t)
	reg, err := Resolve(&protocol.AccessFile{Apps: []protocol.AccessApp{{
		Name: "partner-acme", Key: "acme-key", Tables: []string{"orders"},
		Scope: map[string]string{"user_id": "acme"},
	}}}, idx)
	if err != nil {
		t.Fatal(err)
	}
	a := reg.LookupBearer("acme-key")
	if a == nil {
		t.Fatal("static")
	}
	v, ok := a.ScopeValue("user_id")
	if !ok || v != "acme" {
		t.Fatalf("static scope %q", v)
	}
	_, err = Resolve(&protocol.AccessFile{Apps: []protocol.AccessApp{{
		Name: "mobile", KeySecret: "s3cret", Tables: []string{"orders", "products"},
		Scope: map[string]string{"field": "user_id"},
	}}}, idx)
	if err == nil || err.Code != protocol.ErrConfigError {
		t.Fatalf("expected unscoped table error %#v", err)
	}
}

func TestKeyXorKeySecret(t *testing.T) {
	idx := scopeIdx(t)
	_, err := Resolve(&protocol.AccessFile{Apps: []protocol.AccessApp{{
		Name: "x", Key: "k", KeySecret: "s", Tables: []string{"orders"},
		Scope: map[string]string{"field": "user_id"},
	}}}, idx)
	if err == nil {
		t.Fatal("xor")
	}
}

func TestMintDerivedKeyKnownVector(t *testing.T) {
	got, err := MintDerivedKey("mobile", "42", time.Unix(1767225600, 0), "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	const want = "mobile.42.1767225600.PyV9Flp-KOKj1RxuR0Oa0CTEbPI3QFTTfw-fI-kXVbI"
	if got != want {
		t.Fatalf("got %s", got)
	}
}

func TestStdioScope(t *testing.T) {
	idx := scopeIdx(t)
	reg, err := Resolve(&protocol.AccessFile{Apps: []protocol.AccessApp{{
		Name: "mobile", KeySecret: "s3cret", Tables: []string{"orders"},
		Scope: map[string]string{"field": "user_id"},
	}}}, idx)
	if err != nil {
		t.Fatal(err)
	}
	app := reg.LookupName("mobile")
	got, perr := app.WithStdioScope("42")
	if perr != nil {
		t.Fatal(perr)
	}
	v, _ := got.ScopeValue("user_id")
	if v != "42" {
		t.Fatalf("stdio %q", v)
	}
	if _, perr = app.WithStdioScope("bad value"); perr == nil {
		t.Fatal("expected bad charset")
	}
}
