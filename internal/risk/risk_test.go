package risk

import "testing"

func TestSignalsDeterministic(t *testing.T) {
	r := Report{FixLike: 3, Reverts: 2, Authors: 10, Churn: 500}
	got := signals(r)
	if len(got) != 4 {
		t.Fatalf("expected 4 threshold signals, got %d", len(got))
	}
	if got[0].ID != "repeated-fixes" {
		t.Fatalf("stable ordering changed: %#v", got)
	}
}
func BenchmarkSignals(b *testing.B) {
	r := Report{FixLike: 4, Reverts: 2, Authors: 15, Churn: 2000}
	for i := 0; i < b.N; i++ {
		_ = signals(r)
	}
}
