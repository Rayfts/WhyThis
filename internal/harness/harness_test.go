package harness

import (
	"context"
	"reflect"
	"testing"
)

func TestRegistryHasTenHarnesses(t *testing.T) {
	r := NewRegistry()
	if len(r.items) != 10 {
		t.Fatalf("expected 10 adapters, got %d", len(r.items))
	}
}

func TestBuiltinHarnessContracts(t *testing.T) {
	var aider commandHarness
	var foundAider bool
	for _, h := range builtins() {
		if h.ID() != "aider" {
			continue
		}
		var ok bool
		aider, ok = h.(commandHarness)
		if !ok {
			t.Fatalf("aider adapter has unexpected type %T", h)
		}
		foundAider = true
		break
	}
	if !foundAider {
		t.Fatal("aider adapter not found")
	}

	got := aider.args("/tmp/prompt.md", "ignored")
	want := []string{"--message-file", "/tmp/prompt.md", "--yes"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("aider args = %#v, want %#v", got, want)
	}

	piCaps, err := (piHarness{}).Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(piCaps.EvidenceSources) != 1 || piCaps.EvidenceSources[0] != "mitsuhiko/pi-mono packages/coding-agent/docs/rpc.md" {
		t.Fatalf("unexpected pi evidence sources: %#v", piCaps.EvidenceSources)
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
