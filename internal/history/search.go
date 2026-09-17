package history

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/Rayfts/WhyThis/internal/graph"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

func (a *Archaeologist) SearchQuestion(ctx context.Context, q string) (evidence.Report, error) {
	head, err := a.Git.Head(ctx)
	if err != nil {
		return evidence.Report{}, err
	}
	now := time.Now().UTC()
	b := graph.New()
	rootID := "query:" + strings.ToLower(strings.TrimSpace(q))
	b.AddNode(evidence.Node{ID: rootID, Kind: evidence.KindDocumentation, Label: q, Provenance: prov("user-query", q, head, now)})
	terms := queryTerms(q)
	seen := map[string]bool{}
	var facts []evidence.Claim
	var timeline []evidence.Node
	for _, term := range terms {
		outputs := []string{}
		if out, e := a.Git.Output(ctx, "log", "--all", "--regexp-ignore-case", "--format=%H", "--grep", term); e == nil {
			outputs = append(outputs, out)
		}
		if out, e := a.Git.Output(ctx, "log", "--all", "--format=%H", "-G", regexp.QuoteMeta(term)); e == nil {
			outputs = append(outputs, out)
		}
		for _, out := range outputs {
			for _, sha := range nonEmptyLines(out) {
				if seen[sha] {
					continue
				}
				seen[sha] = true
				bundle, e := a.commitEvidence(ctx, sha, now)
				if e != nil {
					continue
				}
				addCommitBundle(b, bundle)
				timeline = append(timeline, bundle.Commit)
				facts = append(facts, bundle.Claims...)
				b.AddEdge(edge(rootID, bundle.Commit.ID, evidence.EdgeAssociatedWith, sha, now, "query term matched commit message or patch: "+term))
			}
		}
	}
	if len(timeline) > 50 {
		timeline = timeline[:50]
	}
	unknown := []evidence.Claim{}
	if len(timeline) == 0 {
		unknown = append(unknown, evidence.Claim{Class: evidence.Unknown, Text: "No commit message or patch matched the significant words in the question. Provide a file:line or symbol target for stronger archaeology."})
	} else {
		unknown = append(unknown, evidence.Claim{Class: evidence.Unknown, Text: "Textual query matches establish relevance, not causality; causal explanations remain inference unless explicitly documented."})
	}
	return evidence.Report{Target: q, Generated: now, Repository: a.Git.Dir, Revision: head, Graph: b.Build(), Facts: dedupeClaims(facts), Timeline: timeline, Unknowns: unknown}, nil
}

func queryTerms(q string) []string {
	stop := map[string]bool{"why": true, "this": true, "that": true, "here": true, "code": true, "does": true, "was": true, "were": true, "what": true, "when": true, "where": true, "with": true, "from": true, "have": true, "been": true, "added": true, "before": true, "because": true, "into": true, "there": true}
	clean := regexp.MustCompile(`[^A-Za-z0-9_]+`).ReplaceAllString(strings.ToLower(q), " ")
	seen := map[string]bool{}
	var out []string
	for _, w := range strings.Fields(clean) {
		if len(w) < 4 || stop[w] || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
		if len(out) == 5 {
			break
		}
	}
	return out
}
