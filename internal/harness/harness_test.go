package harness

import "testing"

func TestRegistryHasTenHarnesses(t *testing.T) {
	r := NewRegistry()
	if len(r.items) != 10 {
		t.Fatalf("expected 10 adapters, got %d", len(r.items))
	}
}
func TestExtractText(t *testing.T) {
	got := extractText("{\"type\":\"x\",\"text\":\"hello\"}\n")
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}
