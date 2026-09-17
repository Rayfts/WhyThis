package history

import (
	"fmt"
	"testing"
)

func BenchmarkSimilarityPrefilterLargeMonorepo(b *testing.B) {
	paths := make(map[string]map[string]struct{}, 5000)
	order := make([]string, 0, 5000)
	for i := 0; i < 5000; i++ {
		sha := fmt.Sprintf("%040x", i)
		order = append(order, sha)
		paths[sha] = map[string]struct{}{fmt.Sprintf("packages/p%d/file.go", i%400): {}}
	}
	target := map[string]struct{}{"packages/p17/file.go": {}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got := shortlistCandidates(order, paths, target, "", 120, 40)
		if len(got) == 0 {
			b.Fatal("empty shortlist")
		}
	}
}
