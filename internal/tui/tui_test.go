package tui

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Rayfts/WhyThis/internal/harness"
	"github.com/Rayfts/WhyThis/internal/risk"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

type fake struct{}

func (fake) Line(context.Context, string) (evidence.Report, error) {
	return evidence.Report{Target: "x.go:2-2", Revision: "1234567890abcdef", Facts: []evidence.Claim{{Class: evidence.Fact, Text: "fact"}}}, nil
}
func (fake) FileHistory(context.Context, string) (evidence.Report, error) {
	return evidence.Report{}, nil
}
func (fake) Symbol(context.Context, string) (evidence.Report, error) { return evidence.Report{}, nil }
func (fake) Commit(context.Context, string) (evidence.Report, error) { return evidence.Report{}, nil }
func (fake) Similar(context.Context, string, int) (evidence.Report, error) {
	return evidence.Report{}, nil
}
func (fake) Ask(context.Context, string) (evidence.Report, error) { return evidence.Report{}, nil }
func (fake) PR(context.Context, int) (evidence.Report, error)     { return evidence.Report{}, nil }
func (fake) Risk(context.Context, string) (risk.Report, error) {
	return risk.Report{Path: "x.go", Score: 42, Confidence: "high"}, nil
}
func (fake) Harnesses(context.Context) []harness.Capabilities { return nil }

func TestTUIRunsLineViewAndQuits(t *testing.T) {
	in := strings.NewReader("1\nx.go:2\n\nq\n")
	var out bytes.Buffer
	if err := RunWithOptions(context.Background(), fake{}, t.TempDir(), in, &out, Options{Plain: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "FACTS") || !strings.Contains(out.String(), "fact") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}
