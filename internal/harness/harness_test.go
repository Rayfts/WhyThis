package harness

import (
	"context"
	"testing"
)

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

type externalAdapter struct{}

func (externalAdapter) ID() string { return "external-test" }
func (externalAdapter) Detect(context.Context) (Capabilities, error) {
	return Capabilities{ID: "external-test", Available: true}, nil
}
func (externalAdapter) Analyze(context.Context, AnalysisRequest) (AnalysisResult, error) {
	return AnalysisResult{Harness: "external-test", Text: "ok"}, nil
}

func TestRegistryAcceptsExternalAdapter(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(externalAdapter{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Get("external-test"); !ok {
		t.Fatal("external adapter not registered")
	}
	if err := r.Register(externalAdapter{}); err == nil {
		t.Fatal("expected duplicate adapter rejection")
	}
}
