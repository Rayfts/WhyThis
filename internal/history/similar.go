package history

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Rayfts/WhyThis/internal/graph"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

const (
	similarCandidateLimit  = 2000
	similarPatchShortlist  = 120
	similarRecencyFallback = 40
)

type similarHit struct {
	sha, patchID        string
	exact               bool
	score, paths, lines float64
	bundle              commitBundle
}

// Similar ranks related historical patches using stable patch-id equality and
// normalized diff overlap. The measurements are facts; shared intent is not.
func (a *Archaeologist) Similar(ctx context.Context, sha string, limit int) (evidence.Report, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	head, err := a.Git.Head(ctx)
	if err != nil {
		return evidence.Report{}, err
	}
	now := time.Now().UTC()
	targetBundle, err := a.commitEvidence(ctx, sha, now)
	if err != nil {
		return evidence.Report{}, err
	}
	target := targetBundle.Commit
	facts := append([]evidence.Claim(nil), targetBundle.Claims...)
	targetSHA := fmt.Sprint(target.Attributes["sha"])
	patch, err := a.Git.Patch(ctx, targetSHA)
	if err != nil {
		return evidence.Report{}, err
	}
	pid, err := a.Git.PatchID(ctx, targetSHA)
	if err != nil {
		return evidence.Report{}, err
	}
	tsig := patchSig(patch)
	pathIndex, order, err := a.Git.RecentCommitPaths(ctx, similarCandidateLimit)
	if err != nil {
		return evidence.Report{}, err
	}
	shas := shortlistCandidates(order, pathIndex, tsig.paths, targetSHA, similarPatchShortlist, similarRecencyFallback)
	var hits []similarHit
	for _, candidate := range shas {
		if candidate == targetSHA {
			continue
		}
		cp, e := a.Git.Patch(ctx, candidate)
		if e != nil || strings.TrimSpace(cp) == "" {
			continue
		}
		csig := patchSig(cp)
		pathScore, lineScore := overlap(tsig.paths, csig.paths), overlap(tsig.lines, csig.lines)
		cpid, e := a.Git.PatchID(ctx, candidate)
		if e != nil {
			continue
		}
		exact := pid != "" && pid == cpid
		score := .45*pathScore + .55*lineScore
		if exact {
			score = 1
		}
		if !exact && score < .20 {
			continue
		}
		bundle, e := a.commitEvidence(ctx, candidate, now)
		if e != nil {
			continue
		}
		hits = append(hits, similarHit{sha: candidate, patchID: cpid, exact: exact, score: score, paths: pathScore, lines: lineScore, bundle: bundle})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].exact != hits[j].exact {
			return hits[i].exact
		}
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].sha < hits[j].sha
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	b := graph.New()
	addCommitBundle(b, targetBundle)
	timeline := []evidence.Node{target}
	for _, h := range hits {
		if h.bundle.Commit.Attributes == nil {
			h.bundle.Commit.Attributes = map[string]any{}
		}
		h.bundle.Commit.Attributes["similarity_score"], h.bundle.Commit.Attributes["path_overlap"] = h.score, h.paths
		h.bundle.Commit.Attributes["changed_line_overlap"], h.bundle.Commit.Attributes["patch_id"] = h.lines, h.patchID
		h.bundle.Commit.Attributes["exact_patch_id_match"] = h.exact
		addCommitBundle(b, h.bundle)
		b.AddEdge(evidence.Edge{From: target.ID, To: h.bundle.Commit.ID, Kind: evidence.EdgeAssociatedWith, Attributes: map[string]any{"reason": "deterministic patch similarity", "score": h.score, "exact_patch_id_match": h.exact}, Provenance: prov("git-patch-similarity", "git patch-id --stable + normalized diff overlap", head, now)})
		timeline = append(timeline, h.bundle.Commit)
		if h.exact {
			facts = append(facts, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Commit %s has the same stable Git patch-id as %s.", short(h.sha), short(targetSHA)), EvidenceID: []string{target.ID, h.bundle.Commit.ID}})
		} else {
			facts = append(facts, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Commit %s has deterministic structural patch overlap %.0f%% with %s (path %.0f%%, changed-line %.0f%%).", short(h.sha), h.score*100, short(targetSHA), h.paths*100, h.lines*100), EvidenceID: []string{target.ID, h.bundle.Commit.ID}})
		}
	}
	unknown := []evidence.Claim{{Class: evidence.Unknown, Text: "Patch similarity does not establish shared intent or causality; matching rationale requires explicit history or discussion evidence."}}
	unknown = append(unknown, a.historyDepthUnknown(ctx)...)
	if len(hits) == 0 {
		unknown = append(unknown, evidence.Claim{Class: evidence.Unknown, Text: fmt.Sprintf("No similar patch was found after prefiltering the most recent %d commits traversed across refs.", similarCandidateLimit)})
	}
	return evidence.Report{Target: "similar patches to " + targetSHA, Generated: now, Repository: a.Git.Dir, Revision: head, Graph: b.Build(), Facts: dedupeClaims(facts), Unknowns: unknown, Timeline: timeline}, nil
}

type diffSig struct{ paths, lines map[string]struct{} }

func patchSig(p string) diffSig {
	s := diffSig{map[string]struct{}{}, map[string]struct{}{}}
	for _, line := range strings.Split(p, "\n") {
		if strings.HasPrefix(line, "diff --git a/") {
			f := strings.Fields(line)
			if len(f) >= 4 {
				s.paths[strings.TrimPrefix(f[3], "b/")] = struct{}{}
			}
			continue
		}
		if (strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-")) && !strings.HasPrefix(line, "+++") && !strings.HasPrefix(line, "---") {
			v := strings.Join(strings.Fields(strings.TrimSpace(line[1:])), " ")
			if v != "" {
				s.lines[v] = struct{}{}
			}
		}
	}
	return s
}
func overlap(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	n := 0
	u := map[string]struct{}{}
	for k := range a {
		u[k] = struct{}{}
		if _, ok := b[k]; ok {
			n++
		}
	}
	for k := range b {
		u[k] = struct{}{}
	}
	return float64(n) / float64(len(u))
}

func shortlistCandidates(order []string, paths map[string]map[string]struct{}, target map[string]struct{}, targetSHA string, limit, fallback int) []string {
	type scored struct {
		sha   string
		score float64
		pos   int
	}
	var ranked []scored
	var recency []string
	for i, sha := range order {
		if sha == targetSHA {
			continue
		}
		if len(recency) < fallback {
			recency = append(recency, sha)
		}
		score := overlap(target, paths[sha])
		if score > 0 {
			ranked = append(ranked, scored{sha: sha, score: score, pos: i})
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].pos < ranked[j].pos
	})
	seen := map[string]bool{}
	out := make([]string, 0, limit+fallback)
	for _, r := range ranked {
		if len(out) >= limit {
			break
		}
		seen[r.sha] = true
		out = append(out, r.sha)
	}
	for _, sha := range recency {
		if seen[sha] {
			continue
		}
		seen[sha] = true
		out = append(out, sha)
	}
	return out
}
