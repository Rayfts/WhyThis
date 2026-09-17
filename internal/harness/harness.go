package harness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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

type Harness interface {
	ID() string
	Detect(context.Context) (Capabilities, error)
	Analyze(context.Context, AnalysisRequest) (AnalysisResult, error)
}

type Registry struct{ items map[string]Harness }

func NewRegistry() *Registry {
	r := &Registry{items: map[string]Harness{}}
	for _, h := range builtins() {
		r.items[h.ID()] = h
	}
	return r
}
func (r *Registry) Get(id string) (Harness, bool) { h, ok := r.items[id]; return h, ok }
func (r *Registry) List(ctx context.Context) []Capabilities {
	out := make([]Capabilities, 0, len(r.items))
	for _, h := range r.items {
		c, err := h.Detect(ctx)
		if err != nil {
			c.ID = h.ID()
			c.Notes = append(c.Notes, err.Error())
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

type commandHarness struct {
	id          string
	binary      string
	integration string
	structured  bool
	session     bool
	sources     []string
	args        func(promptFile, prompt string) []string
	stdin       bool
}

func (h commandHarness) ID() string { return h.id }
func (h commandHarness) Detect(ctx context.Context) (Capabilities, error) {
	path, err := exec.LookPath(h.binary)
	c := Capabilities{ID: h.id, Available: err == nil, Integration: h.integration, StructuredOutput: h.structured, SessionProtocol: h.session, EvidenceSources: h.sources}
	if err == nil {
		c.Executable = path
	}
	return c, nil
}
func (h commandHarness) Analyze(ctx context.Context, req AnalysisRequest) (AnalysisResult, error) {
	path, err := exec.LookPath(h.binary)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("%s not found: %w", h.binary, err)
	}
	dir, err := os.MkdirTemp("", "whythis-harness-*")
	if err != nil {
		return AnalysisResult{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	promptFile := filepath.Join(dir, "prompt.md")
	if err := os.WriteFile(promptFile, []byte(req.Prompt), 0o600); err != nil {
		return AnalysisResult{}, err
	}
	args := h.args(promptFile, req.Prompt)
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = dir
	if h.stdin {
		cmd.Stdin = strings.NewReader(req.Prompt)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return AnalysisResult{Harness: h.id, Raw: string(out), Format: "raw"}, fmt.Errorf("%s: %w", h.id, err)
	}
	raw := string(out)
	return AnalysisResult{Harness: h.id, Text: extractText(raw), Raw: raw, Format: format(h.structured)}, nil
}
func format(structured bool) string {
	if structured {
		return "structured-stream"
	}
	return "text"
}
