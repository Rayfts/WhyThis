package history

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Rayfts/WhyThis/internal/gitx"
	"github.com/Rayfts/WhyThis/internal/graph"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

type Archaeologist struct {
	Git *gitx.Runner
}

type commitBundle struct {
	Commit  evidence.Node
	Related []evidence.Node
	Claims  []evidence.Claim
	Edges   []evidence.Edge
}

func addCommitBundle(b *graph.Builder, bundle commitBundle) {
	b.AddNode(bundle.Commit)
	for _, n := range bundle.Related {
		b.AddNode(n)
	}
	for _, e := range bundle.Edges {
		b.AddEdge(e)
	}
}

type Target struct {
	Path  string
	Start int
	End   int
}

var issueRef = regexp.MustCompile(`(?i)(?:#|GH-)([0-9]+)`)
var revertRef = regexp.MustCompile(`(?i)this reverts commit ([0-9a-f]{7,64})`)

func ParseTarget(input string) (Target, error) {
	idx := strings.LastIndex(input, ":")
	if idx < 0 {
		return Target{Path: filepath.Clean(input), Start: 1, End: 1}, nil
	}
	path := input[:idx]
	rangePart := input[idx+1:]
	parts := strings.SplitN(rangePart, "-", 2)
	start, err := strconv.Atoi(parts[0])
	if err != nil || start <= 0 {
		return Target{}, fmt.Errorf("invalid line target %q", input)
	}
	end := start
	if len(parts) == 2 {
		end, err = strconv.Atoi(parts[1])
		if err != nil || end < start {
			return Target{}, fmt.Errorf("invalid line range %q", input)
		}
	}
	return Target{Path: filepath.Clean(path), Start: start, End: end}, nil
}

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

	seen := map[string]bool{}
	var timeline []evidence.Node
	var facts []evidence.Claim
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
	return evidence.Report{Target: fmt.Sprintf("%s:%d-%d", target.Path, target.Start, target.End), Generated: now, Repository: a.Git.Dir, Revision: head, Graph: b.Build(), Facts: dedupeClaims(facts), Unknowns: []evidence.Claim{{Class: evidence.Unknown, Text: "Intent that was never recorded in Git or linked discussion cannot be established from repository history alone."}}, Timeline: timeline}, nil
}

func (a *Archaeologist) Symbol(ctx context.Context, symbol string) (evidence.Report, error) {
	head, err := a.Git.Head(ctx)
	if err != nil {
		return evidence.Report{}, err
	}
	raw, err := a.Git.Output(ctx, "log", "--all", "--date=iso-strict", "--format=%H", "-S", symbol, "--pickaxe-regex", "--find-renames")
	if err != nil {
		return evidence.Report{}, err
	}
	now := time.Now().UTC()
	b := graph.New()
	symID := "symbol:" + symbol
	b.AddNode(evidence.Node{ID: symID, Kind: evidence.KindSymbol, Label: symbol, Provenance: prov("git-pickaxe", "git log -S "+symbol, head, now)})
	var timeline []evidence.Node
	var facts []evidence.Claim
	for _, sha := range nonEmptyLines(raw) {
		bundle, e := a.commitEvidence(ctx, sha, now)
		if e != nil {
			continue
		}
		addCommitBundle(b, bundle)
		timeline = append(timeline, bundle.Commit)
		facts = append(facts, bundle.Claims...)
		b.AddEdge(edge(symID, bundle.Commit.ID, evidence.EdgeChangedBy, sha, now, "pickaxe matched symbol text"))
	}
	if len(timeline) == 0 {
		return evidence.Report{Target: symbol, Generated: now, Repository: a.Git.Dir, Revision: head, Graph: b.Build(), Unknowns: []evidence.Claim{{Class: evidence.Unknown, Text: "No Git pickaxe match was found for this symbol text; it may have been generated, renamed, or represented differently historically."}}}, nil
	}
	return evidence.Report{Target: symbol, Generated: now, Repository: a.Git.Dir, Revision: head, Graph: b.Build(), Facts: dedupeClaims(facts), Unknowns: []evidence.Claim{{Class: evidence.Unknown, Text: "Symbol identity across semantic renames is inferred from textual history unless language-aware indexing confirms it."}}, Timeline: timeline}, nil
}

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
	return evidence.Report{Target: sha, Generated: now, Repository: a.Git.Dir, Revision: head, Graph: b.Build(), Facts: bundle.Claims, Timeline: []evidence.Node{bundle.Commit}, Unknowns: []evidence.Claim{{Class: evidence.Unknown, Text: "A commit message can document intent, but unrecorded motivations remain unknown."}}}, nil
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
	n := evidence.Node{ID: id, Kind: evidence.KindCommit, Label: c.Subject, Attributes: attrs, Provenance: prov("git-show", "git show "+c.SHA, c.SHA, now)}
	claims := []evidence.Claim{{Class: evidence.Fact, Text: fmt.Sprintf("Commit %s (%s) changed the relevant history: %s", short(c.SHA), c.Date.Format("2006-01-02"), c.Subject), EvidenceID: []string{id}}}
	var edges []evidence.Edge
	var related []evidence.Node
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

func dedupeNodes(in []evidence.Node) []evidence.Node {
	seen := make(map[string]struct{}, len(in))
	out := make([]evidence.Node, 0, len(in))
	for _, n := range in {
		if _, ok := seen[n.ID]; ok {
			continue
		}
		seen[n.ID] = struct{}{}
		out = append(out, n)
	}
	return out
}

func (a *Archaeologist) fileHistorySHAs(ctx context.Context, path string) ([]string, error) {
	out, err := a.Git.Output(ctx, "log", "--follow", "--format=%H", "--find-renames", "--", filepath.ToSlash(path))
	if err != nil {
		return nil, err
	}
	return nonEmptyLines(out), nil
}
func prov(source, locator, rev string, at time.Time) evidence.Provenance {
	return evidence.Provenance{Source: source, Locator: locator, Repository: "local", Revision: rev, CollectedAt: at}
}
func edge(from, to string, kind evidence.EdgeKind, rev string, at time.Time, reason string) evidence.Edge {
	return evidence.Edge{From: from, To: to, Kind: kind, Attributes: map[string]any{"reason": reason}, Provenance: prov("git", reason, rev, at)}
}
func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
func isZeroSHA(s string) bool { return strings.Trim(s, "0") == "" }
func isTestPath(p string) bool {
	p = strings.ToLower(filepath.ToSlash(p))
	return strings.Contains(p, "/test/") || strings.Contains(p, "/tests/") || strings.HasSuffix(p, "_test.go") || strings.Contains(filepath.Base(p), "test") || strings.Contains(filepath.Base(p), "spec")
}
func dedupeClaims(in []evidence.Claim) []evidence.Claim {
	seen := map[string]bool{}
	out := make([]evidence.Claim, 0, len(in))
	for _, c := range in {
		if !seen[c.Text] {
			seen[c.Text] = true
			out = append(out, c)
		}
	}
	return out
}

func (a *Archaeologist) File(ctx context.Context, path string) (evidence.Report, error) {
	head, err := a.Git.Head(ctx)
	if err != nil {
		return evidence.Report{}, err
	}
	now := time.Now().UTC()
	b := graph.New()
	fileID := "file:" + filepath.ToSlash(path)
	b.AddNode(evidence.Node{ID: fileID, Kind: evidence.KindFile, Label: filepath.ToSlash(path), Provenance: prov("git-log --follow", path, head, now)})
	shas, err := a.fileHistorySHAs(ctx, path)
	if err != nil {
		return evidence.Report{}, err
	}
	var facts []evidence.Claim
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
	return evidence.Report{Target: path, Generated: now, Repository: a.Git.Dir, Revision: head, Graph: b.Build(), Facts: dedupeClaims(facts), Timeline: timeline, Unknowns: []evidence.Claim{{Class: evidence.Unknown, Text: "File history can show changes and recorded discussion references, but it cannot prove undocumented intent."}}}, nil
}

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
