package risk

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Rayfts/WhyThis/internal/gitx"
	"github.com/Rayfts/WhyThis/internal/ownership"
	"github.com/Rayfts/WhyThis/internal/sensitive"
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
	Path                string              `json:"path"`
	Commits             int                 `json:"commits"`
	FixLike             int                 `json:"fix_like_commits"`
	Reverts             int                 `json:"reverts"`
	Authors             int                 `json:"unique_authors"`
	Churn               int                 `json:"churn_lines"`
	CodeOwners          []string            `json:"code_owners,omitempty"`
	CodeOwnerSource     string              `json:"code_owner_source,omitempty"`
	CodeOwnerPattern    string              `json:"code_owner_pattern,omitempty"`
	CodeOwnerLine       int                 `json:"code_owner_line,omitempty"`
	OwnershipConfigured bool                `json:"ownership_configured"`
	OwnershipMatched    bool                `json:"ownership_matched"`
	OwnershipError      string              `json:"ownership_error,omitempty"`
	Sensitive           bool                `json:"sensitive"`
	Sensitivity         []sensitive.Finding `json:"sensitivity,omitempty"`
	Generated           bool                `json:"generated"`
	Vendor              bool                `json:"vendor"`
	Submodule           bool                `json:"submodule"`
	HistoryComplete     bool                `json:"history_complete"`
	Score               int                 `json:"score"`
	Confidence          string              `json:"confidence"`
	Indicators          []Indicator         `json:"indicators"`
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
	if err := s.Err(); err != nil {
		return Report{}, err
	}
	r.Authors = len(authors)
	classification := sensitive.Analyze(g.Dir, path)
	r.Sensitive = classification.Sensitive
	r.Sensitivity = classification.Findings
	r.Generated = classification.Generated
	r.Vendor = classification.Vendor
	if mode, modeErr := g.FileMode(ctx, path); modeErr == nil {
		r.Submodule = mode == "160000"
	}
	shallow, shallowErr := g.IsShallow(ctx)
	r.HistoryComplete = shallowErr == nil && !shallow
	if owners, ownerErr := ownership.Resolve(g.Dir, path); ownerErr != nil {
		r.OwnershipError = ownerErr.Error()
	} else {
		r.CodeOwners = owners.Owners
		r.CodeOwnerSource = owners.File
		r.CodeOwnerPattern = owners.Pattern
		r.CodeOwnerLine = owners.Line
		r.OwnershipConfigured = owners.Configured
		r.OwnershipMatched = owners.Matched
	}
	r.Score = deterministicScore(r)
	r.Confidence = confidence(r)
	r.Indicators = signals(r)
	return r, nil
}

func deterministicScore(r Report) int {
	// This is an explainable historical-risk heuristic, not a probability.
	// Each component is bounded so no single signal dominates the result.
	score := minInt(r.FixLike*8, 32)
	score += minInt(r.Reverts*15, 30)
	score += minInt(r.Churn/250, 20)
	if r.Authors > 3 {
		score += minInt((r.Authors-3)*2, 18)
	}
	if score > 100 {
		return 100
	}
	return score
}

func confidence(r Report) string {
	if !r.HistoryComplete || r.Commits < 5 {
		return "low"
	}
	if r.Commits < 20 {
		return "medium"
	}
	return "high"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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
	if r.Sensitive {
		out = append(out, Indicator{ID: "sensitive-area", Severity: Elevated, Observed: len(r.Sensitivity), Threshold: 1, Explanation: "Path/content matches deterministic sensitive-code categories; review authorization, secrets, cryptography, financial, data-integrity, or deployment implications as applicable."})
	}
	if r.Generated {
		out = append(out, Indicator{ID: "generated-code", Severity: Info, Observed: 1, Threshold: 1, Explanation: "Path looks generated; prefer changing its source generator/schema rather than editing generated output directly."})
	}
	if r.Vendor {
		out = append(out, Indicator{ID: "vendor-code", Severity: Info, Observed: 1, Threshold: 1, Explanation: "Path is under a vendor/third-party dependency area; repository-local history may not represent upstream history."})
	}
	if r.Submodule {
		out = append(out, Indicator{ID: "submodule", Severity: Info, Observed: 1, Threshold: 1, Explanation: "Path is a Git submodule entry; WhyThis can explain the pointer history here, while nested repository history must be analyzed in the submodule repository itself."})
	}
	if r.OwnershipError != "" {
		out = append(out, Indicator{ID: "codeowners-unavailable", Severity: Watch, Observed: 1, Threshold: 1, Explanation: "CODEOWNERS exists but could not be evaluated: " + r.OwnershipError})
	}
	if len(out) == 0 {
		out = append(out, Indicator{ID: "no-threshold-crossed", Severity: Info, Observed: 0, Threshold: 0, Explanation: "No configured historical-risk threshold was crossed. This is not a guarantee of safety."})
	}
	return out
}

func Explain(r Report) string {
	ownershipSummary := "no CODEOWNERS rule"
	if len(r.CodeOwners) > 0 {
		ownershipSummary = "owners " + strings.Join(r.CodeOwners, ", ")
	} else if r.OwnershipConfigured && r.OwnershipMatched {
		ownershipSummary = "matching CODEOWNERS rule with no assigned owner"
	}
	sensitivitySummary := "not classified sensitive"
	if r.Sensitive {
		categories := make([]string, 0, len(r.Sensitivity))
		seen := map[string]bool{}
		for _, finding := range r.Sensitivity {
			if !seen[finding.Category] {
				seen[finding.Category] = true
				categories = append(categories, finding.Category)
			}
		}
		sensitivitySummary = "sensitive categories " + strings.Join(categories, ", ")
	}
	return fmt.Sprintf("%s: score %d/100 (%s confidence), %d commits, %d fix-like, %d reverts, %d authors, %d churn lines, %s, %s", r.Path, r.Score, r.Confidence, r.Commits, r.FixLike, r.Reverts, r.Authors, r.Churn, ownershipSummary, sensitivitySummary)
}
