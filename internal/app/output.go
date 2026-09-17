package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Rayfts/WhyThis/pkg/evidence"
)

func parseCommon(args []string) (commonFlags, []string, error) {
	var c commonFlags
	c.repo = "."
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--json":
			c.json = true
		case "--no-ai":
			c.noAI = true
		case "--repo":
			i++
			if i >= len(args) {
				return c, nil, errors.New("--repo requires a path")
			}
			c.repo = args[i]
		case "--db":
			i++
			if i >= len(args) {
				return c, nil, errors.New("--db requires a path")
			}
			c.db = args[i]
		case "--harness":
			i++
			if i >= len(args) {
				return c, nil, errors.New("--harness requires an id")
			}
			c.harness = args[i]
		case "--help", "-h":
			return c, []string{"help"}, nil
		default:
			rest = append(rest, a)
		}
	}
	return c, rest, nil
}
func argErr(w io.Writer, msg string) int { _, _ = fmt.Fprintln(w, "whythis:", msg); return 2 }
func writeAny(v any, asJSON bool, w io.Writer) int {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(v)
		return 0
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	_, _ = fmt.Fprintln(w, string(b))
	return 0
}
func writeReport(r evidence.Report, asJSON bool, w io.Writer) int {
	if asJSON {
		return writeAny(r, true, w)
	}
	_, _ = fmt.Fprintf(w, "WhyThis — %s\nrevision: %s\n\n", r.Target, short(r.Revision))
	printClaims(w, "FACTS", r.Facts)
	printClaims(w, "INFERENCES", r.Inferences)
	printClaims(w, "UNKNOWN", r.Unknowns)
	if len(r.Timeline) > 0 {
		_, _ = fmt.Fprintln(w, "TIMELINE")
		for _, n := range r.Timeline {
			date := ""
			if n.Attributes != nil {
				date = fmt.Sprint(n.Attributes["date"])
			}
			_, _ = fmt.Fprintf(w, "  %s  %-12s  %s\n", prefixDate(date), strings.TrimPrefix(n.ID, "commit:")[:min(12, len(strings.TrimPrefix(n.ID, "commit:")))], n.Label)
		}
		_, _ = fmt.Fprintln(w)
	}
	_, _ = fmt.Fprintf(w, "evidence: %d nodes, %d edges\n", len(r.Graph.Nodes), len(r.Graph.Edges))
	return 0
}
func printClaims(w io.Writer, title string, claims []evidence.Claim) {
	_, _ = fmt.Fprintln(w, title)
	if len(claims) == 0 {
		_, _ = fmt.Fprintln(w, "  (none)")
		_, _ = fmt.Fprintln(w)
		return
	}
	for _, c := range claims {
		_, _ = fmt.Fprintln(w, " -", c.Text)
		if len(c.EvidenceID) > 0 {
			_, _ = fmt.Fprintln(w, "   evidence:", strings.Join(c.EvidenceID, ", "))
		}
	}
	_, _ = fmt.Fprintln(w)
}
func prefixDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return "          "
}
func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
