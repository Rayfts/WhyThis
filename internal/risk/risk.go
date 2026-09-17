package risk

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Rayfts/WhyThis/internal/gitx"
)

type Severity string

const (
	Info     Severity = "info"
	Watch    Severity = "watch"
	Elevated Severity = "elevated"
)

type Indicator struct {
	ID          string   `json:"id"`
	Severity    Severity `json:"severity"`
	Observed    int      `json:"observed"`
	Threshold   int      `json:"threshold"`
	Explanation string   `json:"explanation"`
}

type Report struct {
	Path       string      `json:"path"`
	Commits    int         `json:"commits"`
	FixLike    int         `json:"fix_like_commits"`
	Reverts    int         `json:"reverts"`
	Authors    int         `json:"unique_authors"`
	Churn      int         `json:"churn_lines"`
	Indicators []Indicator `json:"indicators"`
}

func Analyze(ctx context.Context, g *gitx.Runner, path string) (Report, error) {
	out, err := g.Output(ctx, "log", "--follow", "--format=%H%x1f%aE%x1f%s", "--numstat", "--", path)
	if err != nil {
		return Report{}, err
	}
	r := Report{Path: path}
	authors := map[string]bool{}
	seenCommits := map[string]bool{}
	s := bufio.NewScanner(strings.NewReader(out))
	for s.Scan() {
		line := s.Text()
		if strings.Contains(line, "\x1f") {
			f := strings.Split(line, "\x1f")
			if len(f) >= 3 {
				if !seenCommits[f[0]] {
					r.Commits++
					seenCommits[f[0]] = true
				}
				authors[f[1]] = true
				lower := strings.ToLower(f[2])
				if strings.Contains(lower, "fix") || strings.Contains(lower, "bug") || strings.Contains(lower, "regress") {
					r.FixLike++
				}
				if strings.HasPrefix(lower, "revert") {
					r.Reverts++
				}
			}
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) >= 3 {
			a, _ := strconv.Atoi(cols[0])
			d, _ := strconv.Atoi(cols[1])
			r.Churn += a + d
		}
	}
	r.Authors = len(authors)
	r.Indicators = signals(r)
	return r, s.Err()
}

func signals(r Report) []Indicator {
	var out []Indicator
	add := func(id string, sev Severity, obs, threshold int, why string) {
		if obs >= threshold {
			out = append(out, Indicator{ID: id, Severity: sev, Observed: obs, Threshold: threshold, Explanation: why})
		}
	}
	add("repeated-fixes", Elevated, r.FixLike, 3, "At least three history entries have fix/bug/regression language; inspect linked incidents before modifying this area.")
	add("reverts", Elevated, r.Reverts, 2, "At least two revert-form commits touched this path; previous implementations may have failed or been rolled back.")
	add("high-churn", Watch, r.Churn, 500, "The path accumulated at least 500 added/deleted lines across history, indicating substantial change pressure.")
	add("ownership-churn", Watch, r.Authors, 10, "At least ten distinct author emails touched this path; ownership context may be distributed.")
	if len(out) == 0 {
		out = append(out, Indicator{ID: "no-threshold-crossed", Severity: Info, Observed: 0, Threshold: 0, Explanation: "No configured historical-risk threshold was crossed. This is not a guarantee of safety."})
	}
	return out
}

func Explain(r Report) string {
	return fmt.Sprintf("%s: %d commits, %d fix-like, %d reverts, %d authors, %d churn lines", r.Path, r.Commits, r.FixLike, r.Reverts, r.Authors, r.Churn)
}
