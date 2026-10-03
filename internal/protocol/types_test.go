package protocol

import "testing"

func TestWireFamily(t *testing.T) {
	cases := map[SourceType]SourceType{
		SourcePostgres:    SourcePostgres,
		SourceCockroach:   SourcePostgres,
		SourceRedshift:    SourcePostgres,
		SourceMySQL:       SourceMySQL,
		SourceMariaDB:     SourceMySQL,
		SourcePlanetScale: SourceMySQL,
		SourceREST:        SourceREST,
		SourceMongoDB:     SourceMongoDB,
	}
	for in, want := range cases {
		if got := WireFamily(in); got != want {
			t.Fatalf("WireFamily(%s)=%s want %s", in, got, want)
		}
	}
}
