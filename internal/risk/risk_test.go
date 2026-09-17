package risk

import (
	"testing"

	"github.com/Rayfts/WhyThis/internal/sensitive"
)

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

func TestDeterministicScoreAndConfidence(t *testing.T) {
	r := Report{Commits: 25, FixLike: 4, Reverts: 2, Authors: 12, Churn: 5000, HistoryComplete: true}
	if got := deterministicScore(r); got != 100 {
		t.Fatalf("expected capped score 100, got %d", got)
	}
	if got := confidence(r); got != "high" {
		t.Fatalf("expected high confidence, got %q", got)
	}
	r.HistoryComplete = false
	if got := confidence(r); got != "low" {
		t.Fatalf("shallow history must lower confidence, got %q", got)
	}
}

func TestSignalsIncludeSensitivityAndPathClassifications(t *testing.T) {
	r := Report{Sensitive: true, Sensitivity: []sensitive.Finding{{Category: "access-control"}}, Generated: true, Vendor: true, Submodule: true}
	got := signals(r)
	ids := map[string]bool{}
	for _, s := range got {
		ids[s.ID] = true
	}
	for _, id := range []string{"sensitive-area", "generated-code", "vendor-code", "submodule"} {
		if !ids[id] {
			t.Fatalf("missing %s signal: %+v", id, got)
		}
	}
}
