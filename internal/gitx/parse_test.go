package gitx

import "testing"

func TestParseBlamePorcelain(t *testing.T) {
	raw := "0123456789012345678901234567890123456789 4 9 1\nauthor Ada\nauthor-mail <ada@example.com>\nauthor-time 1700000000\nsummary fix race\nfilename a.go\n\treturn nil\n"
	got := ParseBlamePorcelain(raw)
	if len(got) != 1 || got[0].Commit[:6] != "012345" || got[0].FinalLine != 9 || got[0].Summary != "fix race" {
		t.Fatalf("unexpected parse: %#v", got)
	}
}

func BenchmarkParseBlamePorcelain(b *testing.B) {
	raw := "0123456789012345678901234567890123456789 4 9 1\nauthor Ada\nauthor-mail <ada@example.com>\nauthor-time 1700000000\nsummary fix race\nfilename a.go\n\treturn nil\n"
	for i := 0; i < b.N; i++ {
		_ = ParseBlamePorcelain(raw)
	}
}
