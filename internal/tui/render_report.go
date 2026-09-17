package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Rayfts/WhyThis/pkg/evidence"
)

func header(w io.Writer, repo string) {
	_, _ = fmt.Fprintln(w, "+--------------------------------------------------------------------+")
	_, _ = fmt.Fprintln(w, "| WhyThis - evidence-first code archaeology                          |")
	_, _ = fmt.Fprintf(w, "| repo: %-60s |\n", trim(filepath.Base(repo), 60))
	_, _ = fmt.Fprintln(w, "+--------------------------------------------------------------------+")
}

func renderReport(w io.Writer, repoRoot string, r evidence.Report) {
	_, _ = fmt.Fprintf(w, "\nTARGET  %s\nREV     %s\n", r.Target, short(r.Revision))
	renderSource(w, repoRoot, r.Target)
	renderClaims(w, "FACTS", r.Facts)
	renderClaims(w, "INFERENCES", r.Inferences)
	renderClaims(w, "UNKNOWN", r.Unknowns)
	_, _ = fmt.Fprintln(w, "\nTIMELINE")
	if len(r.Timeline) == 0 {
		_, _ = fmt.Fprintln(w, "  (none)")
	}
	for _, n := range tailNodes(r.Timeline, 12) {
		_, _ = fmt.Fprintf(w, "  %-10s %-12s %s\n", prefix(attr(n, "date"), 10), short(strings.TrimPrefix(n.ID, "commit:")), trim(n.Label, 70))
	}
	_, _ = fmt.Fprintln(w, "\nCOMMIT DETAILS")
	if len(r.Timeline) == 0 {
		_, _ = fmt.Fprintln(w, "  (none)")
	}
	for _, n := range tailNodes(r.Timeline, 5) {
		_, _ = fmt.Fprintf(w, "  %s\n    author: %s <%s>\n    date:   %s\n", trim(n.Label, 78), attr(n, "author"), attr(n, "email"), attr(n, "date"))
	}
	renderGraphSummary(w, r)
}

func renderGraphSummary(w io.Writer, r evidence.Report) {
	_, _ = fmt.Fprintln(w, "\nBLAME")
	count := 0
	for _, n := range r.Graph.Nodes {
		if n.Kind == evidence.KindBlameSegment {
			_, _ = fmt.Fprintf(w, "  %s - %s\n", n.Label, trim(fmt.Sprint(n.Attributes["summary"]), 70))
			count++
		}
	}
	if count == 0 {
		_, _ = fmt.Fprintln(w, "  (none in this view)")
	}
	_, _ = fmt.Fprintln(w, "\nLINKED PR / ISSUE / REVIEW EVIDENCE")
	count = 0
	for _, n := range r.Graph.Nodes {
		if n.Kind == evidence.KindPullRequest || n.Kind == evidence.KindIssue || n.Kind == evidence.KindReviewComment || n.Kind == evidence.KindIssueComment || n.Kind == evidence.KindRelease || n.Kind == evidence.KindCIFailure {
			_, _ = fmt.Fprintf(w, "  %-15s %s\n", n.Kind, trim(n.Label, 78))
			count++
		}
	}
	if count == 0 {
		_, _ = fmt.Fprintln(w, "  (none)")
	}
	_, _ = fmt.Fprintf(w, "\nEVIDENCE GRAPH  %d nodes / %d edges\n", len(r.Graph.Nodes), len(r.Graph.Edges))
	for _, e := range tailEdges(r.Graph.Edges, 12) {
		_, _ = fmt.Fprintf(w, "  %s --%s--> %s\n", trim(e.From, 31), e.Kind, trim(e.To, 31))
	}
}

func renderClaims(w io.Writer, title string, claims []evidence.Claim) {
	_, _ = fmt.Fprintln(w, "\n"+title)
	if len(claims) == 0 {
		_, _ = fmt.Fprintln(w, "  (none)")
		return
	}
	for _, c := range claims {
		_, _ = fmt.Fprintln(w, "  - "+c.Text)
	}
}

func renderSource(w io.Writer, repoRoot, target string) {
	idx := strings.LastIndex(target, ":")
	if idx < 0 {
		return
	}
	path := target[:idx]
	start, err := strconv.Atoi(strings.SplitN(target[idx+1:], "-", 2)[0])
	if err != nil {
		return
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(path)))
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	lo, hi := start-3, start+3
	if lo < 1 {
		lo = 1
	}
	if hi > len(lines) {
		hi = len(lines)
	}
	_, _ = fmt.Fprintln(w, "\nSOURCE")
	for i := lo; i <= hi; i++ {
		mark := " "
		if i == start {
			mark = ">"
		}
		_, _ = fmt.Fprintf(w, " %s %5d | %s\n", mark, i, lines[i-1])
	}
}
