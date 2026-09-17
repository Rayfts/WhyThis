package history

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
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
