package graphql

import "testing"

func TestValidateDocumentQueryOK(t *testing.T) {
	cases := []string{
		`{ users { id name } }`,
		`query { users { id } }`,
		`query Users($id: ID) { users(id: $id) { id } }`,
		`# mutation is a comment
query {
  users { id }
}`,
		`query { users(note: "do not INSERT here") { id } }`,
	}
	for _, c := range cases {
		if err := ValidateDocument(c); err != nil {
			t.Fatalf("ok doc rejected: %q → %v", c, err)
		}
	}
}

func TestValidateDocumentRejectsMutationSubscription(t *testing.T) {
	for _, c := range []string{
		`mutation { updateUser(id: 1) { id } }`,
		`subscription { onUser { id } }`,
		`MUTATION Foo { x }`,
	} {
		if err := ValidateDocument(c); err == nil {
			t.Fatalf("expected reject: %q", c)
		}
	}
}

func TestValidateDocumentRejectsWriteKeywords(t *testing.T) {
	for _, c := range []string{
		`query { DELETE }`,
		`query { x } INSERT INTO t`,
		`query { REPLACE INTO foo }`,
		`query { DROP TABLE x }`,
	} {
		if err := ValidateDocument(c); err == nil {
			t.Fatalf("expected reject: %q", c)
		}
	}
}

func TestValidateDocumentEmpty(t *testing.T) {
	if err := ValidateDocument("   "); err == nil {
		t.Fatal("empty should fail")
	}
}
