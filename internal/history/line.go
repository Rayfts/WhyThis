package history

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Rayfts/WhyThis/internal/gitx"
	"github.com/Rayfts/WhyThis/internal/graph"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

func (a *Archaeologist) Line(ctx context.Context, target Target) (evidence.Report, error) {
	head, err := a.Git.Head(ctx)
	if err != nil {
		return evidence.Report{}, err
	}
	raw, err := a.Git.Blame(ctx, target.Path, target.Start, target.End)
	if err != nil {
		return evidence.Report{}, err
	}
	segments := gitx.ParseBlamePorcelain(raw)
	b := graph.New()
	now := time.Now().UTC()
	repoID := "repo:" + a.Git.Dir
	fileID := "file:" + filepath.ToSlash(target.Path)
	rangeID := fmt.Sprintf("range:%s:%d-%d", filepath.ToSlash(target.Path), target.Start, target.End)
	b.AddNode(evidence.Node{ID: repoID, Kind: evidence.KindRepository, Label: filepath.Base(a.Git.Dir), Provenance: prov("git", a.Git.Dir, head, now)})
	b.AddNode(evidence.Node{ID: fileID, Kind: evidence.KindFile, Label: filepath.ToSlash(target.Path), Provenance: prov("git", "HEAD:"+filepath.ToSlash(target.Path), head, now)})
	b.AddNode(evidence.Node{ID: rangeID, Kind: evidence.KindLineRange, Label: fmt.Sprintf("%s:%d-%d", filepath.ToSlash(target.Path), target.Start, target.End), Attributes: map[string]any{"start": target.Start, "end": target.End}, Provenance: prov("git-blame", fmt.Sprintf("git blame -L %d,%d -- %s", target.Start, target.End, target.Path), head, now)})
	b.AddEdge(edge(repoID, fileID, evidence.EdgeDependsOn, head, now, "repository contains file"))
	b.AddEdge(edge(fileID, rangeID, evidence.EdgeAssociatedWith, head, now, "selected line range"))

	ownershipFacts, ownershipUnknowns := a.addCodeOwnership(b, fileID, target.Path, head, now)
	sensitivityFacts := a.pathSensitivityFacts(fileID, target.Path)
	seen := map[string]bool{}
	var timeline []evidence.Node
	facts := append([]evidence.Claim(nil), ownershipFacts...)
	facts = append(facts, sensitivityFacts...)
	for _, seg := range segments {
		sha := strings.TrimPrefix(seg.Commit, "^")
		if sha == "" || isZeroSHA(sha) {
			continue
		}
		blameID := fmt.Sprintf("blame:%s:%d:%s", filepath.ToSlash(target.Path), seg.FinalLine, short(sha))
		b.AddNode(evidence.Node{ID: blameID, Kind: evidence.KindBlameSegment, Label: fmt.Sprintf("line %d → %s", seg.FinalLine, short(sha)), Attributes: map[string]any{"commit": sha, "author": seg.Author, "summary": seg.Summary, "filename": seg.Filename, "content": seg.Content, "previous": seg.Previous}, Provenance: prov("git-blame", target.Path, head, now)})
		b.AddEdge(edge(rangeID, blameID, evidence.EdgeIntroducedBy, head, now, "blame attribution"))
		if !seen[sha] {
			seen[sha] = true
			bundle, err := a.commitEvidence(ctx, sha, now)
			if err == nil {
				addCommitBundle(b, bundle)
				timeline = append(timeline, bundle.Commit)
				facts = append(facts, bundle.Claims...)
				b.AddEdge(edge(blameID, bundle.Commit.ID, evidence.EdgeIntroducedBy, sha, now, "blame points to commit"))
			}
		}
	}

	// git log -L adds line-range evolution that blame alone cannot show. It is
	// path-local, so the broader --follow history below still contributes rename
	// and surrounding file context.
	if lineSHAs, lineErr := a.Git.LineLogSHAs(ctx, target.Path, target.Start, target.End); lineErr == nil {
		for _, sha := range lineSHAs {
			if seen[sha] {
				continue
			}
			seen[sha] = true
			bundle, bundleErr := a.commitEvidence(ctx, sha, now)
			if bundleErr != nil {
				continue
			}
			addCommitBundle(b, bundle)
			timeline = append(timeline, bundle.Commit)
			facts = append(facts, bundle.Claims...)
			facts = append(facts, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Git line-range history identifies commit %s in the evolution of %s:%d-%d.", short(sha), target.Path, target.Start, target.End), EvidenceID: []string{rangeID, bundle.Commit.ID}})
			b.AddEdge(edge(rangeID, bundle.Commit.ID, evidence.EdgeChangedBy, sha, now, "git log -L line-range history"))
		}
	}

	// File history adds subsequent edits, renames, reverts and fix/test context.
	shas, _ := a.fileHistorySHAs(ctx, target.Path)
	for _, sha := range shas {
		if seen[sha] {
			continue
		}
		seen[sha] = true
		bundle, err := a.commitEvidence(ctx, sha, now)
		if err != nil {
			continue
		}
		addCommitBundle(b, bundle)
		timeline = append(timeline, bundle.Commit)
		facts = append(facts, bundle.Claims...)
		b.AddEdge(edge(fileID, bundle.Commit.ID, evidence.EdgeChangedBy, sha, now, "file history"))
	}
	sort.Slice(timeline, func(i, j int) bool {
		iDate, _ := time.Parse(time.RFC3339, fmt.Sprint(timeline[i].Attributes["date"]))
		jDate, _ := time.Parse(time.RFC3339, fmt.Sprint(timeline[j].Attributes["date"]))
		return iDate.Before(jDate)
	})
	if len(facts) == 0 {
		facts = append(facts, evidence.Claim{Class: evidence.Fact, Text: "Git history returned no attributable commit for the selected range."})
	}
	unknowns := append([]evidence.Claim(nil), ownershipUnknowns...)
	unknowns = append(unknowns, a.historyDepthUnknown(ctx)...)
	unknowns = append(unknowns, evidence.Claim{Class: evidence.Unknown, Text: "Intent that was never recorded in Git or linked discussion cannot be established from repository history alone."})
	return evidence.Report{Target: fmt.Sprintf("%s:%d-%d", target.Path, target.Start, target.End), Generated: now, Repository: a.Git.Dir, Revision: head, Graph: b.Build(), Facts: dedupeClaims(facts), Unknowns: dedupeClaims(unknowns), Timeline: timeline}, nil
}
