package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Rayfts/WhyThis/internal/harness"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

type Orchestrator struct{ Registry *harness.Registry }

func (o *Orchestrator) Synthesize(ctx context.Context, id string, report evidence.Report) (evidence.Report, error) {
	h, ok := o.Registry.Get(id)
	if !ok {
		return report, fmt.Errorf("unknown harness %q", id)
	}
	prompt, err := buildPrompt(report)
	if err != nil {
		return report, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	res, err := h.Analyze(ctx, harness.AnalysisRequest{Prompt: prompt, Timeout: 2 * time.Minute})
	if err != nil {
		return report, err
	}
	text := strings.TrimSpace(res.Text)
	if text == "" {
		return report, fmt.Errorf("%s returned no synthesis", id)
	}
	ids := make([]string, 0, len(report.Graph.Nodes))
	for _, n := range report.Graph.Nodes {
		ids = append(ids, n.ID)
	}
	report.Inferences = append(report.Inferences, evidence.Claim{Class: evidence.Inference, Text: fmt.Sprintf("[%s] %s", id, text), EvidenceID: ids})
	return report, nil
}

func buildPrompt(report evidence.Report) (string, error) {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	return `You are the semantic interpretation layer for WhyThis, a code-archaeology tool.
The JSON below contains deterministic evidence collected from Git and optionally GitHub.

Rules:
1. NEVER invent history, incidents, motivations, PRs, issues, production events, or causal links.
2. Treat only claims already in facts and evidence-node attributes as facts.
3. Any explanation of intent or causality must be labeled INFERENCE and name the supporting evidence node IDs.
4. Explicitly state UNKNOWN for questions the evidence cannot answer.
5. Do not edit files or invoke tools. This is synthesis only.
6. Prefer a concise answer with sections FACTS, INFERENCES, UNKNOWN.

Evidence:
` + string(payload), nil
}
