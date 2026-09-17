package history

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Rayfts/WhyThis/internal/graph"
	"github.com/Rayfts/WhyThis/internal/ownership"
	"github.com/Rayfts/WhyThis/internal/sensitive"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

func (a *Archaeologist) File(ctx context.Context, path string) (evidence.Report, error) {
	head, err := a.Git.Head(ctx)
	if err != nil {
		return evidence.Report{}, err
	}
	now := time.Now().UTC()
	b := graph.New()
	fileID := "file:" + filepath.ToSlash(path)
	b.AddNode(evidence.Node{ID: fileID, Kind: evidence.KindFile, Label: filepath.ToSlash(path), Provenance: prov("git-log --follow", path, head, now)})
	ownershipFacts, ownershipUnknowns := a.addCodeOwnership(b, fileID, path, head, now)
	sensitivityFacts := a.pathSensitivityFacts(fileID, path)
	shas, err := a.fileHistorySHAs(ctx, path)
	if err != nil {
		return evidence.Report{}, err
	}
	facts := append([]evidence.Claim(nil), ownershipFacts...)
	facts = append(facts, sensitivityFacts...)
	var timeline []evidence.Node
	var prev string
	for _, sha := range shas {
		bundle, e := a.commitEvidence(ctx, sha, now)
		if e != nil {
			continue
		}
		addCommitBundle(b, bundle)
		timeline = append(timeline, bundle.Commit)
		facts = append(facts, bundle.Claims...)
		b.AddEdge(edge(fileID, bundle.Commit.ID, evidence.EdgeChangedBy, sha, now, "git log --follow"))
		if prev != "" {
			b.AddEdge(edge(bundle.Commit.ID, prev, evidence.EdgeFollowedBy, sha, now, "chronological file history"))
		}
		prev = bundle.Commit.ID
	}
	// git log is newest-first; expose chronological timeline.
	sort.Slice(timeline, func(i, j int) bool {
		di, _ := time.Parse(time.RFC3339, fmt.Sprint(timeline[i].Attributes["date"]))
		dj, _ := time.Parse(time.RFC3339, fmt.Sprint(timeline[j].Attributes["date"]))
		return di.Before(dj)
	})
	unknowns := append([]evidence.Claim(nil), ownershipUnknowns...)
	unknowns = append(unknowns, a.historyDepthUnknown(ctx)...)
	unknowns = append(unknowns, evidence.Claim{Class: evidence.Unknown, Text: "File history can show changes and recorded discussion references, but it cannot prove undocumented intent."})
	return evidence.Report{Target: path, Generated: now, Repository: a.Git.Dir, Revision: head, Graph: b.Build(), Facts: dedupeClaims(facts), Timeline: timeline, Unknowns: dedupeClaims(unknowns)}, nil
}

func (a *Archaeologist) pathSensitivityFacts(fileID, path string) []evidence.Claim {
	r := sensitive.Analyze(a.Git.Dir, path)
	var out []evidence.Claim
	if r.Sensitive {
		categories := make([]string, 0, len(r.Findings))
		seen := map[string]bool{}
		for _, f := range r.Findings {
			if !seen[f.Category] {
				seen[f.Category] = true
				categories = append(categories, f.Category)
			}
		}
		out = append(out, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Deterministic sensitive-code classification flags %s for: %s.", filepath.ToSlash(path), strings.Join(categories, ", ")), EvidenceID: []string{fileID}})
	}
	if r.Generated {
		out = append(out, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Path classification marks %s as generated code.", filepath.ToSlash(path)), EvidenceID: []string{fileID}})
	}
	if r.Vendor {
		out = append(out, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Path classification marks %s as vendored/third-party code; repository-local history may not represent upstream history.", filepath.ToSlash(path)), EvidenceID: []string{fileID}})
	}
	return out
}

func (a *Archaeologist) historyDepthUnknown(ctx context.Context) []evidence.Claim {
	shallow, err := a.Git.IsShallow(ctx)
	if err != nil {
		return []evidence.Claim{{Class: evidence.Unknown, Text: "WhyThis could not determine whether the local clone is shallow: " + err.Error()}}
	}
	if shallow {
		return []evidence.Claim{{Class: evidence.Unknown, Text: "The repository is a shallow clone; evidence only covers history present locally and may omit older commits, renames, reverts, releases, or discussions."}}
	}
	return nil
}

func (a *Archaeologist) addCodeOwnership(b *graph.Builder, fileID, path, head string, now time.Time) ([]evidence.Claim, []evidence.Claim) {
	resolved, err := ownership.Resolve(a.Git.Dir, path)
	if err != nil {
		return nil, []evidence.Claim{{Class: evidence.Unknown, Text: "CODEOWNERS evidence could not be evaluated: " + err.Error()}}
	}
	if !resolved.Configured {
		return nil, nil
	}
	sourceID := "documentation:" + resolved.File
	b.AddNode(evidence.Node{ID: sourceID, Kind: evidence.KindDocumentation, Label: resolved.File, Attributes: map[string]any{"codeowners": true}, Provenance: prov("codeowners", resolved.File, head, now)})
	b.AddEdge(edge(fileID, sourceID, evidence.EdgeReferences, head, now, "CODEOWNERS source"))
	if !resolved.Matched {
		return []evidence.Claim{{Class: evidence.Fact, Text: fmt.Sprintf("%s exists but no CODEOWNERS rule matches %s.", resolved.File, filepath.ToSlash(path)), EvidenceID: []string{fileID, sourceID}}}, nil
	}
	claims := []evidence.Claim{{Class: evidence.Fact, Text: fmt.Sprintf("CODEOWNERS rule %q on line %d of %s matches %s.", resolved.Pattern, resolved.Line, resolved.File, filepath.ToSlash(path)), EvidenceID: []string{fileID, sourceID}}}
	for _, owner := range resolved.Owners {
		ownerID := "author:codeowner:" + strings.ToLower(owner)
		b.AddNode(evidence.Node{ID: ownerID, Kind: evidence.KindAuthor, Label: owner, Attributes: map[string]any{"codeowner": true, "rule": resolved.Pattern, "source": resolved.File}, Provenance: prov("codeowners", fmt.Sprintf("%s:%d", resolved.File, resolved.Line), head, now)})
		b.AddEdge(edge(fileID, ownerID, evidence.EdgeAssociatedWith, head, now, "CODEOWNERS assignment"))
	}
	if len(resolved.Owners) == 0 {
		claims = append(claims, evidence.Claim{Class: evidence.Fact, Text: "The matching CODEOWNERS rule deliberately assigns no owner to this path.", EvidenceID: []string{fileID, sourceID}})
	} else {
		claims = append(claims, evidence.Claim{Class: evidence.Fact, Text: "Configured code owners: " + strings.Join(resolved.Owners, ", ") + ".", EvidenceID: []string{fileID, sourceID}})
	}
	return claims, nil
}
