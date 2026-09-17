// Package harness defines WhyThis's public coding-agent adapter contract.
// External Go modules can implement Adapter without importing internal packages.
package harness

import (
	"context"
	"errors"
	"time"
)

var ErrInteractiveOnly = errors.New("harness has no verified headless integration")

type Capabilities struct {
	ID               string   `json:"id"`
	Available        bool     `json:"available"`
	Executable       string   `json:"executable,omitempty"`
	Integration      string   `json:"integration"`
	StructuredOutput bool     `json:"structured_output"`
	SessionProtocol  bool     `json:"session_protocol"`
	EvidenceSources  []string `json:"evidence_sources"`
	Notes            []string `json:"notes,omitempty"`
}

type AnalysisRequest struct {
	Prompt  string
	Timeout time.Duration
}

type AnalysisResult struct {
	Harness string `json:"harness"`
	Text    string `json:"text"`
	Raw     string `json:"raw,omitempty"`
	Format  string `json:"format"`
}

// Adapter is the stable extension point for additional coding-agent harnesses.
// Analyze may interpret deterministic WhyThis evidence, but adapters must not
// manufacture provenance or mutate the underlying evidence graph.
type Adapter interface {
	ID() string
	Detect(context.Context) (Capabilities, error)
	Analyze(context.Context, AnalysisRequest) (AnalysisResult, error)
}
