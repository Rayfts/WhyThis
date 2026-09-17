package history

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Rayfts/WhyThis/internal/gitx"
	"github.com/Rayfts/WhyThis/internal/graph"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

func (a *Archaeologist) Commit(ctx context.Context, sha string) (evidence.Report, error) {
	head, err := a.Git.Head(ctx)
	if err != nil {
		return evidence.Report{}, err
	}
	now := time.Now().UTC()
	b := graph.New()
	bundle, err := a.commitEvidence(ctx, sha, now)
	if err != nil {
		return evidence.Report{}, err
	}
	addCommitBundle(b, bundle)
	facts := append([]evidence.Claim(nil), bundle.Claims...)
	timeline := []evidence.Node{bundle.Commit}
	unknowns := []evidence.Claim{{Class: evidence.Unknown, Text: "A commit message can document intent, but unrecorded motivations remain unknown."}}
	unknowns = append(unknowns, a.historyDepthUnknown(ctx)...)

	seen := map[string]bool{strings.TrimPrefix(bundle.Commit.ID, "commit:"): true}
	current := bundle
	links := 0
	for links < 8 {
		target := explicitRevertTarget(current)
		if target == "" || seen[target] {
			break
		}
		seen[target] = true
		next, nextErr := a.commitEvidence(ctx, target, now)
		if nextErr != nil {
			unknowns = append(unknowns, evidence.Claim{Class: evidence.Unknown, Text: fmt.Sprintf("Explicit revert target %s could not be materialized from local history: %v", short(target), nextErr)})
			break
		}
		addCommitBundle(b, next)
		facts = append(facts, next.Claims...)
		timeline = append(timeline, next.Commit)
		current = next
		links++
	}
	if links > 0 {
		facts = append(facts, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Explicit Git revert trailers form a %d-link revert chain from the selected commit.", links), EvidenceID: commitIDs(timeline)})
	}
	if links == 8 && explicitRevertTarget(current) != "" {
		unknowns = append(unknowns, evidence.Claim{Class: evidence.Unknown, Text: "Revert-chain traversal reached the safety bound of 8 explicit links; older links were not expanded."})
	}
	sort.Slice(timeline, func(i, j int) bool {
		return fmt.Sprint(timeline[i].Attributes["date"]) < fmt.Sprint(timeline[j].Attributes["date"])
	})
	return evidence.Report{Target: sha, Generated: now, Repository: a.Git.Dir, Revision: head, Graph: b.Build(), Facts: dedupeClaims(facts), Timeline: timeline, Unknowns: dedupeClaims(unknowns)}, nil
}

func (a *Archaeologist) commitEvidence(ctx context.Context, sha string, now time.Time) (commitBundle, error) {
	raw, err := a.Git.Commit(ctx, sha)
	if err != nil {
		return commitBundle{}, err
	}
	c := gitx.ParseCommitShow(raw)
	if c.SHA == "" {
		return commitBundle{}, fmt.Errorf("could not parse commit %s", sha)
	}
	id := "commit:" + c.SHA
	attrs := map[string]any{"sha": c.SHA, "parents": c.Parents, "author": c.Author, "email": c.Email, "date": c.Date.Format(time.RFC3339), "subject": c.Subject, "body": c.Body, "changes": c.Changes}
	lowerMessage := strings.ToLower(c.Subject + "\n" + c.Body)
	if strings.Contains(lowerMessage, "regress") {
		attrs["regression_marker"] = true
	}
	if strings.Contains(lowerMessage, "fix") || strings.Contains(lowerMessage, "bug") {
		attrs["fix_like_marker"] = true
	}
	if len(c.Parents) > 1 {
		attrs["merge_commit"] = true
	}
	n := evidence.Node{ID: id, Kind: evidence.KindCommit, Label: c.Subject, Attributes: attrs, Provenance: prov("git-show", "git show "+c.SHA, c.SHA, now)}
	claims := []evidence.Claim{{Class: evidence.Fact, Text: fmt.Sprintf("Commit %s (%s) changed the relevant history: %s", short(c.SHA), c.Date.Format("2006-01-02"), c.Subject), EvidenceID: []string{id}}}
	if attrs["regression_marker"] == true {
		claims = append(claims, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Commit %s contains explicit regression language in its recorded message.", short(c.SHA)), EvidenceID: []string{id}})
	}
	if len(c.Parents) > 1 {
		claims = append(claims, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Commit %s is a merge commit with %d parents; WhyThis preserves the DAG rather than flattening it.", short(c.SHA), len(c.Parents)), EvidenceID: []string{id}})
	}
	var edges []evidence.Edge
	var related []evidence.Node
	authorKey := strings.ToLower(strings.TrimSpace(c.Email))
	if authorKey == "" {
		authorKey = strings.ToLower(strings.TrimSpace(c.Author))
	}
	if authorKey != "" {
		authorID := "author:" + authorKey
		related = append(related, evidence.Node{ID: authorID, Kind: evidence.KindAuthor, Label: c.Author, Attributes: map[string]any{"name": c.Author, "email": c.Email}, Provenance: prov("git-show", "commit author", c.SHA, now)})
		edges = append(edges, edge(id, authorID, evidence.EdgeAssociatedWith, c.SHA, now, "Git commit author"))
		claims = append(claims, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Git records %s as the author of commit %s.", c.Author, short(c.SHA)), EvidenceID: []string{id, authorID}})
	}
	for _, m := range issueRef.FindAllStringSubmatch(c.Subject+"\n"+c.Body, -1) {
		if len(m) < 2 {
			continue
		}
		issueID := "issue:" + m[1]
		related = append(related, evidence.Node{ID: issueID, Kind: evidence.KindIssue, Label: "#" + m[1], Attributes: map[string]any{"reference_only": true}, Provenance: prov("git-show", "commit message reference", c.SHA, now)})
		edges = append(edges, edge(id, issueID, evidence.EdgeReferences, c.SHA, now, "commit message reference"))
		claims = append(claims, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Commit %s references issue/PR #%s in its message.", short(c.SHA), m[1]), EvidenceID: []string{id, issueID}})
	}
	if m := revertRef.FindStringSubmatch(c.Body); len(m) == 2 {
		target := "commit:" + m[1]
		related = append(related, evidence.Node{ID: target, Kind: evidence.KindCommit, Label: "reverted commit " + short(m[1]), Attributes: map[string]any{"sha": m[1], "reference_only": true}, Provenance: prov("git-show", "explicit git revert trailer", c.SHA, now)})
		edges = append(edges, edge(target, id, evidence.EdgeRevertedBy, c.SHA, now, "explicit git revert trailer"))
		claims = append(claims, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Commit %s explicitly states that it reverts %s.", short(c.SHA), short(m[1])), EvidenceID: []string{id, target}})
	} else if strings.HasPrefix(strings.ToLower(c.Subject), "revert ") {
		claims = append(claims, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Commit %s has a revert-form subject; no explicit reverted SHA was parsed from its body.", short(c.SHA)), EvidenceID: []string{id}})
	}
	for _, fc := range c.Changes {
		pathID := "file:" + fc.Path
		status := strings.ToUpper(fc.Status)
		if fc.Path != "" && !strings.HasPrefix(status, "R") {
			attrs := map[string]any{"status": fc.Status}
			if strings.HasPrefix(status, "D") {
				attrs["historical_path"] = true
			}
			related = append(related, evidence.Node{ID: pathID, Kind: evidence.KindFile, Label: fc.Path, Attributes: attrs, Provenance: prov("git-show", "changed path", c.SHA, now)})
			switch {
			case strings.HasPrefix(status, "A"):
				edges = append(edges, edge(pathID, id, evidence.EdgeIntroducedBy, c.SHA, now, "path added by commit"))
			case strings.HasPrefix(status, "D"):
				edges = append(edges, edge(pathID, id, evidence.EdgeRemovedBy, c.SHA, now, "path removed by commit"))
			default:
				edges = append(edges, edge(pathID, id, evidence.EdgeChangedBy, c.SHA, now, "path changed by commit"))
			}
		}
		if strings.HasPrefix(fc.Status, "R") && fc.Old != "" {
			oldID := "file:" + fc.Old
			newID := "file:" + fc.Path
			related = append(related,
				evidence.Node{ID: oldID, Kind: evidence.KindFile, Label: fc.Old, Attributes: map[string]any{"historical_path": true}, Provenance: prov("git-show", "rename source", c.SHA, now)},
				evidence.Node{ID: newID, Kind: evidence.KindFile, Label: fc.Path, Provenance: prov("git-show", "rename destination", c.SHA, now)},
			)
			edges = append(edges, edge(newID, oldID, evidence.EdgeRenamedFrom, c.SHA, now, "git rename detection"))
			claims = append(claims, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Git rename detection records %s → %s in %s.", fc.Old, fc.Path, short(c.SHA)), EvidenceID: []string{id}})
		}
		if isTestPath(fc.Path) {
			testID := "test:" + fc.Path
			related = append(related, evidence.Node{ID: testID, Kind: evidence.KindTest, Label: fc.Path, Provenance: prov("git-show", "test-like file changed in same commit", c.SHA, now)})
			edges = append(edges, edge(id, testID, evidence.EdgeTestedBy, c.SHA, now, "test file changed in same commit"))
			claims = append(claims, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("The same commit changed test-like file %s.", fc.Path), EvidenceID: []string{id, testID}})
		}
	}
	return commitBundle{Commit: n, Related: dedupeNodes(related), Claims: claims, Edges: edges}, nil
}

func explicitRevertTarget(bundle commitBundle) string {
	for _, e := range bundle.Edges {
		if e.Kind == evidence.EdgeRevertedBy && e.To == bundle.Commit.ID && strings.HasPrefix(e.From, "commit:") {
			return strings.TrimPrefix(e.From, "commit:")
		}
	}
	return ""
}

func commitIDs(nodes []evidence.Node) []string {
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.Kind == evidence.KindCommit {
			ids = append(ids, n.ID)
		}
	}
	return ids
}
